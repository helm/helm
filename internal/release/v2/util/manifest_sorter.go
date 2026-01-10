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

package util // import "helm.sh/helm/v4/internal/release/v2/util"

import (
	"fmt"
	"log/slog"
	"path"
	"sort"
	"strconv"
	"strings"

	"sigs.k8s.io/yaml"

	v2 "helm.sh/helm/v4/internal/release/v2"
	"helm.sh/helm/v4/pkg/chart/common"
)

// Manifest represents a manifest file, which has a name and some content.
type Manifest struct {
	Name    string
	Content string
	Head    *SimpleHead
}

// manifestFile represents a file that contains a manifest.
type manifestFile struct {
	entries map[string]string
	path    string
}

// result is an intermediate structure used during sorting.
type result struct {
	hooks   []*v2.Hook
	generic []Manifest
}

// TODO: Refactor this out. It's here because naming conventions were not followed through.
// So fix the Test hook names and then remove this.
var events = map[string]v2.HookEvent{
	v2.HookPreInstall.String():   v2.HookPreInstall,
	v2.HookPostInstall.String():  v2.HookPostInstall,
	v2.HookPreDelete.String():    v2.HookPreDelete,
	v2.HookPostDelete.String():   v2.HookPostDelete,
	v2.HookPreUpgrade.String():   v2.HookPreUpgrade,
	v2.HookPostUpgrade.String():  v2.HookPostUpgrade,
	v2.HookPreRollback.String():  v2.HookPreRollback,
	v2.HookPostRollback.String(): v2.HookPostRollback,
	v2.HookTest.String():         v2.HookTest,
	// Support test-success for backward compatibility with Helm 2 tests
	"test-success": v2.HookTest,
}

// SortManifests takes a map of filename/YAML contents, splits the file
// by manifest entries, and sorts the entries into hook types.
//
// The resulting hooks struct will be populated with all of the generated hooks.
// Any file that does not declare one of the hook types will be placed in the
// 'generic' bucket.
//
// Files that do not parse into the expected format are simply placed into a map and
// returned.
func SortManifests(files map[string]string, _ common.VersionSet, ordering KindSortOrder) ([]*v2.Hook, []Manifest, error) {
	result := &result{}

	var sortedFilePaths []string
	for filePath := range files {
		sortedFilePaths = append(sortedFilePaths, filePath)
	}
	sort.Strings(sortedFilePaths)

	for _, filePath := range sortedFilePaths {
		content := files[filePath]

		// Skip partials. We could return these as a separate map, but there doesn't
		// seem to be any need for that at this time.
		if strings.HasPrefix(path.Base(filePath), "_") {
			continue
		}
		// Skip empty files and log this.
		if strings.TrimSpace(content) == "" {
			continue
		}

		manifestFile := &manifestFile{
			entries: SplitManifests(content),
			path:    filePath,
		}

		if err := manifestFile.sort(result); err != nil {
			return result.hooks, result.generic, err
		}
	}

	return sortHooksByKind(result.hooks, ordering), sortManifestsByKind(result.generic, ordering), nil
}

// sort takes a manifestFile object which may contain multiple resource definition
// entries and sorts each entry by hook types, and saves the resulting hooks and
// generic manifests (or non-hooks) to the result struct.
//
// To determine hook type, it looks for a YAML structure like this:
//
//	 kind: SomeKind
//	 apiVersion: v1
//		metadata:
//			annotations:
//				helm.sh/hook: pre-install
//
// To determine the policy to delete the hook, it looks for a YAML structure like this:
//
//	 kind: SomeKind
//	 apiVersion: v1
//	 metadata:
//			annotations:
//				helm.sh/hook-delete-policy: hook-succeeded
//
// To determine the policy to output logs of the hook (for Pod and Job only), it looks for a YAML structure like this:
//
//	 kind: Pod
//	 apiVersion: v1
//	 metadata:
//			annotations:
//				helm.sh/hook-output-log-policy: hook-succeeded,hook-failed
func (file *manifestFile) sort(result *result) error {
	// Go through manifests in order found in file (function `SplitManifests` creates integer-sortable keys)
	var sortedEntryKeys []string
	for entryKey := range file.entries {
		sortedEntryKeys = append(sortedEntryKeys, entryKey)
	}
	sort.Sort(BySplitManifestsOrder(sortedEntryKeys))

	for _, entryKey := range sortedEntryKeys {
		m := file.entries[entryKey]

		var entry SimpleHead
		if err := yaml.Unmarshal([]byte(m), &entry); err != nil {
			return fmt.Errorf("YAML parse error on %s: %w", file.path, err)
		}

		if !hasAnyAnnotation(entry) {
			result.generic = append(result.generic, Manifest{
				Name:    file.path,
				Content: m,
				Head:    &entry,
			})
			continue
		}

		hookTypes, ok := entry.Metadata.Annotations[v2.HookAnnotation]
		if !ok {
			result.generic = append(result.generic, Manifest{
				Name:    file.path,
				Content: m,
				Head:    &entry,
			})
			continue
		}

		hw := calculateHookWeight(entry)

		h := &v2.Hook{
			Name:              entry.Metadata.Name,
			Kind:              entry.Kind,
			Path:              file.path,
			Manifest:          m,
			Events:            []v2.HookEvent{},
			Weight:            hw,
			DeletePolicies:    []v2.HookDeletePolicy{},
			OutputLogPolicies: []v2.HookOutputLogPolicy{},
		}

		isUnknownHook := false
		for hookType := range strings.SplitSeq(hookTypes, ",") {
			hookType = strings.ToLower(strings.TrimSpace(hookType))
			e, ok := events[hookType]
			if !ok {
				isUnknownHook = true
				break
			}
			h.Events = append(h.Events, e)
		}

		if isUnknownHook {
			slog.Info("skipping unknown hooks", "hookTypes", hookTypes)
			continue
		}

		result.hooks = append(result.hooks, h)

		operateAnnotationValues(entry, v2.HookDeleteAnnotation, func(value string) {
			h.DeletePolicies = append(h.DeletePolicies, v2.HookDeletePolicy(value))
		})

		operateAnnotationValues(entry, v2.HookOutputLogAnnotation, func(value string) {
			h.OutputLogPolicies = append(h.OutputLogPolicies, v2.HookOutputLogPolicy(value))
		})
	}

	return nil
}

// hasAnyAnnotation returns true if the given entry has any annotations at all.
func hasAnyAnnotation(entry SimpleHead) bool {
	return entry.Metadata != nil &&
		entry.Metadata.Annotations != nil &&
		len(entry.Metadata.Annotations) != 0
}

// calculateHookWeight finds the weight in the hook weight annotation.
//
// If no weight is found, the assigned weight is 0
func calculateHookWeight(entry SimpleHead) int {
	hws := entry.Metadata.Annotations[v2.HookWeightAnnotation]
	hw, err := strconv.Atoi(hws)
	if err != nil {
		hw = 0
	}
	return hw
}

// operateAnnotationValues finds the given annotation and runs the operate function with the value of that annotation
func operateAnnotationValues(entry SimpleHead, annotation string, operate func(p string)) {
	if dps, ok := entry.Metadata.Annotations[annotation]; ok {
		for dp := range strings.SplitSeq(dps, ",") {
			dp = strings.ToLower(strings.TrimSpace(dp))
			operate(dp)
		}
	}
}
