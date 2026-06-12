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

	// Simulates post-ProcessDependencies state: the aliased entry has had
	// Name rewritten to its alias, and depends-on entries were rewritten to
	// effective names. (The real-pipeline path is covered by the
	// TestProcessDependencies_* tests below.)
	c := newChart("parent",
		&chart.Dependency{Name: "primary-db", Alias: "primary-db", Enabled: true},
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

	_, err := BuildSubchartDAG(c)
	require.Error(t, err)
	assert.ErrorContains(t, err, `depends-on unknown or disabled subchart "cache"`)
}

func TestBuildSubchartDAG_DisabledSubchartNotReferenced(t *testing.T) {
	t.Parallel()

	c := newChart("parent",
		&chart.Dependency{Name: "cache", Enabled: false},
		enabledDependency("app"),
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

func TestBuildSubchartDAG_AnnotationBasedParentDependencies(t *testing.T) {
	t.Parallel()

	c := newChart("parent",
		enabledDependency("postgres"),
		enabledDependency("nginx"),
	)
	c.Metadata.Annotations = map[string]string{
		AnnotationDependsOnSubcharts: `["nginx"]`,
	}

	assertBatches(t, c, [][]string{{"nginx", "postgres"}})
}

func TestBuildSubchartDAG_HIPExample(t *testing.T) {
	t.Parallel()

	c := newChart("foo",
		enabledDependency("nginx"),
		enabledDependency("rabbitmq"),
		enabledDependency("bar", "nginx", "rabbitmq"),
	)
	c.Metadata.Annotations = map[string]string{
		AnnotationDependsOnSubcharts: `["bar", "rabbitmq"]`,
	}

	assertBatches(t, c, [][]string{{"nginx", "rabbitmq"}, {"bar"}})
}

func TestBuildSubchartDAG_MixedDeclarations(t *testing.T) {
	t.Parallel()

	c := newChart("parent",
		enabledDependency("database"),
		enabledDependency("api", "database"),
		enabledDependency("worker"),
	)
	c.Metadata.Annotations = map[string]string{
		AnnotationDependsOnSubcharts: `["worker"]`,
	}

	assertBatches(t, c, [][]string{{"database", "worker"}, {"api"}})
}

func TestBuildSubchartDAG_InvalidAnnotationJSON(t *testing.T) {
	t.Parallel()

	c := newChart("parent", enabledDependency("api"))
	c.Metadata.Annotations = map[string]string{
		AnnotationDependsOnSubcharts: `["api",`,
	}

	_, err := BuildSubchartDAG(c)
	require.Error(t, err)
	assert.ErrorContains(t, err, "parsing "+AnnotationDependsOnSubcharts+" annotation")
}

func TestBuildSubchartDAG_ObjectAnnotationRejected(t *testing.T) {
	t.Parallel()

	c := newChart("parent",
		enabledDependency("postgres"),
		enabledDependency("nginx"),
	)
	c.Metadata.Annotations = map[string]string{
		AnnotationDependsOnSubcharts: `{"nginx":["postgres"]}`,
	}

	_, err := BuildSubchartDAG(c)
	require.Error(t, err)
	assert.ErrorContains(t, err, "JSON string array")
}

func TestBuildSubchartDAG_NonExistentReference(t *testing.T) {
	t.Parallel()

	c := newChart("parent", enabledDependency("app", "missing"))

	_, err := BuildSubchartDAG(c)
	require.Error(t, err)
	assert.ErrorContains(t, err, `depends-on unknown or disabled subchart "missing"`)
}

func TestBuildSubchartDAG_AnnotationUnknownSubchart(t *testing.T) {
	t.Parallel()

	c := newChart("parent", enabledDependency("postgres"))
	c.Metadata.Annotations = map[string]string{
		AnnotationDependsOnSubcharts: `["app"]`,
	}

	_, err := BuildSubchartDAG(c)
	require.Error(t, err)
	assert.ErrorContains(t, err, "unknown or disabled subchart")
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
	// Replace auto-stubs with real chart objects so nested DAG validation works.
	root.SetDependencies(
		&chart.Chart{Metadata: &chart.Metadata{Name: "database"}},
		nested,
	)

	assertBatches(t, root, [][]string{{"database"}, {"application"}})
	assertBatches(t, nested, [][]string{{"cache"}, {"worker"}})
}

// TestBuildSubchartDAG_MetadataOnlyNoLoadedDeps locks in the post-rewrite
// contract: when c.Metadata.Dependencies is non-empty but c.Dependencies()
// is empty (e.g., chart loaded but ProcessDependencies disabled everything),
// the DAG should have no nodes and produce no error.
func TestBuildSubchartDAG_MetadataOnlyNoLoadedDeps(t *testing.T) {
	t.Parallel()

	c := &chart.Chart{
		Metadata: &chart.Metadata{
			Name: "parent",
			Dependencies: []*chart.Dependency{
				{Name: "ghost", Enabled: true},
			},
		},
	}
	// Note: no AddDependency call — c.Dependencies() is empty.

	batches := batchesForChart(t, c)
	assert.Empty(t, batches, "no loaded deps should yield empty DAG")
}

// TestBuildSubchartDAG_AnnotationReferencesUnloadedDep verifies that an
// annotation referencing a subchart present in metadata but pruned from
// c.Dependencies() (e.g., disabled by ProcessDependencies) produces an error.
func TestBuildSubchartDAG_AnnotationReferencesUnloadedDep(t *testing.T) {
	t.Parallel()

	c := &chart.Chart{
		Metadata: &chart.Metadata{
			Name: "parent",
			Dependencies: []*chart.Dependency{
				{Name: "loaded-dep", Enabled: true},
				{Name: "pruned-dep", Enabled: true},
			},
			Annotations: map[string]string{
				AnnotationDependsOnSubcharts: `["pruned-dep"]`,
			},
		},
	}
	// Only loaded-dep is in c.Dependencies(); pruned-dep is not.
	c.AddDependency(&chart.Chart{Metadata: &chart.Metadata{Name: "loaded-dep"}})

	_, err := BuildSubchartDAG(c)
	require.Error(t, err)
	assert.ErrorContains(t, err, `unknown or disabled subchart "pruned-dep"`)
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
	c := &chart.Chart{
		Metadata: &chart.Metadata{
			Name:         name,
			Dependencies: deps,
		},
	}
	// Simulate post-ProcessDependencies state: enabled deps appear in
	// c.Dependencies() under their effective name (alias if set).
	for _, dep := range deps {
		if dep == nil || !dep.Enabled {
			continue
		}
		subName := dep.Alias
		if subName == "" {
			subName = dep.Name
		}
		c.AddDependency(&chart.Chart{
			Metadata: &chart.Metadata{Name: subName},
		})
	}
	return c
}

func enabledDependency(name string, dependsOn ...string) *chart.Dependency {
	return &chart.Dependency{
		Name:      name,
		Enabled:   true,
		DependsOn: dependsOn,
	}
}

// pipelineChart builds a parent chart whose loaded dependencies are named
// after the entries' ORIGINAL chart names, exactly as loader.LoadDir produces
// them, so tests exercise the real ProcessDependencies pipeline (alias rename
// + depends-on resolution) instead of hand-constructing post-processed state.
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

// TestProcessDependencies_ResolvesDependsOnByOriginalName is the regression
// test for hip-0025-92r. Through the real pipeline, processDependencyEnabled
// rewrites an aliased entry's Name to its alias; before the fix that made the
// original chart name unrecoverable and depends-on: ["postgres"] failed to
// resolve at DAG build time.
func TestProcessDependencies_ResolvesDependsOnByOriginalName(t *testing.T) {
	t.Parallel()

	c := pipelineChart(
		pipelineDependency("postgres", "primary-db"),
		pipelineDependency("app", "", "postgres"),
	)

	require.NoError(t, ProcessDependencies(c, map[string]any{}))

	// The alias rename ran, and the depends-on reference was rewritten to the
	// effective name — which is also what release storage persists, keeping
	// the sequenced uninstall path working.
	assert.Equal(t, "primary-db", c.Metadata.Dependencies[0].Name)
	assert.Equal(t, []string{"primary-db"}, c.Metadata.Dependencies[1].DependsOn)

	assertBatches(t, c, [][]string{{"primary-db"}, {"app"}})
}

func TestProcessDependencies_ResolvesDependsOnByAlias(t *testing.T) {
	t.Parallel()

	c := pipelineChart(
		pipelineDependency("postgres", "primary-db"),
		pipelineDependency("app", "", "primary-db"),
	)

	require.NoError(t, ProcessDependencies(c, map[string]any{}))

	assert.Equal(t, []string{"primary-db"}, c.Metadata.Dependencies[1].DependsOn)
	assertBatches(t, c, [][]string{{"primary-db"}, {"app"}})
}

func TestProcessDependencies_AmbiguousDependsOnRejected(t *testing.T) {
	t.Parallel()

	// The same chart aliased twice makes its original name ambiguous;
	// referencing it by that name must be rejected, not silently resolved.
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
	assertBatches(t, c, [][]string{{"app", "primary-db"}})
}

func TestProcessDependencies_AmbiguousAnnotationRejected(t *testing.T) {
	t.Parallel()

	c := pipelineChart(
		pipelineDependency("postgres", "db1"),
		pipelineDependency("postgres", "db2"),
	)
	c.Metadata.Annotations = map[string]string{
		AnnotationDependsOnSubcharts: `["postgres"]`,
	}

	err := ProcessDependencies(c, map[string]any{})
	require.Error(t, err)
	assert.ErrorContains(t, err, `references ambiguous subchart "postgres"`)
}

func TestProcessDependencies_UnknownDependsOnReportedByDAG(t *testing.T) {
	t.Parallel()

	// Unknown references are left alone by the resolver so BuildSubchartDAG
	// keeps reporting them with its established error message.
	c := pipelineChart(
		pipelineDependency("app", "", "missing"),
	)

	require.NoError(t, ProcessDependencies(c, map[string]any{}))

	_, err := BuildSubchartDAG(c)
	require.Error(t, err)
	assert.ErrorContains(t, err, `unknown or disabled subchart "missing"`)
}

func TestProcessDependencies_DependsOnRewriteIdempotent(t *testing.T) {
	t.Parallel()

	c := pipelineChart(
		pipelineDependency("postgres", "primary-db"),
		pipelineDependency("app", "", "postgres"),
	)

	require.NoError(t, ProcessDependencies(c, map[string]any{}))
	// Lint and the sequenced-uninstall re-render run ProcessDependencies again
	// on the already-processed chart; the second pass must neither error nor
	// corrupt the rewritten references.
	require.NoError(t, ProcessDependencies(c, map[string]any{}))

	assert.Equal(t, []string{"primary-db"}, c.Metadata.Dependencies[1].DependsOn)
	assertBatches(t, c, [][]string{{"primary-db"}, {"app"}})
}

func TestProcessDependencies_PlainAndAliasedSameChart(t *testing.T) {
	t.Parallel()

	// "svc" is deployed both under its own name and under an alias: the bare
	// name matches two subcharts and is ambiguous, while the alias resolves.
	ambiguous := pipelineChart(
		pipelineDependency("svc", ""),
		pipelineDependency("svc", "svc2"),
		pipelineDependency("app", "", "svc"),
	)
	err := ProcessDependencies(ambiguous, map[string]any{})
	require.Error(t, err)
	assert.ErrorContains(t, err, `ambiguous subchart reference "svc"`)

	byAlias := pipelineChart(
		pipelineDependency("svc", ""),
		pipelineDependency("svc", "svc2"),
		pipelineDependency("app", "", "svc2"),
	)
	require.NoError(t, ProcessDependencies(byAlias, map[string]any{}))
	assertBatches(t, byAlias, [][]string{{"svc", "svc2"}, {"app"}})
}

func TestProcessDependencies_SwappedAliasesAmbiguous(t *testing.T) {
	t.Parallel()

	// x is aliased to y while y is aliased to x: each bare name now refers to
	// two different subcharts, so references to either are rejected as
	// ambiguous rather than silently resolved to one of them.
	c := pipelineChart(
		pipelineDependency("x", "y"),
		pipelineDependency("y", "x"),
		pipelineDependency("app", "", "x"),
	)

	err := ProcessDependencies(c, map[string]any{})
	require.Error(t, err)
	assert.ErrorContains(t, err, `ambiguous subchart reference "x"`)
}
