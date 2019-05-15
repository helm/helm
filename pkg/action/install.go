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
	"fmt"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"text/template"
	"time"

	"github.com/Masterminds/sprig"
	"github.com/ghodss/yaml"
	"github.com/pkg/errors"

	"helm.sh/helm/pkg/chart"
	"helm.sh/helm/pkg/chartutil"
	"helm.sh/helm/pkg/cli"
	"helm.sh/helm/pkg/downloader"
	"helm.sh/helm/pkg/engine"
	"helm.sh/helm/pkg/getter"
	"helm.sh/helm/pkg/hooks"
	"helm.sh/helm/pkg/release"
	"helm.sh/helm/pkg/releaseutil"
	"helm.sh/helm/pkg/repo"
	"helm.sh/helm/pkg/strvals"
	"helm.sh/helm/pkg/version"
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

// Install performs an installation operation.
type Install struct {
	cfg *Configuration

	ChartPathOptions
	ValueOptions

	DryRun           bool
	DisableHooks     bool
	Replace          bool
	Wait             bool
	Devel            bool
	DependencyUpdate bool
	Timeout          time.Duration
	Namespace        string
	ReleaseName      string
	GenerateName     bool
	NameTemplate     string
}

type ValueOptions struct {
	ValueFiles   []string
	StringValues []string
	Values       []string
	rawValues    map[string]interface{}
}

type ChartPathOptions struct {
	CaFile   string // --ca-file
	CertFile string // --cert-file
	KeyFile  string // --key-file
	Keyring  string // --keyring
	Password string // --password
	RepoURL  string // --repo
	Username string // --username
	Verify   bool   // --verify
	Version  string // --version
}

// NewInstall creates a new Install object with the given configuration.
func NewInstall(cfg *Configuration) *Install {
	return &Install{
		cfg: cfg,
	}
}

// Run executes the installation
//
// If DryRun is set to true, this will prepare the release, but not install it
func (i *Install) Run(chrt *chart.Chart) (*release.Release, error) {
	if err := i.availableName(); err != nil {
		return nil, err
	}

	caps, err := i.cfg.getCapabilities()
	if err != nil {
		return nil, err
	}

	options := chartutil.ReleaseOptions{
		Name:      i.ReleaseName,
		Namespace: i.Namespace,
		IsInstall: true,
	}
	valuesToRender, err := chartutil.ToRenderValues(chrt, i.rawValues, options, caps)
	if err != nil {
		return nil, err
	}

	rel := i.createRelease(chrt, i.rawValues)
	var manifestDoc *bytes.Buffer
	rel.Hooks, manifestDoc, rel.Info.Notes, err = i.cfg.renderResources(chrt, valuesToRender)
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
	if err := i.validateManifest(manifestDoc); err != nil {
		return rel, err
	}

	// Bail out here if it is a dry run
	if i.DryRun {
		rel.Info.Description = "Dry run complete"
		return rel, nil
	}

	// If Replace is true, we need to supersede the last release.
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

	// pre-install hooks
	if !i.DisableHooks {
		if err := i.execHook(rel.Hooks, hooks.PreInstall); err != nil {
			rel.SetStatus(release.StatusFailed, "failed pre-install: "+err.Error())
			_ = i.replaceRelease(rel)
			return rel, err
		}
	}

	// At this point, we can do the install. Note that before we were detecting whether to
	// do an update, but it's not clear whether we WANT to do an update if the re-use is set
	// to true, since that is basically an upgrade operation.
	buf := bytes.NewBufferString(rel.Manifest)
	if err := i.cfg.KubeClient.Create(buf); err != nil {
		rel.SetStatus(release.StatusFailed, fmt.Sprintf("Release %q failed: %s", i.ReleaseName, err.Error()))
		i.recordRelease(rel) // Ignore the error, since we have another error to deal with.
		return rel, errors.Wrapf(err, "release %s failed", i.ReleaseName)
	}

	if i.Wait {
		buf := bytes.NewBufferString(rel.Manifest)
		if err := i.cfg.KubeClient.Wait(buf, i.Timeout); err != nil {
			rel.SetStatus(release.StatusFailed, fmt.Sprintf("Release %q failed: %s", i.ReleaseName, err.Error()))
			i.recordRelease(rel) // Ignore the error, since we have another error to deal with.
			return rel, errors.Wrapf(err, "release %s failed", i.ReleaseName)
		}

	}

	if !i.DisableHooks {
		if err := i.execHook(rel.Hooks, hooks.PostInstall); err != nil {
			rel.SetStatus(release.StatusFailed, "failed post-install: "+err.Error())
			_ = i.replaceRelease(rel)
			return rel, err
		}
	}

	rel.SetStatus(release.StatusDeployed, "Install complete")

	// This is a tricky case. The release has been created, but the result
	// cannot be recorded. The truest thing to tell the user is that the
	// release was created. However, the user will not be able to do anything
	// further with this release.
	//
	// One possible strategy would be to do a timed retry to see if we can get
	// this stored in the future.
	i.recordRelease(rel)

	return rel, nil
}

