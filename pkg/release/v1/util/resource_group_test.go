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
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseResourceGroups_NoAnnotations(t *testing.T) {
	t.Parallel()

	result, warnings := parseResourceGroups(t,
		makeManifest("plain-a", "chart/templates/plain-a.yaml", nil),
		makeManifest("plain-b", "chart/templates/plain-b.yaml", nil),
	)

	assert.Empty(t, result.Groups)
	assert.Empty(t, result.GroupDeps)
	require.Len(t, result.Unsequenced, 2)
	assert.Equal(t, []string{"chart/templates/plain-a.yaml", "chart/templates/plain-b.yaml"}, manifestPaths(result.Unsequenced))
	assert.Empty(t, warnings)
}

func TestParseResourceGroups_GroupWithDependency(t *testing.T) {
	t.Parallel()

	result, warnings, batches := parseResourceGroupBatches(t,
		makeManifest("database", "chart/templates/database.yaml", map[string]string{
			AnnotationResourceGroup: "database",
		}),
		makeManifest("app", "chart/templates/app.yaml", map[string]string{
			AnnotationResourceGroup:           "app",
			AnnotationDependsOnResourceGroups: `["database"]`,
		}),
	)

	require.Len(t, result.Groups, 2)
	assert.Equal(t, [][]string{{"database"}, {"app"}}, batches)
	assert.Empty(t, result.Unsequenced)
	assert.Empty(t, warnings)
}

func TestParseResourceGroups_MultipleResourcesPerGroup(t *testing.T) {
	t.Parallel()

	result, warnings, batches := parseResourceGroupBatches(t,
		makeManifest("database-config", "chart/templates/database-config.yaml", map[string]string{
			AnnotationResourceGroup: "database",
		}),
		makeManifest("database-secret", "chart/templates/database-secret.yaml", map[string]string{
			AnnotationResourceGroup: "database",
		}),
		makeManifest("app", "chart/templates/app.yaml", map[string]string{
			AnnotationResourceGroup:           "app",
			AnnotationDependsOnResourceGroups: `["database"]`,
		}),
	)

	require.Len(t, result.Groups["database"], 2)
	assert.Equal(t, []string{
		"chart/templates/database-config.yaml",
		"chart/templates/database-secret.yaml",
	}, manifestPaths(result.Groups["database"]))
	assert.Equal(t, [][]string{{"database"}, {"app"}}, batches)
	assert.Empty(t, warnings)
}

func TestParseResourceGroups_ResourceAssignedToMultipleGroups(t *testing.T) {
	t.Parallel()

	_, _, err := ParseResourceGroups([]Manifest{
		makeManifest("shared", "chart/templates/database.yaml", map[string]string{
			AnnotationResourceGroup: "database",
		}),
		makeManifest("shared", "chart/templates/cache.yaml", map[string]string{
			AnnotationResourceGroup: "cache",
		}),
	})

	require.Error(t, err)
	assert.ErrorContains(t, err, "assigned to multiple resource groups")
	assert.ErrorContains(t, err, "database")
	assert.ErrorContains(t, err, "cache")
}

func TestParseResourceGroups_NonExistentGroupReferenceWarning(t *testing.T) {
	t.Parallel()

	result, warnings := parseResourceGroups(t,
		makeManifest("app", "chart/templates/app.yaml", map[string]string{
			AnnotationResourceGroup:           "app",
			AnnotationDependsOnResourceGroups: `["missing"]`,
		}),
	)

	assert.Empty(t, result.Groups)
	require.Len(t, result.Unsequenced, 1)
	assert.Equal(t, []string{"chart/templates/app.yaml"}, manifestPaths(result.Unsequenced))
	require.Len(t, warnings, 1)
	assert.Contains(t, warnings[0], "non-existent group")
	assert.Contains(t, warnings[0], "missing")
}

