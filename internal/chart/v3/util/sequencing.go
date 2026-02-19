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

	chart "helm.sh/helm/v4/internal/chart/v3"
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
		if err := dag.AddEdge(upstream, c.Metadata.Name); err != nil {
			return nil, err
		}
	}

	if err := dag.DetectCycles(); err != nil {
		return nil, err
	}

	return dag, nil
}
