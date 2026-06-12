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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	chart "helm.sh/helm/v4/pkg/chart/v2"
	chartutil "helm.sh/helm/v4/pkg/chart/v2/util"
	releaseutil "helm.sh/helm/v4/pkg/release/v1/util"
)

func TestBuild_NilChart_FlatPlan(t *testing.T) {
	t.Parallel()

	manifests := []releaseutil.Manifest{
		makeManifest("db", "parent/templates/db.yaml", groupAnnotations("db")),
		makeManifest("app", "parent/templates/app.yaml", groupAnnotations("app", "db")),
		makeManifest("plain", "parent/templates/plain.yaml", nil),
	}

	plan, err := Build(nil, manifests)
	require.NoError(t, err)
	assert.Equal(t, []ChartLevel{{Path: "", Depth: 0}}, plan.Levels)
	assert.Equal(t, []batchSummary{
		{ChartPath: "", Depth: 0, Kind: BatchKindGroups, Groups: []string{"db"}},
		{ChartPath: "", Depth: 0, Kind: BatchKindGroups, Groups: []string{"app"}},
		{ChartPath: "", Depth: 0, Kind: BatchKindUnsequenced, Groups: []string{""}},
	}, batchSummaries(plan))
	assertPlanComplete(t, plan, manifests)
}

func TestBuild_EmptyChart(t *testing.T) {
	t.Parallel()

	plan, err := Build(newChart("parent"), nil)
	require.NoError(t, err)
	assert.Equal(t, []ChartLevel{{Path: "parent", Depth: 0}}, plan.Levels)
	assert.Empty(t, plan.Batches)
	assert.Empty(t, plan.Warnings)
}

func TestBuild_NoAnnotations_SingleFlatBatch(t *testing.T) {
	t.Parallel()

	manifests := []releaseutil.Manifest{
		makeManifest("a", "parent/templates/a.yaml", nil),
		makeManifest("b", "parent/templates/b.yaml", nil),
		makeManifest("c", "parent/templates/c.yaml", nil),
	}

	plan, err := Build(newChart("parent"), manifests)
	require.NoError(t, err)
	require.Len(t, plan.Batches, 1)
	assert.Equal(t, BatchKindUnsequenced, plan.Batches[0].Kind)
	assert.Equal(t, manifestNames(manifests), manifestNames(plan.Batches[0].Manifests()))
	assert.True(t, plan.Batches[0].Wait)
	assertPlanComplete(t, plan, manifests)
}

func TestBuild_ResourceGroupOrdering(t *testing.T) {
	t.Parallel()

	manifests := []releaseutil.Manifest{
		makeManifest("db", "parent/templates/db.yaml", groupAnnotations("db")),
		makeManifest("app", "parent/templates/app.yaml", groupAnnotations("app", "db")),
		makeManifest("plain", "parent/templates/plain.yaml", nil),
	}

	plan, err := Build(newChart("parent"), manifests)
	require.NoError(t, err)
	assert.Equal(t, []batchSummary{
		{ChartPath: "parent", Depth: 0, Kind: BatchKindGroups, Groups: []string{"db"}},
		{ChartPath: "parent", Depth: 0, Kind: BatchKindGroups, Groups: []string{"app"}},
		{ChartPath: "parent", Depth: 0, Kind: BatchKindUnsequenced, Groups: []string{""}},
	}, batchSummaries(plan))
	for _, batch := range plan.Batches {
		assert.True(t, batch.Wait)
	}
	assert.Empty(t, plan.Batches[0].LeafGroups)
	assert.Equal(t, []string{"app"}, plan.Batches[1].LeafGroups)
	assertPlanComplete(t, plan, manifests)
}

