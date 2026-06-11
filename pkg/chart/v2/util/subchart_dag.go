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
	// AnnotationDependsOnSubcharts is the Chart.yaml annotation key for declaring
	// parent-chart dependencies on subcharts. The value is a JSON string array of
	// subchart names (or aliases) that must be ready before parent resources are
	// installed.
	AnnotationDependsOnSubcharts = "helm.sh/depends-on/subcharts"
)

// BuildSubchartDAG constructs a DAG from a chart's subchart dependency declarations.
//
// Subchart-to-subchart ordering is read from the depends-on field on Chart.yaml
// dependency entries. The helm.sh/depends-on/subcharts metadata annotation is
// validated as the parent chart's dependencies on subcharts; parent resources
// are deployed after subchart batches by the action layer.
//
// Subcharts are keyed by their effective name (alias if set, otherwise name).
// ProcessDependencies must have been called on the chart first so that
// c.Dependencies() reflects the post-processed state (disabled subcharts
// pruned, aliases applied).
func BuildSubchartDAG(c *chart.Chart) (*DAG, error) {
	dag := NewDAG()

	if c == nil || c.Metadata == nil {
		return dag, nil
	}

	// Build the set of subcharts that survived ProcessDependencies.
	loaded := make(map[string]bool, len(c.Dependencies()))
	for _, sub := range c.Dependencies() {
		loaded[sub.Name()] = true
	}

	byName := make(map[string]bool, len(c.Metadata.Dependencies))
	for _, dep := range c.Metadata.Dependencies {
		if dep == nil {
			continue
		}
		name := effectiveDependencyName(dep)
		if loaded[name] {
			byName[name] = true
			dag.AddNode(name)
		}
	}

	for _, dep := range c.Metadata.Dependencies {
		if dep == nil {
			continue
		}
		name := effectiveDependencyName(dep)
		if !byName[name] {
			continue
		}

		for _, prerequisite := range dep.DependsOn {
			if err := addSubchartEdge(dag, byName, name, prerequisite); err != nil {
				return nil, err
			}
		}
	}

	if err := validateParentSubchartDependencies(c.Metadata.Annotations[AnnotationDependsOnSubcharts], byName); err != nil {
		return nil, err
	}

	return dag, nil
}

func validateParentSubchartDependencies(annotation string, byName map[string]bool) error {
	annotation = strings.TrimSpace(annotation)
	if annotation == "" {
		return nil
	}

	var prerequisites []string
	if err := json.Unmarshal([]byte(annotation), &prerequisites); err != nil {
		return fmt.Errorf("parsing %s annotation as JSON string array: %w", AnnotationDependsOnSubcharts, err)
	}

	for _, prerequisite := range prerequisites {
		if !byName[prerequisite] {
			return fmt.Errorf("annotation %s references unknown or disabled subchart %q", AnnotationDependsOnSubcharts, prerequisite)
		}
	}

	return nil
}

func addSubchartEdge(dag *DAG, byName map[string]bool, subchartName, prerequisite string) error {
	if !byName[prerequisite] {
		return fmt.Errorf("subchart %q depends-on unknown or disabled subchart %q", subchartName, prerequisite)
	}
	if err := dag.AddEdge(prerequisite, subchartName); err != nil {
		return fmt.Errorf("adding sequencing edge %s→%s: %w", prerequisite, subchartName, err)
	}
	return nil
}

func effectiveDependencyName(dep *chart.Dependency) string {
	if dep.Alias != "" {
		return dep.Alias
	}
	return dep.Name
}
