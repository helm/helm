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

package sequence

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	chart "helm.sh/helm/v4/pkg/chart/v2"
	chartutil "helm.sh/helm/v4/pkg/chart/v2/util"
	releaseutil "helm.sh/helm/v4/pkg/release/v1/util"
)

type builder struct {
	plan *Plan
}

// GroupManifestsByDirectSubchart groups manifests by the direct subchart they belong to.
// chartPath is the full path-prefix for the current chart level — at the top level
// it is the chart name (e.g. "parent"); at deeper recursion levels it is the joined
// path through "/charts/" segments (e.g. "parent/charts/sub").
// The current chart level's own manifests are returned under the empty string key "".
// Direct subcharts are keyed by their immediate directory name under
// "<chartPath>/charts/<subchart>/". Nested grandchildren are grouped under their
// direct subchart parent ("sub"), since nested sequencing is handled recursively.
func GroupManifestsByDirectSubchart(manifests []releaseutil.Manifest, chartPath string) map[string][]releaseutil.Manifest {
	result := make(map[string][]releaseutil.Manifest)
	if chartPath == "" {
		result[""] = append(result[""], manifests...)
		return result
	}

	chartsPrefix := chartPath + "/charts/"
	for _, m := range manifests {
		if !strings.HasPrefix(m.Name, chartsPrefix) {
			result[""] = append(result[""], m)
			continue
		}
		rest := m.Name[len(chartsPrefix):]
		subchartName, _, ok := strings.Cut(rest, "/")
		if !ok {
			result[""] = append(result[""], m)
			continue
		}
		result[subchartName] = append(result[subchartName], m)
	}
	return result
}

// FindSubchart resolves a direct dependency by effective name (alias if set,
// else name) — the single home for the chart-walker subchart lookup. It uses
// the parent chart's Metadata.Dependencies to resolve aliases, since the alias
// is stored on the Dependency struct in Chart.yaml, not on the subchart's own
// Metadata.
func FindSubchart(chrt *chart.Chart, nameOrAlias string) *chart.Chart {
	if chrt == nil {
		return nil
	}
	aliasMap := make(map[string]string) // chart name → effective name (alias or name)
	if chrt.Metadata != nil {
		for _, dep := range chrt.Metadata.Dependencies {
			if dep == nil {
				continue
			}
			effective := dep.Name
			if dep.Alias != "" {
				effective = dep.Alias
			}
			aliasMap[dep.Name] = effective
		}
	}
	for _, dep := range chrt.Dependencies() {
		effective := dep.Name()
		if alias, ok := aliasMap[dep.Name()]; ok {
			effective = alias
		}
		if effective == nameOrAlias || dep.Name() == nameOrAlias {
			return dep
		}
	}
	return nil
}

// Build constructs the deployment plan for chrt's rendered manifests.
//
// Preconditions: manifests are hook-free (SortManifests output or a stored
// rel.Manifest — hooks are stored separately) and chrt has been through
// ProcessDependencies (true for both freshly-loaded and storage-decoded
// charts, per BuildSubchartDAG's contract). Build does not re-filter hooks.
// chrt == nil yields a flat single-level plan at ChartPath "".
//
// Errors (fatal — the caller must not proceed):
//   - subchart DAG construction failure: depends-on referencing unknown/disabled
//     subcharts, malformed helm.sh/depends-on/subcharts JSON (BuildSubchartDAG)
//   - cycle in a subchart DAG or a resource-group DAG (any level)
//   - a resource assigned to two different groups (ParseResourceGroups)
//
// Warnings (non-fatal, returned in Plan.Warnings; the manifest is still placed):
//   - invalid depends-on/resource-groups JSON on a resource → demoted to unsequenced
//   - group referencing a missing group → whole group demoted (transitively)
//   - isolated group among ≥2 groups → demoted to unsequenced
//   - resource with only one of the two readiness annotations (falls back to kstatus)
//   - rendered subchart not declared in Chart.yaml (deployed after declared subcharts)
//   - rendered subchart not resolvable to a chart object (flat fallback at its path)
//
// Postcondition: every input manifest appears in exactly one batch;
// len(manifests) == Σ len(batch.Manifests()). Unit-enforced.
//
// Build is deterministic: identical input yields an identical plan.
func Build(chrt *chart.Chart, manifests []releaseutil.Manifest) (*Plan, error) {
	b := &builder{plan: &Plan{}}
	if chrt == nil {
		b.plan.Levels = append(b.plan.Levels, ChartLevel{Path: "", Depth: 0})
		if err := b.appendResourceGroupBatches("", 0, manifests); err != nil {
			return nil, err
		}
		return b.plan, nil
	}

	if err := b.buildLevel(chrt, manifests, chrt.Name(), 0); err != nil {
		return nil, err
	}
	return b.plan, nil
}

func (b *builder) warnf(chartPath, format string, args ...any) {
	b.plan.Warnings = append(b.plan.Warnings, Warning{
		ChartPath: chartPath,
		Message:   fmt.Sprintf(format, args...),
	})
}