func TestBuild_LeafGroups_Diamond(t *testing.T) {
	t.Parallel()

	manifests := []releaseutil.Manifest{
		makeManifest("base", "parent/templates/base.yaml", groupAnnotations("base")),
		makeManifest("left", "parent/templates/left.yaml", groupAnnotations("left", "base")),
		makeManifest("right", "parent/templates/right.yaml", groupAnnotations("right", "base")),
		makeManifest("top", "parent/templates/top.yaml", groupAnnotations("top", "left", "right")),
	}

	plan, err := Build(newChart("parent"), manifests)
	require.NoError(t, err)
	assert.Equal(t, [][]string{{"base"}, {"left", "right"}, {"top"}}, batchGroupNames(plan))
	assert.Empty(t, plan.Batches[0].LeafGroups)
	assert.Empty(t, plan.Batches[1].LeafGroups)
	assert.Equal(t, []string{"top"}, plan.Batches[2].LeafGroups)
	assertPlanComplete(t, plan, manifests)
}

func TestBuild_NestedSubcharts_ThreeLevels(t *testing.T) {
	t.Parallel()

	grand := newChart("grand")
	child := newChart("child", enabledDependency("grand"))
	child.SetDependencies(grand)
	parent := newChart("parent", enabledDependency("child"))
	parent.SetDependencies(child)
	manifests := []releaseutil.Manifest{
		makeManifest("parent", "parent/templates/parent.yaml", nil),
		makeManifest("child", "parent/charts/child/templates/child.yaml", nil),
		makeManifest("grand", "parent/charts/child/charts/grand/templates/grand.yaml", nil),
	}

	plan, err := Build(parent, manifests)
	require.NoError(t, err)
	assert.Equal(t, []string{
		"parent/charts/child/charts/grand",
		"parent/charts/child",
		"parent",
	}, batchChartPaths(plan))
	assert.Equal(t, []int{2, 1, 0}, batchDepths(plan))
	assert.Equal(t, []ChartLevel{
		{Path: "parent", Depth: 0, SubchartBatches: [][]string{{"child"}}},
		{Path: "parent/charts/child", Depth: 1, SubchartBatches: [][]string{{"grand"}}},
		{Path: "parent/charts/child/charts/grand", Depth: 2},
	}, plan.Levels)
	assertPlanComplete(t, plan, manifests)
}

func TestBuild_SubchartDependencyOrder(t *testing.T) {
	t.Parallel()

	parent := newChart(
		"parent",
		enabledDependency("postgres"),
		enabledDependency("rabbitmq", "postgres"),
		enabledDependency("app", "rabbitmq"),
	)
	manifests := []releaseutil.Manifest{
		makeManifest("app", "parent/charts/app/templates/app.yaml", nil),
		makeManifest("rabbitmq", "parent/charts/rabbitmq/templates/rabbitmq.yaml", nil),
		makeManifest("postgres", "parent/charts/postgres/templates/postgres.yaml", nil),
	}

	plan, err := Build(parent, manifests)
	require.NoError(t, err)
	assert.Equal(t, []string{"parent/charts/postgres", "parent/charts/rabbitmq", "parent/charts/app"}, batchChartPaths(plan))
	require.Len(t, plan.Levels, 4)
	assert.Equal(t, [][]string{{"postgres"}, {"rabbitmq"}, {"app"}}, plan.Levels[0].SubchartBatches)
	assertPlanComplete(t, plan, manifests)
}

func TestBuild_Aliases_RealPipeline(t *testing.T) {
	t.Parallel()

	parent := pipelineChart(
		pipelineDependency("postgres", "primary-db"),
		pipelineDependency("app", "", "postgres"),
	)
	require.NoError(t, chartutil.ProcessDependencies(parent, map[string]any{}))
	manifests := []releaseutil.Manifest{
		makeManifest("app", "parent/charts/app/templates/app.yaml", nil),
		makeManifest("primary", "parent/charts/primary-db/templates/primary.yaml", nil),
	}

	plan, err := Build(parent, manifests)
	require.NoError(t, err)
	assert.Equal(t, []string{"parent/charts/primary-db", "parent/charts/app"}, batchChartPaths(plan))
	require.NotEmpty(t, plan.Levels)
	assert.Equal(t, [][]string{{"primary-db"}, {"app"}}, plan.Levels[0].SubchartBatches)
	assertPlanComplete(t, plan, manifests)
}