func TestParseResourceGroups_CascadingMissingDeps(t *testing.T) {
	t.Parallel()

	manifests := []Manifest{
		makeManifest("app", "chart/templates/app.yaml", map[string]string{
			AnnotationResourceGroup:           "app",
			AnnotationDependsOnResourceGroups: `["database"]`,
		}),
		makeManifest("database", "chart/templates/database.yaml", map[string]string{
			AnnotationResourceGroup:           "database",
			AnnotationDependsOnResourceGroups: `["missing"]`,
		}),
	}

	for range 64 {
		result, warnings := parseResourceGroups(t, manifests...)

		assert.Empty(t, result.Groups)
		assert.Empty(t, result.GroupDeps)
		require.Len(t, result.Unsequenced, 2)
		assert.ElementsMatch(t, []string{
			"chart/templates/app.yaml",
			"chart/templates/database.yaml",
		}, manifestPaths(result.Unsequenced))
		require.Len(t, warnings, 2)
		assert.Contains(t, warnings[0], `group "database" depends-on non-existent group "missing"`)
		assert.Contains(t, warnings[1], `group "app" depends-on non-existent group "database"`)

		dag, err := BuildResourceGroupDAG(result)
		require.NoError(t, err)
		batches, err := dag.GetBatches()
		require.NoError(t, err)
		assert.Empty(t, batches)
	}
}

func TestParseResourceGroups_InvalidDependsOnJSON(t *testing.T) {
	t.Parallel()

	result, warnings := parseResourceGroups(t,
		makeManifest("app", "chart/templates/app.yaml", map[string]string{
			AnnotationResourceGroup:           "app",
			AnnotationDependsOnResourceGroups: `not-valid-json`,
		}),
	)

	assert.Empty(t, result.Groups)
	require.Len(t, result.Unsequenced, 1)
	assert.Equal(t, []string{"chart/templates/app.yaml"}, manifestPaths(result.Unsequenced))
	require.Len(t, warnings, 1)
	assert.Contains(t, warnings[0], "invalid JSON")
}

func TestParseResourceGroups_MixedSequencedAndUnsequenced(t *testing.T) {
	t.Parallel()

	result, warnings := parseResourceGroups(t,
		makeManifest("database", "chart/templates/database.yaml", map[string]string{
			AnnotationResourceGroup: "database",
		}),
		makeManifest("plain", "chart/templates/plain.yaml", nil),
	)

	require.Len(t, result.Groups, 1)
	assert.Contains(t, result.Groups, "database")
	require.Len(t, result.Unsequenced, 1)
	assert.Equal(t, []string{"chart/templates/plain.yaml"}, manifestPaths(result.Unsequenced))
	assert.Empty(t, warnings)
}

func TestResourceGroupDAG_CycleDetection(t *testing.T) {
	t.Parallel()

	result, _ := parseResourceGroups(t,
		makeManifest("a", "chart/templates/a.yaml", map[string]string{
			AnnotationResourceGroup:           "a",
			AnnotationDependsOnResourceGroups: `["c"]`,
		}),
		makeManifest("b", "chart/templates/b.yaml", map[string]string{
			AnnotationResourceGroup:           "b",
			AnnotationDependsOnResourceGroups: `["a"]`,
		}),
		makeManifest("c", "chart/templates/c.yaml", map[string]string{
			AnnotationResourceGroup:           "c",
			AnnotationDependsOnResourceGroups: `["b"]`,
		}),
	)

	dag, err := BuildResourceGroupDAG(result)
	require.NoError(t, err)

	batches, err := dag.GetBatches()
	require.Error(t, err)
	assert.Nil(t, batches)
	assert.ErrorContains(t, err, "cycle")
}

func TestParseResourceGroups_DeduplicatesDependencies(t *testing.T) {
	t.Parallel()

	result, warnings, batches := parseResourceGroupBatches(t,
		makeManifest("database", "chart/templates/database.yaml", map[string]string{
			AnnotationResourceGroup: "database",
		}),
		makeManifest("app-config", "chart/templates/app-config.yaml", map[string]string{
			AnnotationResourceGroup:           "app",
			AnnotationDependsOnResourceGroups: `["database"]`,
		}),
		makeManifest("app-secret", "chart/templates/app-secret.yaml", map[string]string{
			AnnotationResourceGroup:           "app",
			AnnotationDependsOnResourceGroups: `["database"]`,
		}),
	)

	assert.Equal(t, []string{"database"}, result.GroupDeps["app"])
	assert.Equal(t, [][]string{{"database"}, {"app"}}, batches)
	assert.Empty(t, warnings)
}

