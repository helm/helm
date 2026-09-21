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
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"

	chart "helm.sh/helm/v4/internal/chart/v3"
	chartutil "helm.sh/helm/v4/internal/chart/v3/util"
	releaseutil "helm.sh/helm/v4/internal/release/v2/util"
)

type builder struct {
	plan *Plan
}

// Build constructs a deployment plan for a chart's rendered manifests.
// A nil chart produces a flat plan whose chart path is empty.
func Build(chrt *chart.Chart, manifests []releaseutil.Manifest) (*Plan, error) {
	b := &builder{plan: &Plan{}}
	if chrt == nil {
		b.plan.Levels = append(b.plan.Levels, ChartLevel{Path: "", Depth: 0})
		if err := b.appendChartBatches("", 0, manifests); err != nil {
			return nil, err
		}
		return b.plan, nil
	}
	if chrt.Metadata != nil && chrt.Metadata.APIVersion != "" && chrt.Metadata.APIVersion != chart.APIVersionV3 {
		return nil, fmt.Errorf("declared apiVersion %q is not supported; expected %q", chrt.Metadata.APIVersion, chart.APIVersionV3)
	}

	if err := b.buildLevel(chrt, manifests, chrt.Name(), 0); err != nil {
		return nil, err
	}
	return b.plan, nil
}

func (b *builder) warnf(kind WarningKind, chartPath, format string, args ...any) {
	b.plan.Warnings = append(b.plan.Warnings, Warning{
		Kind:      kind,
		ChartPath: chartPath,
		Message:   fmt.Sprintf(format, args...),
	})
}

func (b *builder) buildLevel(chrt *chart.Chart, manifests []releaseutil.Manifest, chartPath string, depth int) error {
	levelIdx := len(b.plan.Levels)
	b.plan.Levels = append(b.plan.Levels, ChartLevel{Path: chartPath, Depth: depth})
	grouped := GroupManifestsByDirectSubchart(manifests, chartPath)

	dag, err := chartutil.BuildSubchartDAG(chrt)
	if err != nil {
		return fmt.Errorf("building subchart DAG for %s: %w", chartPath, err)
	}
	subchartBatches, err := dag.GetBatches()
	if err != nil {
		return fmt.Errorf("subchart circular dependency detected in %s: %w", chartPath, err)
	}
	b.plan.Levels[levelIdx].SubchartBatches = subchartBatches
	if chrt.Metadata != nil {
		annotation := strings.TrimSpace(chrt.Metadata.Annotations[chartutil.AnnotationDependsOnSubcharts])
		if annotation != "" {
			var parentDependsOn []string
			if err := json.Unmarshal([]byte(annotation), &parentDependsOn); err != nil {
				return fmt.Errorf("parsing %s annotation for %s: %w", chartutil.AnnotationDependsOnSubcharts, chartPath, err)
			}
			b.plan.Levels[levelIdx].ParentDependsOn = parentDependsOn
		}
	}

	declared := make(map[string]bool, len(subchartBatches))
	for _, subchartBatch := range subchartBatches {
		for _, name := range subchartBatch {
			declared[name] = true
			if err := b.buildSubchart(chrt, chartPath, name, grouped[name], depth, levelIdx); err != nil {
				return err
			}
		}
	}

	for _, name := range slices.Sorted(maps.Keys(grouped)) {
		if name == "" || declared[name] {
			continue
		}
		b.plan.Levels[levelIdx].Undeclared = append(b.plan.Levels[levelIdx].Undeclared, name)
		b.warnf(WarningKindUndeclaredSubchart, chartPath, "rendered subchart %q is not declared in Chart.yaml dependencies; sequencing it after declared subcharts", name)
		if err := b.buildSubchart(chrt, chartPath, name, grouped[name], depth, levelIdx); err != nil {
			return err
		}
	}
	slices.Sort(b.plan.Levels[levelIdx].Unresolved)

	return b.appendChartBatches(chartPath, depth, grouped[""])
}

func (b *builder) buildSubchart(parent *chart.Chart, chartPath, name string, manifests []releaseutil.Manifest, depth, parentLevelIdx int) error {
	if len(manifests) == 0 {
		return nil
	}

	subchartPath := chartPath + "/charts/" + name
	subchart := FindSubchart(parent, name)
	if subchart == nil {
		b.plan.Levels[parentLevelIdx].Unresolved = append(b.plan.Levels[parentLevelIdx].Unresolved, name)
		return b.buildStructuralLevel(subchartPath, depth+1, manifests)
	}
	return b.buildLevel(subchart, manifests, subchartPath, depth+1)
}

