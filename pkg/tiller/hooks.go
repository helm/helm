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
	"fmt"
	"log"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/ghodss/yaml"

	"k8s.io/helm/pkg/chartutil"
	"k8s.io/helm/pkg/hooks"
	"k8s.io/helm/pkg/manifest"
	"k8s.io/helm/pkg/proto/hapi/chart"
	"k8s.io/helm/pkg/proto/hapi/release"
	util "k8s.io/helm/pkg/releaseutil"
)

// SortType is used for determining sort function
type SortType int

const (
	// SortInstall is used for installing a chart
	SortInstall SortType = iota
	// SortUninstall is used for removing a chart
	SortUninstall
)

var events = map[string]release.Hook_Event{
	hooks.PreInstall:         release.Hook_PRE_INSTALL,
	hooks.PostInstall:        release.Hook_POST_INSTALL,
	hooks.PreDelete:          release.Hook_PRE_DELETE,
	hooks.PostDelete:         release.Hook_POST_DELETE,
	hooks.PreUpgrade:         release.Hook_PRE_UPGRADE,
	hooks.PostUpgrade:        release.Hook_POST_UPGRADE,
	hooks.PreRollback:        release.Hook_PRE_ROLLBACK,
	hooks.PostRollback:       release.Hook_POST_ROLLBACK,
	hooks.ReleaseTestSuccess: release.Hook_RELEASE_TEST_SUCCESS,
	hooks.ReleaseTestFailure: release.Hook_RELEASE_TEST_FAILURE,
	hooks.CRDInstall:         release.Hook_CRD_INSTALL,
}

// deletePolices represents a mapping between the key in the annotation for label deleting policy and its real meaning
var deletePolices = map[string]release.Hook_DeletePolicy{
	hooks.HookSucceeded:      release.Hook_SUCCEEDED,
	hooks.HookFailed:         release.Hook_FAILED,
	hooks.BeforeHookCreation: release.Hook_BEFORE_HOOK_CREATION,
}

// Timeout used when deleting resources with a hook-delete-policy.
const defaultHookDeleteTimeoutInSeconds = int64(60)

// Manifest represents a manifest file, which has a name and some content.
type Manifest = manifest.Manifest

type result struct {
	hooks   []*release.Hook
	generic []Manifest
}

type manifestFile struct {
	entries map[string]string
	path    string
	apis    chartutil.VersionSet
}

// sortManifests takes a map of filename/YAML contents, splits the file
// by manifest entries, and sorts the entries into hook types.
//
// The resulting hooks struct will be populated with all of the generated hooks.
// Any file that does not declare one of the hook types will be placed in the
// 'generic' bucket.
//
// Files that do not parse into the expected format are simply placed into a map and
// returned.
func sortManifests(ch *chart.Chart, files map[string]string, apis chartutil.VersionSet, sort SortType) ([]*release.Hook, []Manifest, error) {
	result := &result{}

	for filePath, c := range files {

		// Skip partials. We could return these as a separate map, but there doesn't
		// seem to be any need for that at this time.
		if strings.HasPrefix(path.Base(filePath), "_") {
			continue
		}
		// Skip empty files and log this.
		if len(strings.TrimSpace(c)) == 0 {
			log.Printf("info: manifest %q is empty. Skipping.", filePath)
			continue
		}

		manifestFile := &manifestFile{
			entries: util.SplitManifests(c),
			path:    filePath,
			apis:    apis,
		}

		if err := manifestFile.sort(ch, result); err != nil {
			return result.hooks, result.generic, err
		}
	}

	return result.hooks, sortByWeight(sortByKind(result.generic, sort), sort), nil
}

