/*
Copyright The Helm Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package action

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/Masterminds/sprig/v3"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/resource"
	"sigs.k8s.io/yaml"

	"helm.sh/helm/v3/internal/experimental/registry"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/downloader"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/kube"
	kubefake "helm.sh/helm/v3/pkg/kube/fake"
	"helm.sh/helm/v3/pkg/postrender"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/releaseutil"
	"helm.sh/helm/v3/pkg/repo"
	"helm.sh/helm/v3/pkg/storage"
	"helm.sh/helm/v3/pkg/storage/driver"
)

// releaseNameMaxLen is the maximum length of a release name.
//
// As of Kubernetes 1.4, the max limit on a name is 63 chars. We reserve 10 for
// charts to add data. Effectively, that gives us 53 chars.
// See https://github.com/helm/helm/issues/1528
const releaseNameMaxLen = 53

// NOTESFILE_SUFFIX that we want to treat special. It goes through the templating engine
// but it's not a yaml file (resource) hence can't have hooks, etc. And the user actually
// wants to see this file after rendering in the status command. However, it must be a suffix
// since there can be filepath in front of it.
const notesFileSuffix = "NOTES.txt"

const defaultDirectoryPermission = 0755

// Install performs an installation operation.
type Install struct {
	cfg *Configuration

	ChartPathOptions

	ClientOnly               bool
	CreateNamespace          bool
	DryRun                   bool
	DisableHooks             bool
	Replace                  bool
	Wait                     bool
	WaitForJobs              bool
	Devel                    bool
	DependencyUpdate         bool
	Timeout                  time.Duration
	Namespace                string
	ReleaseName              string
	GenerateName             bool
	NameTemplate             string
	Description              string
	OutputDir                string
	Atomic                   bool
	SkipCRDs                 bool
	SubNotes                 bool
	DisableOpenAPIValidation bool
	IncludeCRDs              bool
	// KubeVersion allows specifying a custom kubernetes version to use and
	// APIVersions allows a manual set of supported API Versions to be passed
	// (for things like templating). These are ignored if ClientOnly is false
	KubeVersion *chartutil.KubeVersion
	APIVersions chartutil.VersionSet
	// Used by helm template to render charts with .Release.IsUpgrade. Ignored if Dry-Run is false
	IsUpgrade bool
	// Used by helm template to add the release as part of OutputDir path
	// OutputDir/<ReleaseName>
	UseReleaseName bool
	PostRenderer   postrender.PostRenderer
	// Lock to control raceconditions when the process receives a SIGTERM
	Lock sync.Mutex
}

// ChartPathOptions captures common options used for controlling chart paths
type ChartPathOptions struct {
	CaFile                string // --ca-file
	CertFile              string // --cert-file
	KeyFile               string // --key-file
	InsecureSkipTLSverify bool   // --insecure-skip-verify
	Keyring               string // --keyring
	Password              string // --password
	PassCredentialsAll    bool   // --pass-credentials
	RepoURL               string // --repo
	Username              string // --username
	Verify                bool   // --verify
	Version               string // --version
}

// NewInstall creates a new Install object with the given configuration.
func NewInstall(cfg *Configuration) *Install {
	return &Install{
		cfg: cfg,
	}
}

func (i *Install) installCRDs(crds []chart.CRD) error {
	// We do these one file at a time in the order they were read.
	totalItems := []*resource.Info{}
	for _, obj := range crds {
		// Read in the resources
		res, err := i.cfg.KubeClient.Build(bytes.NewBuffer(obj.File.Data), false)
		if err != nil {
			return errors.Wrapf(err, "failed to install CRD %s", obj.Name)
		}

		// Send them to Kube
		if _, err := i.cfg.KubeClient.Create(res); err != nil {
			// If the error is CRD already exists, continue.
			if apierrors.IsAlreadyExists(err) {
				crdName := res[0].Name
				i.cfg.Log("CRD %s is already present. Skipping.", crdName)
				continue
			}
			return errors.Wrapf(err, "failed to install CRD %s", obj.Name)
		}
		totalItems = append(totalItems, res...)
	}
	if len(totalItems) > 0 {
		// Invalidate the local cache, since it will not have the new CRDs
		// present.
		discoveryClient, err := i.cfg.RESTClientGetter.ToDiscoveryClient()
		if err != nil {
			return err
		}
		i.cfg.Log("Clearing discovery cache")
		discoveryClient.Invalidate()
		// Give time for the CRD to be recognized.

		if err := i.cfg.KubeClient.Wait(totalItems, 60*time.Second); err != nil {
			return err
		}

		// Make sure to force a rebuild of the cache.
		discoveryClient.ServerGroups()
	}
	return nil
}

// Run executes the installation
//
// If DryRun is set to true, this will prepare the release, but not install it

func (i *Install) Run(chrt *chart.Chart, vals map[string]interface{}) (*release.Release, error) {
	ctx := context.Background()
	return i.RunWithContext(ctx, chrt, vals)
}

// Run executes the installation with Context
func (i *Install) RunWithContext(ctx context.Context, chrt *chart.Chart, vals map[string]interface{}) (*release.Release, error) {
	// Check reachability of cluster unless in client-only mode (e.g. `helm template` without `--validate`)
	if !i.ClientOnly {
		if err := i.cfg.KubeClient.IsReachable(); err != nil {
			return nil, err
		}
	}

	if err := i.availableName(); err != nil {
		return nil, err
	}

	// Pre-install anything in the crd/ directory. We do this before Helm
	// contacts the upstream server and builds the capabilities object.
	if crds := chrt.CRDObjects(); !i.ClientOnly && !i.SkipCRDs && len(crds) > 0 {
		// On dry run, bail here
		if i.DryRun {
			i.cfg.Log("WARNING: This chart or one of its subcharts contains CRDs. Rendering may fail or contain inaccuracies.")
		} else if err := i.installCRDs(crds); err != nil {
			return nil, err
		}
	}

	if i.ClientOnly {
		// Add mock objects in here so it doesn't use Kube API server
		// NOTE(bacongobbler): used for `helm template`
		i.cfg.Capabilities = chartutil.DefaultCapabilities.Copy()
		if i.KubeVersion != nil {
			i.cfg.Capabilities.KubeVersion = *i.KubeVersion
		}
		i.cfg.Capabilities.APIVersions = append(i.cfg.Capabilities.APIVersions, i.APIVersions...)
		i.cfg.KubeClient = &kubefake.PrintingKubeClient{Out: ioutil.Discard}

		mem := driver.NewMemory()
		mem.SetNamespace(i.Namespace)
		i.cfg.Releases = storage.Init(mem)
	} else if !i.ClientOnly && len(i.APIVersions) > 0 {
		i.cfg.Log("API Version list given outside of client only mode, this list will be ignored")
	}

	if err := chartutil.ProcessDependencies(chrt, vals); err != nil {
		return nil, err
	}

	// Make sure if Atomic is set, that wait is set as well. This makes it so
	// the user doesn't have to specify both
	i.Wait = i.Wait || i.Atomic

	caps, err := i.cfg.getCapabilities()
	if err != nil {
		return nil, err
	}

	// special case for helm template --is-upgrade
	isUpgrade := i.IsUpgrade && i.DryRun
	options := chartutil.ReleaseOptions{
		Name:      i.ReleaseName,
		Namespace: i.Namespace,
		Revision:  1,
		IsInstall: !isUpgrade,
		IsUpgrade: isUpgrade,
	}
	valuesToRender, err := chartutil.ToRenderValues(chrt, vals, options, caps)
	if err != nil {
		return nil, err
	}

	rel := i.createRelease(chrt, vals)

	var manifestDoc *bytes.Buffer
	rel.Hooks, manifestDoc, rel.Info.Notes, err = i.cfg.renderResources(chrt, valuesToRender, i.ReleaseName, i.OutputDir, i.SubNotes, i.UseReleaseName, i.IncludeCRDs, i.PostRenderer, i.DryRun)
	// Even for errors, attach this if available
	if manifestDoc != nil {
		rel.Manifest = manifestDoc.String()
	}
	// Check error from render
	if err != nil {
		rel.SetStatus(release.StatusFailed, fmt.Sprintf("failed to render resource: %s", err.Error()))
		// Return a release with partial data so that the client can show debugging information.
		return rel, err
	}

	// Mark this release as in-progress
	rel.SetStatus(release.StatusPendingInstall, "Initial install underway")

	var toBeAdopted kube.ResourceList
	resources, err := i.cfg.KubeClient.Build(bytes.NewBufferString(rel.Manifest), !i.DisableOpenAPIValidation)
	if err != nil {
		return nil, errors.Wrap(err, "unable to build kubernetes objects from release manifest")
	}

	// It is safe to use "force" here because these are resources currently rendered by the chart.
	err = resources.Visit(setMetadataVisitor(rel.Name, rel.Namespace, true))
	if err != nil {
		return nil, err
	}

	// Install requires an extra validation step of checking that resources
	// don't already exist before we actually create resources. If we continue
	// forward and create the release object with resources that already exist,
	// we'll end up in a state where we will delete those resources upon
	// deleting the release because the manifest will be pointing at that
	// resource
	if !i.ClientOnly && !isUpgrade && len(resources) > 0 {
		toBeAdopted, err = existingResourceConflict(resources, rel.Name, rel.Namespace)
		if err != nil {
			return nil, errors.Wrap(err, "rendered manifests contain a resource that already exists. Unable to continue with install")
		}
	}

	// Bail out here if it is a dry run
	if i.DryRun {
		rel.Info.Description = "Dry run complete"
		return rel, nil
	}

	if i.CreateNamespace {
		ns := &v1.Namespace{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Namespace",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: i.Namespace,
				Labels: map[string]string{
					"name": i.Namespace,
				},
			},
		}
		buf, err := yaml.Marshal(ns)
		if err != nil {
			return nil, err
		}
		resourceList, err := i.cfg.KubeClient.Build(bytes.NewBuffer(buf), true)
		if err != nil {
			return nil, err
		}
		if _, err := i.cfg.KubeClient.Create(resourceList); err != nil && !apierrors.IsAlreadyExists(err) {
			return nil, err
		}
	}

	// If Replace is true, we need to supercede the last release.
	if i.Replace {
		if err := i.replaceRelease(rel); err != nil {
			return nil, err
		}
	}

	// Store the release in history before continuing (new in Helm 3). We always know
	// that this is a create operation.
	if err := i.cfg.Releases.Create(rel); err != nil {
		// We could try to recover gracefully here, but since nothing has been installed
		// yet, this is probably safer than trying to continue when we know storage is
		// not working.
		return rel, err
	}
	rChan := make(chan resultMessage)
	go i.performInstall(rChan, rel, toBeAdopted, resources)
	go i.handleContext(ctx, rChan, rel)
	result := <-rChan
	//start preformInstall go routine
	return result.r, result.e
}

func (i *Install) performInstall(c chan<- resultMessage, rel *release.Release, toBeAdopted kube.ResourceList, resources kube.ResourceList) {

	// pre-install hooks
	if !i.DisableHooks {
		if err := i.cfg.execHook(rel, release.HookPreInstall, i.Timeout); err != nil {
			i.reportToRun(c, rel, fmt.Errorf("failed pre-install: %s", err))
			return
		}
	}

	// At this point, we can do the install. Note that before we were detecting whether to
	// do an update, but it's not clear whether we WANT to do an update if the re-use is set
	// to true, since that is basically an upgrade operation.
	if len(toBeAdopted) == 0 && len(resources) > 0 {
		if _, err := i.cfg.KubeClient.Create(resources); err != nil {
			i.reportToRun(c, rel, err)
			return
		}
	} else if len(resources) > 0 {
		if _, err := i.cfg.KubeClient.Update(toBeAdopted, resources, false); err != nil {
			i.reportToRun(c, rel, err)
			return
		}
	}

	if i.Wait {
		if i.WaitForJobs {
			if err := i.cfg.KubeClient.WaitWithJobs(resources, i.Timeout); err != nil {
				i.reportToRun(c, rel, err)
				return
			}
		} else {
			if err := i.cfg.KubeClient.Wait(resources, i.Timeout); err != nil {
				i.reportToRun(c, rel, err)
				return
			}
		}
	}

	if !i.DisableHooks {
		if err := i.cfg.execHook(rel, release.HookPostInstall, i.Timeout); err != nil {
			i.reportToRun(c, rel, fmt.Errorf("failed post-install: %s", err))
			return
		}
	}

	if len(i.Description) > 0 {
		rel.SetStatus(release.StatusDeployed, i.Description)
	} else {
		rel.SetStatus(release.StatusDeployed, "Install complete")
	}

	// This is a tricky case. The release has been created, but the result
	// cannot be recorded. The truest thing to tell the user is that the
	// release was created. However, the user will not be able to do anything
	// further with this release.
	//
	// One possible strategy would be to do a timed retry to see if we can get
	// this stored in the future.
	if err := i.recordRelease(rel); err != nil {
		i.cfg.Log("failed to record the release: %s", err)
	}

	i.reportToRun(c, rel, nil)
}
func (i *Install) handleContext(ctx context.Context, c chan<- resultMessage, rel *release.Release) {
	go func() {
		<-ctx.Done()
		err := ctx.Err()
		i.reportToRun(c, rel, err)
	}()
}
func (i *Install) reportToRun(c chan<- resultMessage, rel *release.Release, err error) {
	i.Lock.Lock()
	if err != nil {
		rel, err = i.failRelease(rel, err)
	}
	c <- resultMessage{r: rel, e: err}
	i.Lock.Unlock()
}
func (i *Install) failRelease(rel *release.Release, err error) (*release.Release, error) {
	rel.SetStatus(release.StatusFailed, fmt.Sprintf("Release %q failed: %s", i.ReleaseName, err.Error()))
	if i.Atomic {
		i.cfg.Log("Install failed and atomic is set, uninstalling release")
		uninstall := NewUninstall(i.cfg)
		uninstall.DisableHooks = i.DisableHooks
		uninstall.KeepHistory = false
		uninstall.Timeout = i.Timeout
		if _, uninstallErr := uninstall.Run(i.ReleaseName); uninstallErr != nil {
			return rel, errors.Wrapf(uninstallErr, "an error occurred while uninstalling the release. original install error: %s", err)
		}
		return rel, errors.Wrapf(err, "release %s failed, and has been uninstalled due to atomic being set", i.ReleaseName)
	}
	i.recordRelease(rel) // Ignore the error, since we have another error to deal with.
	return rel, err
}

// availableName tests whether a name is available
//
// Roughly, this will return an error if name is
//
//	- empty
//	- too long
//	- already in use, and not deleted
//	- used by a deleted release, and i.Replace is false
func (i *Install) availableName() error {
	start := i.ReleaseName
	if start == "" {
		return errors.New("name is required")
	}

	if len(start) > releaseNameMaxLen {
		return errors.Errorf("release name %q exceeds max length of %d", start, releaseNameMaxLen)
	}

	if i.DryRun {
		return nil
	}

	h, err := i.cfg.Releases.History(start)
	if err != nil || len(h) < 1 {
		return nil
	}
	releaseutil.Reverse(h, releaseutil.SortByRevision)
	rel := h[0]

	if st := rel.Info.Status; i.Replace && (st == release.StatusUninstalled || st == release.StatusFailed) {
		return nil
	}
	return errors.New("cannot re-use a name that is still in use")
}

// createRelease creates a new release object
func (i *Install) createRelease(chrt *chart.Chart, rawVals map[string]interface{}) *release.Release {
	ts := i.cfg.Now()
	return &release.Release{
		Name:      i.ReleaseName,
		Namespace: i.Namespace,
		Chart:     chrt,
		Config:    rawVals,
		Info: &release.Info{
			FirstDeployed: ts,
			LastDeployed:  ts,
			Status:        release.StatusUnknown,
		},
		Version: 1,
	}
}

// recordRelease with an update operation in case reuse has been set.
func (i *Install) recordRelease(r *release.Release) error {
	// This is a legacy function which has been reduced to a oneliner. Could probably
	// refactor it out.
	return i.cfg.Releases.Update(r)
}

// replaceRelease replaces an older release with this one
//
// This allows us to re-use names by superseding an existing release with a new one
func (i *Install) replaceRelease(rel *release.Release) error {
	hist, err := i.cfg.Releases.History(rel.Name)
	if err != nil || len(hist) == 0 {
		// No releases exist for this name, so we can return early
		return nil
	}

	releaseutil.Reverse(hist, releaseutil.SortByRevision)
	last := hist[0]

	// Update version to the next available
	rel.Version = last.Version + 1

	// Do not change the status of a failed release.
	if last.Info.Status == release.StatusFailed {
		return nil
	}

	// For any other status, mark it as superseded and store the old record
	last.SetStatus(release.StatusSuperseded, "superseded by new release")
	return i.recordRelease(last)
}

// write the <data> to <output-dir>/<name>. <append> controls if the file is created or content will be appended
func writeToFile(outputDir string, name string, data string, append bool) error {
	outfileName := strings.Join([]string{outputDir, name}, string(filepath.Separator))

	err := ensureDirectoryForFile(outfileName)
	if err != nil {
		return err
	}

	f, err := createOrOpenFile(outfileName, append)
	if err != nil {
		return err
	}

	defer f.Close()

	_, err = f.WriteString(fmt.Sprintf("---\n# Source: %s\n%s\n", name, data))

	if err != nil {
		return err
	}

	fmt.Printf("wrote %s\n", outfileName)
	return nil
}

func createOrOpenFile(filename string, append bool) (*os.File, error) {
	if append {
		return os.OpenFile(filename, os.O_APPEND|os.O_WRONLY, 0600)
	}
	return os.Create(filename)
}

// check if the directory exists to create file. creates if don't exists
func ensureDirectoryForFile(file string) error {
	baseDir := path.Dir(file)
	_, err := os.Stat(baseDir)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	return os.MkdirAll(baseDir, defaultDirectoryPermission)
}

// NameAndChart returns the name and chart that should be used.
//
// This will read the flags and handle name generation if necessary.
func (i *Install) NameAndChart(args []string) (string, string, error) {
	flagsNotSet := func() error {
		if i.GenerateName {
			return errors.New("cannot set --generate-name and also specify a name")
		}
		if i.NameTemplate != "" {
			return errors.New("cannot set --name-template and also specify a name")
		}
		return nil
	}

	if len(args) > 2 {
		return args[0], args[1], errors.Errorf("expected at most two arguments, unexpected arguments: %v", strings.Join(args[2:], ", "))
	}

	if len(args) == 2 {
		return args[0], args[1], flagsNotSet()
	}

	if i.NameTemplate != "" {
		name, err := TemplateName(i.NameTemplate)
		return name, args[0], err
	}

	if i.ReleaseName != "" {
		return i.ReleaseName, args[0], nil
	}

	if !i.GenerateName {
		return "", args[0], errors.New("must either provide a name or specify --generate-name")
	}

	base := filepath.Base(args[0])
	if base == "." || base == "" {
		base = "chart"
	}
	// if present, strip out the file extension from the name
	if idx := strings.Index(base, "."); idx != -1 {
		base = base[0:idx]
	}

	return fmt.Sprintf("%s-%d", base, time.Now().Unix()), args[0], nil
}

// TemplateName renders a name template, returning the name or an error.
func TemplateName(nameTemplate string) (string, error) {
	if nameTemplate == "" {
		return "", nil
	}

	t, err := template.New("name-template").Funcs(sprig.TxtFuncMap()).Parse(nameTemplate)
	if err != nil {
		return "", err
	}
	var b bytes.Buffer
	if err := t.Execute(&b, nil); err != nil {
		return "", err
	}

	return b.String(), nil
}

// CheckDependencies checks the dependencies for a chart.
func CheckDependencies(ch *chart.Chart, reqs []*chart.Dependency) error {
	var missing []string

OUTER:
	for _, r := range reqs {
		for _, d := range ch.Dependencies() {
			if d.Name() == r.Name {
				continue OUTER
			}
		}
		missing = append(missing, r.Name)
	}

	if len(missing) > 0 {
		return errors.Errorf("found in Chart.yaml, but missing in charts/ directory: %s", strings.Join(missing, ", "))
	}
	return nil
}

// LocateChart looks for a chart directory in known places, and returns either the full path or an error.
//
// This does not ensure that the chart is well-formed; only that the requested filename exists.
//
// Order of resolution:
// - relative to current working directory
// - if path is absolute or begins with '.', error out here
// - URL
//
// If 'verify' was set on ChartPathOptions, this will attempt to also verify the chart.
func (c *ChartPathOptions) LocateChart(name string, settings *cli.EnvSettings) (string, error) {
	name = strings.TrimSpace(name)
	version := strings.TrimSpace(c.Version)

	if _, err := os.Stat(name); err == nil {
		abs, err := filepath.Abs(name)
		if err != nil {
			return abs, err
		}
		if c.Verify {
			if _, err := downloader.VerifyChart(abs, c.Keyring); err != nil {
				return "", err
			}
		}
		return abs, nil
	}
	if filepath.IsAbs(name) || strings.HasPrefix(name, ".") {
		return name, errors.Errorf("path %q not found", name)
	}

	dl := downloader.ChartDownloader{
		Out:     os.Stdout,
		Keyring: c.Keyring,
		Getters: getter.All(settings),
		Options: []getter.Option{
			getter.WithPassCredentialsAll(c.PassCredentialsAll),
			getter.WithTLSClientConfig(c.CertFile, c.KeyFile, c.CaFile),
			getter.WithInsecureSkipVerifyTLS(c.InsecureSkipTLSverify),
		},
		RepositoryConfig: settings.RepositoryConfig,
		RepositoryCache:  settings.RepositoryCache,
	}

	if registry.IsOCI(name) {
		if version == "" {
			return "", errors.New("version is explicitly required for OCI registries")
		}
		dl.Options = append(dl.Options, getter.WithTagName(version))
	}

	if c.Verify {
		dl.Verify = downloader.VerifyAlways
	}
	if c.RepoURL != "" {
		chartURL, err := repo.FindChartInAuthAndTLSAndPassRepoURL(c.RepoURL, c.Username, c.Password, name, version,
			c.CertFile, c.KeyFile, c.CaFile, c.InsecureSkipTLSverify, c.PassCredentialsAll, getter.All(settings))
		if err != nil {
			return "", err
		}
		name = chartURL

		// Only pass the user/pass on when the user has said to or when the
		// location of the chart repo and the chart are the same domain.
		u1, err := url.Parse(c.RepoURL)
		if err != nil {
			return "", err
		}
		u2, err := url.Parse(chartURL)
		if err != nil {
			return "", err
		}

		// Host on URL (returned from url.Parse) contains the port if present.
		// This check ensures credentials are not passed between different
		// services on different ports.
		if c.PassCredentialsAll || (u1.Scheme == u2.Scheme && u1.Host == u2.Host) {
			dl.Options = append(dl.Options, getter.WithBasicAuth(c.Username, c.Password))
		} else {
			dl.Options = append(dl.Options, getter.WithBasicAuth("", ""))
		}
	} else {
		dl.Options = append(dl.Options, getter.WithBasicAuth(c.Username, c.Password))
	}

	if err := os.MkdirAll(settings.RepositoryCache, 0755); err != nil {
		return "", err
	}

	filename, _, err := dl.DownloadTo(name, version, settings.RepositoryCache)
	if err == nil {
		lname, err := filepath.Abs(filename)
		if err != nil {
			return filename, err
		}
		return lname, nil
	} else if settings.Debug {
		return filename, err
	}

	atVersion := ""
	if version != "" {
		atVersion = fmt.Sprintf(" at version %q", version)
	}

	return filename, errors.Errorf("failed to download %q%s", name, atVersion)
}
