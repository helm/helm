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
	"github.com/stretchr/testify/require"

	chart "helm.sh/helm/v4/internal/chart/v3"
	chartutil "helm.sh/helm/v4/internal/chart/v3/util"
	releaseutil "helm.sh/helm/v4/internal/release/v2/manifest"
)

func TestBuild_NilChart_FlatPlan(t *testing.T) {
	t.Parallel()

	manifests := []releaseutil.Manifest{
		{Name: "parent/templates/first.yaml"},
		{Name: "parent/charts/database/templates/second.yaml"},
	}

	plan, err := Build(nil, manifests)
	require.NoError(t, err)
	assert.Equal(t, []ChartLevel{{Path: "", Depth: 0}}, plan.Levels)
	require.Len(t, plan.Batches, 1)
	assert.Equal(t, Batch{
		ChartPath: "",
		Depth:     0,
		Manifests: manifests,
		Wait:      true,
	}, plan.Batches[0])
	assert.Empty(t, plan.Warnings)
}

func TestBuild_EmptyChart(t *testing.T) {
	t.Parallel()

	plan, err := Build(newTestChart("parent"), nil)
	require.NoError(t, err)
	assert.Equal(t, []ChartLevel{{Path: "parent", Depth: 0}}, plan.Levels)
	assert.Empty(t, plan.Batches)
	assert.Empty(t, plan.Warnings)
}

func TestBuild_NoAnnotations_SingleFlatBatch(t *testing.T) {
	t.Parallel()

	manifests := []releaseutil.Manifest{
		{Name: "parent/templates/first.yaml"},
		{Name: "parent/templates/second.yaml"},
		{Name: "parent/templates/third.yaml"},
	}

	plan, err := Build(newTestChart("parent"), manifests)
	require.NoError(t, err)
	assert.Equal(t, []ChartLevel{{Path: "parent", Depth: 0}}, plan.Levels)
	require.Len(t, plan.Batches, 1)
	assert.Equal(t, Batch{
		ChartPath: "parent",
		Depth:     0,
		Manifests: manifests,
		Wait:      true,
	}, plan.Batches[0])
	assert.Empty(t, plan.Warnings)
}

func TestBuild_NestedSubcharts_ThreeLevels(t *testing.T) {
	t.Parallel()

	grand := newTestChart("grand")
	child := newTestChart("child")
	child.Metadata.Dependencies = []*chart.Dependency{{Name: "grand", Enabled: true}}
	child.SetDependencies(grand)
	parent := newTestChart("parent")
	parent.Metadata.Dependencies = []*chart.Dependency{{Name: "child", Enabled: true}}
	parent.SetDependencies(child)
	manifests := []releaseutil.Manifest{
		{Name: "parent/templates/parent.yaml"},
		{Name: "parent/charts/child/templates/child.yaml"},
		{Name: "parent/charts/child/charts/grand/templates/grand.yaml"},
	}

	plan, err := Build(parent, manifests)
	require.NoError(t, err)
	assert.Equal(t, []string{
		"parent/charts/child/charts/grand",
		"parent/charts/child",
		"parent",
	}, planBatchPaths(plan))
	assert.Equal(t, []int{2, 1, 0}, planBatchDepths(plan))
	assert.Equal(t, []ChartLevel{
		{Path: "parent", Depth: 0, SubchartBatches: [][]string{{"child"}}},
		{Path: "parent/charts/child", Depth: 1, SubchartBatches: [][]string{{"grand"}}},
		{Path: "parent/charts/child/charts/grand", Depth: 2},
	}, plan.Levels)
}

