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
	"helm.sh/helm/v4/internal/release/v2/resourcegroup"
)

func TestBuild_ResourceGroupOrdering(t *testing.T) {
	t.Parallel()

	manifests := []releaseutil.Manifest{
		groupManifest("parent/templates/database.yaml", "database", "database"),
		groupManifest("parent/templates/app.yaml", "app", "app", "database"),
		{Name: "parent/templates/plain.yaml"},
	}

	plan, err := Build(newTestChart("parent"), manifests)
	require.NoError(t, err)
	assert.Equal(t, []groupBatchSummary{
		{ChartPath: "parent", Kind: BatchKindGroups, Groups: []string{"database"}},
		{ChartPath: "parent", Kind: BatchKindGroups, Groups: []string{"app"}},
		{ChartPath: "parent", Kind: BatchKindUnsequenced, Groups: []string{""}},
	}, summarizeGroupBatches(plan))
	assert.Empty(t, plan.Batches[0].LeafGroups)
	assert.Equal(t, []string{"app"}, plan.Batches[1].LeafGroups)
	assert.Empty(t, plan.Warnings)
	assertGroupPlanComplete(t, plan, manifests)
}

func TestBuild_LeafGroups_Diamond(t *testing.T) {
	t.Parallel()

	manifests := []releaseutil.Manifest{
		groupManifest("parent/templates/base.yaml", "base", "base"),
		groupManifest("parent/templates/left.yaml", "left", "left", "base"),
		groupManifest("parent/templates/right.yaml", "right", "right", "base"),
		groupManifest("parent/templates/top.yaml", "top", "top", "left", "right"),
	}

	plan, err := Build(newTestChart("parent"), manifests)
	require.NoError(t, err)
	assert.Equal(t, [][]string{{"base"}, {"left", "right"}, {"top"}}, groupBatchNames(plan))
	assert.Empty(t, plan.Batches[0].LeafGroups)
	assert.Empty(t, plan.Batches[1].LeafGroups)
	assert.Equal(t, []string{"top"}, plan.Batches[2].LeafGroups)
	assertGroupPlanComplete(t, plan, manifests)
}

func TestBuild_ResourceGroupDemotion(t *testing.T) {
	t.Parallel()

	invalidJSON := groupManifest("parent/templates/invalid.yaml", "invalid", "app")
	invalidJSON.Head.Metadata.Annotations[resourcegroup.AnnotationDependsOnResourceGroups] = "not-json"
	tests := []struct {
		name             string
		manifests        []releaseutil.Manifest
		wantGroups       [][]string
		wantUnsequenced  []string
		warningKind      WarningKind
		warningFragments []string
		warningCount     int
	}{
		{
			name: "isolated group",
			manifests: []releaseutil.Manifest{
				groupManifest("parent/templates/database.yaml", "database", "database"),
				groupManifest("parent/templates/app.yaml", "app", "app", "database"),
				groupManifest("parent/templates/metrics.yaml", "metrics", "metrics"),
				{Name: "parent/templates/plain.yaml"},
			},
			wantGroups:       [][]string{{"database"}, {"app"}, {""}},
			wantUnsequenced:  []string{"parent/templates/plain.yaml", "parent/templates/metrics.yaml"},
			warningKind:      WarningKindIsolatedGroup,
			warningFragments: []string{"isolated", "metrics"},
			warningCount:     1,
		},
		{
			name: "single group is kept",
			manifests: []releaseutil.Manifest{
				groupManifest("parent/templates/solo.yaml", "solo", "solo"),
			},
			wantGroups: [][]string{{"solo"}},
		},
		{
			name: "all isolated groups",
			manifests: []releaseutil.Manifest{
				groupManifest("parent/templates/beta.yaml", "beta", "beta"),
				groupManifest("parent/templates/alpha.yaml", "alpha", "alpha"),
			},
			wantGroups:       [][]string{{""}},
			wantUnsequenced:  []string{"parent/templates/alpha.yaml", "parent/templates/beta.yaml"},
			warningKind:      WarningKindIsolatedGroup,
			warningFragments: []string{"isolated"},
			warningCount:     2,
		},
		{
			name: "missing dependency cascades",
			manifests: []releaseutil.Manifest{
				groupManifest("parent/templates/database.yaml", "database", "database", "missing"),
				groupManifest("parent/templates/app.yaml", "app", "app", "database"),
			},
			wantGroups:       [][]string{{""}},
			wantUnsequenced:  []string{"parent/templates/database.yaml", "parent/templates/app.yaml"},
			warningKind:      WarningKindResourceGroupDemotion,
			warningFragments: []string{"depends-on non-existent group"},
			warningCount:     2,
		},
		{
			name:             "invalid dependency JSON",
			manifests:        []releaseutil.Manifest{invalidJSON},
			wantGroups:       [][]string{{""}},
			wantUnsequenced:  []string{"parent/templates/invalid.yaml"},
			warningKind:      WarningKindResourceGroupDemotion,
			warningFragments: []string{"invalid JSON"},
			warningCount:     1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			plan, err := Build(newTestChart("parent"), tt.manifests)
			require.NoError(t, err)
			assert.Equal(t, tt.wantGroups, groupBatchNames(plan))
			if tt.wantUnsequenced != nil {
				require.Equal(t, BatchKindUnsequenced, plan.Batches[len(plan.Batches)-1].Kind)
				assert.Equal(t, tt.wantUnsequenced, groupManifestNames(plan.Batches[len(plan.Batches)-1].Manifests))
			}
			require.Len(t, plan.Warnings, tt.warningCount)
			for _, warning := range plan.Warnings {
				assert.Equal(t, tt.warningKind, warning.Kind)
				assert.Equal(t, "parent", warning.ChartPath)
				for _, fragment := range tt.warningFragments {
					assert.Contains(t, warning.Message, fragment)
				}
			}
			assertGroupPlanComplete(t, plan, tt.manifests)
		})
	}
}