func (b *builder) buildLevel(c *chart.Chart, manifests []releaseutil.Manifest, chartPath string, depth int) error {
	levelIdx := len(b.plan.Levels)
	b.plan.Levels = append(b.plan.Levels, ChartLevel{Path: chartPath, Depth: depth})

	grouped := GroupManifestsByDirectSubchart(manifests, chartPath)

	dag, err := chartutil.BuildSubchartDAG(c)
	if err != nil {
		return fmt.Errorf("building subchart DAG for %s: %w", chartPath, err)
	}

	batches, err := dag.GetBatches()
	if err != nil {
		return fmt.Errorf("getting subchart batches for %s: %w", chartPath, err)
	}
	b.plan.Levels[levelIdx].SubchartBatches = batches

	declared := make(map[string]bool, len(batches))
	for _, batch := range batches {
		for _, name := range batch {
			declared[name] = true
			if err := b.buildSubchart(c, chartPath, name, grouped[name], depth, levelIdx); err != nil {
				return err
			}
		}
	}

	for _, name := range slices.Sorted(maps.Keys(grouped)) {
		if name == "" || declared[name] {
			continue
		}
		b.plan.Levels[levelIdx].Undeclared = append(b.plan.Levels[levelIdx].Undeclared, name)
		b.warnf(chartPath, "rendered subchart %q is not declared in Chart.yaml dependencies; sequencing it after declared subcharts", name)
		if err := b.buildSubchart(c, chartPath, name, grouped[name], depth, levelIdx); err != nil {
			return err
		}
	}
	slices.Sort(b.plan.Levels[levelIdx].Unresolved)

	return b.appendResourceGroupBatches(chartPath, depth, grouped[""])
}

func (b *builder) buildSubchart(parent *chart.Chart, chartPath, name string, manifests []releaseutil.Manifest, depth, parentLevelIdx int) error {
	if len(manifests) == 0 {
		return nil
	}

	subPath := chartPath + "/charts/" + name
	sub := FindSubchart(parent, name)
	if sub == nil {
		b.plan.Levels[parentLevelIdx].Unresolved = append(b.plan.Levels[parentLevelIdx].Unresolved, name)
		b.warnf(chartPath, "subchart %q not found in chart dependencies; deploying its manifests without subchart sequencing", name)
		return b.appendResourceGroupBatches(subPath, depth+1, manifests)
	}

	return b.buildLevel(sub, manifests, subPath, depth+1)
}

func (b *builder) appendResourceGroupBatches(chartPath string, depth int, manifests []releaseutil.Manifest) error {
	if len(manifests) == 0 {
		return nil
	}

	for _, manifest := range manifests {
		if manifest.Head == nil || manifest.Head.Metadata == nil {
			continue
		}
		annotations := manifest.Head.Metadata.Annotations
		_, hasSuccess := annotations[releaseutil.AnnotationReadinessSuccess]
		_, hasFailure := annotations[releaseutil.AnnotationReadinessFailure]
		if hasSuccess != hasFailure {
			b.warnf(chartPath, "resource %q has only one of %s and %s; falling back to kstatus readiness", manifest.Head.Metadata.Name, releaseutil.AnnotationReadinessSuccess, releaseutil.AnnotationReadinessFailure)
		}
	}

	result, warnings, err := releaseutil.ParseResourceGroups(manifests)
	if err != nil {
		return fmt.Errorf("parsing resource-group annotations for %s: %w", chartPath, err)
	}
	for _, warning := range warnings {
		b.plan.Warnings = append(b.plan.Warnings, Warning{
			ChartPath: chartPath,
			Message:   warning,
		})
	}

	unsequenced := result.Unsequenced
	if len(result.Groups) >= 2 {
		dependents := resourceGroupDependents(result.GroupDeps)
		for _, groupName := range slices.Sorted(maps.Keys(result.Groups)) {
			if len(result.GroupDeps[groupName]) != 0 || dependents[groupName] {
				continue
			}
			b.warnf(chartPath, "resource-group %q is isolated (no depends-on edges and no dependents); deploying it in the unsequenced batch after sequenced groups", groupName)
			unsequenced = append(unsequenced, result.Groups[groupName]...)
			delete(result.Groups, groupName)
			delete(result.GroupDeps, groupName)
		}
	}

	if len(result.Groups) > 0 {
		dag, err := releaseutil.BuildResourceGroupDAG(result)
		if err != nil {
			return fmt.Errorf("building resource-group DAG for %s: %w", chartPath, err)
		}

		groupBatches, err := dag.GetBatches()
		if err != nil {
			return fmt.Errorf("getting resource-group batches for %s: %w", chartPath, err)
		}

		dependents := resourceGroupDependents(result.GroupDeps)
		for _, groupBatch := range groupBatches {
			batch := Batch{
				ChartPath: chartPath,
				Depth:     depth,
				Kind:      BatchKindGroups,
				Wait:      true,
			}
			for _, groupName := range groupBatch {
				batch.Groups = append(batch.Groups, Group{
					Name:      groupName,
					Manifests: result.Groups[groupName],
				})
				if !dependents[groupName] {
					batch.LeafGroups = append(batch.LeafGroups, groupName)
				}
			}
			batch.HasCustomReadiness = hasCustomReadiness(batch.Manifests())
			b.plan.Batches = append(b.plan.Batches, batch)
		}
	}

	if len(unsequenced) > 0 {
		b.plan.Batches = append(b.plan.Batches, Batch{
			ChartPath:          chartPath,
			Depth:              depth,
			Kind:               BatchKindUnsequenced,
			Groups:             []Group{{Name: "", Manifests: unsequenced}},
			Wait:               true,
			HasCustomReadiness: hasCustomReadiness(unsequenced),
		})
	}

	return nil
}

func resourceGroupDependents(groupDeps map[string][]string) map[string]bool {
	dependents := make(map[string]bool)
	for _, deps := range groupDeps {
		for _, dep := range deps {
			dependents[dep] = true
		}
	}
	return dependents
}

func hasCustomReadiness(manifests []releaseutil.Manifest) bool {
	for _, manifest := range manifests {
		if manifest.Head == nil || manifest.Head.Metadata == nil {
			continue
		}
		annotations := manifest.Head.Metadata.Annotations
		if annotations == nil {
			continue
		}
		if annotations[releaseutil.AnnotationReadinessSuccess] != "" && annotations[releaseutil.AnnotationReadinessFailure] != "" {
			return true
		}
	}
	return false
}
