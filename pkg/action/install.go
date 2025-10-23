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
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"text/template"
	"time"

	"github.com/Masterminds/sprig/v3"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/resource"
	"sigs.k8s.io/yaml"

	ci "helm.sh/helm/v4/pkg/chart"
	"helm.sh/helm/v4/pkg/chart/common"
	"helm.sh/helm/v4/pkg/chart/common/util"
	chart "helm.sh/helm/v4/pkg/chart/v2"
	chartutil "helm.sh/helm/v4/pkg/chart/v2/util"
	"helm.sh/helm/v4/pkg/cli"
	"helm.sh/helm/v4/pkg/downloader"
	"helm.sh/helm/v4/pkg/getter"
	"helm.sh/helm/v4/pkg/kube"
	kubefake "helm.sh/helm/v4/pkg/kube/fake"
	"helm.sh/helm/v4/pkg/postrenderer"
	"helm.sh/helm/v4/pkg/registry"
	ri "helm.sh/helm/v4/pkg/release"
	rcommon "helm.sh/helm/v4/pkg/release/common"
	release "helm.sh/helm/v4/pkg/release/v1"
	releaseutil "helm.sh/helm/v4/pkg/release/v1/util"
	"helm.sh/helm/v4/pkg/repo/v1"
	"helm.sh/helm/v4/pkg/storage"
	"helm.sh/helm/v4/pkg/storage/driver"
)

// notesFileSuffix that we want to treat special. It goes through the templating engine
// but it's not a yaml file (resource) hence can't have hooks, etc. And the user actually
// wants to see this file after rendering in the status command. However, it must be a suffix
// since there can be filepath in front of it.
const notesFileSuffix = "NOTES.txt"

const defaultDirectoryPermission = 0755

// Install performs an installation operation.
type Install struct {
	cfg *Configuration

	ChartPathOptions

	// ForceReplace will, if set to `true`, ignore certain warnings and perform the install anyway.
	//
	// This should be used with caution.
	ForceReplace bool
	// ForceConflicts causes server-side apply to force conflicts ("Overwrite value, become sole manager")
	// see: https://kubernetes.io/docs/reference/using-api/server-side-apply/#conflicts
	ForceConflicts bool
	// ServerSideApply when true (default) will enable changes to be applied via Kubernetes server-side apply
	// see: https://kubernetes.io/docs/reference/using-api/server-side-apply/
	ServerSideApply bool
	CreateNamespace bool
	// DryRunStrategy can be set to prepare, but not execute the operation and whether or not to interact with the remote cluster
	DryRunStrategy DryRunStrategy
	// HideSecret can be set to true when DryRun is enabled in order to hide
	// Kubernetes Secrets in the output. It cannot be used outside of DryRun.
	HideSecret       bool
	DisableHooks     bool
	Replace          bool
	WaitStrategy     kube.WaitStrategy
	WaitForJobs      bool
	Devel            bool
	DependencyUpdate bool
	Timeout          time.Duration
	Namespace        string
	ReleaseName      string
	GenerateName     bool
	NameTemplate     string
	Description      string
	OutputDir        string
	// RollbackOnFailure enables rolling back (uninstalling) the release on failure if set
	RollbackOnFailure        bool
	SkipCRDs                 bool
	SubNotes                 bool
	HideNotes                bool
	SkipSchemaValidation     bool
	DisableOpenAPIValidation bool
	IncludeCRDs              bool
	Labels                   map[string]string
	// KubeVersion allows specifying a custom kubernetes version to use and
	// APIVersions allows a manual set of supported API Versions to be passed
	// (for things like templating). These are ignored if ClientOnly is false
	KubeVersion *common.KubeVersion
	APIVersions common.VersionSet
	// Used by helm template to render charts with .Release.IsUpgrade. Ignored if Dry-Run is false
	IsUpgrade bool
	// Enable DNS lookups when rendering templates
	EnableDNS bool
	// Used by helm template to add the release as part of OutputDir path
	// OutputDir/<ReleaseName>
	UseReleaseName bool
	// TakeOwnership will ignore the check for helm annotations and take ownership of the resources.
	TakeOwnership bool
	PostRenderer  postrenderer.PostRenderer
	// Lock to control raceconditions when the process receives a SIGTERM
	Lock           sync.Mutex
	goroutineCount atomic.Int32
}