func TestBuild_SubchartDependencyOrder(t *testing.T) {
	t.Parallel()

	parent := newTestChart("parent")
	parent.Metadata.Dependencies = []*chart.Dependency{
		{Name: "postgres", Enabled: true},
		{Name: "rabbitmq", DependsOn: []string{"postgres"}, Enabled: true},
		{Name: "app", DependsOn: []string{"rabbitmq"}, Enabled: true},
	}
	manifests := []releaseutil.Manifest{
		{Name: "parent/charts/app/templates/app.yaml"},
		{Name: "parent/charts/rabbitmq/templates/rabbitmq.yaml"},
		{Name: "parent/charts/postgres/templates/postgres.yaml"},
	}

	plan, err := Build(parent, manifests)
	require.NoError(t, err)
	assert.Equal(t, []string{
		"parent/charts/postgres",
		"parent/charts/rabbitmq",
		"parent/charts/app",
	}, planBatchPaths(plan))
	require.NotEmpty(t, plan.Levels)
	assert.Equal(t, [][]string{{"postgres"}, {"rabbitmq"}, {"app"}}, plan.Levels[0].SubchartBatches)
}

func TestBuild_IndependentSubchartsShareAStage(t *testing.T) {
	t.Parallel()

	exporter := newTestChart("exporter")
	nginx := newTestChart("nginx")
	nginx.Metadata.Dependencies = []*chart.Dependency{{Name: "exporter", Enabled: true}}
	nginx.SetDependencies(exporter)
	parent := newTestChart("parent")
	parent.Metadata.Dependencies = []*chart.Dependency{
		{Name: "nginx", Enabled: true},
		{Name: "rabbitmq", Enabled: true},
		{Name: "bar", DependsOn: []string{"nginx", "rabbitmq"}, Enabled: true},
	}
	parent.SetDependencies(nginx, newTestChart("rabbitmq"), newTestChart("bar"))
	manifests := []releaseutil.Manifest{
		{Name: "parent/templates/parent.yaml"},
		{Name: "parent/charts/bar/templates/bar.yaml"},
		{Name: "parent/charts/rabbitmq/templates/rabbitmq.yaml"},
		{Name: "parent/charts/nginx/templates/nginx.yaml"},
		{Name: "parent/charts/nginx/charts/exporter/templates/exporter.yaml"},
	}

	plan, err := Build(parent, manifests)
	require.NoError(t, err)
	// nginx and rabbitmq start together; nginx keeps its own exporter-first order;
	// bar waits for both.
	assert.Equal(t, []string{
		"parent/charts/nginx/charts/exporter",
		"parent/charts/rabbitmq",
		"parent/charts/nginx",
		"parent/charts/bar",
		"parent",
	}, planBatchPaths(plan))
	waits := make([]bool, len(plan.Batches))
	for i, batch := range plan.Batches {
		waits[i] = batch.Wait
	}
	assert.Equal(t, []bool{false, true, true, true, true}, waits)
}

func TestBuild_Aliases_RealPipeline(t *testing.T) {
	t.Parallel()

	postgres := newTestChart("postgres")
	app := newTestChart("app")
	parent := newTestChart("parent")
	parent.Metadata.Dependencies = []*chart.Dependency{
		{Name: "postgres", Version: "0.1.0", Alias: "primary-db"},
		{Name: "app", Version: "0.1.0", DependsOn: []string{"postgres"}},
	}
	parent.SetDependencies(postgres, app)
	require.NoError(t, chartutil.ProcessDependencies(parent, map[string]any{}))
	manifests := []releaseutil.Manifest{
		{Name: "parent/charts/app/templates/app.yaml"},
		{Name: "parent/charts/primary-db/templates/database.yaml"},
	}

	plan, err := Build(parent, manifests)
	require.NoError(t, err)
	assert.Equal(t, []string{
		"parent/charts/primary-db",
		"parent/charts/app",
	}, planBatchPaths(plan))
	require.NotEmpty(t, plan.Levels)
	assert.Equal(t, [][]string{{"primary-db"}, {"app"}}, plan.Levels[0].SubchartBatches)
}

