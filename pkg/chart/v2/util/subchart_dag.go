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
// ProcessDependencies must have been called on the chart first: it prunes
// disabled subcharts, applies alias renames, and rewrites depends-on
// references — dependency depends-on entries and the
// helm.sh/depends-on/subcharts annotation — to effective names (see
// resolveDependsOnReferences). BuildSubchartDAG therefore resolves references
// against effective names only. This holds for charts decoded from release
// storage too: they were processed before being stored, so their persisted
// depends-on references already use effective names.
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

	// Each loaded subchart becomes a DAG node keyed by its effective name.
	nodes := make(map[string]bool, len(c.Metadata.Dependencies))
	for _, dep := range c.Metadata.Dependencies {
		if dep == nil {
			continue
		}
		eff := effectiveDependencyName(dep)
		if !loaded[eff] || nodes[eff] {
			continue
		}
		nodes[eff] = true
		dag.AddNode(eff)
	}

	for _, dep := range c.Metadata.Dependencies {
		if dep == nil {
			continue
		}
		eff := effectiveDependencyName(dep)
		if !nodes[eff] {
			continue
		}

		for _, prerequisite := range dep.DependsOn {
			if !nodes[prerequisite] {
				return nil, fmt.Errorf("subchart %q depends-on unknown or disabled subchart %q", eff, prerequisite)
			}
			if err := dag.AddEdge(prerequisite, eff); err != nil {
				return nil, fmt.Errorf("adding sequencing edge %s→%s: %w", prerequisite, eff, err)
			}
		}
	}

	if err := validateParentSubchartDependencies(c.Metadata.Annotations[AnnotationDependsOnSubcharts], nodes); err != nil {
		return nil, err
	}

	return dag, nil
}

// resolveDependsOnReferences rewrites depends-on references — Chart.yaml
// dependency depends-on entries and the helm.sh/depends-on/subcharts
// annotation — from a subchart's original chart name to its effective name
// (alias if set, otherwise name). It must run while original names are still
// present in c.Metadata.Dependencies, i.e. before processDependencyEnabled
// applies its Name = Alias rewrite; that rewrite makes original names
// unrecoverable. The rewritten references are what release storage persists
// and what BuildSubchartDAG consumes, including at uninstall time when the
// chart is decoded from the release record.
//
// A reference matching more than one subchart (e.g. the original name of a
// chart pulled in under two aliases) is rejected as ambiguous. References
// matching no dependency are left unchanged so BuildSubchartDAG can report
// them as unknown or disabled, and a malformed annotation is left for
// BuildSubchartDAG's JSON parse error. Resolution is idempotent: effective
// names resolve to themselves, so reprocessing an already-processed chart is
// a no-op.
func resolveDependsOnReferences(c *chart.Chart) error {
	refs := newSubchartRefs()
	for _, dep := range c.Metadata.Dependencies {
		if dep == nil {
			continue
		}
		eff := effectiveDependencyName(dep)
		refs.register(eff, eff)
		refs.register(dep.Name, eff)
	}

	for _, dep := range c.Metadata.Dependencies {
		if dep == nil {
			continue
		}
		for i, ref := range dep.DependsOn {
			eff, found, isAmbiguous := refs.resolve(ref)
			if isAmbiguous {
				return fmt.Errorf("subchart %q depends-on ambiguous subchart reference %q; reference it by alias to disambiguate", effectiveDependencyName(dep), ref)
			}
			if found {
				dep.DependsOn[i] = eff
			}
		}
	}

	return resolveAnnotationDependsOn(c, refs)
}

// resolveAnnotationDependsOn rewrites the parent chart's
// helm.sh/depends-on/subcharts annotation entries to effective subchart names.
func resolveAnnotationDependsOn(c *chart.Chart, refs *subchartRefs) error {
	annotation := strings.TrimSpace(c.Metadata.Annotations[AnnotationDependsOnSubcharts])
	if annotation == "" {
		return nil
	}

	var prerequisites []string
	if err := json.Unmarshal([]byte(annotation), &prerequisites); err != nil {
		// Malformed JSON is reported by BuildSubchartDAG with full context.
		return nil
	}

	changed := false
	for i, ref := range prerequisites {
		eff, found, isAmbiguous := refs.resolve(ref)
		if isAmbiguous {
			return fmt.Errorf("annotation %s references ambiguous subchart %q; reference it by alias to disambiguate", AnnotationDependsOnSubcharts, ref)
		}
		if found && eff != ref {
			prerequisites[i] = eff
			changed = true
		}
	}
	if !changed {
		return nil
	}

	encoded, err := json.Marshal(prerequisites)
	if err != nil {
		return fmt.Errorf("re-encoding %s annotation: %w", AnnotationDependsOnSubcharts, err)
	}
	c.Metadata.Annotations[AnnotationDependsOnSubcharts] = string(encoded)
	return nil
}

// subchartRefs resolves depends-on references to effective subchart names
// while processing dependencies, i.e. while original chart names are still
// present. A reference may be a subchart's effective name (alias if set,
// otherwise name) or its original chart name. A reference that maps to more
// than one subchart is recorded as ambiguous and rejected on use, rather than
// being silently resolved to one of them.
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

func validateParentSubchartDependencies(annotation string, nodes map[string]bool) error {
	annotation = strings.TrimSpace(annotation)
	if annotation == "" {
		return nil
	}

	var prerequisites []string
	if err := json.Unmarshal([]byte(annotation), &prerequisites); err != nil {
		return fmt.Errorf("parsing %s annotation as JSON string array: %w", AnnotationDependsOnSubcharts, err)
	}

	for _, prerequisite := range prerequisites {
		if !nodes[prerequisite] {
			return fmt.Errorf("annotation %s references unknown or disabled subchart %q", AnnotationDependsOnSubcharts, prerequisite)
		}
	}

	return nil
}

func effectiveDependencyName(dep *chart.Dependency) string {
	if dep.Alias != "" {
		return dep.Alias
	}
	return dep.Name
}