func TestBuild_ResourceGroupFatalErrors(t *testing.T) {
	t.Parallel()

	child := newTestChart("child")
	parent := newTestChart("parent")
	parent.Metadata.Dependencies = []*chart.Dependency{{Name: "child", Enabled: true}}
	parent.SetDependencies(child)
	tests := []struct {
		name      string
		chart     *chart.Chart
		manifests []releaseutil.Manifest
		contains  []string
	}{
		{
			name:  "nested cycle",
			chart: parent,
			manifests: []releaseutil.Manifest{
				groupManifest("parent/charts/child/templates/a.yaml", "a", "a", "b"),
				groupManifest("parent/charts/child/templates/b.yaml", "b", "b", "a"),
			},
			contains: []string{"cycle", "parent/charts/child"},
		},
		{
			name:  "duplicate resource membership",
			chart: newTestChart("parent"),
			manifests: []releaseutil.Manifest{
				groupManifest("parent/templates/one.yaml", "same", "one"),
				groupManifest("parent/templates/two.yaml", "same", "two"),
			},
			contains: []string{"assigned to multiple resource groups"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			plan, err := Build(tt.chart, tt.manifests)
			require.Error(t, err)
			assert.Nil(t, plan)
			for _, fragment := range tt.contains {
				assert.ErrorContains(t, err, fragment)
			}
		})
	}
}

func TestBuild_GroupNameScopedPerChartLevel(t *testing.T) {
	t.Parallel()

	parent, manifests := nestedGroupFixture()
	plan, err := Build(parent, manifests)
	require.NoError(t, err)
	assert.Equal(t, []groupBatchSummary{
		{ChartPath: "parent/charts/child", Depth: 1, Kind: BatchKindGroups, Groups: []string{"database"}},
		{ChartPath: "parent/charts/child", Depth: 1, Kind: BatchKindGroups, Groups: []string{"app"}},
		{ChartPath: "parent/charts/child", Depth: 1, Kind: BatchKindUnsequenced, Groups: []string{""}},
		{ChartPath: "parent", Kind: BatchKindGroups, Groups: []string{"database"}},
		{ChartPath: "parent", Kind: BatchKindGroups, Groups: []string{"app"}},
		{ChartPath: "parent", Kind: BatchKindUnsequenced, Groups: []string{""}},
	}, summarizeGroupBatches(plan))
	assertGroupPlanComplete(t, plan, manifests)
}

func TestBuild_ResourceGroupsDeterministic(t *testing.T) {
	t.Parallel()

	chartOne, manifestsOne := nestedGroupFixture()
	chartTwo, manifestsTwo := nestedGroupFixture()
	planOne, err := Build(chartOne, manifestsOne)
	require.NoError(t, err)
	planTwo, err := Build(chartTwo, manifestsTwo)
	require.NoError(t, err)
	assert.Equal(t, planOne, planTwo)
}