// ChartPathOptions captures common options used for controlling chart paths
type ChartPathOptions struct {
	CaFile                string // --ca-file
	CertFile              string // --cert-file
	KeyFile               string // --key-file
	InsecureSkipTLSverify bool   // --insecure-skip-verify
	PlainHTTP             bool   // --plain-http
	Keyring               string // --keyring
	Password              string // --password
	PassCredentialsAll    bool   // --pass-credentials
	RepoURL               string // --repo
	Username              string // --username
	Verify                bool   // --verify
	Version               string // --version

	// registryClient provides a registry client but is not added with
	// options from a flag
	registryClient *registry.Client
}

// NewInstall creates a new Install object with the given configuration.
func NewInstall(cfg *Configuration) *Install {
	in := &Install{
		cfg:             cfg,
		ServerSideApply: true,
		DryRunStrategy:  DryRunNone,
	}
	in.registryClient = cfg.RegistryClient

	return in
}

// SetRegistryClient sets the registry client for the install action
func (i *Install) SetRegistryClient(registryClient *registry.Client) {
	i.registryClient = registryClient
}

// GetRegistryClient get the registry client.
func (i *Install) GetRegistryClient() *registry.Client {
	return i.registryClient
}

func (i *Install) installCRDs(crds []chart.CRD) error {
	// We do these one file at a time in the order they were read.
	totalItems := []*resource.Info{}
	for _, obj := range crds {
		// Read in the resources
		res, err := i.cfg.KubeClient.Build(bytes.NewBuffer(obj.File.Data), false)
		if err != nil {
			return fmt.Errorf("failed to install CRD %s: %w", obj.Name, err)
		}

		// Send them to Kube
		if _, err := i.cfg.KubeClient.Create(
			res,
			kube.ClientCreateOptionServerSideApply(i.ServerSideApply, i.ForceConflicts)); err != nil {
			// If the error is CRD already exists, continue.
			if apierrors.IsAlreadyExists(err) {
				crdName := res[0].Name
				slog.Debug("CRD is already present. Skipping", "crd", crdName)
				continue
			}
			return fmt.Errorf("failed to install CRD %s: %w", obj.Name, err)
		}
		totalItems = append(totalItems, res...)
	}
	if len(totalItems) > 0 {
		waiter, err := i.cfg.KubeClient.GetWaiter(i.WaitStrategy)
		if err != nil {
			return fmt.Errorf("unable to get waiter: %w", err)
		}
		// Give time for the CRD to be recognized.
		if err := waiter.Wait(totalItems, 60*time.Second); err != nil {
			return err
		}

		// If we have already gathered the capabilities, we need to invalidate
		// the cache so that the new CRDs are recognized. This should only be
		// the case when an action configuration is reused for multiple actions,
		// as otherwise it is later loaded by ourselves when getCapabilities
		// is called later on in the installation process.
		if i.cfg.Capabilities != nil {
			discoveryClient, err := i.cfg.RESTClientGetter.ToDiscoveryClient()
			if err != nil {
				return err
			}

			slog.Debug("clearing discovery cache")
			discoveryClient.Invalidate()

			_, _ = discoveryClient.ServerGroups()
		}

		// Invalidate the REST mapper, since it will not have the new CRDs
		// present.
		restMapper, err := i.cfg.RESTClientGetter.ToRESTMapper()
		if err != nil {
			return err
		}
		if resettable, ok := restMapper.(meta.ResettableRESTMapper); ok {
			slog.Debug("clearing REST mapper cache")
			resettable.Reset()
		}
	}
	return nil
}

// Run executes the installation
//
// If DryRun is set to true, this will prepare the release, but not install it

func (i *Install) Run(chrt ci.Charter, vals map[string]interface{}) (ri.Releaser, error) {
	ctx := context.Background()
	return i.RunWithContext(ctx, chrt, vals)
}

