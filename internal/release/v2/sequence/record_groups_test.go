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

	chart "helm.sh/helm/v4/internal/chart/v3"
	releaseutil "helm.sh/helm/v4/internal/release/v2/manifest"
)

func TestPlanRecord_GroupJSONRoundTrip(t *testing.T) {
	t.Parallel()

	groupOnly := []releaseutil.Manifest{
		groupManifest("parent/templates/database.yaml", "database", "database"),
		groupManifest("parent/templates/app.yaml", "app", "app", "database"),
	}
	combinedChart, combinedManifests := nestedGroupFixture()
	tests := []struct {
		name      string
		chart     *chart.Chart
		manifests []releaseutil.Manifest
	}{
		{name: "group only", chart: newTestChart("parent"), manifests: groupOnly},
		{name: "combined subcharts and groups", chart: combinedChart, manifests: combinedManifests},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			plan, err := Build(tt.chart, tt.manifests)
			require.NoError(t, err)
			require.NotEmpty(t, plan.Batches)

			record, err := NewPlanRecord(plan, tt.manifests)
			require.NoError(t, err)
			assert.Equal(t, planRecordVersion, record.Version)

			data, err := json.Marshal(record)
			require.NoError(t, err)
			var decoded PlanRecord
			require.NoError(t, json.Unmarshal(data, &decoded))
			assert.Equal(t, record, &decoded)

			restored, err := decoded.Restore(tt.manifests)
			require.NoError(t, err)
			assert.Equal(t, plan.Levels, restored.Levels)
			assert.Equal(t, plan.Batches, restored.Batches)
		})
	}
}

func TestPlanReverse_PreservesGroupBatches(t *testing.T) {
	t.Parallel()

	chrt, manifests := nestedGroupFixture()
	plan, err := Build(chrt, manifests)
	require.NoError(t, err)
	require.Greater(t, len(plan.Batches), 1)
	original := append([]Batch(nil), plan.Batches...)

	reversed := plan.Reverse()
	require.Len(t, reversed.Batches, len(plan.Batches))
	for i, batch := range original {
		assert.Equal(t, batch, reversed.Batches[len(reversed.Batches)-1-i])
	}
	assert.Equal(t, original, plan.Batches, "Reverse must not mutate the original group batches")
	assert.Equal(t, original, reversed.Reverse().Batches)
}

func TestRecoverPlan_GroupRecordMismatchFallsBackToAnnotations(t *testing.T) {
	t.Parallel()

	const storedManifest = `---
# Source: parent/templates/database.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: database
  annotations:
    helm.sh/resource-group: database
---
# Source: parent/templates/app.yaml
apiVersion: v1
kind: Deployment
metadata:
  name: app
  annotations:
    helm.sh/resource-group: app
    helm.sh/depends-on-resource-groups: '["database"]'
`
	chrt := newTestChart("parent")
	manifests, err := ParseStoredManifests(storedManifest)
	require.NoError(t, err)
	want, err := Build(chrt, manifests)
	require.NoError(t, err)
	assert.Equal(t, []groupBatchSummary{
		{ChartPath: "parent", Kind: BatchKindGroups, Groups: []string{"database"}},
		{ChartPath: "parent", Kind: BatchKindGroups, Groups: []string{"app"}},
	}, summarizeGroupBatches(want))

	record, err := NewPlanRecord(want, manifests)
	require.NoError(t, err)
	require.Len(t, record.Batches, 2)
	require.Len(t, record.Batches[0].Groups, 1)
	record.Batches[0].Groups[0].Manifests[0] = record.Batches[1].Manifests[0]

	_, err = record.Restore(manifests)
	require.ErrorContains(t, err, "group manifest references do not match manifests in batch 0")

	recovered, err := RecoverPlan(chrt, record, storedManifest)
	require.NoError(t, err)
	assert.Equal(t, want, recovered)

	rebuilt, err := RecoverPlan(chrt, nil, storedManifest)
	require.NoError(t, err)
	assert.Equal(t, want, rebuilt)
}
