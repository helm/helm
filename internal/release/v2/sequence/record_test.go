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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	releaseutil "helm.sh/helm/v4/internal/release/v2/manifest"
)

func TestPlanRecord_JSONRoundTripLossless(t *testing.T) {
	t.Parallel()

	manifests := []releaseutil.Manifest{
		{Name: "parent/templates/parent.yaml", Content: "parent"},
		{Name: "parent/charts/zeta/templates/zeta.yaml", Content: "zeta"},
		{Name: "parent/charts/alpha/templates/alpha.yaml", Content: "alpha"},
		{Name: "parent/charts/zeta/charts/cache/templates/cache.yaml", Content: "cache"},
	}
	plan := &Plan{
		Levels: []ChartLevel{
			{
				Path:            "parent",
				Depth:           0,
				SubchartBatches: [][]string{{"zeta"}, {"alpha"}},
				ParentDependsOn: []string{"alpha"},
			},
			{
				Path:            "parent/charts/zeta",
				Depth:           1,
				SubchartBatches: [][]string{{"cache"}},
			},
			{Path: "parent/charts/zeta/charts/cache", Depth: 2},
			{Path: "parent/charts/alpha", Depth: 1},
		},
		Batches: []Batch{
			{ChartPath: "parent/charts/zeta/charts/cache", Depth: 2, Manifests: manifests[3:4], Wait: true},
			{ChartPath: "parent/charts/zeta", Depth: 1, Manifests: manifests[1:2], Wait: true},
			{ChartPath: "parent/charts/alpha", Depth: 1, Manifests: manifests[2:3], Wait: true},
			{ChartPath: "parent", Depth: 0, Manifests: manifests[0:1], Wait: true},
		},
	}

	record, err := NewPlanRecord(plan, manifests)
	require.NoError(t, err)
	assert.Equal(t, 1, record.Version)
	require.Len(t, record.Levels, 4)
	assert.Equal(t, []Edge{{From: "zeta", To: "alpha"}}, record.Levels[0].Edges)

	data, err := json.Marshal(record)
	require.NoError(t, err)
	var decoded PlanRecord
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, record, &decoded)

	restored, err := decoded.Restore(manifests)
	require.NoError(t, err)
	assert.Equal(t, plan.Levels, restored.Levels)
	assert.Equal(t, plan.Batches, restored.Batches)
}

func TestPlanRecord_RestoreIntegrity(t *testing.T) {
	t.Parallel()

	manifests := []releaseutil.Manifest{{Name: "parent/templates/config.yaml"}}
	tests := []struct {
		name    string
		record  *PlanRecord
		wantErr string
	}{
		{
			name: "index out of range",
			record: &PlanRecord{Version: 1, Batches: []BatchRecord{{
				Kind:      "unsequenced",
				Manifests: []ManifestRef{{Index: 1, Path: manifests[0].Name}},
			}}},
			wantErr: "manifest index 1 out of range",
		},
		{
			name: "path mismatch",
			record: &PlanRecord{Version: 1, Batches: []BatchRecord{{
				Kind:      "unsequenced",
				Manifests: []ManifestRef{{Index: 0, Path: "parent/templates/other.yaml"}},
			}}},
			wantErr: "manifest path mismatch at index 0",
		},
		{
			name:    "unsupported version",
			record:  &PlanRecord{Version: 2},
			wantErr: "unsupported plan record version 2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := tt.record.Restore(manifests)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}
