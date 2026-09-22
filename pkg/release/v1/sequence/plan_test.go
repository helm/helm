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
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	releaseutil "helm.sh/helm/v4/pkg/release/v1/util"
)

func TestPlanReverse(t *testing.T) {
	t.Parallel()

	manifests := []releaseutil.Manifest{
		makeManifest("base", "parent/templates/base.yaml", groupAnnotations("base")),
		makeManifest("left", "parent/templates/left.yaml", groupAnnotations("left", "base")),
		makeManifest("right", "parent/templates/right.yaml", groupAnnotations("right", "base")),
		makeManifest("top", "parent/templates/top.yaml", groupAnnotations("top", "left", "right")),
		makeManifest("plain", "parent/templates/plain.yaml", nil),
	}
	plan, err := Build(newChart("parent"), manifests)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(plan.Batches), 4)

	original := batchSummaries(plan)
	reversed := plan.Reverse()
	require.NotNil(t, reversed)
	require.Len(t, reversed.Batches, len(plan.Batches))

	for i := range reversed.Batches {
		assert.Equal(t, original[len(original)-1-i], summarizeBatch(reversed.Batches[i]))
	}
	assert.Equal(t, original, batchSummaries(plan), "Reverse must not mutate the original plan")
	assert.Equal(t, plan.Levels, reversed.Levels)
	assert.Equal(t, plan.Warnings, reversed.Warnings)
	assert.Equal(t, original, batchSummaries(reversed.Reverse()))
}

func TestDisplayPath(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"parent":                        "parent",
		"parent/charts/db":              "parent/db",
		"parent/charts/db/charts/redis": "parent/db/redis",
		"":                              "",
	}

	for input, expected := range tests {
		assert.Equal(t, expected, DisplayPath(input))
	}
}

func TestBatchManifests(t *testing.T) {
	t.Parallel()

	first := makeManifest("first", "parent/templates/first.yaml", nil)
	second := makeManifest("second", "parent/templates/second.yaml", nil)
	third := makeManifest("third", "parent/templates/third.yaml", nil)
	batch := Batch{
		Groups: []Group{
			{Name: "a", Manifests: []releaseutil.Manifest{first, second}},
			{Name: "b", Manifests: []releaseutil.Manifest{third}},
		},
	}

	assert.Equal(t, []releaseutil.Manifest{first, second, third}, batch.Manifests())
}

func TestReadinessAnnotationValues(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "helm.sh/readiness-success", releaseutil.AnnotationReadinessSuccess)
	assert.Equal(t, "helm.sh/readiness-failure", releaseutil.AnnotationReadinessFailure)
}

func TestPackageImportPurity(t *testing.T) {
	t.Parallel()

	entries, err := os.ReadDir(".")
	require.NoError(t, err)

	allowedHelmImports := map[string]bool{
		"helm.sh/helm/v4/pkg/chart/v2":        true,
		"helm.sh/helm/v4/pkg/chart/v2/util":   true,
		"helm.sh/helm/v4/pkg/release/v1/util": true,
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}

		file, err := parser.ParseFile(token.NewFileSet(), entry.Name(), nil, parser.ImportsOnly)
		require.NoError(t, err)
		for _, imported := range file.Imports {
			path, err := strconv.Unquote(imported.Path.Value)
			require.NoError(t, err)
			if strings.HasPrefix(path, "helm.sh/helm/v4/") {
				assert.Truef(t, allowedHelmImports[path], "forbidden helm import %q in %s", path, entry.Name())
			}
			assert.Falsef(t, strings.HasPrefix(path, "k8s.io/"), "forbidden Kubernetes import %q in %s", path, entry.Name())
		}
	}
}

func TestParseStoredManifests(t *testing.T) {
	t.Parallel()

	stream := "---\n# Source: parent/templates/a.yaml\napiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: a\n  annotations:\n    example.com/key: value\n" +
		"---\n# Source: parent/charts/db/templates/b.yaml\napiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: b\n" +
		"---\napiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: c\n"

	parsed, err := ParseStoredManifests(stream)
	require.NoError(t, err)
	require.Len(t, parsed, 3)
	assert.Equal(t, []string{"parent/templates/a.yaml", "parent/charts/db/templates/b.yaml", "manifest-2"}, manifestNames(parsed))
	assert.Equal(t, "ConfigMap", parsed[0].Head.Kind)
	assert.Equal(t, "a", parsed[0].Head.Metadata.Name)
	assert.Equal(t, "value", parsed[0].Head.Metadata.Annotations["example.com/key"])
	assert.Equal(t, "b", parsed[1].Head.Metadata.Name)
	assert.Equal(t, "c", parsed[2].Head.Metadata.Name)

	_, err = ParseStoredManifests("---\n# Source: bad.yaml\napiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: [\n")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "YAML parse error")

	flat, err := Build(nil, parsed)
	require.NoError(t, err)
	assertPlanComplete(t, flat, parsed)
	require.Len(t, flat.Batches, 1)
	assert.Empty(t, flat.Batches[0].ChartPath)

	db := makeManifest("db", "parent/templates/db.yaml", groupAnnotations("db"))
	app := makeManifest("app", "parent/templates/app.yaml", groupAnnotations("app", "db"))
	stored := fmt.Sprintf("---\n# Source: %s\n%s---\n# Source: %s\n%s", db.Name, db.Content, app.Name, app.Content)
	parsedStored, err := ParseStoredManifests(stored)
	require.NoError(t, err)

	fromStored, err := Build(newChart("parent"), parsedStored)
	require.NoError(t, err)
	fromDirect, err := Build(newChart("parent"), []releaseutil.Manifest{db, app})
	require.NoError(t, err)
	assert.Equal(t, batchSummaries(fromDirect), batchSummaries(fromStored))
	assertPlanComplete(t, fromStored, parsedStored)
}
