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

package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	chart "helm.sh/helm/v4/internal/chart/v3"
)

func TestBuildSubchartDAG_Empty(t *testing.T) {
	t.Parallel()

	assert.Empty(t, subchartBatches(t, subchartDAGChart("parent")))
}

func TestBuildSubchartDAG_NoDependencies(t *testing.T) {
	t.Parallel()

	c := subchartDAGChart("parent",
		enabledSubchartDependency("nginx"),
		enabledSubchartDependency("rabbitmq"),
		enabledSubchartDependency("postgres"),
	)

	assert.Equal(t, [][]string{{"nginx", "postgres", "rabbitmq"}}, subchartBatches(t, c))
}

func TestBuildSubchartDAG_LinearOrder(t *testing.T) {
	t.Parallel()

	c := subchartDAGChart("parent",
		enabledSubchartDependency("postgres"),
		enabledSubchartDependency("rabbitmq", "postgres"),
		enabledSubchartDependency("app", "rabbitmq"),
	)

	assert.Equal(t, [][]string{{"postgres"}, {"rabbitmq"}, {"app"}}, subchartBatches(t, c))
}

func TestBuildSubchartDAG_AliasResolution(t *testing.T) {
	t.Parallel()

	c := subchartDAGChart("parent",
		&chart.Dependency{Name: "primary-db", Alias: "primary-db", Enabled: true},
		enabledSubchartDependency("app", "primary-db"),
	)

	assert.Equal(t, [][]string{{"primary-db"}, {"app"}}, subchartBatches(t, c))
}

func TestBuildSubchartDAG_DisabledSubchart(t *testing.T) {
	t.Parallel()

	c := subchartDAGChart("parent",
		&chart.Dependency{Name: "cache", Enabled: false},
		enabledSubchartDependency("app", "cache"),
	)

	_, err := BuildSubchartDAG(c)
	require.Error(t, err)
	assert.ErrorContains(t, err, `depends-on unknown or disabled subchart "cache"`)
}

func TestBuildSubchartDAG_DisabledSubchartNotReferenced(t *testing.T) {
	t.Parallel()

	c := subchartDAGChart("parent",
		&chart.Dependency{Name: "cache", Enabled: false},
		enabledSubchartDependency("app"),
	)

	assert.Equal(t, [][]string{{"app"}}, subchartBatches(t, c))
}

func TestBuildSubchartDAG_CycleDetection(t *testing.T) {
	t.Parallel()

	c := subchartDAGChart("parent",
		enabledSubchartDependency("a", "b"),
		enabledSubchartDependency("b", "c"),
		enabledSubchartDependency("c", "a"),
	)

	dag, err := BuildSubchartDAG(c)
	require.NoError(t, err)
	batches, err := dag.GetBatches()
	require.Error(t, err)
	assert.Nil(t, batches)
	assert.ErrorContains(t, err, "cycle")
}

func TestBuildSubchartDAG_AnnotationBasedParentDependencies(t *testing.T) {
	t.Parallel()

	c := subchartDAGChart("parent",
		enabledSubchartDependency("postgres"),
		enabledSubchartDependency("nginx"),
	)
	c.Metadata.Annotations = map[string]string{AnnotationDependsOnSubcharts: `["nginx"]`}

	assert.Equal(t, [][]string{{"nginx", "postgres"}}, subchartBatches(t, c))
}

func TestBuildSubchartDAG_HIPExample(t *testing.T) {
	t.Parallel()

	c := subchartDAGChart("foo",
		enabledSubchartDependency("nginx"),
		enabledSubchartDependency("rabbitmq"),
		enabledSubchartDependency("bar", "nginx", "rabbitmq"),
	)
	c.Metadata.Annotations = map[string]string{AnnotationDependsOnSubcharts: `["bar", "rabbitmq"]`}

	assert.Equal(t, [][]string{{"nginx", "rabbitmq"}, {"bar"}}, subchartBatches(t, c))
}

