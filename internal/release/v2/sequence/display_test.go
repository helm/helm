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
	"testing"

	"github.com/stretchr/testify/assert"

	releaseutil "helm.sh/helm/v4/internal/release/v2/manifest"
)

func TestPlanBatchDisplay(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		plan      *Plan
		chartPath string
		want      BatchDisplay
	}{
		{
			name: "preserves group order and labels unsequenced manifests",
			plan: &Plan{Batches: []Batch{
				{
					ChartPath: "parent/charts/child",
					Kind:      BatchKindGroups,
					Groups:    []Group{{Name: "child-group"}},
				},
				{
					ChartPath: "parent",
					Kind:      BatchKindGroups,
					Groups:    []Group{{Name: "databases"}},
				},
				{
					ChartPath: "parent",
					Kind:      BatchKindGroups,
					Groups:    []Group{{Name: "app"}, {Name: "workers"}},
				},
				{
					ChartPath: "parent",
					Kind:      BatchKindUnsequenced,
					Manifests: []releaseutil.Manifest{
						displayManifest("parent/templates/raw.yaml", "", ""),
						displayManifest("parent/templates/database.yaml", "", "database"),
						displayManifest("parent/templates/api.yaml", "Deployment", "api"),
					},
				},
			}},
			chartPath: "parent",
			want: BatchDisplay{
				ResourceGroupBatches: [][]string{{"databases"}, {"app", "workers"}},
				Unsequenced:          []string{"Deployment/api", "database", "parent/templates/raw.yaml"},
			},
		},
		{
			name:      "nil plan has no display batches",
			plan:      nil,
			chartPath: "parent",
			want:      BatchDisplay{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, tt.plan.BatchDisplay(tt.chartPath))
		})
	}
}

func displayManifest(path, kind, name string) releaseutil.Manifest {
	manifest := releaseutil.Manifest{Name: path}
	if name == "" {
		return manifest
	}
	manifest.Head = &releaseutil.SimpleHead{
		Kind: kind,
		Metadata: &struct {
			Name        string            `json:"name"`
			Namespace   string            `json:"namespace,omitempty"`
			Annotations map[string]string `json:"annotations"`
		}{Name: name},
	}
	return manifest
}
