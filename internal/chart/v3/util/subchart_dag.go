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

	chart "helm.sh/helm/v4/internal/chart/v3"
)

const (
	// AnnotationDependsOnSubcharts declares which subcharts must be ready before
	// parent resources are installed. Its value is a JSON string array.
	AnnotationDependsOnSubcharts = "helm.sh/depends-on/subcharts"
)

// BuildSubchartDAG constructs a DAG from a chart's subchart dependency declarations.
//
// Subcharts are keyed by effective name (alias if set, otherwise name).
// ProcessDependencies must run first so disabled subcharts are pruned and
// references use effective names. Metadata from storage-decoded charts is
// trusted when the unexported loaded-dependency tree is absent.
func BuildSubchartDAG(c *chart.Chart) (*DAG, error) {
	dag := NewDAG()
	if c == nil || c.Metadata == nil {
		return dag, nil
	}

	loaded := make(map[string]bool, len(c.Dependencies()))
	for _, sub := range c.Dependencies() {
		loaded[sub.Name()] = true
	}
	trustMetadata := len(c.Dependencies()) == 0

	nodes := make(map[string]bool, len(c.Metadata.Dependencies))
	for _, dep := range c.Metadata.Dependencies {
		if dep == nil {
			continue
		}
		name := effectiveDependencyName(dep)
		trustedFromMetadata := trustMetadata && dep.Enabled
		if (!loaded[name] && !trustedFromMetadata) || nodes[name] {
			continue
		}
		nodes[name] = true
		dag.AddNode(name)
	}

	for _, dep := range c.Metadata.Dependencies {
		if dep == nil {
			continue
		}
		name := effectiveDependencyName(dep)
		if !nodes[name] {
			continue
		}
		for _, prerequisite := range dep.DependsOn {
			if !nodes[prerequisite] {
				return nil, fmt.Errorf("subchart %q depends-on unknown or disabled subchart %q", name, prerequisite)
			}
			if err := dag.AddEdge(prerequisite, name); err != nil {
				return nil, fmt.Errorf("adding sequencing edge %s→%s: %w", prerequisite, name, err)
			}
		}
	}

	if err := validateParentSubchartDependencies(c.Metadata.Annotations[AnnotationDependsOnSubcharts], nodes); err != nil {
		return nil, err
	}
	return dag, nil
}

// resolveDependsOnReferences rewrites dependency and parent annotation
// references from original chart names to effective names before aliases make
// the original names unrecoverable. Unknown references remain unchanged for
// BuildSubchartDAG to report.
func resolveDependsOnReferences(c *chart.Chart) error {
	refs := newSubchartRefs()
	for _, dep := range c.Metadata.Dependencies {
		if dep == nil {
			continue
		}
		effective := effectiveDependencyName(dep)
		refs.register(effective, effective)
		refs.register(dep.Name, effective)
	}

	for _, dep := range c.Metadata.Dependencies {
		if dep == nil {
			continue
		}
		for i, ref := range dep.DependsOn {
			effective, found, ambiguous := refs.resolve(ref)
			if ambiguous {
				return fmt.Errorf("subchart %q depends-on ambiguous subchart reference %q; reference it by alias to disambiguate", effectiveDependencyName(dep), ref)
			}
			if found {
				dep.DependsOn[i] = effective
			}
		}
	}

	return resolveAnnotationDependsOn(c, refs)
}

func resolveAnnotationDependsOn(c *chart.Chart, refs *subchartRefs) error {
	annotation := strings.TrimSpace(c.Metadata.Annotations[AnnotationDependsOnSubcharts])
	if annotation == "" {
		return nil
	}

	var prerequisites []string
	if err := json.Unmarshal([]byte(annotation), &prerequisites); err != nil {
		return nil
	}

	changed := false
	for i, ref := range prerequisites {
		effective, found, ambiguous := refs.resolve(ref)
		if ambiguous {
			return fmt.Errorf("annotation %s references ambiguous subchart %q; reference it by alias to disambiguate", AnnotationDependsOnSubcharts, ref)
		}
		if found && effective != ref {
			prerequisites[i] = effective
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

func (s *subchartRefs) register(ref, effective string) {
	if ref == "" {
		return
	}
	if existing, ok := s.byRef[ref]; ok && existing != effective {
		s.ambiguous[ref] = true
		return
	}
	s.byRef[ref] = effective
}

func (s *subchartRefs) resolve(ref string) (effective string, found, ambiguous bool) {
	if s.ambiguous[ref] {
		return "", false, true
	}
	effective, found = s.byRef[ref]
	return effective, found, false
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
