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
	"log/slog"

	chart "helm.sh/helm/v4/pkg/chart/v2"
)

const (
	// AnnotationDependsOnSubcharts is the Chart.yaml annotation key for declaring
	// subchart deployment ordering. The value is a JSON object mapping subchart
	// names (or aliases) to lists of prerequisite subcharts.
	AnnotationDependsOnSubcharts = "helm.sh/depends-on/subcharts"
)

type subchartInfo struct {
	disabled bool
}

// BuildSubchartDAG constructs a DAG from a chart's subchart dependency declarations.
//
// Ordering is read from:
//   - the depends-on field on Chart.yaml dependency entries
//   - the helm.sh/depends-on/subcharts metadata annotation
//
// Subcharts are keyed by their effective name (alias if set, otherwise name).
// Disabled subcharts are excluded from the DAG, and dependencies pointing at
// disabled subcharts are ignored.
func BuildSubchartDAG(c *chart.Chart) (*DAG, error) {
	dag := NewDAG()

	if c == nil || c.Metadata == nil || len(c.Metadata.Dependencies) == 0 {
		return dag, nil
	}

	byName := make(map[string]subchartInfo, len(c.Metadata.Dependencies))
	for _, dep := range c.Metadata.Dependencies {
		name := effectiveDependencyName(dep)
		disabled := !dep.Enabled && dep.Condition == "" && len(dep.Tags) == 0
		byName[name] = subchartInfo{disabled: disabled}
		if !disabled {
			dag.AddNode(name)
		}
	}

	for _, dep := range c.Metadata.Dependencies {
		name := effectiveDependencyName(dep)
		if byName[name].disabled {
			continue
		}

		for _, prerequisite := range dep.DependsOn {
			if err := addSubchartEdge(dag, byName, name, prerequisite); err != nil {
				return nil, err
			}
		}
	}

	annotation := c.Metadata.Annotations[AnnotationDependsOnSubcharts]
	if annotation == "" {
		return dag, nil
	}

	var declared map[string][]string
	if err := json.Unmarshal([]byte(annotation), &declared); err != nil {
		return nil, fmt.Errorf("parsing %s annotation: %w", AnnotationDependsOnSubcharts, err)
	}

	for subchartName, prerequisites := range declared {
		info, ok := byName[subchartName]
		if !ok {
			return nil, fmt.Errorf("annotation %s references unknown subchart %q", AnnotationDependsOnSubcharts, subchartName)
		}
		if info.disabled {
			slog.Info("skipping annotation dependency for disabled subchart", "subchart", subchartName)
			continue
		}

		for _, prerequisite := range prerequisites {
			if err := addSubchartEdge(dag, byName, subchartName, prerequisite); err != nil {
				return nil, err
			}
		}
	}

	return dag, nil
}

func addSubchartEdge(dag *DAG, byName map[string]subchartInfo, subchartName, prerequisite string) error {
	info, ok := byName[prerequisite]
	if !ok {
		return fmt.Errorf("subchart %q depends-on unknown subchart %q", subchartName, prerequisite)
	}
	if info.disabled {
		slog.Info("ignoring dependency on disabled subchart", "subchart", subchartName, "disabledPrerequisite", prerequisite)
		return nil
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
