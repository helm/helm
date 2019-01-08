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
	"path"
	"sort"
	"strings"
	"time"

	"github.com/pkg/errors"
	"k8s.io/helm/pkg/chart"
	"k8s.io/helm/pkg/chartutil"
	"k8s.io/helm/pkg/engine"
	"k8s.io/helm/pkg/hapi/release"
	"k8s.io/helm/pkg/hooks"
	"k8s.io/helm/pkg/releaseutil"
	"k8s.io/helm/pkg/version"
)

// releaseNameMaxLen is the maximum length of a release name.
//
// As of Kubernetes 1.4, the max limit on a name is 63 chars. We reserve 10 for
// charts to add data. Effectively, that gives us 53 chars.
// See https://github.com/kubernetes/helm/issues/1528
const releaseNameMaxLen = 53

// NOTESFILE_SUFFIX that we want to treat special. It goes through the templating engine
// but it's not a yaml file (resource) hence can't have hooks, etc. And the user actually
// wants to see this file after rendering in the status command. However, it must be a suffix
// since there can be filepath in front of it.
const notesFileSuffix = "NOTES.txt"

// Install performs an installation operation.
type Install struct {
	cfg *Configuration

	DryRun       bool
	DisableHooks bool
	Replace      bool
	Wait         bool
	Devel        bool
	DepUp        bool
	Timeout      int64
	Namespace    string
	ReleaseName  string
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
func (i *Install) Run(chrt *chart.Chart, rawValues map[string]interface{}) (*release.Release, error) {
	if err := i.availableName(); err != nil {
		return nil, err
	}

	caps, err := i.cfg.capabilities()
	if err != nil {
		return nil, err
	}

	options := chartutil.ReleaseOptions{
		Name:      i.ReleaseName,
		IsInstall: true,
	}
	valuesToRender, err := chartutil.ToRenderValues(chrt, rawValues, options, caps)
	if err != nil {
		return nil, err
	}

	rel := i.createRelease(chrt, rawValues)
	var manifestDoc *bytes.Buffer
	rel.Hooks, manifestDoc, rel.Info.Notes, err = i.renderResources(chrt, valuesToRender, caps.APIVersions)
	// Even for errors, attach this if available
	if manifestDoc != nil {
		rel.Manifest = manifestDoc.String()
	}
	// Check error from render
	if err != nil {
		rel.SetStatus(release.StatusFailed, fmt.Sprintf("failed to render resource: %s", err.Error()))
		rel.Version = 0 // Why do we do this?
		return rel, err
	}

	// Mark this release as in-progress
	rel.SetStatus(release.StatusPendingInstall, "Intiial install underway")
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
			i.replaceRelease(rel)
			return rel, err
		}
	}

	// At this point, we can do the install. Note that before we were detecting whether to
	// do an update, but it's not clear whether we WANT to do an update if the re-use is set
	// to true, since that is basically an upgrade operation.
	buf := bytes.NewBufferString(rel.Manifest)
	if err := i.cfg.KubeClient.Create(i.Namespace, buf, i.Timeout, i.Wait); err != nil {
		rel.SetStatus(release.StatusFailed, fmt.Sprintf("Release %q failed: %s", i.ReleaseName, err.Error()))
		i.recordRelease(rel) // Ignore the error, since we have another error to deal with.
		return rel, errors.Wrapf(err, "release %s failed", i.ReleaseName)
	}

	if !i.DisableHooks {
		if err := i.execHook(rel.Hooks, hooks.PostInstall); err != nil {
			rel.SetStatus(release.StatusFailed, "failed post-install: "+err.Error())
			i.replaceRelease(rel)
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
func (i *Install) renderResources(ch *chart.Chart, values chartutil.Values, vs chartutil.VersionSet) ([]*release.Hook, *bytes.Buffer, string, error) {
	hooks := []*release.Hook{}
	buf := bytes.NewBuffer(nil)
	// Guard to make sure Helm is at the right version to handle this chart.
	sver := version.GetVersion()
	if ch.Metadata.HelmVersion != "" &&
		!version.IsCompatibleRange(ch.Metadata.HelmVersion, sver) {
		return hooks, buf, "", errors.Errorf("chart incompatible with Helm %s", sver)
	}

	if ch.Metadata.KubeVersion != "" {
		cap, _ := values["Capabilities"].(*chartutil.Capabilities)
		gitVersion := cap.KubeVersion.String()
		k8sVersion := strings.Split(gitVersion, "+")[0]
		if !version.IsCompatibleRange(ch.Metadata.KubeVersion, k8sVersion) {
			return hooks, buf, "", errors.Errorf("chart requires kubernetesVersion: %s which is incompatible with Kubernetes %s", ch.Metadata.KubeVersion, k8sVersion)
		}
	}

	files, err := engine.New().Render(ch, values)
	if err != nil {
		return hooks, buf, "", err
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
	// TODO: Can we migrate SortManifests out of pkg/tiller?
	hooks, manifests, err := releaseutil.SortManifests(files, vs, releaseutil.InstallOrder)
	if err != nil {
		// By catching parse errors here, we can prevent bogus releases from going
		// to Kubernetes.
		//
		// We return the files as a big blob of data to help the user debug parser
		// errors.
		b := bytes.NewBuffer(nil)
		for name, content := range files {
			if len(strings.TrimSpace(content)) == 0 {
				continue
			}
			b.WriteString("\n---\n# Source: " + name + "\n")
			b.WriteString(content)
		}
		return hooks, b, "", err
	}

	// Aggregate all valid manifests into one big doc.
	b := bytes.NewBuffer(nil)
	for _, m := range manifests {
		b.WriteString("\n---\n# Source: " + m.Name + "\n")
		b.WriteString(m.Content)
	}

	return hooks, b, notes, nil
}

// validateManifest checks to see whether the given manifest is valid for the current Kubernetes
func (i *Install) validateManifest(manifest io.Reader) error {
	_, err := i.cfg.KubeClient.BuildUnstructured(i.Namespace, manifest)
	return err
}

// execHook executes all of the hooks for the given hook event.
func (i *Install) execHook(hs []*release.Hook, hook string) error {
	name := i.ReleaseName
	namespace := i.Namespace
	timeout := i.Timeout
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
		if err := i.deleteHookByPolicy(h, hooks.BeforeHookCreation, hook); err != nil {
			return err
		}

		b := bytes.NewBufferString(h.Manifest)
		if err := i.cfg.KubeClient.Create(namespace, b, timeout, false); err != nil {
			return errors.Wrapf(err, "warning: Release %s %s %s failed", name, hook, h.Path)
		}
		b.Reset()
		b.WriteString(h.Manifest)

		if err := i.cfg.KubeClient.WatchUntilReady(namespace, b, timeout, false); err != nil {
			// If a hook is failed, checkout the annotation of the hook to determine whether the hook should be deleted
			// under failed condition. If so, then clear the corresponding resource object in the hook
			if err := i.deleteHookByPolicy(h, hooks.HookFailed, hook); err != nil {
				return err
			}
			return err
		}
	}

	// If all hooks are succeeded, checkout the annotation of each hook to determine whether the hook should be deleted
	// under succeeded condition. If so, then clear the corresponding resource object in each hook
	for _, h := range executingHooks {
		if err := i.deleteHookByPolicy(h, hooks.HookSucceeded, hook); err != nil {
			return err
		}
		h.LastRun = time.Now()
	}

	return nil
}

// deleteHookByPolicy deletes a hook if the hook policy instructs it to
func (i *Install) deleteHookByPolicy(h *release.Hook, policy, hook string) error {
	b := bytes.NewBufferString(h.Manifest)
	if hookHasDeletePolicy(h, policy) {
		if errHookDelete := i.cfg.KubeClient.Delete(i.Namespace, b); errHookDelete != nil {
			return errHookDelete
		}
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