// sort takes a manifestFile object which may contain multiple resource definition
// entries and sorts each entry by hook types, and saves the resulting hooks and
// generic manifests (or non-hooks) to the result struct.
//
// To determine hook type, it looks for a YAML structure like this:
//
//  kind: SomeKind
//  apiVersion: v1
// 	metadata:
//		annotations:
//			helm.sh/hook: pre-install
//
// To determine the policy to delete the hook, it looks for a YAML structure like this:
//
//  kind: SomeKind
//  apiVersion: v1
//  metadata:
// 		annotations:
// 			helm.sh/hook-delete-policy: hook-succeeded
func (file *manifestFile) sort(ch *chart.Chart, result *result) error {
	for _, m := range file.entries {
		var entry manifest.SimpleHead
		err := yaml.Unmarshal([]byte(m), &entry)

		if err != nil {
			e := fmt.Errorf("YAML parse error on %s: %s", file.path, err)
			return e
		}

		var weight manifest.Weight
		if ch != nil {
			weight = manifest.Weight{
				Chart:    getChartWeight(ch, file.path),
				Manifest: 0,
			}
		}

		if !hasAnyAnnotation(entry) {
			result.generic = append(result.generic, Manifest{
				Name:    file.path,
				Content: m,
				Head:    &entry,
				Weight:  &weight,
			})
			continue
		}

		if mw, err := strconv.ParseUint(entry.Metadata.Annotations[manifest.ManifestOrderWeight], 10, 32); err == nil {
			weight.Manifest = uint32(mw)
		}

		hookTypes, ok := entry.Metadata.Annotations[hooks.HookAnno]
		if !ok {
			result.generic = append(result.generic, Manifest{
				Name:    file.path,
				Content: m,
				Head:    &entry,
				Weight:  &weight,
			})
			continue
		}

		hw := calculateHookWeight(entry)

		h := &release.Hook{
			Name:           entry.Metadata.Name,
			Kind:           entry.Kind,
			Path:           file.path,
			Manifest:       m,
			Events:         []release.Hook_Event{},
			Weight:         hw,
			DeletePolicies: []release.Hook_DeletePolicy{},
		}

		isUnknownHook := false
		for _, hookType := range strings.Split(hookTypes, ",") {
			hookType = strings.ToLower(strings.TrimSpace(hookType))
			e, ok := events[hookType]
			if !ok {
				isUnknownHook = true
				break
			}
			h.Events = append(h.Events, e)
		}

		if isUnknownHook {
			log.Printf("info: skipping unknown hook: %q", hookTypes)
			continue
		}

		result.hooks = append(result.hooks, h)

		operateAnnotationValues(entry, hooks.HookDeleteAnno, func(value string) {
			policy, exist := deletePolices[value]
			if exist {
				h.DeletePolicies = append(h.DeletePolicies, policy)
			} else {
				log.Printf("info: skipping unknown hook delete policy: %q", value)
			}
		})

		// Only check for delete timeout annotation if there is a deletion policy.
		if len(h.DeletePolicies) > 0 {
			h.DeleteTimeout = defaultHookDeleteTimeoutInSeconds
			operateAnnotationValues(entry, hooks.HookDeleteTimeoutAnno, func(value string) {
				timeout, err := strconv.ParseInt(value, 10, 64)
				if err != nil || timeout < 0 {
					log.Printf("info: ignoring invalid hook delete timeout value: %q", value)
				}
				h.DeleteTimeout = timeout
			})
		}
	}
	return nil
}

func getChartWeight(ch *chart.Chart, name string) uint32 {
	if ch == nil {
		return 0
	}

	for _, tpl := range ch.Templates {
		if path.Base(tpl.Name) == path.Base(name) && ch.Metadata.Name == getOwnerChart(name) {
			return ch.Metadata.Weight
		}
	}

	for _, chart := range ch.Dependencies {
		if w := getChartWeight(chart, name); w != 0 {
			return w
		}
	}

	return 0
}

func getOwnerChart(path string) string {
	parts := strings.Split(path, string(os.PathSeparator))
	if len(parts) >= 3 {
		return parts[len(parts)-3]
	}
	return ""
}

func hasAnyAnnotation(entry manifest.SimpleHead) bool {
	if entry.Metadata == nil ||
		entry.Metadata.Annotations == nil ||
		len(entry.Metadata.Annotations) == 0 {
		return false
	}

	return true
}

func calculateHookWeight(entry manifest.SimpleHead) int32 {
	hws := entry.Metadata.Annotations[hooks.HookWeightAnno]
	hw, err := strconv.Atoi(hws)
	if err != nil {
		hw = 0
	}

	return int32(hw)
}

func operateAnnotationValues(entry manifest.SimpleHead, annotation string, operate func(p string)) {
	if dps, ok := entry.Metadata.Annotations[annotation]; ok {
		for _, dp := range strings.Split(dps, ",") {
			dp = strings.ToLower(strings.TrimSpace(dp))
			operate(dp)
		}
	}
}