func TestParseResourceGroups_IsolatedGroupsRemainInBatch0(t *testing.T) {
	t.Parallel()

	_, warnings, batches := parseResourceGroupBatches(t,
		makeManifest("database", "chart/templates/database.yaml", map[string]string{
			AnnotationResourceGroup: "database",
		}),
		makeManifest("metrics", "chart/templates/metrics.yaml", map[string]string{
			AnnotationResourceGroup: "metrics",
		}),
		makeManifest("app", "chart/templates/app.yaml", map[string]string{
			AnnotationResourceGroup:           "app",
			AnnotationDependsOnResourceGroups: `["database"]`,
		}),
	)

	assert.Equal(t, [][]string{{"database", "metrics"}, {"app"}}, batches)
	assert.Empty(t, warnings)
}

func TestParseResourceGroups_ComplexDAG(t *testing.T) {
	t.Parallel()

	_, warnings, batches := parseResourceGroupBatches(t,
		makeManifest("cache", "chart/templates/cache.yaml", map[string]string{
			AnnotationResourceGroup: "cache",
		}),
		makeManifest("database", "chart/templates/database.yaml", map[string]string{
			AnnotationResourceGroup: "database",
		}),
		makeManifest("queue", "chart/templates/queue.yaml", map[string]string{
			AnnotationResourceGroup: "queue",
		}),
		makeManifest("api", "chart/templates/api.yaml", map[string]string{
			AnnotationResourceGroup:           "api",
			AnnotationDependsOnResourceGroups: `["database","queue"]`,
		}),
		makeManifest("worker", "chart/templates/worker.yaml", map[string]string{
			AnnotationResourceGroup:           "worker",
			AnnotationDependsOnResourceGroups: `["cache","queue"]`,
		}),
		makeManifest("frontend", "chart/templates/frontend.yaml", map[string]string{
			AnnotationResourceGroup:           "frontend",
			AnnotationDependsOnResourceGroups: `["api"]`,
		}),
	)

	assert.Equal(t, [][]string{
		{"cache", "database", "queue"},
		{"api", "worker"},
		{"frontend"},
	}, batches)
	assert.Empty(t, warnings)
}

func parseResourceGroups(t *testing.T, manifests ...Manifest) (ResourceGroupResult, []string) {
	t.Helper()

	result, warnings, err := ParseResourceGroups(manifests)
	require.NoError(t, err)

	return result, warnings
}

func parseResourceGroupBatches(t *testing.T, manifests ...Manifest) (ResourceGroupResult, []string, [][]string) {
	t.Helper()

	result, warnings := parseResourceGroups(t, manifests...)
	dag, err := BuildResourceGroupDAG(result)
	require.NoError(t, err)

	batches, err := dag.GetBatches()
	require.NoError(t, err)

	return result, warnings, batches
}

func makeManifest(name, sourcePath string, annotations map[string]string) Manifest {
	var content strings.Builder
	fmt.Fprintf(&content, "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: %s\n", name)
	if len(annotations) > 0 {
		content.WriteString("  annotations:\n")
		for key, value := range annotations {
			fmt.Fprintf(&content, "    %s: %q\n", key, value)
		}
	}

	head := &SimpleHead{
		Version: "v1",
		Kind:    "ConfigMap",
		Metadata: &struct {
			Name        string            `json:"name"`
			Namespace   string            `json:"namespace,omitempty"`
			Annotations map[string]string `json:"annotations"`
		}{
			Name:        name,
			Annotations: annotations,
		},
	}

	return Manifest{
		Name:    sourcePath,
		Content: content.String(),
		Head:    head,
	}
}

func manifestPaths(manifests []Manifest) []string {
	paths := make([]string, 0, len(manifests))
	for _, manifest := range manifests {
		paths = append(paths, manifest.Name)
	}

	return paths
}
