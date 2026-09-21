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
	"strings"

	chart "helm.sh/helm/v4/internal/chart/v3"
	releaseutil "helm.sh/helm/v4/internal/release/v2/util"
)

// Build constructs a deployment plan for a chart's rendered manifests.
// A nil chart produces a flat plan whose chart path is empty.
func Build(chrt *chart.Chart, manifests []releaseutil.Manifest) (*Plan, error) {
	chartPath := ""
	if chrt != nil {
		chartPath = chrt.Name()
	}

	plan := &Plan{Levels: []ChartLevel{{Path: chartPath, Depth: 0}}}
	batches, warnings, err := chartBatches(chartPath, manifests)
	if err != nil {
		return nil, err
	}
	for i := range batches {
		batches[i].Depth = 0
	}
	plan.Batches = append(plan.Batches, batches...)
	plan.Warnings = append(plan.Warnings, warnings...)
	return plan, nil
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