func TestBuild_SubchartCycle_Fatal(t *testing.T) {
	t.Parallel()

	parent := newTestChart("parent")
	parent.Metadata.Dependencies = []*chart.Dependency{
		{Name: "a", Enabled: true, DependsOn: []string{"b"}},
		{Name: "b", Enabled: true, DependsOn: []string{"a"}},
	}

	plan, err := Build(parent, nil)
	require.Error(t, err)
	assert.Nil(t, plan)
	require.ErrorContains(t, err, "subchart circular dependency detected in parent")
	assert.ErrorContains(t, err, "cycle detected among nodes: a, b")
}

func TestBuild_UnknownDependsOnRef_Fatal(t *testing.T) {
	t.Parallel()

	parent := newTestChart("parent")
	parent.Metadata.Dependencies = []*chart.Dependency{
		{Name: "app", Enabled: true, DependsOn: []string{"missing"}},
	}

	plan, err := Build(parent, nil)
	require.Error(t, err)
	assert.Nil(t, plan)
	require.ErrorContains(t, err, "building subchart DAG for parent")
	assert.ErrorContains(t, err, `depends-on unknown or disabled subchart "missing"`)
}

func TestBuild_EveryManifestExactlyOnce(t *testing.T) {
	t.Parallel()

	chrt, manifests := builderAcceptanceFixture()
	plan, err := Build(chrt, manifests)
	require.NoError(t, err)

	actual := make([]releaseutil.Manifest, 0, len(manifests))
	for _, batch := range plan.Batches {
		actual = append(actual, batch.Manifests...)
	}
	require.Len(t, actual, len(manifests))
	assert.ElementsMatch(t, manifests, actual)
}

func TestBuild_Deterministic(t *testing.T) {
	t.Parallel()

	chrt, manifests := builderAcceptanceFixture()
	first, err := Build(chrt, manifests)
	require.NoError(t, err)
	second, err := Build(chrt, manifests)
	require.NoError(t, err)

	assert.Equal(t, first, second)
}

func TestBuild_UndeclaredSubchartWarnedAndPlaced(t *testing.T) {
	t.Parallel()

	declared := newTestChart("declared")
	vendored := newTestChart("vendored")
	parent := newTestChart("parent")
	parent.Metadata.Dependencies = []*chart.Dependency{{Name: "declared", Enabled: true}}
	parent.SetDependencies(declared, vendored)
	manifests := []releaseutil.Manifest{
		{Name: "parent/charts/declared/templates/declared.yaml"},
		{Name: "parent/charts/vendored/templates/vendored.yaml"},
		{Name: "parent/templates/parent.yaml"},
	}

	plan, err := Build(parent, manifests)
	require.NoError(t, err)
	assert.Equal(t, []string{
		"parent/charts/declared",
		"parent/charts/vendored",
		"parent",
	}, planBatchPaths(plan))
	require.NotEmpty(t, plan.Levels)
	assert.Equal(t, []string{"vendored"}, plan.Levels[0].Undeclared)
	require.Len(t, plan.Warnings, 1)
	assert.Equal(t, WarningKindUndeclaredSubchart, plan.Warnings[0].Kind)
	assert.Equal(t, "parent", plan.Warnings[0].ChartPath)
	assert.Contains(t, plan.Warnings[0].Message, "not declared")
}

func TestBuild_StructuralWalk_StorageDecodedNested(t *testing.T) {
	t.Parallel()

	parent := newTestChart("parent")
	parent.Metadata.Dependencies = []*chart.Dependency{{Name: "child", Enabled: true}}
	manifests := []releaseutil.Manifest{
		{Name: "parent/charts/child/templates/child.yaml"},
		{Name: "parent/charts/child/charts/database/templates/database.yaml"},
		{Name: "parent/charts/child/charts/cache/templates/cache.yaml"},
		{Name: "parent/templates/parent.yaml"},
	}

	plan, err := Build(parent, manifests)
	require.NoError(t, err)
	assert.Equal(t, []string{
		"parent/charts/child/charts/cache",
		"parent/charts/child/charts/database",
		"parent/charts/child",
		"parent",
	}, planBatchPaths(plan))
	require.Len(t, plan.Levels, 4)
	assert.Equal(t, []string{"child"}, plan.Levels[0].Unresolved)
	assert.Equal(t, [][]string{{"cache", "database"}}, plan.Levels[1].SubchartBatches)
	require.Len(t, plan.Warnings, 1)
	assert.Equal(t, WarningKindUnresolvedSubchart, plan.Warnings[0].Kind)
	assert.Equal(t, "parent/charts/child", plan.Warnings[0].ChartPath)
	assert.Contains(t, plan.Warnings[0].Message, "name order")
}

