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
	"fmt"
	"strconv"
	"strings"

	"helm.sh/helm/v4/pkg/chart/common/util"
	release "helm.sh/helm/v4/pkg/release"
	"helm.sh/helm/v4/pkg/release/common"
	rspb "helm.sh/helm/v4/pkg/release/v1"
)

// MergeValues is the action for intelligently merging values from multiple release revisions.
//
// It provides the implementation of 'helm values merge'.
type MergeValues struct {
	cfg *Configuration

	// Revisions specifies the revisions to merge. If empty, all revisions will be used.
	Revisions []int
	// AllRevisions indicates whether to include all revisions of the release.
	AllRevisions bool
	// OutputFormat specifies the output format (yaml, json).
	OutputFormat string
	// MergeStrategy defines how to handle conflicts between revisions.
	MergeStrategy string
}

// NewMergeValues creates a new MergeValues object with the given configuration.
func NewMergeValues(cfg *Configuration) *MergeValues {
	return &MergeValues{
		cfg:           cfg,
		OutputFormat:  "yaml",
		MergeStrategy: "latest", // default strategy: use latest value for conflicts
	}
}

// Run executes 'helm values merge' against the given release.
func (m *MergeValues) Run(name string) (map[string]interface{}, error) {
	if err := m.cfg.KubeClient.IsReachable(); err != nil {
		return nil, err
	}

	// Get the release history to determine which revisions to merge
	history, err := m.cfg.Releases.History(name)
	if err != nil {
		return nil, fmt.Errorf("could not get history for release %q: %w", name, err)
	}

	if len(history) == 0 {
		return nil, fmt.Errorf("release %q not found", name)
	}

	// Determine which revisions to merge
	revisionsToMerge, err := m.getRevisionsToMerge(history)
	if err != nil {
		return nil, err
	}

	if len(revisionsToMerge) == 0 {
		return nil, fmt.Errorf("no revisions found to merge for release %q", name)
	}

	// Collect values from specified revisions
	var allValues []map[string]interface{}
	var revisionInfo []string

	for _, rev := range revisionsToMerge {
		rel, err := m.cfg.Releases.Get(name, rev)
		if err != nil {
			return nil, fmt.Errorf("could not get release %q revision %d: %w", name, rev, err)
		}

		r, err := releaserToV1Release(rel)
		if err != nil {
			return nil, err
		}
		var _ *rspb.Release = r // Explicitly use the type

		// Get computed values (merged with chart defaults)
		computedVals, err := util.CoalesceValues(r.Chart, r.Config)
		if err != nil {
			return nil, fmt.Errorf("could not coalesce values for revision %d: %w", rev, err)
		}

		allValues = append(allValues, computedVals)
		revisionInfo = append(revisionInfo, fmt.Sprintf("revision:%d,version:%s,updated:%s",
			rev, r.Chart.Metadata.Version, r.Info.LastDeployed.Format("2006-01-02T15:04:05Z")))
	}

	// Merge all values using the specified strategy
	mergedValues, err := m.mergeValues(allValues)
	if err != nil {
		return nil, fmt.Errorf("could not merge values: %w", err)
	}

	// Add metadata about the merge operation if not empty
	if len(mergedValues) > 0 {
		if mergedValues["helm"] == nil {
			mergedValues["helm"] = map[string]interface{}{}
		}
		helmMeta := mergedValues["helm"].(map[string]interface{})
		helmMeta["mergeMetadata"] = map[string]interface{}{
			"releaseName":    name,
			"revisions":      revisionsToMerge,
			"revisionInfo":   revisionInfo,
			"mergeStrategy":  m.MergeStrategy,
		}
	}

	return mergedValues, nil
}

// getRevisionsToMerge determines which revisions to merge based on configuration
func (m *MergeValues) getRevisionsToMerge(history []release.Releaser) ([]int, error) {
	var revisions []int

	if m.AllRevisions {
		// Include all revisions
		for _, rel := range history {
			r, err := releaserToV1Release(rel)
			if err != nil {
				return nil, err
			}
			revisions = append(revisions, r.Version)
		}
	} else if len(m.Revisions) > 0 {
		// Use specified revisions
		revisions = m.Revisions
	} else {
		// Default: use all deployed revisions
		for _, rel := range history {
			r, err := releaserToV1Release(rel)
			if err != nil {
				return nil, err
			}
			if r.Info.Status == common.StatusDeployed {
				revisions = append(revisions, r.Version)
			}
		}
	}

	// Validate that revisions exist
	availableVersions := make(map[int]bool)
	for _, rel := range history {
		r, err := releaserToV1Release(rel)
		if err != nil {
			return nil, err
		}
		var _ *rspb.Release = r // Explicitly use the type
		availableVersions[r.Version] = true
	}

	var validRevisions []int
	for _, rev := range revisions {
		if !availableVersions[rev] {
			return nil, fmt.Errorf("revision %d does not exist for release", rev)
		}
		validRevisions = append(validRevisions, rev)
	}

	return validRevisions, nil
}