// RunWithContext executes the installation with Context
//
// When the task is cancelled through ctx, the function returns and the install
// proceeds in the background.
func (i *Install) RunWithContext(ctx context.Context, ch ci.Charter, vals map[string]interface{}) (ri.Releaser, error) {
	var chrt *chart.Chart
	switch c := ch.(type) {
	case *chart.Chart:
		chrt = c
	case chart.Chart:
		chrt = &c
	default:
		return nil, errors.New("invalid chart apiVersion")
	}

	if interactWithServer(i.DryRunStrategy) {
		if err := i.cfg.KubeClient.IsReachable(); err != nil {
			slog.Error(fmt.Sprintf("cluster reachability check failed: %v", err))
			return nil, fmt.Errorf("cluster reachability check failed: %w", err)
		}
	}

	// HideSecret must be used with dry run. Otherwise, return an error.
	if !isDryRun(i.DryRunStrategy) && i.HideSecret {
		slog.Error("hiding Kubernetes secrets requires a dry-run mode")
		return nil, errors.New("hiding Kubernetes secrets requires a dry-run mode")
	}

	if err := i.availableName(); err != nil {
		slog.Error("release name check failed", slog.Any("error", err))
		return nil, fmt.Errorf("release name check failed: %w", err)
	}

	if err := chartutil.ProcessDependencies(chrt, vals); err != nil {
		slog.Error("chart dependencies processing failed", slog.Any("error", err))
		return nil, fmt.Errorf("chart dependencies processing failed: %w", err)
	}

	// Pre-install anything in the crd/ directory. We do this before Helm
	// contacts the upstream server and builds the capabilities object.
	if crds := chrt.CRDObjects(); interactWithServer(i.DryRunStrategy) && !i.SkipCRDs && len(crds) > 0 {
		// On dry run, bail here
		if isDryRun(i.DryRunStrategy) {
			slog.Warn("This chart or one of its subcharts contains CRDs. Rendering may fail or contain inaccuracies.")
		} else if err := i.installCRDs(crds); err != nil {
			return nil, err
		}
	}

	if !interactWithServer(i.DryRunStrategy) {
		// Add mock objects in here so it doesn't use Kube API server
		// NOTE(bacongobbler): used for `helm template`
		i.cfg.Capabilities = common.DefaultCapabilities.Copy()
		if i.KubeVersion != nil {
			i.cfg.Capabilities.KubeVersion = *i.KubeVersion
		}
		i.cfg.Capabilities.APIVersions = append(i.cfg.Capabilities.APIVersions, i.APIVersions...)
		i.cfg.KubeClient = &kubefake.PrintingKubeClient{Out: io.Discard}

		mem := driver.NewMemory()
		mem.SetNamespace(i.Namespace)
		i.cfg.Releases = storage.Init(mem)
	} else if interactWithServer(i.DryRunStrategy) && len(i.APIVersions) > 0 {
		slog.Debug("API Version list given outside of client only mode, this list will be ignored")
	}

	// Make sure if RollbackOnFailure is set, that wait is set as well. This makes it so
	// the user doesn't have to specify both
	if i.WaitStrategy == kube.HookOnlyStrategy && i.RollbackOnFailure {
		i.WaitStrategy = kube.StatusWatcherStrategy
	}

	caps, err := i.cfg.getCapabilities()
	if err != nil {
		return nil, err
	}

	// special case for helm template --is-upgrade
	isUpgrade := i.IsUpgrade && isDryRun(i.DryRunStrategy)
	options := common.ReleaseOptions{
		Name:      i.ReleaseName,
		Namespace: i.Namespace,
		Revision:  1,
		IsInstall: !isUpgrade,
		IsUpgrade: isUpgrade,
	}
	valuesToRender, err := util.ToRenderValuesWithSchemaValidation(chrt, vals, options, caps, i.SkipSchemaValidation)
	if err != nil {
		return nil, err
	}

	if driver.ContainsSystemLabels(i.Labels) {
		return nil, fmt.Errorf("user supplied labels contains system reserved label name. System labels: %+v", driver.GetSystemLabels())
	}

	rel := i.createRelease(chrt, vals, i.Labels)

	var manifestDoc *bytes.Buffer
	rel.Hooks, manifestDoc, rel.Info.Notes, err = i.cfg.renderResources(chrt, valuesToRender, i.ReleaseName,
		i.OutputDir, i.SubNotes, i.UseReleaseName, i.IncludeCRDs, i.PostRenderer, interactWithServer(i.DryRunStrategy),
		i.EnableDNS, i.HideSecret, isDryRun(i.DryRunStrategy))
	// Even for errors, attach this if available
	if manifestDoc != nil {
		rel.Manifest = manifestDoc.String()
	}
	// Check error from render
	if err != nil {
		rel.SetStatus(rcommon.StatusFailed, fmt.Sprintf("failed to render resource: %s", err.Error()))
		// Return a release with partial data so that the client can show debugging information.
		return rel, err
	}

	// Mark this release as in-progress
	rel.SetStatus(rcommon.StatusPendingInstall, "Initial install underway")

	var toBeAdopted kube.ResourceList
	resources, err := i.cfg.KubeClient.Build(bytes.NewBufferString(rel.Manifest), !i.DisableOpenAPIValidation)
	if err != nil {
		return nil, fmt.Errorf("unable to build kubernetes objects from release manifest: %w", err)
	}

	// It is safe to use "forceOwnership" here because these are resources currently rendered by the chart.
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
	if interactWithServer(i.DryRunStrategy) && !isUpgrade && len(resources) > 0 {
		if i.TakeOwnership {
			toBeAdopted, err = requireAdoption(resources)
		} else {
			toBeAdopted, err = existingResourceConflict(resources, rel.Name, rel.Namespace)
		}
		if err != nil {
			return nil, fmt.Errorf("unable to continue with install: %w", err)
		}
	}

	// Bail out here if it is a dry run
	if isDryRun(i.DryRunStrategy) {
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
		if _, err := i.cfg.KubeClient.Create(
			resourceList,
			kube.ClientCreateOptionServerSideApply(i.ServerSideApply, false)); err != nil && !apierrors.IsAlreadyExists(err) {
			return nil, err
		}
	}

	// If Replace is true, we need to supersede the last release.
	if i.Replace {
		if err := i.replaceRelease(rel); err != nil {
			return nil, err
		}
	}

	// Store the release in history before continuing. We always know that this is a create operation
	if err := i.cfg.Releases.Create(rel); err != nil {
		// We could try to recover gracefully here, but since nothing has been installed
		// yet, this is probably safer than trying to continue when we know storage is
		// not working.
		return rel, err
	}

	rel, err = i.performInstallCtx(ctx, rel, toBeAdopted, resources)
	if err != nil {
		rel, err = i.failRelease(rel, err)
	}
	return rel, err
}

