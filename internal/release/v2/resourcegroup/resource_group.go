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

package resourcegroup

import (
	"encoding/json"
	"fmt"

	chartutil "helm.sh/helm/v4/internal/chart/v3/util"
	releasev2manifest "helm.sh/helm/v4/internal/release/v2/manifest"
)

const (
	// AnnotationResourceGroup declares the resource group a resource belongs to.
	AnnotationResourceGroup = "helm.sh/resource-group"

	// AnnotationDependsOnResourceGroups declares prerequisite resource groups for a
	// resource's group as a JSON string array.
	AnnotationDependsOnResourceGroups = "helm.sh/depends-on-resource-groups"
)

// ResourceGroupResult holds the output of ParseResourceGroups.
type ResourceGroupResult struct { //nolint:revive // Retain the accepted parser seam after moving to the leaf package.
	// Groups maps group names to manifests assigned to those groups.
	Groups map[string][]releasev2manifest.Manifest

	// GroupDeps maps group names to the group names they depend on.
	GroupDeps map[string][]string

	// Unsequenced contains manifests that should be deployed outside the DAG.
	Unsequenced []releasev2manifest.Manifest
}

// ParseResourceGroups extracts resource-group annotations from rendered
// manifests and partitions them into sequenced groups and unsequenced manifests.
//
// Manifests without a resource-group annotation are treated as unsequenced.
// Invalid depends-on annotations emit a warning and demote that manifest to the
// unsequenced batch. References to unknown groups emit a warning and demote the
// entire referencing group to the unsequenced batch. If the same resource is
// assigned to different groups, ParseResourceGroups returns an error.
func ParseResourceGroups(manifests []releasev2manifest.Manifest) (ResourceGroupResult, []string, error) {
	result := ResourceGroupResult{
		Groups:    make(map[string][]releasev2manifest.Manifest),
		GroupDeps: make(map[string][]string),
	}
	var warnings []string

	resourceAssignments := make(map[string]string)
	groupDeps := make(map[string][]string)
	groupOrder := make([]string, 0)

	for _, manifest := range manifests {
		groupName, hasGroup := resourceGroupName(manifest)
		if !hasGroup {
			result.Unsequenced = append(result.Unsequenced, manifest)
			continue
		}

		resourceID := resourceGroupResourceID(manifest)
		if existingGroup, ok := resourceAssignments[resourceID]; ok && existingGroup != groupName {
			return ResourceGroupResult{}, warnings, fmt.Errorf(
				"resource %q assigned to multiple resource groups %q and %q",
				resourceID,
				existingGroup,
				groupName,
			)
		}
		resourceAssignments[resourceID] = groupName

		deps, warning, err := resourceGroupDependencies(manifest)
		if err != nil {
			warnings = append(warnings, warning)
			result.Unsequenced = append(result.Unsequenced, manifest)
			continue
		}

		if _, ok := result.Groups[groupName]; !ok {
			groupOrder = append(groupOrder, groupName)
		}

		result.Groups[groupName] = append(result.Groups[groupName], manifest)
		groupDeps[groupName] = appendUniqueStrings(groupDeps[groupName], deps...)
	}

	for changed := true; changed; {
		changed = false

		for _, groupName := range groupOrder {
			groupManifests, ok := result.Groups[groupName]
			if !ok {
				continue
			}

			missingDep, hasMissing := firstMissingDependency(result.Groups, groupDeps[groupName])
			if !hasMissing {
				continue
			}

			warnings = append(warnings, fmt.Sprintf(
				"group %q depends-on non-existent group %q; moving group to unsequenced batch",
				groupName,
				missingDep,
			))
			result.Unsequenced = append(result.Unsequenced, groupManifests...)
			delete(result.Groups, groupName)
			changed = true
		}
	}

	for _, groupName := range groupOrder {
		if _, ok := result.Groups[groupName]; !ok {
			continue
		}
		result.GroupDeps[groupName] = groupDeps[groupName]
	}

	return result, warnings, nil
}

// BuildResourceGroupDAG constructs a DAG from the resource-group parse result.
// Each group becomes a DAG node and every dependency becomes an edge.
func BuildResourceGroupDAG(result ResourceGroupResult) (*chartutil.DAG, error) {
	dag := chartutil.NewDAG()

	for groupName := range result.Groups {
		dag.AddNode(groupName)
	}

	for groupName, deps := range result.GroupDeps {
		for _, dep := range deps {
			if err := dag.AddEdge(dep, groupName); err != nil {
				return nil, fmt.Errorf("resource-group sequencing edge %s→%s: %w", dep, groupName, err)
			}
		}
	}

	return dag, nil
}

func resourceGroupName(manifest releasev2manifest.Manifest) (string, bool) {
	if manifest.Head == nil || manifest.Head.Metadata == nil || len(manifest.Head.Metadata.Annotations) == 0 {
		return "", false
	}

	groupName, ok := manifest.Head.Metadata.Annotations[AnnotationResourceGroup]
	if !ok || groupName == "" {
		return "", false
	}
	return groupName, true
}

func resourceGroupDependencies(manifest releasev2manifest.Manifest) ([]string, string, error) {
	annotations := manifest.Head.Metadata.Annotations
	rawDeps, ok := annotations[AnnotationDependsOnResourceGroups]
	if !ok {
		return nil, "", nil
	}

	var deps []string
	if err := json.Unmarshal([]byte(rawDeps), &deps); err != nil {
		return nil, fmt.Sprintf(
			"manifest %q: invalid JSON in %s annotation: %v; moving to unsequenced batch",
			manifest.Name,
			AnnotationDependsOnResourceGroups,
			err,
		), err
	}

	return deps, "", nil
}

// firstMissingDependency returns the first dependency that does not name an
// existing group, and whether one was found. The boolean disambiguates an
// empty-string dependency from no missing dependency.
func firstMissingDependency(groups map[string][]releasev2manifest.Manifest, deps []string) (string, bool) {
	for _, dep := range deps {
		if _, ok := groups[dep]; !ok {
			return dep, true
		}
	}
	return "", false
}

func appendUniqueStrings(existing []string, values ...string) []string {
	seen := make(map[string]struct{}, len(existing))
	for _, value := range existing {
		seen[value] = struct{}{}
	}

	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		existing = append(existing, value)
		seen[value] = struct{}{}
	}
	return existing
}

func resourceGroupResourceID(manifest releasev2manifest.Manifest) string {
	if manifest.Head == nil || manifest.Head.Metadata == nil || manifest.Head.Metadata.Name == "" {
		return manifest.Name
	}

	return fmt.Sprintf("%s/%s/%s/%s", manifest.Head.Version, manifest.Head.Kind, manifest.Head.Metadata.Namespace, manifest.Head.Metadata.Name)
}