func TestBuild_ParentDependsOnResolved(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		chart           func(t *testing.T) *chart.Chart
		parentDependsOn []string
	}{
		{
			name: "resolved alias from dependency pipeline",
			chart: func(t *testing.T) *chart.Chart {
				t.Helper()
				database := newTestChart("database")
				app := newTestChart("app")
				parent := newTestChart("parent")
				parent.Metadata.Dependencies = []*chart.Dependency{
					{Name: "database", Version: "0.1.0", Alias: "primary-db"},
					{Name: "app", Version: "0.1.0"},
				}
				parent.Metadata.Annotations = map[string]string{
					chartutil.AnnotationDependsOnSubcharts: `["database"]`,
				}
				parent.SetDependencies(database, app)
				require.NoError(t, chartutil.ProcessDependencies(parent, map[string]any{}))
				return parent
			},
			parentDependsOn: []string{"primary-db"},
		},
		{
			name:  "annotation absent",
			chart: func(*testing.T) *chart.Chart { return newTestChart("without-annotation") },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			plan, err := Build(tt.chart(t), nil)
			require.NoError(t, err)
			require.NotEmpty(t, plan.Levels)
			assert.Equal(t, tt.parentDependsOn, plan.Levels[0].ParentDependsOn)
		})
	}
}

func TestBuild_DeclaredAPIVersionRejected(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		apiVersion string
		errMessage string
	}{
		{name: "chart v1", apiVersion: "v1", errMessage: `declared apiVersion "v1" is not supported`},
		{name: "chart v2", apiVersion: "v2", errMessage: `declared apiVersion "v2" is not supported`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			chrt := newTestChart("parent")
			chrt.Metadata.APIVersion = tt.apiVersion

			plan, err := Build(chrt, nil)
			require.Error(t, err)
			assert.Nil(t, plan)
			assert.ErrorContains(t, err, tt.errMessage)
		})
	}
}

func TestGroupManifestsByDirectSubchart(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		chartPath string
		manifests []releaseutil.Manifest
		expected  map[string][]string
	}{
		{
			name:      "root chart groups grandchildren with their direct parent",
			chartPath: "parent",
			manifests: []releaseutil.Manifest{
				{Name: "parent/templates/one.yaml"},
				{Name: "parent/charts/database/templates/one.yaml"},
				{Name: "parent/charts/database/charts/cache/templates/one.yaml"},
			},
			expected: map[string][]string{
				"":         {"parent/templates/one.yaml"},
				"database": {"parent/charts/database/templates/one.yaml", "parent/charts/database/charts/cache/templates/one.yaml"},
			},
		},
		{
			name:      "flat plan keeps every manifest at the current level",
			chartPath: "",
			manifests: []releaseutil.Manifest{
				{Name: "parent/templates/one.yaml"},
				{Name: "parent/charts/database/templates/one.yaml"},
				{Name: "parent/charts/database/charts/cache/templates/one.yaml"},
			},
			expected: map[string][]string{
				"": {
					"parent/templates/one.yaml",
					"parent/charts/database/templates/one.yaml",
					"parent/charts/database/charts/cache/templates/one.yaml",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			grouped := GroupManifestsByDirectSubchart(tt.manifests, tt.chartPath)
			actual := make(map[string][]string, len(grouped))
			for name, groupedManifests := range grouped {
				for _, manifest := range groupedManifests {
					actual[name] = append(actual[name], manifest.Name)
				}
			}

			assert.Equal(t, tt.expected, actual)
		})
	}
}