func TestBuild_SubchartCycle_Fatal(t *testing.T) {
	t.Parallel()

	parent := newChart("parent", enabledDependency("a", "b"), enabledDependency("b", "a"))
	plan, err := Build(parent, []releaseutil.Manifest{makeManifest("a", "parent/charts/a/templates/a.yaml", nil)})
	require.Error(t, err)
	assert.Nil(t, plan)
	assert.Contains(t, err.Error(), "cycle")
}

func TestBuild_UnknownDependsOnRef_Fatal(t *testing.T) {
	t.Parallel()

	parent := newChart("parent", enabledDependency("app", "missing"))
	plan, err := Build(parent, nil)
	require.Error(t, err)
	assert.Nil(t, plan)
	assert.Contains(t, err.Error(), `depends-on unknown or disabled subchart "missing"`)
}

func TestBuild_ResourceGroupCycle_Fatal_NestedLevel(t *testing.T) {
	t.Parallel()

	parent := newChart("parent", enabledDependency("child"))
	manifests := []releaseutil.Manifest{
		makeManifest("a", "parent/charts/child/templates/a.yaml", groupAnnotations("a", "b")),
		makeManifest("b", "parent/charts/child/templates/b.yaml", groupAnnotations("b", "a")),
	}

	plan, err := Build(parent, manifests)
	require.Error(t, err)
	assert.Nil(t, plan)
	assert.Contains(t, err.Error(), "cycle")
	assert.Contains(t, err.Error(), "parent/charts/child")
}

func TestBuild_MultiGroupAssignment_Fatal(t *testing.T) {
	t.Parallel()

	manifests := []releaseutil.Manifest{
		makeTypedManifest("same", "parent/templates/one.yaml", "v1", "ConfigMap", "default", groupAnnotations("one")),
		makeTypedManifest("same", "parent/templates/two.yaml", "v1", "ConfigMap", "default", groupAnnotations("two")),
	}

	plan, err := Build(newChart("parent"), manifests)
	require.Error(t, err)
	assert.Nil(t, plan)
	assert.Contains(t, err.Error(), "assigned to multiple resource groups")
}

func TestBuild_IsolatedGroupDemoted(t *testing.T) {
	t.Parallel()

	manifests := []releaseutil.Manifest{
		makeManifest("db", "parent/templates/db.yaml", groupAnnotations("db")),
		makeManifest("app", "parent/templates/app.yaml", groupAnnotations("app", "db")),
		makeManifest("plain", "parent/templates/plain.yaml", nil),
		makeManifest("metrics", "parent/templates/metrics.yaml", groupAnnotations("metrics")),
	}

	plan, err := Build(newChart("parent"), manifests)
	require.NoError(t, err)
	assert.Equal(t, [][]string{{"db"}, {"app"}, {""}}, batchGroupNames(plan))
	assert.NotContains(t, slices.Concat(batchGroupNames(plan)[:2]...), "metrics")
	require.Len(t, plan.Batches, 3)
	assert.Equal(t, BatchKindUnsequenced, plan.Batches[2].Kind)
	assert.ElementsMatch(t, []string{"parent/templates/plain.yaml", "parent/templates/metrics.yaml"}, manifestNames(plan.Batches[2].Manifests()))
	assertWarningContains(t, plan, "parent", "isolated", "metrics")
	assertPlanComplete(t, plan, manifests)
}

func TestBuild_TwoGroupChain_NotDemoted(t *testing.T) {
	t.Parallel()

	manifests := []releaseutil.Manifest{
		makeManifest("db", "parent/templates/db.yaml", groupAnnotations("db")),
		makeManifest("app", "parent/templates/app.yaml", groupAnnotations("app", "db")),
	}

	plan, err := Build(newChart("parent"), manifests)
	require.NoError(t, err)
	assert.Equal(t, [][]string{{"db"}, {"app"}}, batchGroupNames(plan))
	assertNoWarningContains(t, plan, "isolated")
	assertPlanComplete(t, plan, manifests)
}