func (i *Install) performInstallCtx(ctx context.Context, rel *release.Release, toBeAdopted kube.ResourceList, resources kube.ResourceList) (*release.Release, error) {
	type Msg struct {
		r *release.Release
		e error
	}
	resultChan := make(chan Msg, 1)

	go func() {
		i.goroutineCount.Add(1)
		rel, err := i.performInstall(rel, toBeAdopted, resources)
		resultChan <- Msg{rel, err}
		i.goroutineCount.Add(-1)
	}()
	select {
	case <-ctx.Done():
		err := ctx.Err()
		return rel, err
	case msg := <-resultChan:
		return msg.r, msg.e
	}
}

// getGoroutineCount return the number of running routines
func (i *Install) getGoroutineCount() int32 {
	return i.goroutineCount.Load()
}

func (i *Install) performInstall(rel *release.Release, toBeAdopted kube.ResourceList, resources kube.ResourceList) (*release.Release, error) {
	var err error
	// pre-install hooks
	if !i.DisableHooks {
		if err := i.cfg.execHook(rel, release.HookPreInstall, i.WaitStrategy, i.Timeout, i.ServerSideApply); err != nil {
			return rel, fmt.Errorf("failed pre-install: %s", err)
		}
	}

	// At this point, we can do the install. Note that before we were detecting whether to
	// do an update, but it's not clear whether we WANT to do an update if the reuse is set
	// to true, since that is basically an upgrade operation.
	if len(toBeAdopted) == 0 && len(resources) > 0 {
		_, err = i.cfg.KubeClient.Create(
			resources,
			kube.ClientCreateOptionServerSideApply(i.ServerSideApply, false))
	} else if len(resources) > 0 {
		updateThreeWayMergeForUnstructured := i.TakeOwnership && !i.ServerSideApply // Use three-way merge when taking ownership (and not using server-side apply)
		_, err = i.cfg.KubeClient.Update(
			toBeAdopted,
			resources,
			kube.ClientUpdateOptionForceReplace(i.ForceReplace),
			kube.ClientUpdateOptionServerSideApply(i.ServerSideApply, i.ForceConflicts),
			kube.ClientUpdateOptionThreeWayMergeForUnstructured(updateThreeWayMergeForUnstructured),
			kube.ClientUpdateOptionUpgradeClientSideFieldManager(true))
	}
	if err != nil {
		return rel, err
	}

	waiter, err := i.cfg.KubeClient.GetWaiter(i.WaitStrategy)
	if err != nil {
		return rel, fmt.Errorf("failed to get waiter: %w", err)
	}

	if i.WaitForJobs {
		err = waiter.WaitWithJobs(resources, i.Timeout)
	} else {
		err = waiter.Wait(resources, i.Timeout)
	}
	if err != nil {
		return rel, err
	}

	if !i.DisableHooks {
		if err := i.cfg.execHook(rel, release.HookPostInstall, i.WaitStrategy, i.Timeout, i.ServerSideApply); err != nil {
			return rel, fmt.Errorf("failed post-install: %s", err)
		}
	}

	if len(i.Description) > 0 {
		rel.SetStatus(rcommon.StatusDeployed, i.Description)
	} else {
		rel.SetStatus(rcommon.StatusDeployed, "Install complete")
	}

	// This is a tricky case. The release has been created, but the result
	// cannot be recorded. The truest thing to tell the user is that the
	// release was created. However, the user will not be able to do anything
	// further with this release.
	//
	// One possible strategy would be to do a timed retry to see if we can get
	// this stored in the future.
	if err := i.recordRelease(rel); err != nil {
		slog.Error("failed to record the release", slog.Any("error", err))
	}

	return rel, nil
}