// availableName tests whether a name is available
//
// Roughly, this will return an error if name is
//
//	- empty
//	- too long
// 	- already in use, and not deleted
//	- used by a deleted release, and i.Replace is false
func (i *Install) availableName() error {
	start := i.ReleaseName
	if start == "" {
		return errors.New("name is required")
	}

	if len(start) > releaseNameMaxLen {
		return errors.Errorf("release name %q exceeds max length of %d", start, releaseNameMaxLen)
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

// renderResources renders the templates in a chart
func (c *Configuration) renderResources(ch *chart.Chart, values chartutil.Values) ([]*release.Hook, *bytes.Buffer, string, error) {
	hs := []*release.Hook{}
	b := bytes.NewBuffer(nil)

	caps, err := c.getCapabilities()
	if err != nil {
		return hs, b, "", err
	}

	if ch.Metadata.KubeVersion != "" {
		if !version.IsCompatibleRange(ch.Metadata.KubeVersion, caps.KubeVersion.String()) {
			return hs, b, "", errors.Errorf("chart requires kubernetesVersion: %s which is incompatible with Kubernetes %s", ch.Metadata.KubeVersion, caps.KubeVersion.String())
		}
	}

	files, err := engine.Render(ch, values)
	if err != nil {
		return hs, b, "", err
	}

	// NOTES.txt gets rendered like all the other files, but because it's not a hook nor a resource,
	// pull it out of here into a separate file so that we can actually use the output of the rendered
	// text file. We have to spin through this map because the file contains path information, so we
	// look for terminating NOTES.txt. We also remove it from the files so that we don't have to skip
	// it in the sortHooks.
	notes := ""
	for k, v := range files {
		if strings.HasSuffix(k, notesFileSuffix) {
			// Only apply the notes if it belongs to the parent chart
			// Note: Do not use filePath.Join since it creates a path with \ which is not expected
			if k == path.Join(ch.Name(), "templates", notesFileSuffix) {
				notes = v
			}
			delete(files, k)
		}
	}

	// Sort hooks, manifests, and partials. Only hooks and manifests are returned,
	// as partials are not used after renderer.Render. Empty manifests are also
	// removed here.
	hs, manifests, err := releaseutil.SortManifests(files, caps.APIVersions, releaseutil.InstallOrder)
	if err != nil {
		// By catching parse errors here, we can prevent bogus releases from going
		// to Kubernetes.
		//
		// We return the files as a big blob of data to help the user debug parser
		// errors.
		for name, content := range files {
			if strings.TrimSpace(content) == "" {
				continue
			}
			fmt.Fprintf(b, "---\n# Source: %s\n%s\n", name, content)
		}
		return hs, b, "", err
	}

	// Aggregate all valid manifests into one big doc.
	for _, m := range manifests {
		fmt.Fprintf(b, "---\n# Source: %s\n%s\n", m.Name, m.Content)
	}

	return hs, b, notes, nil
}

// validateManifest checks to see whether the given manifest is valid for the current Kubernetes
func (i *Install) validateManifest(manifest io.Reader) error {
	_, err := i.cfg.KubeClient.BuildUnstructured(manifest)
	return err
}

// execHook executes all of the hooks for the given hook event.
func (i *Install) execHook(hs []*release.Hook, hook string) error {
	executingHooks := []*release.Hook{}

	for _, h := range hs {
		for _, e := range h.Events {
			if string(e) == hook {
				executingHooks = append(executingHooks, h)
			}
		}
	}

	sort.Sort(hookByWeight(executingHooks))

	for _, h := range executingHooks {
		if err := deleteHookByPolicy(i.cfg, h, hooks.BeforeHookCreation); err != nil {
			return err
		}

		b := bytes.NewBufferString(h.Manifest)
		if err := i.cfg.KubeClient.Create(b); err != nil {
			return errors.Wrapf(err, "warning: Release %s %s %s failed", i.ReleaseName, hook, h.Path)
		}
		b.Reset()
		b.WriteString(h.Manifest)

		if err := i.cfg.KubeClient.WatchUntilReady(b, i.Timeout); err != nil {
			// If a hook is failed, checkout the annotation of the hook to determine whether the hook should be deleted
			// under failed condition. If so, then clear the corresponding resource object in the hook
			if err := deleteHookByPolicy(i.cfg, h, hooks.HookFailed); err != nil {
				return err
			}
			return err
		}
	}

	// If all hooks are succeeded, checkout the annotation of each hook to determine whether the hook should be deleted
	// under succeeded condition. If so, then clear the corresponding resource object in each hook
	for _, h := range executingHooks {
		if err := deleteHookByPolicy(i.cfg, h, hooks.HookSucceeded); err != nil {
			return err
		}
		h.LastRun = time.Now()
	}

	return nil
}

// deletePolices represents a mapping between the key in the annotation for label deleting policy and its real meaning
// FIXME: Can we refactor this out?
var deletePolices = map[string]release.HookDeletePolicy{
	hooks.HookSucceeded:      release.HookSucceeded,
	hooks.HookFailed:         release.HookFailed,
	hooks.BeforeHookCreation: release.HookBeforeHookCreation,
}

// hookHasDeletePolicy determines whether the defined hook deletion policy matches the hook deletion polices
// supported by helm. If so, mark the hook as one should be deleted.
func hookHasDeletePolicy(h *release.Hook, policy string) bool {
	dp, ok := deletePolices[policy]
	if !ok {
		return false
	}
	for _, v := range h.DeletePolicies {
		if dp == v {
			return true
		}
	}
	return false
}

// hookByWeight is a sorter for hooks
type hookByWeight []*release.Hook

func (x hookByWeight) Len() int      { return len(x) }
func (x hookByWeight) Swap(i, j int) { x[i], x[j] = x[j], x[i] }
func (x hookByWeight) Less(i, j int) bool {
	if x[i].Weight == x[j].Weight {
		return x[i].Name < x[j].Name
	}
	return x[i].Weight < x[j].Weight
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

	return fmt.Sprintf("%s-%d", base, time.Now().Unix()), args[0], nil
}

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
// - chart repos in $HELM_HOME
// - URL
//
// If 'verify' is true, this will attempt to also verify the chart.
func (c *ChartPathOptions) LocateChart(name string, settings cli.EnvSettings) (string, error) {
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

	crepo := filepath.Join(settings.Home.Repository(), name)
	if _, err := os.Stat(crepo); err == nil {
		return filepath.Abs(crepo)
	}

	dl := downloader.ChartDownloader{
		HelmHome: settings.Home,
		Out:      os.Stdout,
		Keyring:  c.Keyring,
		Getters:  getter.All(settings),
		Username: c.Username,
		Password: c.Password,
	}
	if c.Verify {
		dl.Verify = downloader.VerifyAlways
	}
	if c.RepoURL != "" {
		chartURL, err := repo.FindChartInAuthRepoURL(c.RepoURL, c.Username, c.Password, name, version,
			c.CertFile, c.KeyFile, c.CaFile, getter.All(settings))
		if err != nil {
			return "", err
		}
		name = chartURL
	}

	if _, err := os.Stat(settings.Home.Archive()); os.IsNotExist(err) {
		os.MkdirAll(settings.Home.Archive(), 0744)
	}

	filename, _, err := dl.DownloadTo(name, version, settings.Home.Archive())
	if err == nil {
		lname, err := filepath.Abs(filename)
		if err != nil {
			return filename, err
		}
		return lname, nil
	} else if settings.Debug {
		return filename, err
	}

	return filename, errors.Errorf("failed to download %q (hint: running `helm repo update` may help)", name)
}

// MergeValues merges values from files specified via -f/--values and
// directly via --set or --set-string, marshaling them to YAML
func (v *ValueOptions) MergeValues(settings cli.EnvSettings) error {
	base := map[string]interface{}{}

	// User specified a values files via -f/--values
	for _, filePath := range v.ValueFiles {
		currentMap := map[string]interface{}{}

		bytes, err := readFile(filePath, settings)
		if err != nil {
			return err
		}

		if err := yaml.Unmarshal(bytes, &currentMap); err != nil {
			return errors.Wrapf(err, "failed to parse %s", filePath)
		}
		// Merge with the previous map
		base = mergeMaps(base, currentMap)
	}

	// User specified a value via --set
	for _, value := range v.Values {
		if err := strvals.ParseInto(value, base); err != nil {
			return errors.Wrap(err, "failed parsing --set data")
		}
	}

	// User specified a value via --set-string
	for _, value := range v.StringValues {
		if err := strvals.ParseIntoString(value, base); err != nil {
			return errors.Wrap(err, "failed parsing --set-string data")
		}
	}

	v.rawValues = base
	return nil
}

// mergeValues merges source and destination map, preferring values from the source map
func mergeValues(dest, src map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{})
	for k, v := range dest {
		out[k] = v
	}
	for k, v := range src {
		if _, ok := out[k]; !ok {
			// If the key doesn't exist already, then just set the key to that value
		} else if nextMap, ok := v.(map[string]interface{}); !ok {
			// If it isn't another map, overwrite the value
		} else if destMap, isMap := out[k].(map[string]interface{}); !isMap {
			// Edge case: If the key exists in the destination, but isn't a map
			// If the source map has a map for this key, prefer it
		} else {
			// If we got to this point, it is a map in both, so merge them
			out[k] = mergeValues(destMap, nextMap)
			continue
		}
		out[k] = v
	}
	return out
}

func mergeMaps(a, b map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(a))
	for k, v := range a {
		out[k] = v
	}
	for k, v := range b {
		if v, ok := v.(map[string]interface{}); ok {
			if bv, ok := out[k]; ok {
				if bv, ok := bv.(map[string]interface{}); ok {
					out[k] = mergeMaps(bv, v)
					continue
				}
			}
		}
		out[k] = v
	}
	return out
}

// readFile load a file from stdin, the local directory, or a remote file with a url.
func readFile(filePath string, settings cli.EnvSettings) ([]byte, error) {
	if strings.TrimSpace(filePath) == "-" {
		return ioutil.ReadAll(os.Stdin)
	}
	u, _ := url.Parse(filePath)
	p := getter.All(settings)

	// FIXME: maybe someone handle other protocols like ftp.
	getterConstructor, err := p.ByScheme(u.Scheme)

	if err != nil {
		return ioutil.ReadFile(filePath)
	}

	getter, err := getterConstructor(filePath, "", "", "")
	if err != nil {
		return []byte{}, err
	}
	data, err := getter.Get(filePath)
	return data.Bytes(), err
}