func TestBuild_SingleGroup_NotDemoted(t *testing.T) {
	t.Parallel()

	manifests := []releaseutil.Manifest{
		makeManifest("solo", "parent/templates/solo.yaml", groupAnnotations("solo")),
	}

	plan, err := Build(newChart("parent"), manifests)
	require.NoError(t, err)
	assert.Equal(t, [][]string{{"solo"}}, batchGroupNames(plan))
	assert.Equal(t, BatchKindGroups, plan.Batches[0].Kind)
	assertNoWarningContains(t, plan, "isolated")
	assertPlanComplete(t, plan, manifests)
}

func TestBuild_AllGroupsIsolated_AllDemoted(t *testing.T) {
	t.Parallel()

	manifests := []releaseutil.Manifest{
		makeManifest("beta", "parent/templates/beta.yaml", groupAnnotations("beta")),
		makeManifest("alpha", "parent/templates/alpha.yaml", groupAnnotations("alpha")),
	}

	plan, err := Build(newChart("parent"), manifests)
	require.NoError(t, err)
	require.Len(t, plan.Batches, 1)
	assert.Equal(t, BatchKindUnsequenced, plan.Batches[0].Kind)
	assert.Equal(t, []string{"parent/templates/alpha.yaml", "parent/templates/beta.yaml"}, manifestNames(plan.Batches[0].Manifests()))
	require.Len(t, plan.Warnings, 2)
	assert.Contains(t, plan.Warnings[0].Message, "alpha")
	assert.Contains(t, plan.Warnings[1].Message, "beta")
	assertPlanComplete(t, plan, manifests)
}

func TestBuild_MissingGroupDep_TransitiveDemotion(t *testing.T) {
	t.Parallel()

	manifests := []releaseutil.Manifest{
		makeManifest("app", "parent/templates/app.yaml", groupAnnotations("app", "nope")),
	}

	plan, err := Build(newChart("parent"), manifests)
	require.NoError(t, err)
	require.Len(t, plan.Batches, 1)
	assert.Equal(t, BatchKindUnsequenced, plan.Batches[0].Kind)
	assert.Equal(t, []string{"parent/templates/app.yaml"}, manifestNames(plan.Batches[0].Manifests()))
	assertWarningContains(t, plan, "parent", "depends-on non-existent group")
	assertPlanComplete(t, plan, manifests)
}

func TestBuild_InvalidDependsOnJSON_Demoted(t *testing.T) {
	t.Parallel()

	annotations := groupAnnotations("app")
	annotations[releaseutil.AnnotationDependsOnResourceGroups] = "not-json"
	manifests := []releaseutil.Manifest{
		makeManifest("app", "parent/templates/app.yaml", annotations),
	}

	plan, err := Build(newChart("parent"), manifests)
	require.NoError(t, err)
	require.Len(t, plan.Batches, 1)
	assert.Equal(t, BatchKindUnsequenced, plan.Batches[0].Kind)
	assert.Equal(t, []string{"parent/templates/app.yaml"}, manifestNames(plan.Batches[0].Manifests()))
	assertWarningContains(t, plan, "parent", "invalid JSON")
	assertPlanComplete(t, plan, manifests)
}

func TestBuild_UndeclaredSubchartIncluded(t *testing.T) {
	t.Parallel()

	parent := newChart("parent", enabledDependency("declared"))
	parent.AddDependency(newChart("vendored"))
	manifests := []releaseutil.Manifest{
		makeManifest("declared", "parent/charts/declared/templates/declared.yaml", nil),
		makeManifest("vendored", "parent/charts/vendored/templates/vendored.yaml", nil),
		makeManifest("parent", "parent/templates/parent.yaml", nil),
	}

	plan, err := Build(parent, manifests)
	require.NoError(t, err)
	assert.Equal(t, []string{"parent/charts/declared", "parent/charts/vendored", "parent"}, batchChartPaths(plan))
	require.NotEmpty(t, plan.Levels)
	assert.Equal(t, []string{"vendored"}, plan.Levels[0].Undeclared)
	assertWarningContains(t, plan, "parent", "not declared")
	assertPlanComplete(t, plan, manifests)

	reversedPaths := batchChartPaths(plan.Reverse())
	assert.Contains(t, reversedPaths, "parent/charts/vendored")
}