func (i *Install) failRelease(rel *release.Release, err error) (*release.Release, error) {
	rel.SetStatus(rcommon.StatusFailed, fmt.Sprintf("Release %q failed: %s", i.ReleaseName, err.Error()))
	if i.RollbackOnFailure {
		slog.Debug("install failed and rollback-on-failure is set, uninstalling release", "release", i.ReleaseName)
		uninstall := NewUninstall(i.cfg)
		uninstall.DisableHooks = i.DisableHooks
		uninstall.KeepHistory = false
		uninstall.Timeout = i.Timeout
		uninstall.WaitStrategy = i.WaitStrategy
		if _, uninstallErr := uninstall.Run(i.ReleaseName); uninstallErr != nil {
			return rel, fmt.Errorf("an error occurred while uninstalling the release. original install error: %w: %w", err, uninstallErr)
		}
		return rel, fmt.Errorf("release %s failed, and has been uninstalled due to rollback-on-failure being set: %w", i.ReleaseName, err)
	}
	i.recordRelease(rel) // Ignore the error, since we have another error to deal with.
	return rel, err
}

// availableName tests whether a name is available
//
// Roughly, this will return an error if name is
//
//   - empty
//   - too long
//   - already in use, and not deleted
//   - used by a deleted release, and i.Replace is false
func (i *Install) availableName() error {
	start := i.ReleaseName

	if err := chartutil.ValidateReleaseName(start); err != nil {
		return fmt.Errorf("release name %q: %w", start, err)
	}
	// On dry run, bail here
	if isDryRun(i.DryRunStrategy) {
		return nil
	}

	h, err := i.cfg.Releases.History(start)
	if err != nil || len(h) < 1 {
		return nil
	}

	hl, err := releaseListToV1List(h)
	if err != nil {
		return err
	}

	releaseutil.Reverse(hl, releaseutil.SortByRevision)
	rel := hl[0]

	if st := rel.Info.Status; i.Replace && (st == rcommon.StatusUninstalled || st == rcommon.StatusFailed) {
		return nil
	}
	return errors.New("cannot reuse a name that is still in use")
}

