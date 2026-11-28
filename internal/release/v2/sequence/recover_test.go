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

package sequence_test

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	chart "helm.sh/helm/v4/internal/chart/v3"
	releasev2 "helm.sh/helm/v4/internal/release/v2"
	"helm.sh/helm/v4/internal/release/v2/sequence"
)

const recoveryStoredManifest = `---
# Source: parent/templates/parent.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: parent
---
# Source: parent/charts/platform/charts/alpha/templates/alpha.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: alpha
---
# Source: parent/charts/platform/charts/zeta/templates/zeta.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: zeta
---
# Source: parent/charts/platform/templates/platform.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: platform
`

func TestRecoverPlan_PersistedNestedOrderSurvivesCodec(t *testing.T) {
	t.Parallel()

	chrt := nestedRecoveryChart()
	manifests, err := sequence.ParseStoredManifests(recoveryStoredManifest)
	require.NoError(t, err)
	original, err := sequence.Build(chrt, manifests)
	require.NoError(t, err)
	assert.Equal(t, []string{
		"parent/charts/platform/charts/zeta",
		"parent/charts/platform/charts/alpha",
		"parent/charts/platform",
		"parent",
	}, recoveryBatchPaths(original))

	record, err := sequence.NewPlanRecord(original, manifests)
	require.NoError(t, err)
	decoded := codecRoundTrip(t, &releasev2.Release{
		Chart:    chrt,
		Manifest: recoveryStoredManifest,
		Plan:     record,
	})
	require.NotNil(t, decoded.Chart)
	assert.Empty(t, decoded.Chart.Dependencies(), "the release codec must strip the unexported dependency tree")

	rebuilt, err := sequence.Build(decoded.Chart, manifests)
	require.NoError(t, err)
	assert.Equal(t, []string{
		"parent/charts/platform/charts/alpha",
		"parent/charts/platform/charts/zeta",
		"parent/charts/platform",
		"parent",
	}, recoveryBatchPaths(rebuilt), "metadata-only fallback cannot recover the nested dependency edge")

	recovered, err := sequence.RecoverPlan(decoded.Chart, decoded.Plan, decoded.Manifest)
	require.NoError(t, err)
	assert.Equal(t, original.Levels, recovered.Levels)
	assert.Equal(t, original.Batches, recovered.Batches)
}

func TestRecoverPlan_FallsBackToBuild(t *testing.T) {
	t.Parallel()

	chrt := recoveryChart("parent")
	storedManifest := `---
# Source: parent/templates/config.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: config
`
	manifests, err := sequence.ParseStoredManifests(storedManifest)
	require.NoError(t, err)
	want, err := sequence.Build(chrt, manifests)
	require.NoError(t, err)

	tests := []struct {
		name   string
		record *sequence.PlanRecord
	}{
		{name: "nil record"},
		{
			name: "bad manifest index",
			record: &sequence.PlanRecord{Version: 1, Batches: []sequence.BatchRecord{{
				Kind:      "unsequenced",
				Manifests: []sequence.ManifestRef{{Index: 1, Path: manifests[0].Name}},
			}}},
		},
		{
			name: "manifest path mismatch",
			record: &sequence.PlanRecord{Version: 1, Batches: []sequence.BatchRecord{{
				Kind:      "unsequenced",
				Manifests: []sequence.ManifestRef{{Index: 0, Path: "parent/templates/other.yaml"}},
			}}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := sequence.RecoverPlan(chrt, tt.record, storedManifest)
			require.NoError(t, err)
			assert.Equal(t, want, got)
		})
	}
}

func nestedRecoveryChart() *chart.Chart {
	zeta := recoveryChart("zeta")
	alpha := recoveryChart("alpha")
	platform := recoveryChart("platform")
	platform.Metadata.Dependencies = []*chart.Dependency{
		{Name: "zeta", Enabled: true},
		{Name: "alpha", DependsOn: []string{"zeta"}, Enabled: true},
	}
	platform.SetDependencies(zeta, alpha)

	parent := recoveryChart("parent")
	parent.Metadata.Dependencies = []*chart.Dependency{{Name: "platform", Enabled: true}}
	parent.SetDependencies(platform)
	return parent
}

func recoveryChart(name string) *chart.Chart {
	return &chart.Chart{Metadata: &chart.Metadata{
		APIVersion: chart.APIVersionV3,
		Name:       name,
		Version:    "0.1.0",
	}}
}

func recoveryBatchPaths(plan *sequence.Plan) []string {
	paths := make([]string, len(plan.Batches))
	for i, batch := range plan.Batches {
		paths[i] = batch.ChartPath
	}
	return paths
}

func codecRoundTrip(t *testing.T, input *releasev2.Release) *releasev2.Release {
	t.Helper()

	data, err := json.Marshal(input)
	require.NoError(t, err)
	var compressed bytes.Buffer
	writer, err := gzip.NewWriterLevel(&compressed, gzip.BestCompression)
	require.NoError(t, err)
	_, err = writer.Write(data)
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	encoded := base64.StdEncoding.EncodeToString(compressed.Bytes())
	compressedData, err := base64.StdEncoding.DecodeString(encoded)
	require.NoError(t, err)
	reader, err := gzip.NewReader(bytes.NewReader(compressedData))
	require.NoError(t, err)
	decodedData, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.NoError(t, reader.Close())

	var decoded releasev2.Release
	require.NoError(t, json.Unmarshal(decodedData, &decoded))
	return &decoded
}