func TestBuild_UnresolvableSubchart_FlatFallback(t *testing.T) {
	t.Parallel()

	manifests := []releaseutil.Manifest{
		makeManifest("db", "parent/charts/ghost/templates/db.yaml", groupAnnotations("db")),
		makeManifest("app", "parent/charts/ghost/templates/app.yaml", groupAnnotations("app", "db")),
	}

	plan, err := Build(newChart("parent"), manifests)
	require.NoError(t, err)
	assertWarningContains(t, plan, "parent", "not found in chart dependencies")
	require.Len(t, plan.Levels, 1)
	assert.Equal(t, []string{"ghost"}, plan.Levels[0].Unresolved)
	assert.Equal(t, []string{"parent/charts/ghost", "parent/charts/ghost"}, batchChartPaths(plan))
	assert.Equal(t, []int{1, 1}, batchDepths(plan))
	assert.Equal(t, [][]string{{"db"}, {"app"}}, batchGroupNames(plan))
	assertPlanComplete(t, plan, manifests)
}

func TestBuild_HookManifestNotFiltered(t *testing.T) {
	t.Parallel()

	manifests := []releaseutil.Manifest{
		makeManifest("hook", "parent/templates/hook.yaml", map[string]string{"helm.sh/hook": "pre-install"}),
	}

	plan, err := Build(newChart("parent"), manifests)
	require.NoError(t, err)
	require.Len(t, plan.Batches, 1)
	assert.Equal(t, []string{"parent/templates/hook.yaml"}, manifestNames(plan.Batches[0].Manifests()))
	assertPlanComplete(t, plan, manifests)
}

func TestBuild_HasCustomReadiness(t *testing.T) {
	t.Parallel()

	groupA := groupAnnotations("a")
	groupA[releaseutil.AnnotationReadinessSuccess] = `{.status.ready} == true`
	groupA[releaseutil.AnnotationReadinessFailure] = `{.status.failed} == true`
	groupB := groupAnnotations("b", "a")
	groupB[releaseutil.AnnotationReadinessSuccess] = `{.status.ready} == true`
	manifests := []releaseutil.Manifest{
		makeManifest("a", "parent/templates/a.yaml", groupA),
		makeManifest("b", "parent/templates/b.yaml", groupB),
		makeManifest("plain", "parent/templates/plain.yaml", nil),
	}

	plan, err := Build(newChart("parent"), manifests)
	require.NoError(t, err)
	require.Len(t, plan.Batches, 3)
	assert.True(t, plan.Batches[0].HasCustomReadiness)
	assert.False(t, plan.Batches[1].HasCustomReadiness)
	assert.False(t, plan.Batches[2].HasCustomReadiness)
	assertWarningContains(t, plan, "parent", "only one of")
	assertPlanComplete(t, plan, manifests)
}

func TestBuild_Deterministic(t *testing.T) {
	t.Parallel()

	chartOne, manifestsOne := kitchenSinkFixture()
	chartTwo, manifestsTwo := kitchenSinkFixture()

	planOne, err := Build(chartOne, manifestsOne)
	require.NoError(t, err)
	planTwo, err := Build(chartTwo, manifestsTwo)
	require.NoError(t, err)
	assert.Equal(t, planOne, planTwo)
}

func TestBuild_Completeness_KitchenSink(t *testing.T) {
	t.Parallel()

	chrt, manifests := kitchenSinkFixture()
	plan, err := Build(chrt, manifests)
	require.NoError(t, err)
	assertPlanComplete(t, plan, manifests)
	for _, batch := range plan.Batches {
		assert.NotEmpty(t, batch.Manifests())
		if batch.Kind == BatchKindUnsequenced {
			require.Len(t, batch.Groups, 1)
			assert.Empty(t, batch.Groups[0].Name)
		}
	}
}