func releaseListToV1List(ls []ri.Releaser) ([]*release.Release, error) {
	rls := make([]*release.Release, 0, len(ls))
	for _, val := range ls {
		rel, err := releaserToV1Release(val)
		if err != nil {
			return nil, err
		}
		rls = append(rls, rel)
	}

	return rls, nil
}

func releaseV1ListToReleaserList(ls []*release.Release) ([]ri.Releaser, error) {
	rls := make([]ri.Releaser, 0, len(ls))
	for _, val := range ls {
		rls = append(rls, val)
	}

	return rls, nil
}

// createRelease creates a new release object
func (i *Install) createRelease(chrt *chart.Chart, rawVals map[string]interface{}, labels map[string]string) *release.Release {
	ts := i.cfg.Now()

	r := &release.Release{
		Name:      i.ReleaseName,
		Namespace: i.Namespace,
		Chart:     chrt,
		Config:    rawVals,
		Info: &release.Info{
			FirstDeployed: ts,
			LastDeployed:  ts,
			Status:        rcommon.StatusUnknown,
		},
		Version:     1,
		Labels:      labels,
		ApplyMethod: string(determineReleaseSSApplyMethod(i.ServerSideApply)),
	}

	return r
}

// recordRelease with an update operation in case reuse has been set.
func (i *Install) recordRelease(r *release.Release) error {
	// This is a legacy function which has been reduced to a oneliner. Could probably
	// refactor it out.
	return i.cfg.Releases.Update(r)
}

// replaceRelease replaces an older release with this one
//
// This allows us to reuse names by superseding an existing release with a new one
func (i *Install) replaceRelease(rel *release.Release) error {
	hist, err := i.cfg.Releases.History(rel.Name)
	if err != nil || len(hist) == 0 {
		// No releases exist for this name, so we can return early
		return nil
	}
	hl, err := releaseListToV1List(hist)
	if err != nil {
		return err
	}

	releaseutil.Reverse(hl, releaseutil.SortByRevision)
	last := hl[0]

	// Update version to the next available
	rel.Version = last.Version + 1

	// Do not change the status of a failed release.
	if last.Info.Status == rcommon.StatusFailed {
		return nil
	}

	// For any other status, mark it as superseded and store the old record
	last.SetStatus(rcommon.StatusSuperseded, "superseded by new release")
	return i.recordRelease(last)
}

// write the <data> to <output-dir>/<name>. <appendData> controls if the file is created or content will be appended
func writeToFile(outputDir string, name string, data string, appendData bool) error {
	outfileName := strings.Join([]string{outputDir, name}, string(filepath.Separator))

	err := ensureDirectoryForFile(outfileName)
	if err != nil {
		return err
	}

	f, err := createOrOpenFile(outfileName, appendData)
	if err != nil {
		return err
	}

	defer f.Close()

	_, err = fmt.Fprintf(f, "---\n# Source: %s\n%s\n", name, data)

	if err != nil {
		return err
	}

	fmt.Printf("wrote %s\n", outfileName)
	return nil
}

func createOrOpenFile(filename string, appendData bool) (*os.File, error) {
	if appendData {
		return os.OpenFile(filename, os.O_APPEND|os.O_WRONLY, 0600)
	}
	return os.Create(filename)
}