func TestGroupManifestsByDirectSubchart_Nested(t *testing.T) {
	t.Parallel()

	manifests := []releaseutil.Manifest{
		{Name: "parent/charts/database/templates/one.yaml"},
		{Name: "parent/charts/database/charts/cache/templates/one.yaml"},
	}

	grouped := GroupManifestsByDirectSubchart(manifests, "parent/charts/database")
	require.Len(t, grouped[""], 1)
	require.Len(t, grouped["cache"], 1)
	assert.Equal(t, manifests[0], grouped[""][0])
	assert.Equal(t, manifests[1], grouped["cache"][0])
}

func TestFindSubchart(t *testing.T) {
	t.Parallel()

	foo := newTestChart("foo")
	bar := newTestChart("bar")
	database := newTestChart("database")
	parent := newTestChart("parent")
	parent.Metadata.Dependencies = []*chart.Dependency{
		{Name: "foo", Alias: "bar"},
		{Name: "bar"},
	}
	parent.SetDependencies(foo, bar)
	plainParent := newTestChart("plain-parent")
	plainParent.Metadata.Dependencies = []*chart.Dependency{{Name: "database"}}
	plainParent.SetDependencies(database)

	tests := []struct {
		name     string
		parent   *chart.Chart
		query    string
		expected *chart.Chart
	}{
		{name: "underlying name without alias", parent: plainParent, query: "database", expected: database},
		{name: "effective alias", parent: parent, query: "bar", expected: foo},
		{name: "underlying name remains valid", parent: parent, query: "foo", expected: foo},
		{name: "unknown name", parent: parent, query: "missing"},
		{name: "nil parent", query: "foo"},
		{name: "no loaded dependencies", parent: newTestChart("empty"), query: "foo"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			actual := FindSubchart(tt.parent, tt.query)
			if tt.expected == nil {
				assert.Nil(t, actual)
				return
			}
			require.NotNil(t, actual)
			assert.Same(t, tt.expected, actual)
		})
	}
}

func newTestChart(name string) *chart.Chart {
	return &chart.Chart{Metadata: &chart.Metadata{
		APIVersion: chart.APIVersionV3,
		Name:       name,
		Version:    "0.1.0",
	}}
}

func builderAcceptanceFixture() (*chart.Chart, []releaseutil.Manifest) {
	database := newTestChart("database")
	app := newTestChart("app")
	empty := newTestChart("empty")
	library := newTestChart("library")
	library.Metadata.Type = "library"
	vendored := newTestChart("vendored")

	parent := newTestChart("parent")
	parent.Metadata.Dependencies = []*chart.Dependency{
		{Name: "database", Enabled: true},
		{Name: "app", Enabled: true, DependsOn: []string{"database"}},
		{Name: "empty", Enabled: true},
		{Name: "library", Enabled: true},
	}
	parent.SetDependencies(database, app, empty, library, vendored)

	return parent, []releaseutil.Manifest{
		{Name: "parent/templates/parent.yaml", Content: "parent"},
		{Name: "parent/charts/app/templates/app.yaml", Content: "app"},
		{Name: "parent/charts/database/templates/database.yaml", Content: "database"},
		{Name: "parent/charts/vendored/templates/vendored.yaml", Content: "vendored"},
	}
}

func planBatchPaths(plan *Plan) []string {
	paths := make([]string, len(plan.Batches))
	for i, batch := range plan.Batches {
		paths[i] = batch.ChartPath
	}
	return paths
}

func planBatchDepths(plan *Plan) []int {
	depths := make([]int, len(plan.Batches))
	for i, batch := range plan.Batches {
		depths[i] = batch.Depth
	}
	return depths
}
