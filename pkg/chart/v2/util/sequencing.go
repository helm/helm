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
	"strings"

	chart "helm.sh/helm/v4/pkg/chart/v2"
)

const (
	// AnnotationDependsOnSubcharts is the Chart.yaml annotation key for
	// declaring subchart dependencies that must be deployed and ready
	// before the current chart's resources are installed.
	AnnotationDependsOnSubcharts = "helm.sh/depends-on/subcharts"

	// AnnotationResourceGroup is the template annotation key for declaring
	// the resource-group a resource belongs to.
	AnnotationResourceGroup = "helm.sh/resource-group"

	// AnnotationDependsOnResourceGroups is the template annotation key for
	// declaring resource-group dependencies.
	AnnotationDependsOnResourceGroups = "helm.sh/depends-on/resource-groups"
)

// ParseDependsOnSubcharts extracts the list of subchart names from the
// helm.sh/depends-on/subcharts annotation in Chart.yaml metadata.
// Returns nil if the annotation is not present.
func ParseDependsOnSubcharts(md *chart.Metadata) ([]string, error) {
	if md == nil || md.Annotations == nil {
		return nil, nil
	}
	raw, ok := md.Annotations[AnnotationDependsOnSubcharts]
	if !ok || raw == "" {
		return nil, nil
	}
	var names []string
	if err := json.Unmarshal([]byte(raw), &names); err != nil {
		return nil, fmt.Errorf("invalid %s annotation: %w", AnnotationDependsOnSubcharts, err)
	}
	return names, nil
}

// ResourceGroupAnnotation holds the parsed resource-group annotation value
// for a single rendered manifest document.
type ResourceGroupAnnotation struct {
	// Group is the resource-group name assigned to this resource.
	Group string
	// DependsOn is the list of resource-group names this group depends on.
	DependsOn []string
}

// ParseResourceGroupAnnotations extracts resource-group metadata from a rendered
// manifest document (single YAML document string). It reads:
//   - metadata.annotations["helm.sh/resource-group"]
//   - metadata.annotations["helm.sh/depends-on/resource-groups"] (JSON array)
//
// Returns nil if the resource has no resource-group annotation.
func ParseResourceGroupAnnotations(doc string) (*ResourceGroupAnnotation, error) {
	group := extractAnnotationValue(doc, AnnotationResourceGroup)
	if group == "" {
		return nil, nil
	}

	rga := &ResourceGroupAnnotation{Group: group}

	depsRaw := extractAnnotationValue(doc, AnnotationDependsOnResourceGroups)
	if depsRaw != "" {
		var deps []string
		if err := json.Unmarshal([]byte(depsRaw), &deps); err != nil {
			return nil, fmt.Errorf("invalid %s annotation: %w", AnnotationDependsOnResourceGroups, err)
		}
		rga.DependsOn = deps
	}

	return rga, nil
}

// extractAnnotationValue does a simple line-based scan for a YAML annotation
// key and returns its value. This avoids a full YAML parse for performance.
// It handles both quoted and unquoted values.
func extractAnnotationValue(doc string, key string) string {
	// Look for the annotation key in lines
	for line := range strings.SplitSeq(doc, "\n") {
		trimmed := strings.TrimSpace(line)
		// Check for key: value or "key": value patterns
		if prefix, ok := strings.CutPrefix(trimmed, key+":"); ok {
			return strings.Trim(strings.TrimSpace(prefix), "\"'")
		}
		// Also check for quoted key
		if prefix, ok := strings.CutPrefix(trimmed, "\""+key+"\":"); ok {
			return strings.Trim(strings.TrimSpace(prefix), "\"'")
		}
	}
	return ""
}

