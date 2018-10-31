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

package tiller

import (
	"bytes"
	"path"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/discovery"

	"k8s.io/helm/pkg/chart"
	"k8s.io/helm/pkg/chartutil"
	"k8s.io/helm/pkg/engine"
	"k8s.io/helm/pkg/hapi"
	"k8s.io/helm/pkg/hapi/release"
	"k8s.io/helm/pkg/hooks"
	relutil "k8s.io/helm/pkg/releaseutil"
	"k8s.io/helm/pkg/storage"
	"k8s.io/helm/pkg/storage/driver"
	"k8s.io/helm/pkg/tiller/environment"
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

var (
	// errMissingChart indicates that a chart was not provided.
	errMissingChart = errors.New("no chart provided")
	// errMissingRelease indicates that a release (name) was not provided.
	errMissingRelease = errors.New("no release provided")
	// errInvalidRevision indicates that an invalid release revision number was provided.
	errInvalidRevision = errors.New("invalid release revision")
	//errInvalidName indicates that an invalid release name was provided
	errInvalidName = errors.New("invalid release name, must match regex ^(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])+$ and the length must not longer than 53")
)

// ValidName is a regular expression for names.
//
// According to the Kubernetes help text, the regular expression it uses is:
//
//	(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])?
//
// We modified that. First, we added start and end delimiters. Second, we changed
// the final ? to + to require that the pattern match at least once. This modification
// prevents an empty string from matching.
var ValidName = regexp.MustCompile("^(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])+$")

// ReleaseServer implements the server-side gRPC endpoint for the HAPI services.
type ReleaseServer struct {
	engine    Engine
	discovery discovery.DiscoveryInterface

	// Releases stores records of releases.
	Releases *storage.Storage
	// KubeClient is a Kubernetes API client.
	KubeClient environment.KubeClient

	Log func(string, ...interface{})
}

// NewReleaseServer creates a new release server.
func NewReleaseServer(discovery discovery.DiscoveryInterface, kubeClient environment.KubeClient) *ReleaseServer {
	return &ReleaseServer{
		engine:     engine.New(),
		discovery:  discovery,
		Releases:   storage.Init(driver.NewMemory()),
		KubeClient: kubeClient,
		Log:        func(_ string, _ ...interface{}) {},
	}
}

// reuseValues copies values from the current release to a new release if the
// new release does not have any values.
//
// If the request already has values, or if there are no values in the current
// release, this does nothing.
//
// This is skipped if the req.ResetValues flag is set, in which case the
// request values are not altered.
func (s *ReleaseServer) reuseValues(req *hapi.UpdateReleaseRequest, current *release.Release) error {
	if req.ResetValues {
		// If ResetValues is set, we comletely ignore current.Config.
		s.Log("resetting values to the chart's original version")
		return nil
	}

	// If the ReuseValues flag is set, we always copy the old values over the new config's values.
	if req.ReuseValues {
		s.Log("reusing the old release's values")

		// We have to regenerate the old coalesced values:
		oldVals, err := chartutil.CoalesceValues(current.Chart, current.Config)
		if err != nil {
			return errors.Wrap(err, "failed to rebuild old values")
		}

		// merge new values with current
		b := append(current.Config, '\n')
		req.Values = append(b, req.Values...)

		req.Chart.Values = oldVals

		// yaml unmarshal and marshal to remove duplicate keys
		y := map[string]interface{}{}
		if err := yaml.Unmarshal(req.Values, &y); err != nil {
			return err
		}
		data, err := yaml.Marshal(y)
		if err != nil {
			return err
		}

		req.Values = data
		return nil
	}

	// If req.Values is empty, but current.Config is not, copy current into the
	// request.
	if (len(req.Values) == 0 || bytes.Equal(req.Values, []byte("{}\n"))) &&
		len(current.Config) > 0 &&
		!bytes.Equal(current.Config, []byte("{}\n")) {
		s.Log("copying values from %s (v%d) to new release.", current.Name, current.Version)
		req.Values = current.Config
	}
	return nil
}

func (s *ReleaseServer) uniqName(start string, reuse bool) (string, error) {

	if start == "" {
		return "", errors.New("name is required")
	}

	if len(start) > releaseNameMaxLen {
		return "", errors.Errorf("release name %q exceeds max length of %d", start, releaseNameMaxLen)
	}

	h, err := s.Releases.History(start)
	if err != nil || len(h) < 1 {
		return start, nil
	}
	relutil.Reverse(h, relutil.SortByRevision)
	rel := h[0]

	if st := rel.Info.Status; reuse && (st == release.StatusUninstalled || st == release.StatusFailed) {
		// Allowe re-use of names if the previous release is marked deleted.
		s.Log("name %s exists but is not in use, reusing name", start)
		return start, nil
	} else if reuse {
		return "", errors.New("cannot re-use a name that is still in use")
	}

	return "", errors.Errorf("a release named %s already exists.\nRun: helm ls --all %s; to check the status of the release\nOr run: helm del --purge %s; to delete it", start, start, start)

}

// capabilities builds a Capabilities from discovery information.
func capabilities(disc discovery.DiscoveryInterface) (*chartutil.Capabilities, error) {
	sv, err := disc.ServerVersion()
	if err != nil {
		return nil, err
	}
	vs, err := GetVersionSet(disc)
	if err != nil {
		return nil, errors.Wrap(err, "could not get apiVersions from Kubernetes")
	}
	return &chartutil.Capabilities{
		APIVersions: vs,
		KubeVersion: sv,
		HelmVersion: version.GetBuildInfo(),
	}, nil
}

// GetVersionSet retrieves a set of available k8s API versions
func GetVersionSet(client discovery.ServerGroupsInterface) (chartutil.VersionSet, error) {
	groups, err := client.ServerGroups()
	if err != nil {
		return chartutil.DefaultVersionSet, err
	}

	// FIXME: The Kubernetes test fixture for cli appears to always return nil
	// for calls to Discovery().ServerGroups(). So in this case, we return
	// the default API list. This is also a safe value to return in any other
	// odd-ball case.
	if groups.Size() == 0 {
		return chartutil.DefaultVersionSet, nil
	}

	versions := metav1.ExtractGroupVersions(groups)
	return chartutil.NewVersionSet(versions...), nil
}

func (s *ReleaseServer) renderResources(ch *chart.Chart, values chartutil.Values, vs chartutil.VersionSet) ([]*release.Hook, *bytes.Buffer, string, error) {
	// Guard to make sure Helm is at the right version to handle this chart.
	sver := version.GetVersion()
	if ch.Metadata.HelmVersion != "" &&
		!version.IsCompatibleRange(ch.Metadata.HelmVersion, sver) {
		return nil, nil, "", errors.Errorf("chart incompatible with Helm %s", sver)
	}

	if ch.Metadata.KubeVersion != "" {
		cap, _ := values["Capabilities"].(*chartutil.Capabilities)
		gitVersion := cap.KubeVersion.String()
		k8sVersion := strings.Split(gitVersion, "+")[0]
		if !version.IsCompatibleRange(ch.Metadata.KubeVersion, k8sVersion) {
			return nil, nil, "", errors.Errorf("chart requires kubernetesVersion: %s which is incompatible with Kubernetes %s", ch.Metadata.KubeVersion, k8sVersion)
		}
	}

	s.Log("rendering %s chart using values", ch.Name())
	files, err := s.engine.Render(ch, values)
	if err != nil {
		return nil, nil, "", err
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
	hooks, manifests, err := SortManifests(files, vs, InstallOrder)
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
		return nil, b, "", err
	}

	// Aggregate all valid manifests into one big doc.
	b := bytes.NewBuffer(nil)
	for _, m := range manifests {
		b.WriteString("\n---\n# Source: " + m.Name + "\n")
		b.WriteString(m.Content)
	}

	return hooks, b, notes, nil
}

// recordRelease with an update operation in case reuse has been set.
func (s *ReleaseServer) recordRelease(r *release.Release, reuse bool) {
	if reuse {
		if err := s.Releases.Update(r); err != nil {
			s.Log("warning: Failed to update release %s: %s", r.Name, err)
		}
	} else if err := s.Releases.Create(r); err != nil {
		s.Log("warning: Failed to record release %s: %s", r.Name, err)
	}
}

func (s *ReleaseServer) execHook(hs []*release.Hook, name, namespace, hook string, timeout int64) error {
	code, ok := events[hook]
	if !ok {
		return errors.Errorf("unknown hook %s", hook)
	}

	s.Log("executing %d %s hooks for %s", len(hs), hook, name)
	executingHooks := []*release.Hook{}
	for _, h := range hs {
		for _, e := range h.Events {
			if e == code {
				executingHooks = append(executingHooks, h)
			}
		}
	}

	sort.Sort(hookByWeight(executingHooks))

	for _, h := range executingHooks {
		if err := s.deleteHookIfShouldBeDeletedByDeletePolicy(h, hooks.BeforeHookCreation, name, namespace, hook, s.KubeClient); err != nil {
			return err
		}

		b := bytes.NewBufferString(h.Manifest)
		if err := s.KubeClient.Create(namespace, b, timeout, false); err != nil {
			return errors.Wrapf(err, "warning: Release %s %s %s failed", name, hook, h.Path)
		}
		// No way to rewind a bytes.Buffer()?
		b.Reset()
		b.WriteString(h.Manifest)

		if err := s.KubeClient.WatchUntilReady(namespace, b, timeout, false); err != nil {
			s.Log("warning: Release %s %s %s could not complete: %s", name, hook, h.Path, err)
			// If a hook is failed, checkout the annotation of the hook to determine whether the hook should be deleted
			// under failed condition. If so, then clear the corresponding resource object in the hook
			if err := s.deleteHookIfShouldBeDeletedByDeletePolicy(h, hooks.HookFailed, name, namespace, hook, s.KubeClient); err != nil {
				return err
			}
			return err
		}
	}

	s.Log("hooks complete for %s %s", hook, name)
	// If all hooks are succeeded, checkout the annotation of each hook to determine whether the hook should be deleted
	// under succeeded condition. If so, then clear the corresponding resource object in each hook
	for _, h := range executingHooks {
		if err := s.deleteHookIfShouldBeDeletedByDeletePolicy(h, hooks.HookSucceeded, name, namespace, hook, s.KubeClient); err != nil {
			return err
		}
		h.LastRun = time.Now()
	}

	return nil
}

func validateManifest(c environment.KubeClient, ns string, manifest []byte) error {
	r := bytes.NewReader(manifest)
	_, err := c.BuildUnstructured(ns, r)
	return err
}

func validateReleaseName(releaseName string) error {
	if releaseName == "" {
		return errMissingRelease
	}

	if !ValidName.MatchString(releaseName) || (len(releaseName) > releaseNameMaxLen) {
		return errInvalidName
	}

	return nil
}

func (s *ReleaseServer) deleteHookIfShouldBeDeletedByDeletePolicy(h *release.Hook, policy, name, namespace, hook string, kubeCli environment.KubeClient) error {
	b := bytes.NewBufferString(h.Manifest)
	if hookHasDeletePolicy(h, policy) {
		s.Log("deleting %s hook %s for release %s due to %q policy", hook, h.Name, name, policy)
		if errHookDelete := kubeCli.Delete(namespace, b); errHookDelete != nil {
			s.Log("warning: Release %s %s %S could not be deleted: %s", name, hook, h.Path, errHookDelete)
			return errHookDelete
		}
	}
	return nil
}

// hookShouldBeDeleted determines whether the defined hook deletion policy matches the hook deletion polices
// supported by helm. If so, mark the hook as one should be deleted.
func hookHasDeletePolicy(h *release.Hook, policy string) bool {
	if dp, ok := deletePolices[policy]; ok {
		for _, v := range h.DeletePolicies {
			if dp == v {
				return true
			}
		}
	}
	return false
}