type batchSummary struct {
	ChartPath string
	Depth     int
	Kind      BatchKind
	Groups    []string
}

func summarizeBatch(batch Batch) batchSummary {
	return batchSummary{
		ChartPath: batch.ChartPath,
		Depth:     batch.Depth,
		Kind:      batch.Kind,
		Groups:    groupNames(batch),
	}
}

func batchSummaries(plan *Plan) []batchSummary {
	summaries := make([]batchSummary, 0, len(plan.Batches))
	for _, batch := range plan.Batches {
		summaries = append(summaries, summarizeBatch(batch))
	}
	return summaries
}

func batchGroupNames(plan *Plan) [][]string {
	names := make([][]string, 0, len(plan.Batches))
	for _, batch := range plan.Batches {
		names = append(names, groupNames(batch))
	}
	return names
}

func batchChartPaths(plan *Plan) []string {
	paths := make([]string, 0, len(plan.Batches))
	for _, batch := range plan.Batches {
		paths = append(paths, batch.ChartPath)
	}
	return paths
}

func batchDepths(plan *Plan) []int {
	depths := make([]int, 0, len(plan.Batches))
	for _, batch := range plan.Batches {
		depths = append(depths, batch.Depth)
	}
	return depths
}

func groupNames(batch Batch) []string {
	names := make([]string, 0, len(batch.Groups))
	for _, group := range batch.Groups {
		names = append(names, group.Name)
	}
	return names
}

func manifestNames(manifests []releaseutil.Manifest) []string {
	names := make([]string, 0, len(manifests))
	for _, manifest := range manifests {
		names = append(names, manifest.Name)
	}
	return names
}

func assertPlanComplete(t *testing.T, plan *Plan, manifests []releaseutil.Manifest) {
	t.Helper()

	require.NotNil(t, plan)
	expected := make(map[string]int, len(manifests))
	for _, manifest := range manifests {
		expected[manifest.Name]++
	}

	actual := make(map[string]int, len(manifests))
	var total int
	for _, batch := range plan.Batches {
		for _, manifest := range batch.Manifests() {
			actual[manifest.Name]++
			total++
		}
	}

	assert.Equal(t, len(manifests), total)
	assert.Equal(t, expected, actual)
}

func assertWarningContains(t *testing.T, plan *Plan, chartPath string, parts ...string) {
	t.Helper()

	for _, warning := range plan.Warnings {
		if warning.ChartPath != chartPath {
			continue
		}
		matches := true
		for _, part := range parts {
			if !strings.Contains(warning.Message, part) {
				matches = false
				break
			}
		}
		if matches {
			return
		}
	}
	assert.Failf(t, "missing warning", "chartPath=%q parts=%v warnings=%v", chartPath, parts, plan.Warnings)
}

func assertNoWarningContains(t *testing.T, plan *Plan, part string) {
	t.Helper()

	for _, warning := range plan.Warnings {
		assert.NotContains(t, warning.Message, part)
	}
}

func makeManifest(name, sourcePath string, annotations map[string]string) releaseutil.Manifest {
	return makeTypedManifest(name, sourcePath, "v1", "ConfigMap", "", annotations)
}

func makeTypedManifest(name, sourcePath, version, kind, namespace string, annotations map[string]string) releaseutil.Manifest {
	annotations = maps.Clone(annotations)
	var content strings.Builder
	fmt.Fprintf(&content, "apiVersion: %s\nkind: %s\nmetadata:\n  name: %s\n", version, kind, name)
	if namespace != "" {
		fmt.Fprintf(&content, "  namespace: %s\n", namespace)
	}
	if len(annotations) > 0 {
		content.WriteString("  annotations:\n")
		for _, key := range slices.Sorted(maps.Keys(annotations)) {
			fmt.Fprintf(&content, "    %s: %q\n", key, annotations[key])
		}
	}

	head := &releaseutil.SimpleHead{
		Version: version,
		Kind:    kind,
		Metadata: &struct {
			Name        string            `json:"name"`
			Namespace   string            `json:"namespace,omitempty"`
			Annotations map[string]string `json:"annotations"`
		}{
			Name:        name,
			Namespace:   namespace,
			Annotations: annotations,
		},
	}

	return releaseutil.Manifest{
		Name:    sourcePath,
		Content: content.String(),
		Head:    head,
	}
}