func TestBuildSubchartDAG_MixedDeclarations(t *testing.T) {
	t.Parallel()

	c := subchartDAGChart("parent",
		enabledSubchartDependency("database"),
		enabledSubchartDependency("api", "database"),
		enabledSubchartDependency("worker"),
	)
	c.Metadata.Annotations = map[string]string{AnnotationDependsOnSubcharts: `["worker"]`}

	assert.Equal(t, [][]string{{"database", "worker"}, {"api"}}, subchartBatches(t, c))
}

func TestBuildSubchartDAG_InvalidAnnotationJSON(t *testing.T) {
	t.Parallel()

	c := subchartDAGChart("parent", enabledSubchartDependency("app"))
	c.Metadata.Annotations = map[string]string{AnnotationDependsOnSubcharts: `["app",`}

	_, err := BuildSubchartDAG(c)
	require.Error(t, err)
	assert.ErrorContains(t, err, "parsing "+AnnotationDependsOnSubcharts+" annotation")
}

func TestBuildSubchartDAG_ObjectAnnotationRejected(t *testing.T) {
	t.Parallel()

	c := subchartDAGChart("parent",
		enabledSubchartDependency("postgres"),
		enabledSubchartDependency("nginx"),
	)
	c.Metadata.Annotations = map[string]string{AnnotationDependsOnSubcharts: `{"nginx":["postgres"]}`}

	_, err := BuildSubchartDAG(c)
	require.Error(t, err)
	assert.ErrorContains(t, err, "JSON string array")
}

func TestBuildSubchartDAG_NonExistentReference(t *testing.T) {
	t.Parallel()

	c := subchartDAGChart("parent", enabledSubchartDependency("app", "missing"))

	_, err := BuildSubchartDAG(c)
	require.Error(t, err)
	assert.ErrorContains(t, err, `depends-on unknown or disabled subchart "missing"`)
}

func TestBuildSubchartDAG_AnnotationUnknownSubchart(t *testing.T) {
	t.Parallel()

	c := subchartDAGChart("parent", enabledSubchartDependency("postgres"))
	c.Metadata.Annotations = map[string]string{AnnotationDependsOnSubcharts: `["app"]`}

	_, err := BuildSubchartDAG(c)
	require.Error(t, err)
	assert.ErrorContains(t, err, `unknown or disabled subchart "app"`)
}

func TestBuildSubchartDAG_NestedSubcharts(t *testing.T) {
	t.Parallel()

	root := subchartDAGChart("parent",
		enabledSubchartDependency("database"),
		enabledSubchartDependency("application", "database"),
	)
	nested := subchartDAGChart("application",
		enabledSubchartDependency("cache"),
		enabledSubchartDependency("worker", "cache"),
	)
	root.SetDependencies(
		&chart.Chart{Metadata: &chart.Metadata{Name: "database"}},
		nested,
	)

	assert.Equal(t, [][]string{{"database"}, {"application"}}, subchartBatches(t, root))
	assert.Equal(t, [][]string{{"cache"}, {"worker"}}, subchartBatches(t, nested))
}

func TestBuildSubchartDAG_StorageDecodedMetadataTrusted(t *testing.T) {
	t.Parallel()

	c := &chart.Chart{Metadata: &chart.Metadata{
		Name: "parent",
		Dependencies: []*chart.Dependency{
			{Name: "db", Enabled: true},
			{Name: "app", Enabled: true, DependsOn: []string{"db"}},
		},
	}}

	assert.Equal(t, [][]string{{"db"}, {"app"}}, subchartBatches(t, c))
}

func TestBuildSubchartDAG_MetadataOnlyNotEnabled_Ignored(t *testing.T) {
	t.Parallel()

	c := &chart.Chart{Metadata: &chart.Metadata{
		Name:         "parent",
		Dependencies: []*chart.Dependency{{Name: "ghost"}},
	}}

	assert.Empty(t, subchartBatches(t, c))
}

func TestBuildSubchartDAG_AnnotationReferencesUnloadedDep(t *testing.T) {
	t.Parallel()

	c := &chart.Chart{Metadata: &chart.Metadata{
		Name: "parent",
		Dependencies: []*chart.Dependency{
			{Name: "loaded", Enabled: true},
			{Name: "pruned", Enabled: true},
		},
		Annotations: map[string]string{AnnotationDependsOnSubcharts: `["pruned"]`},
	}}
	c.AddDependency(&chart.Chart{Metadata: &chart.Metadata{Name: "loaded"}})

	_, err := BuildSubchartDAG(c)
	require.Error(t, err)
	assert.ErrorContains(t, err, `unknown or disabled subchart "pruned"`)
}

