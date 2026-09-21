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
	"slices"

	releaseutil "helm.sh/helm/v4/internal/release/v2/manifest"
)

// BatchDisplay contains the resource-group and unsequenced batches rendered
// for one chart level in a plan.
type BatchDisplay struct {
	ResourceGroupBatches [][]string
	Unsequenced          []string
}

// BatchDisplay returns display-ready batches for one chart path. Resource
// group and group-name order match the plan; unsequenced labels are sorted.
func (p *Plan) BatchDisplay(chartPath string) BatchDisplay {
	var display BatchDisplay
	if p == nil {
		return display
	}

	for _, batch := range p.Batches {
		if batch.ChartPath != chartPath {
			continue
		}
		switch batch.Kind {
		case BatchKindGroups:
			names := make([]string, 0, len(batch.Groups))
			for _, group := range batch.Groups {
				names = append(names, group.Name)
			}
			display.ResourceGroupBatches = append(display.ResourceGroupBatches, names)
		case BatchKindUnsequenced:
			for _, manifest := range batch.Manifests {
				display.Unsequenced = append(display.Unsequenced, manifestDisplayLabel(manifest))
			}
		}
	}
	slices.Sort(display.Unsequenced)
	return display
}

func manifestDisplayLabel(manifest releaseutil.Manifest) string {
	if manifest.Head == nil || manifest.Head.Metadata == nil || manifest.Head.Metadata.Name == "" {
		return manifest.Name
	}
	if manifest.Head.Kind == "" {
		return manifest.Head.Metadata.Name
	}
	return manifest.Head.Kind + "/" + manifest.Head.Metadata.Name
}