// BuildResourceGroupDAG constructs a dependency DAG for resource-groups found
// in a set of rendered manifest documents. Returns the DAG, a map of group name
// to manifest documents, the list of unsequenced documents, and any error.
//
// Emits warnings (returned as []string) for:
//   - References to non-existent groups
//   - Resources assigned to multiple groups (error)
func BuildResourceGroupDAG(docs []string) (*DAG, map[string][]string, []string, []string, error) {
	dag := NewDAG()
	grouped := make(map[string][]string) // group name → list of YAML docs
	var unsequenced []string
	var warnings []string

	// First pass: collect all groups and their documents
	knownGroups := make(map[string]bool)
	type docMeta struct {
		index int
		group string
		deps  []string
	}
	var metas []docMeta

	for i, doc := range docs {
		rga, err := ParseResourceGroupAnnotations(doc)
		if err != nil {
			return nil, nil, nil, nil, fmt.Errorf("document %d: %w", i, err)
		}
		if rga == nil {
			unsequenced = append(unsequenced, doc)
			metas = append(metas, docMeta{index: i})
			continue
		}
		knownGroups[rga.Group] = true
		grouped[rga.Group] = append(grouped[rga.Group], doc)
		metas = append(metas, docMeta{index: i, group: rga.Group, deps: rga.DependsOn})
	}

	// Second pass: build DAG edges
	for _, m := range metas {
		if m.group == "" {
			continue
		}
		dag.AddNode(m.group)
		for _, dep := range m.deps {
			if !knownGroups[dep] {
				warnings = append(warnings, fmt.Sprintf(
					"resource-group %q depends on %q, but group %q does not exist in rendered manifests",
					m.group, dep, dep,
				))
				continue
			}
			if err := dag.AddEdge(dep, m.group); err != nil {
				return nil, nil, nil, nil, err
			}
		}
	}

	if err := dag.DetectCycles(); err != nil {
		return nil, nil, nil, nil, err
	}

	return dag, grouped, unsequenced, warnings, nil
}

// BuildSubchartDAG constructs a dependency DAG for subcharts of the given chart.
// It combines dependencies declared via:
//   - Dependency.DependsOn field entries in Chart.yaml dependencies
//   - Metadata.Annotations["helm.sh/depends-on/subcharts"] for the parent chart
//
// Returns the DAG and any validation error (cycles, unknown references).
func BuildSubchartDAG(c *chart.Chart) (*DAG, error) {
	dag := NewDAG()

	if c.Metadata == nil {
		return dag, nil
	}

	// Build a lookup of known dependency names/aliases
	depNames := make(map[string]bool)
	for _, dep := range c.Metadata.Dependencies {
		key := dep.Name
		if dep.Alias != "" {
			key = dep.Alias
		}
		depNames[key] = true
		dag.AddNode(key)
	}

	// Add the parent chart node
	dag.AddNode(c.Metadata.Name)

	// Process Dependency.DependsOn field entries
	for _, dep := range c.Metadata.Dependencies {
		key := dep.Name
		if dep.Alias != "" {
			key = dep.Alias
		}
		for _, upstream := range dep.DependsOn {
			if !depNames[upstream] {
				return nil, fmt.Errorf(
					"dependency %q declares depends-on %q, but %q is not a known dependency",
					key, upstream, upstream,
				)
			}
			// upstream must be ready before key can be installed
			if err := dag.AddEdge(upstream, key); err != nil {
				return nil, err
			}
		}
	}

	// Process helm.sh/depends-on/subcharts annotation on parent chart
	annotationDeps, err := ParseDependsOnSubcharts(c.Metadata)
	if err != nil {
		return nil, err
	}
	for _, upstream := range annotationDeps {
		if !depNames[upstream] {
			return nil, fmt.Errorf(
				"chart %q annotation %s references %q, but %q is not a known dependency",
				c.Metadata.Name, AnnotationDependsOnSubcharts, upstream, upstream,
			)
		}
		// upstream must be ready before parent chart resources
		if err := dag.AddEdge(upstream, c.Metadata.Name); err != nil {
			return nil, err
		}
	}

	// Validate no cycles
	if err := dag.DetectCycles(); err != nil {
		return nil, err
	}

	return dag, nil
}
