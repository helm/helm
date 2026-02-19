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
	// names (or aliases) to lists of their prerequisites.
	//
	// Example:
	//   annotations:
	//     helm.sh/depends-on/subcharts: '{"nginx": ["postgres", "redis"]}'
	AnnotationDependsOnSubcharts = "helm.sh/depends-on/subcharts"
)

// depInfo holds metadata about a dependency for subchart DAG construction.
type depInfo struct {
	dep      *chart.Dependency
	disabled bool
}

// BuildSubchartDAG constructs a DAG from a chart's subchart dependency declarations.
//
// Dependency ordering is read from two sources:
//  1. The `depends-on` field on each entry in Chart.yaml dependencies.
//  2. The `helm.sh/depends-on/subcharts` annotation on the Chart.yaml metadata.
//
// Subcharts are identified by their effective name (alias if set, otherwise name).
// When a referenced subchart is disabled (Enabled == false), the dependency edge
// is silently removed and an info-level log is emitted, since a disabled chart
// produces no resources. A reference to a truly non-existent subchart name is
// an error.
//
// Returns the constructed DAG, ready to call GetBatches() on.
func BuildSubchartDAG(c *chart.Chart) (*DAG, error) {
	d := NewDAG()

	if c.Metadata == nil || len(c.Metadata.Dependencies) == 0 {
		return d, nil
	}

	// Build a map of effective-name → Dependency for quick lookup.
	// effective name = alias if set, otherwise name.
	// Track disabled subcharts separately — they are valid names but produce no resources.
	byName := make(map[string]*depInfo)
	for _, dep := range c.Metadata.Dependencies {
		effective := dep.Name
		if dep.Alias != "" {
			effective = dep.Alias
		}
		// A dependency is disabled when Enabled is false AND there's no condition or
		// tag that could re-enable it at runtime. Conditions and tags are evaluated
		// at install-time from user-supplied values, so we can't resolve them here.
		// If Enabled is false but a condition/tag is set, treat as potentially enabled.
		disabled := !dep.Enabled && dep.Condition == "" && len(dep.Tags) == 0
		byName[effective] = &depInfo{dep: dep, disabled: disabled}
	}

	// Register all non-disabled subcharts as DAG nodes.
	for name, info := range byName {
		if !info.disabled {
			d.AddNode(name)
		}
	}

	// Process DependsOn fields from each dependency.
	for _, dep := range c.Metadata.Dependencies {
		effective := dep.Name
		if dep.Alias != "" {
			effective = dep.Alias
		}
		if info := byName[effective]; info != nil && info.disabled {
			continue // skip disabled subcharts entirely
		}
		for _, prereq := range dep.DependsOn {
			if err := addSubchartEdge(d, byName, effective, prereq); err != nil {
				return nil, err
			}
		}
	}

	// Process helm.sh/depends-on/subcharts annotation.
	if c.Metadata.Annotations != nil {
		if raw, ok := c.Metadata.Annotations[AnnotationDependsOnSubcharts]; ok && raw != "" {
			var annotationDeps map[string][]string
			if err := json.Unmarshal([]byte(raw), &annotationDeps); err != nil {
				return nil, fmt.Errorf("parsing %s annotation: %w", AnnotationDependsOnSubcharts, err)
			}
			for subchart, prereqs := range annotationDeps {
				if info := byName[subchart]; info == nil {
					return nil, fmt.Errorf("annotation %s references unknown subchart %q", AnnotationDependsOnSubcharts, subchart)
				} else if info.disabled {
					slog.Info("skipping annotation dependency for disabled subchart", "subchart", subchart)
					continue
				}
				for _, prereq := range prereqs {
					if err := addSubchartEdge(d, byName, subchart, prereq); err != nil {
						return nil, err
					}
				}
			}
		}
	}

	return d, nil
}

// addSubchartEdge adds an edge prereq→subchart to the DAG, handling disabled prereqs.
func addSubchartEdge(d *DAG, byName map[string]*depInfo, subchart, prereq string) error {
	info, ok := byName[prereq]
	if !ok {
		return fmt.Errorf("subchart %q depends-on unknown subchart %q", subchart, prereq)
	}
	if info.disabled {
		slog.Info("ignoring dependency on disabled subchart", "subchart", subchart, "disabledPrereq", prereq)
		return nil
	}
	if err := d.AddEdge(prereq, subchart); err != nil {
		return fmt.Errorf("adding sequencing edge %s→%s: %w", prereq, subchart, err)
	}
	return nil
}