// check if the directory exists to create file. creates if doesn't exist
func ensureDirectoryForFile(file string) error {
	baseDir := filepath.Dir(file)
	_, err := os.Stat(baseDir)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
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
		return args[0], args[1], fmt.Errorf("expected at most two arguments, unexpected arguments: %v", strings.Join(args[2:], ", "))
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
func CheckDependencies(ch ci.Charter, reqs []ci.Dependency) error {
	ac, err := ci.NewAccessor(ch)
	if err != nil {
		return err
	}

	var missing []string

OUTER:
	for _, r := range reqs {
		rac, err := ci.NewDependencyAccessor(r)
		if err != nil {
			return err
		}
		for _, d := range ac.Dependencies() {
			dac, err := ci.NewAccessor(d)
			if err != nil {
				return err
			}
			if dac.Name() == rac.Name() {
				continue OUTER
			}
		}
		missing = append(missing, rac.Name())
	}

	if len(missing) > 0 {
		return fmt.Errorf("found in Chart.yaml, but missing in charts/ directory: %s", strings.Join(missing, ", "))
	}
	return nil
}

func portOrDefault(u *url.URL) string {
	if p := u.Port(); p != "" {
		return p
	}

	switch u.Scheme {
	case "http":
		return "80"
	case "https":
		return "443"
	default:
		return ""
	}
}

func urlEqual(u1, u2 *url.URL) bool {
	return u1.Scheme == u2.Scheme && u1.Hostname() == u2.Hostname() && portOrDefault(u1) == portOrDefault(u2)
}

// LocateChart looks for a chart directory in known places, and returns either the full path or an error.
//
// This does not ensure that the chart is well-formed; only that the requested filename exists.
//
// Order of resolution:
// - relative to current working directory when --repo flag is not presented
// - if path is absolute or begins with '.', error out here
// - URL
//
// If 'verify' was set on ChartPathOptions, this will attempt to also verify the chart.
func (c *ChartPathOptions) LocateChart(name string, settings *cli.EnvSettings) (string, error) {
	if registry.IsOCI(name) && c.registryClient == nil {
		return "", fmt.Errorf("unable to lookup chart %q, missing registry client", name)
	}

	name = strings.TrimSpace(name)
	version := strings.TrimSpace(c.Version)

	if c.RepoURL == "" {
		if _, err := os.Stat(name); err == nil {
			abs, err := filepath.Abs(name)
			if err != nil {
				return abs, err
			}
			if c.Verify {
				if _, err := downloader.VerifyChart(abs, abs+".prov", c.Keyring); err != nil {
					return "", err
				}
			}
			return abs, nil
		}
		if filepath.IsAbs(name) || strings.HasPrefix(name, ".") {
			return name, fmt.Errorf("path %q not found", name)
		}
	}

	dl := downloader.ChartDownloader{
		Out:     os.Stdout,
		Keyring: c.Keyring,
		Getters: getter.All(settings),
		Options: []getter.Option{
			getter.WithPassCredentialsAll(c.PassCredentialsAll),
			getter.WithTLSClientConfig(c.CertFile, c.KeyFile, c.CaFile),
			getter.WithInsecureSkipVerifyTLS(c.InsecureSkipTLSverify),
			getter.WithPlainHTTP(c.PlainHTTP),
			getter.WithBasicAuth(c.Username, c.Password),
		},
		RepositoryConfig: settings.RepositoryConfig,
		RepositoryCache:  settings.RepositoryCache,
		ContentCache:     settings.ContentCache,
		RegistryClient:   c.registryClient,
	}

	if registry.IsOCI(name) {
		dl.Options = append(dl.Options, getter.WithRegistryClient(c.registryClient))
	}

	if c.Verify {
		dl.Verify = downloader.VerifyAlways
	}
	if c.RepoURL != "" {
		chartURL, err := repo.FindChartInRepoURL(
			c.RepoURL,
			name,
			getter.All(settings),
			repo.WithChartVersion(version),
			repo.WithClientTLS(c.CertFile, c.KeyFile, c.CaFile),
			repo.WithUsernamePassword(c.Username, c.Password),
			repo.WithInsecureSkipTLSverify(c.InsecureSkipTLSverify),
			repo.WithPassCredentialsAll(c.PassCredentialsAll),
		)
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
		if c.PassCredentialsAll || urlEqual(u1, u2) {
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

	filename, _, err := dl.DownloadToCache(name, version)
	if err != nil {
		return "", err
	}

	lname, err := filepath.Abs(filename)
	if err != nil {
		return filename, err
	}
	return lname, nil
}
