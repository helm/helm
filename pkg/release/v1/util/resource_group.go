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

package util

import (
	"encoding/json"
	"fmt"

	chartutil "helm.sh/helm/v4/pkg/chart/v2/util"
)

const (
	// AnnotationResourceGroup declares the resource-group a resource belongs to.
	// Value is a single group name string.
	//
	// Example: helm.sh/resource-group: "database"
	AnnotationResourceGroup = "helm.sh/resource-group"

	// AnnotationDependsOnResourceGroups declares which resource-groups must be
	// deployed before this resource's group. Value is a JSON array of group names.
	//
	// Example: helm.sh/depends-on/resource-groups: '["database", "queue"]'
	AnnotationDependsOnResourceGroups = "helm.sh/depends-on/resource-groups"
)

// ResourceGroupResult holds the output of ParseResourceGroups.
type ResourceGroupResult struct {
	// Groups maps group name → list of manifests belonging to that group.
	Groups map[string][]Manifest

	// GroupDeps maps group name → list of group names it depends on.
	GroupDeps map[string][]string

	// Unsequenced holds manifests that have no valid group assignment.
	// These are deployed after all sequenced groups.
	Unsequenced []Manifest
}

// ParseResourceGroups scans a slice of rendered manifests, extracts
// helm.sh/resource-group and helm.sh/depends-on/resource-groups annotations,
// and returns a ResourceGroupResult.
//
// Manifests with no annotations are placed in Unsequenced.
// Manifests referencing non-existent groups via depends-on are moved to
// Unsequenced and a warning string is returned.
//
// Groups with valid annotations but no depends-on edges are root nodes (batch 0).
func ParseResourceGroups(manifests []Manifest) (ResourceGroupResult, []string) {
	result := ResourceGroupResult{
		Groups:    make(map[string][]Manifest),
		GroupDeps: make(map[string][]string),
	}
	var warnings []string

	type pendingDep struct {
		groupName string
		manifest  Manifest
		deps      []string
	}
	var pending []pendingDep

	for _, m := range manifests {
		if m.Head == nil || m.Head.Metadata == nil || len(m.Head.Metadata.Annotations) == 0 {
			result.Unsequenced = append(result.Unsequenced, m)
			continue
		}

		groupName, hasGroup := m.Head.Metadata.Annotations[AnnotationResourceGroup]
		if !hasGroup || groupName == "" {
			result.Unsequenced = append(result.Unsequenced, m)
			continue
		}

		// Parse optional depends-on
		var deps []string
		if rawDeps, ok := m.Head.Metadata.Annotations[AnnotationDependsOnResourceGroups]; ok {
			if err := json.Unmarshal([]byte(rawDeps), &deps); err != nil {
				warnings = append(warnings, fmt.Sprintf(
					"manifest %q: invalid JSON in %s annotation: %v; moving to unsequenced batch",
					m.Name, AnnotationDependsOnResourceGroups, err,
				))
				result.Unsequenced = append(result.Unsequenced, m)
				continue
			}
		}

		result.Groups[groupName] = append(result.Groups[groupName], m)
		pending = append(pending, pendingDep{groupName: groupName, manifest: m, deps: deps})
	}

	// Now process depends-on edges; validate that referenced groups exist.
	// We need all group names known first (already populated above).
	for _, p := range pending {
		for _, dep := range p.deps {
			if _, ok := result.Groups[dep]; !ok {
				warnings = append(warnings, fmt.Sprintf(
					"group %q in manifest %q depends-on non-existent group %q; moving %q to unsequenced batch",
					p.groupName, p.manifest.Name, dep, p.groupName,
				))
				// Move all manifests of this group to unsequenced.
				result.Unsequenced = append(result.Unsequenced, result.Groups[p.groupName]...)
				delete(result.Groups, p.groupName)
				delete(result.GroupDeps, p.groupName)
				goto nextPending
			}
		}
		// Only record deps if we haven't already moved this group to unsequenced.
		if _, ok := result.Groups[p.groupName]; ok {
			if len(p.deps) > 0 {
				existing := result.GroupDeps[p.groupName]
				result.GroupDeps[p.groupName] = append(existing, p.deps...)
			}
		}
	nextPending:
	}

	return result, warnings
}

// BuildResourceGroupDAG constructs a DAG from the resource-group parse result.
// Each group name becomes a DAG node; GroupDeps entries become edges.
func BuildResourceGroupDAG(r ResourceGroupResult) (*chartutil.DAG, error) {
	d := chartutil.NewDAG()
	for groupName := range r.Groups {
		d.AddNode(groupName)
	}
	for groupName, deps := range r.GroupDeps {
		for _, dep := range deps {
			if err := d.AddEdge(dep, groupName); err != nil {
				return nil, fmt.Errorf("resource-group sequencing edge %s→%s: %w", dep, groupName, err)
			}
		}
	}
	return d, nil
}
