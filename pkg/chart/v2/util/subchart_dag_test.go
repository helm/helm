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

	chart "helm.sh/helm/v4/pkg/chart/v2"
)

func TestBuildSubchartDAG_Empty(t *testing.T) {
	t.Parallel()

	batches := batchesForChart(t, newChart("parent"))
	assert.Empty(t, batches)
}

func TestBuildSubchartDAG_NoDependencies(t *testing.T) {
	t.Parallel()

	c := newChart("parent",
		enabledDependency("nginx"),
		enabledDependency("rabbitmq"),
		enabledDependency("postgres"),
	)

	assertBatches(t, c, [][]string{{"nginx", "postgres", "rabbitmq"}})
}

func TestBuildSubchartDAG_LinearOrder(t *testing.T) {
	t.Parallel()

	c := newChart("parent",
		enabledDependency("postgres"),
		enabledDependency("rabbitmq", "postgres"),
		enabledDependency("app", "rabbitmq"),
	)

	assertBatches(t, c, [][]string{{"postgres"}, {"rabbitmq"}, {"app"}})
}

func TestBuildSubchartDAG_AliasResolution(t *testing.T) {
	t.Parallel()

	c := newChart("parent",
		aliasedDependency("postgres", "primary-db"),
		enabledDependency("app", "primary-db"),
	)

	assertBatches(t, c, [][]string{{"primary-db"}, {"app"}})
}

func TestBuildSubchartDAG_DisabledSubchart(t *testing.T) {
	t.Parallel()

	c := newChart("parent",
		&chart.Dependency{Name: "cache", Enabled: false},
		enabledDependency("app", "cache"),
	)

	assertBatches(t, c, [][]string{{"app"}})
}

func TestBuildSubchartDAG_CycleDetection(t *testing.T) {
	t.Parallel()

	c := newChart("parent",
		enabledDependency("a", "b"),
		enabledDependency("b", "c"),
		enabledDependency("c", "a"),
	)

	dag, err := BuildSubchartDAG(c)
	require.NoError(t, err)

	batches, err := dag.GetBatches()
	require.Error(t, err)
	assert.Nil(t, batches)
	assert.ErrorContains(t, err, "cycle")
}

func TestBuildSubchartDAG_AnnotationBased(t *testing.T) {
	t.Parallel()

	c := newChart("parent",
		enabledDependency("postgres"),
		enabledDependency("nginx"),
	)
	c.Metadata.Annotations = map[string]string{
		AnnotationDependsOnSubcharts: `{"nginx":["postgres"]}`,
	}

	assertBatches(t, c, [][]string{{"postgres"}, {"nginx"}})
}

func TestBuildSubchartDAG_MixedDeclarations(t *testing.T) {
	t.Parallel()

	c := newChart("parent",
		enabledDependency("database"),
		enabledDependency("api", "database"),
		enabledDependency("worker"),
	)
	c.Metadata.Annotations = map[string]string{
		AnnotationDependsOnSubcharts: `{"worker":["api"]}`,
	}

	assertBatches(t, c, [][]string{{"database"}, {"api"}, {"worker"}})
}

func TestBuildSubchartDAG_InvalidAnnotationJSON(t *testing.T) {
	t.Parallel()

	c := newChart("parent", enabledDependency("api"))
	c.Metadata.Annotations = map[string]string{
		AnnotationDependsOnSubcharts: `{"api":`,
	}

	_, err := BuildSubchartDAG(c)
	require.Error(t, err)
	assert.ErrorContains(t, err, "parsing "+AnnotationDependsOnSubcharts+" annotation")
}

func TestBuildSubchartDAG_NonExistentReference(t *testing.T) {
	t.Parallel()

	c := newChart("parent", enabledDependency("app", "missing"))

	_, err := BuildSubchartDAG(c)
	require.Error(t, err)
	assert.ErrorContains(t, err, `depends-on unknown subchart "missing"`)
}

func TestBuildSubchartDAG_AnnotationUnknownSubchart(t *testing.T) {
	t.Parallel()

	c := newChart("parent", enabledDependency("postgres"))
	c.Metadata.Annotations = map[string]string{
		AnnotationDependsOnSubcharts: `{"app":["postgres"]}`,
	}

	_, err := BuildSubchartDAG(c)
	require.Error(t, err)
	assert.ErrorContains(t, err, "unknown subchart")
}

func TestBuildSubchartDAG_NestedSubcharts(t *testing.T) {
	t.Parallel()

	root := newChart("parent",
		enabledDependency("database"),
		enabledDependency("application", "database"),
	)
	nested := newChart("application",
		enabledDependency("cache"),
		enabledDependency("worker", "cache"),
	)
	root.AddDependency(nested)

	assertBatches(t, root, [][]string{{"database"}, {"application"}})
	assertBatches(t, nested, [][]string{{"cache"}, {"worker"}})
}

func assertBatches(t *testing.T, c *chart.Chart, expected [][]string) {
	t.Helper()
	assert.Equal(t, expected, batchesForChart(t, c))
}

func batchesForChart(t *testing.T, c *chart.Chart) [][]string {
	t.Helper()

	dag, err := BuildSubchartDAG(c)
	require.NoError(t, err)

	batches, err := dag.GetBatches()
	require.NoError(t, err)

	return batches
}

func newChart(name string, deps ...*chart.Dependency) *chart.Chart {
	return &chart.Chart{
		Metadata: &chart.Metadata{
			Name:         name,
			Dependencies: deps,
		},
	}
}

func enabledDependency(name string, dependsOn ...string) *chart.Dependency {
	return &chart.Dependency{
		Name:      name,
		Enabled:   true,
		DependsOn: dependsOn,
	}
}

func aliasedDependency(name, alias string, dependsOn ...string) *chart.Dependency {
	return &chart.Dependency{
		Name:      name,
		Alias:     alias,
		Enabled:   true,
		DependsOn: dependsOn,
	}
}
