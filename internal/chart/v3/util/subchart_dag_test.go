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

func TestBuildSubchartDAG_LinearOrder(t *testing.T) {
	t.Parallel()

	c := subchartDAGChart("parent",
		enabledSubchartDependency("postgres"),
		enabledSubchartDependency("rabbitmq", "postgres"),
		enabledSubchartDependency("app", "rabbitmq"),
	)

	assert.Equal(t, [][]string{{"postgres"}, {"rabbitmq"}, {"app"}}, subchartBatches(t, c))
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

func TestBuildSubchartDAG_InvalidAnnotationJSON(t *testing.T) {
	t.Parallel()

	c := subchartDAGChart("parent", enabledSubchartDependency("app"))
	c.Metadata.Annotations = map[string]string{AnnotationDependsOnSubcharts: `["app",`}

	_, err := BuildSubchartDAG(c)
	require.Error(t, err)
	assert.ErrorContains(t, err, "parsing "+AnnotationDependsOnSubcharts+" annotation")
}

func TestBuildSubchartDAG_AnnotationUnknownSubchart(t *testing.T) {
	t.Parallel()

	c := subchartDAGChart("parent", enabledSubchartDependency("postgres"))
	c.Metadata.Annotations = map[string]string{AnnotationDependsOnSubcharts: `["app"]`}

	_, err := BuildSubchartDAG(c)
	require.Error(t, err)
	assert.ErrorContains(t, err, `unknown or disabled subchart "app"`)
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