// mergeValues merges multiple values maps using the specified strategy
func (m *MergeValues) mergeValues(valuesList []map[string]interface{}) (map[string]interface{}, error) {
	if len(valuesList) == 0 {
		return make(map[string]interface{}), nil
	}

	if len(valuesList) == 1 {
		return valuesList[0], nil
	}

	// Start with an empty map
	result := make(map[string]interface{})

	switch m.MergeStrategy {
	case "latest":
		// Latest wins strategy: later values override earlier ones
		for _, values := range valuesList {
			result = util.CoalesceTables(result, values)
		}
	case "first":
		// First wins strategy: earlier values take precedence
		// We need to reverse the order and then use CoalesceTables
		for i := len(valuesList) - 1; i >= 0; i-- {
			result = util.CoalesceTables(result, valuesList[i])
		}
	case "merge":
		// Deep merge strategy: attempt to intelligently merge arrays and objects
		result = m.deepMergeValues(valuesList)
	default:
		return nil, fmt.Errorf("unknown merge strategy: %s", m.MergeStrategy)
	}

	return result, nil
}

// deepMergeValues performs a deep merge of values with intelligent conflict resolution
func (m *MergeValues) deepMergeValues(valuesList []map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})

	// Track which keys were set by which revision for debugging
	keySources := make(map[string]int)

	for i, values := range valuesList {
		for key, value := range values {
			if _, exists := result[key]; !exists {
				// Key doesn't exist yet, add it
				result[key] = m.deepCopyValue(value)
				keySources[key] = i + 1 // revision numbers start from 1
			} else {
				// Key exists, merge the values
				result[key] = m.mergeTwoValues(result[key], value)
				// Update source to latest revision that modified this key
				keySources[key] = i + 1
			}
		}
	}

	// Add merge metadata for debugging
	if len(result) > 0 {
		result["_mergeSources"] = keySources
	}

	return result
}

// mergeTwoValues merges two values intelligently
func (m *MergeValues) mergeTwoValues(existing, newValue interface{}) interface{} {
	switch existing := existing.(type) {
	case map[string]interface{}:
		// If existing is a map, try to merge with new value
		if newMap, ok := newValue.(map[string]interface{}); ok {
			result := make(map[string]interface{})
			// Copy existing values
			for k, v := range existing {
				result[k] = v
			}
			// Merge new values
			for k, v := range newMap {
				if existingV, exists := result[k]; exists {
					result[k] = m.mergeTwoValues(existingV, v)
				} else {
					result[k] = v
				}
			}
			return result
		}
		// Types don't match, new value wins
		return newValue
	case []interface{}:
		// If existing is a slice, append new values if they're also slices
		if newSlice, ok := newValue.([]interface{}); ok {
			// Concatenate slices, avoiding duplicates
			result := make([]interface{}, len(existing))
			copy(result, existing)
			for _, v := range newSlice {
				if !m.sliceContains(result, v) {
					result = append(result, v)
				}
			}
			return result
		}
		// Types don't match, new value wins
		return newValue
	default:
		// For primitive types, new value wins
		return newValue
	}
}

// sliceContains checks if a slice contains a specific value
func (m *MergeValues) sliceContains(slice []interface{}, value interface{}) bool {
	for _, v := range slice {
		if m.valuesEqual(v, value) {
			return true
		}
	}
	return false
}

// valuesEqual compares two values for equality
func (m *MergeValues) valuesEqual(a, b interface{}) bool {
	// Simple equality check - could be enhanced for more complex types
	return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
}

// deepCopyValue creates a deep copy of a value
func (m *MergeValues) deepCopyValue(value interface{}) interface{} {
	switch v := value.(type) {
	case map[string]interface{}:
		result := make(map[string]interface{})
		for key, val := range v {
			result[key] = m.deepCopyValue(val)
		}
		return result
	case []interface{}:
		result := make([]interface{}, len(v))
		for i, val := range v {
			result[i] = m.deepCopyValue(val)
		}
		return result
	default:
		return v
	}
}

// ParseRevisions parses a revision specification string into a slice of revision numbers
func ParseRevisions(revisionSpec string) ([]int, error) {
	if revisionSpec == "" {
		return nil, nil
	}

	var revisions []int
	parts := strings.Split(revisionSpec, ",")

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if strings.Contains(part, "..") {
			// Handle range specification like "1..5"
			rangeParts := strings.Split(part, "..")
			if len(rangeParts) != 2 {
				return nil, fmt.Errorf("invalid revision range: %s", part)
			}
			start, err := strconv.Atoi(strings.TrimSpace(rangeParts[0]))
			if err != nil {
				return nil, fmt.Errorf("invalid start revision in range %s: %w", part, err)
			}
			end, err := strconv.Atoi(strings.TrimSpace(rangeParts[1]))
			if err != nil {
				return nil, fmt.Errorf("invalid end revision in range %s: %w", part, err)
			}
			if start > end {
				return nil, fmt.Errorf("start revision %d cannot be greater than end revision %d", start, end)
			}
			for i := start; i <= end; i++ {
				revisions = append(revisions, i)
			}
		} else {
			// Handle single revision
			rev, err := strconv.Atoi(part)
			if err != nil {
				return nil, fmt.Errorf("invalid revision number %s: %w", part, err)
			}
			revisions = append(revisions, rev)
		}
	}

	return revisions, nil
}