func TestBuild_HookManifestNotFiltered(t *testing.T) {
	t.Parallel()

	manifest := annotatedManifest("parent/templates/hook.yaml", "hook", "", map[string]string{"helm.sh/hook": "pre-install"})
	plan, err := Build(newTestChart("parent"), []releaseutil.Manifest{manifest})
	require.NoError(t, err)
	require.Len(t, plan.Batches, 1)
	assert.Equal(t, []releaseutil.Manifest{manifest}, plan.Batches[0].Manifests)
	assertGroupPlanComplete(t, plan, []releaseutil.Manifest{manifest})
}

type groupBatchSummary struct {
	ChartPath string
	Depth     int
	Kind      BatchKind
	Groups    []string
}

func summarizeGroupBatches(plan *Plan) []groupBatchSummary {
	summaries := make([]groupBatchSummary, 0, len(plan.Batches))
	for _, batch := range plan.Batches {
		summary := groupBatchSummary{ChartPath: batch.ChartPath, Depth: batch.Depth, Kind: batch.Kind}
		for _, group := range batch.Groups {
			summary.Groups = append(summary.Groups, group.Name)
		}
		summaries = append(summaries, summary)
	}
	return summaries
}

func groupBatchNames(plan *Plan) [][]string {
	summaries := summarizeGroupBatches(plan)
	names := make([][]string, len(summaries))
	for i, summary := range summaries {
		names[i] = summary.Groups
	}
	return names
}

func assertGroupPlanComplete(t *testing.T, plan *Plan, manifests []releaseutil.Manifest) {
	t.Helper()

	want := make(map[string]int, len(manifests))
	for _, manifest := range manifests {
		want[manifest.Name]++
	}
	got := make(map[string]int, len(manifests))
	for _, batch := range plan.Batches {
		assert.True(t, batch.Wait)
		var flattened []releaseutil.Manifest
		for _, group := range batch.Groups {
			flattened = append(flattened, group.Manifests...)
		}
		assert.Equal(t, batch.Manifests, flattened)
		for _, manifest := range batch.Manifests {
			got[manifest.Name]++
		}
	}
	assert.Equal(t, want, got)
}

func groupManifest(path, name, group string, dependencies ...string) releaseutil.Manifest {
	annotations := map[string]string{resourcegroup.AnnotationResourceGroup: group}
	if len(dependencies) > 0 {
		encoded, err := json.Marshal(dependencies)
		if err != nil {
			panic(err)
		}
		annotations[resourcegroup.AnnotationDependsOnResourceGroups] = string(encoded)
	}
	return annotatedManifest(path, name, "", annotations)
}

func annotatedManifest(path, name, namespace string, annotations map[string]string) releaseutil.Manifest {
	return releaseutil.Manifest{
		Name: path,
		Head: &releaseutil.SimpleHead{
			Version: "v1",
			Kind:    "ConfigMap",
			Metadata: &struct {
				Name        string            `json:"name"`
				Namespace   string            `json:"namespace,omitempty"`
				Annotations map[string]string `json:"annotations"`
			}{Name: name, Namespace: namespace, Annotations: annotations},
		},
	}
}

func groupManifestNames(manifests []releaseutil.Manifest) []string {
	names := make([]string, len(manifests))
	for i, manifest := range manifests {
		names[i] = manifest.Name
	}
	return names
}

func nestedGroupFixture() (*chart.Chart, []releaseutil.Manifest) {
	child := newTestChart("child")
	parent := newTestChart("parent")
	parent.Metadata.Dependencies = []*chart.Dependency{{Name: "child", Enabled: true}}
	parent.SetDependencies(child)
	return parent, []releaseutil.Manifest{
		groupManifest("parent/charts/child/templates/database.yaml", "child-database", "database"),
		groupManifest("parent/charts/child/templates/app.yaml", "child-app", "app", "database"),
		{Name: "parent/charts/child/templates/plain.yaml"},
		groupManifest("parent/templates/database.yaml", "parent-database", "database"),
		groupManifest("parent/templates/app.yaml", "parent-app", "app", "database"),
		{Name: "parent/templates/plain.yaml"},
	}
}