func TestProcessDependencies_ResolvesDependsOnByOriginalName(t *testing.T) {
	t.Parallel()

	c := pipelineChart(
		pipelineDependency("postgres", "primary-db"),
		pipelineDependency("app", "", "postgres"),
	)

	require.NoError(t, ProcessDependencies(c, map[string]any{}))
	assert.Equal(t, "primary-db", c.Metadata.Dependencies[0].Name)
	assert.Equal(t, []string{"primary-db"}, c.Metadata.Dependencies[1].DependsOn)
	assert.Equal(t, [][]string{{"primary-db"}, {"app"}}, subchartBatches(t, c))
}

func TestProcessDependencies_ResolvesDependsOnByAlias(t *testing.T) {
	t.Parallel()

	c := pipelineChart(
		pipelineDependency("postgres", "primary-db"),
		pipelineDependency("app", "", "primary-db"),
	)

	require.NoError(t, ProcessDependencies(c, map[string]any{}))
	assert.Equal(t, []string{"primary-db"}, c.Metadata.Dependencies[1].DependsOn)
	assert.Equal(t, [][]string{{"primary-db"}, {"app"}}, subchartBatches(t, c))
}

func TestProcessDependencies_AmbiguousDependsOnRejected(t *testing.T) {
	t.Parallel()

	c := pipelineChart(
		pipelineDependency("postgres", "db1"),
		pipelineDependency("postgres", "db2"),
		pipelineDependency("app", "", "postgres"),
	)

	err := ProcessDependencies(c, map[string]any{})
	require.Error(t, err)
	assert.ErrorContains(t, err, `ambiguous subchart reference "postgres"`)
}

func TestProcessDependencies_RewritesSubchartAnnotation(t *testing.T) {
	t.Parallel()

	c := pipelineChart(
		pipelineDependency("postgres", "primary-db"),
		pipelineDependency("app", ""),
	)
	c.Metadata.Annotations = map[string]string{
		AnnotationDependsOnSubcharts: `["postgres", "app"]`,
	}

	require.NoError(t, ProcessDependencies(c, map[string]any{}))
	assert.Equal(t, `["primary-db","app"]`, c.Metadata.Annotations[AnnotationDependsOnSubcharts])
	assert.Equal(t, [][]string{{"app", "primary-db"}}, subchartBatches(t, c))
}

func subchartBatches(t *testing.T, c *chart.Chart) [][]string {
	t.Helper()

	dag, err := BuildSubchartDAG(c)
	require.NoError(t, err)
	batches, err := dag.GetBatches()
	require.NoError(t, err)
	return batches
}

func subchartDAGChart(name string, deps ...*chart.Dependency) *chart.Chart {
	c := &chart.Chart{Metadata: &chart.Metadata{Name: name, Dependencies: deps}}
	for _, dep := range deps {
		if dep == nil || !dep.Enabled {
			continue
		}
		name := dep.Name
		if dep.Alias != "" {
			name = dep.Alias
		}
		c.AddDependency(&chart.Chart{Metadata: &chart.Metadata{Name: name}})
	}
	return c
}

func enabledSubchartDependency(name string, dependsOn ...string) *chart.Dependency {
	return &chart.Dependency{Name: name, Enabled: true, DependsOn: dependsOn}
}

func pipelineChart(deps ...*chart.Dependency) *chart.Chart {
	c := &chart.Chart{Metadata: &chart.Metadata{
		Name:         "parent",
		Version:      "0.1.0",
		APIVersion:   chart.APIVersionV3,
		Dependencies: deps,
	}}
	added := make(map[string]bool)
	for _, dep := range deps {
		if added[dep.Name] {
			continue
		}
		added[dep.Name] = true
		c.AddDependency(&chart.Chart{Metadata: &chart.Metadata{
			Name:       dep.Name,
			Version:    "0.1.0",
			APIVersion: chart.APIVersionV3,
		}})
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