func (b *builder) buildStructuralLevel(chartPath string, depth int, manifests []releaseutil.Manifest) error {
	levelIdx := len(b.plan.Levels)
	b.plan.Levels = append(b.plan.Levels, ChartLevel{Path: chartPath, Depth: depth})
	grouped := GroupManifestsByDirectSubchart(manifests, chartPath)
	subcharts := slices.DeleteFunc(slices.Sorted(maps.Keys(grouped)), func(name string) bool {
		return name == ""
	})

	if len(subcharts) > 0 {
		b.plan.Levels[levelIdx].SubchartBatches = [][]string{subcharts}
	}
	if len(subcharts) >= 2 {
		b.warnf(WarningKindUnresolvedSubchart, chartPath, "chart metadata for %s is unavailable; sequencing its subcharts %v in name order (depends-on between them, if any, is not recoverable)", chartPath, subcharts)
	}
	for _, name := range subcharts {
		if err := b.buildStructuralLevel(chartPath+"/charts/"+name, depth+1, grouped[name]); err != nil {
			return err
		}
	}
	return b.appendChartBatches(chartPath, depth, grouped[""])
}

func (b *builder) appendChartBatches(chartPath string, depth int, manifests []releaseutil.Manifest) error {
	batches, warnings, err := chartBatches(chartPath, manifests)
	if err != nil {
		return err
	}
	for i := range batches {
		batches[i].Depth = depth
	}
	b.plan.Batches = append(b.plan.Batches, batches...)
	b.plan.Warnings = append(b.plan.Warnings, warnings...)
	return nil
}

// chartBatches returns the batches owned by one chart level. PR1 keeps every
// manifest in one hard-barrier batch; PR2 replaces this body with resource
// group partitioning while retaining this signature.
func chartBatches(chartPath string, manifests []releaseutil.Manifest) ([]Batch, []Warning, error) {
	if len(manifests) == 0 {
		return nil, nil, nil
	}

	return []Batch{{
		ChartPath: chartPath,
		Manifests: manifests,
		Wait:      true,
	}}, nil, nil
}

// GroupManifestsByDirectSubchart groups manifests by the direct subchart they belong to.
// The current chart level's own manifests use the empty string key. Nested
// descendants are grouped under their direct subchart parent because deeper
// sequencing is handled recursively.
func GroupManifestsByDirectSubchart(manifests []releaseutil.Manifest, chartPath string) map[string][]releaseutil.Manifest {
	result := make(map[string][]releaseutil.Manifest)
	if chartPath == "" {
		result[""] = append(result[""], manifests...)
		return result
	}

	chartsPrefix := chartPath + "/charts/"
	for _, manifest := range manifests {
		if !strings.HasPrefix(manifest.Name, chartsPrefix) {
			result[""] = append(result[""], manifest)
			continue
		}

		rest := manifest.Name[len(chartsPrefix):]
		subchartName, _, ok := strings.Cut(rest, "/")
		if !ok {
			result[""] = append(result[""], manifest)
			continue
		}
		result[subchartName] = append(result[subchartName], manifest)
	}
	return result
}

// FindSubchart resolves a loaded direct dependency by its effective name or
// underlying chart name. Aliases live on the parent's dependency metadata,
// not on the loaded subchart itself.
func FindSubchart(chrt *chart.Chart, nameOrAlias string) *chart.Chart {
	if chrt == nil {
		return nil
	}

	aliases := make(map[string]string)
	if chrt.Metadata != nil {
		for _, dependency := range chrt.Metadata.Dependencies {
			if dependency == nil {
				continue
			}
			effectiveName := dependency.Name
			if dependency.Alias != "" {
				effectiveName = dependency.Alias
			}
			aliases[dependency.Name] = effectiveName
		}
	}

	for _, dependency := range chrt.Dependencies() {
		effectiveName := dependency.Name()
		if alias, ok := aliases[dependency.Name()]; ok {
			effectiveName = alias
		}
		if effectiveName == nameOrAlias || dependency.Name() == nameOrAlias {
			return dependency
		}
	}
	return nil
}