func groupAnnotations(group string, deps ...string) map[string]string {
	annotations := map[string]string{
		releaseutil.AnnotationResourceGroup: group,
	}
	if len(deps) > 0 {
		encoded, err := json.Marshal(deps)
		if err != nil {
			panic(err)
		}
		annotations[releaseutil.AnnotationDependsOnResourceGroups] = string(encoded)
	}
	return annotations
}

func newChart(name string, deps ...*chart.Dependency) *chart.Chart {
	c := &chart.Chart{
		Metadata: &chart.Metadata{
			Name:         name,
			Version:      "0.1.0",
			APIVersion:   chart.APIVersionV2,
			Dependencies: deps,
		},
	}
	for _, dep := range deps {
		if dep == nil || !dep.Enabled {
			continue
		}
		subName := dep.Alias
		if subName == "" {
			subName = dep.Name
		}
		c.AddDependency(newChart(subName))
	}
	return c
}

func enabledDependency(name string, dependsOn ...string) *chart.Dependency {
	return &chart.Dependency{
		Name:      name,
		Version:   "0.1.0",
		Enabled:   true,
		DependsOn: dependsOn,
	}
}

func aliasedDependency(name, alias string, dependsOn ...string) *chart.Dependency {
	return &chart.Dependency{
		Name:      name,
		Version:   "0.1.0",
		Alias:     alias,
		Enabled:   true,
		DependsOn: dependsOn,
	}
}

func pipelineChart(deps ...*chart.Dependency) *chart.Chart {
	c := &chart.Chart{
		Metadata: &chart.Metadata{
			Name:         "parent",
			Version:      "0.1.0",
			APIVersion:   chart.APIVersionV2,
			Dependencies: deps,
		},
	}
	added := make(map[string]bool)
	for _, dep := range deps {
		if added[dep.Name] {
			continue
		}
		added[dep.Name] = true
		c.AddDependency(&chart.Chart{
			Metadata: &chart.Metadata{
				Name:       dep.Name,
				Version:    "0.1.0",
				APIVersion: chart.APIVersionV2,
			},
		})
	}
	return c
}

func pipelineDependency(name, alias string, dependsOn ...string) *chart.Dependency {
	return &chart.Dependency{
		Name:      name,
		Version:   "0.1.0",
		Alias:     alias,
		DependsOn: dependsOn,
	}
}

func kitchenSinkFixture() (*chart.Chart, []releaseutil.Manifest) {
	primary := newChart("primary-db")
	worker := newChart("worker")
	app := newChart("app", enabledDependency("worker"))
	app.SetDependencies(worker)
	vendored := newChart("vendored")

	parent := newChart(
		"parent",
		aliasedDependency("postgres", "primary-db"),
		enabledDependency("app", "primary-db"),
	)
	parent.SetDependencies(primary, app, vendored)

	manifests := []releaseutil.Manifest{
		makeManifest("primary", "parent/charts/primary-db/templates/primary.yaml", nil),
		makeManifest("worker", "parent/charts/app/charts/worker/templates/worker.yaml", nil),
		makeManifest("app-sub", "parent/charts/app/templates/app.yaml", groupAnnotations("app-sub")),
		makeManifest("vendored", "parent/charts/vendored/templates/vendored.yaml", nil),
		makeManifest("db", "parent/templates/db.yaml", groupAnnotations("db")),
		makeManifest("app", "parent/templates/app.yaml", groupAnnotations("app", "db")),
		makeManifest("metrics", "parent/templates/metrics.yaml", groupAnnotations("metrics")),
		makeManifest("plain", "parent/templates/plain.yaml", nil),
	}
	return parent, manifests
}
