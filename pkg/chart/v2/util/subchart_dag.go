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
// Subcharts are keyed by their effective name (alias if set, otherwise name). A
// depends-on reference may use that effective name or the subchart's original
// name when an alias is set; a reference that matches more than one subchart is
// rejected as ambiguous. ProcessDependencies must have been called on the chart
// first so that c.Dependencies() reflects the post-processed state (disabled
// subcharts pruned, aliases applied).
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

	// Resolve depends-on references by effective name or, when an alias is set,
	// by the subchart's original name. Each loaded subchart becomes a DAG node
	// keyed by its effective name.
	refs := newSubchartRefs()
	for _, dep := range c.Metadata.Dependencies {
		if dep == nil {
			continue
		}
		eff := effectiveDependencyName(dep)
		if !loaded[eff] {
			continue
		}
		if !refs.has(eff) {
			dag.AddNode(eff)
		}
		refs.register(eff, eff)
		refs.register(dep.Name, eff)
	}

	for _, dep := range c.Metadata.Dependencies {
		if dep == nil {
			continue
		}
		eff := effectiveDependencyName(dep)
		if !loaded[eff] {
			continue
		}

		for _, prerequisite := range dep.DependsOn {
			if err := addSubchartEdge(dag, refs, eff, prerequisite); err != nil {
				return nil, err
			}
		}
	}

	if err := validateParentSubchartDependencies(c.Metadata.Annotations[AnnotationDependsOnSubcharts], refs); err != nil {
		return nil, err
	}

	return dag, nil
}

// subchartRefs resolves depends-on references to effective subchart node names.
// A reference may be a subchart's effective name (alias if set, otherwise name)
// or its original name when an alias is set. A reference that maps to more than
// one subchart is recorded as ambiguous and rejected on use, rather than being
// silently resolved to one of them.
type subchartRefs struct {
	byRef     map[string]string
	ambiguous map[string]bool
}

func newSubchartRefs() *subchartRefs {
	return &subchartRefs{
		byRef:     make(map[string]string),
		ambiguous: make(map[string]bool),
	}
}

// register maps ref to the effective subchart name eff. If ref already resolves
// to a different subchart, it is flagged ambiguous.
func (s *subchartRefs) register(ref, eff string) {
	if ref == "" {
		return
	}
	if existing, ok := s.byRef[ref]; ok && existing != eff {
		s.ambiguous[ref] = true
		return
	}
	s.byRef[ref] = eff
}

// resolve returns the effective subchart name for ref. found is false for an
// unknown reference; isAmbiguous is true when ref maps to multiple subcharts.
func (s *subchartRefs) resolve(ref string) (eff string, found, isAmbiguous bool) {
	if s.ambiguous[ref] {
		return "", false, true
	}
	eff, found = s.byRef[ref]
	return eff, found, false
}

// has reports whether ref resolves to a known, unambiguous subchart.
func (s *subchartRefs) has(ref string) bool {
	_, found, _ := s.resolve(ref)
	return found
}

func validateParentSubchartDependencies(annotation string, refs *subchartRefs) error {
	annotation = strings.TrimSpace(annotation)
	if annotation == "" {
		return nil
	}

	var prerequisites []string
	if err := json.Unmarshal([]byte(annotation), &prerequisites); err != nil {
		return fmt.Errorf("parsing %s annotation as JSON string array: %w", AnnotationDependsOnSubcharts, err)
	}

	for _, prerequisite := range prerequisites {
		_, found, isAmbiguous := refs.resolve(prerequisite)
		if isAmbiguous {
			return fmt.Errorf("annotation %s references ambiguous subchart %q; reference it by alias to disambiguate", AnnotationDependsOnSubcharts, prerequisite)
		}
		if !found {
			return fmt.Errorf("annotation %s references unknown or disabled subchart %q", AnnotationDependsOnSubcharts, prerequisite)
		}
	}

	return nil
}

func addSubchartEdge(dag *DAG, refs *subchartRefs, subchartName, prerequisite string) error {
	eff, found, isAmbiguous := refs.resolve(prerequisite)
	if isAmbiguous {
		return fmt.Errorf("subchart %q depends-on ambiguous subchart reference %q; reference it by alias to disambiguate", subchartName, prerequisite)
	}
	if !found {
		return fmt.Errorf("subchart %q depends-on unknown or disabled subchart %q", subchartName, prerequisite)
	}
	if err := dag.AddEdge(eff, subchartName); err != nil {
		return fmt.Errorf("adding sequencing edge %s→%s: %w", eff, subchartName, err)
	}
	return nil
}

func effectiveDependencyName(dep *chart.Dependency) string {
	if dep.Alias != "" {
		return dep.Alias
	}
	return dep.Name
}
