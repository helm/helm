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
	"go/parser"
	"go/token"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	releaseutil "helm.sh/helm/v4/internal/release/v2/manifest"
)

func TestPlanReverse(t *testing.T) {
	t.Parallel()

	first := Batch{
		ChartPath: "parent/charts/db", Depth: 1, Wait: true,
		Manifests: []releaseutil.Manifest{{Name: "db/templates/config.yaml"}},
	}
	second := Batch{
		ChartPath: "parent", Depth: 0, Wait: true,
		Manifests: []releaseutil.Manifest{{Name: "parent/templates/app.yaml"}},
	}
	plan := &Plan{
		Batches:  []Batch{first, second},
		Levels:   []ChartLevel{{Path: "parent"}},
		Warnings: []Warning{{Kind: WarningKindUndeclaredSubchart, Message: "undeclared"}},
	}

	reversed := plan.Reverse()
	require.NotNil(t, reversed)
	assert.Equal(t, []Batch{second, first}, reversed.Batches)
	assert.Equal(t, []Batch{first, second}, plan.Batches, "Reverse must not mutate the original plan")
	require.NotEmpty(t, reversed.Levels)
	require.NotEmpty(t, reversed.Warnings)
	assert.Same(t, &plan.Levels[0], &reversed.Levels[0])
	assert.Same(t, &plan.Warnings[0], &reversed.Warnings[0])
	assert.Equal(t, plan, reversed.Reverse())
	assert.Nil(t, (*Plan)(nil).Reverse())
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

func TestPackageImportPurity(t *testing.T) {
	t.Parallel()

	entries, err := os.ReadDir(".")
	require.NoError(t, err)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}

		file, err := parser.ParseFile(token.NewFileSet(), entry.Name(), nil, parser.ImportsOnly)
		require.NoError(t, err)
		for _, imported := range file.Imports {
			path, err := strconv.Unquote(imported.Path.Value)
			require.NoError(t, err)
			for _, forbidden := range []string{
				"helm.sh/helm/v4/pkg/action",
				"helm.sh/helm/v4/pkg/cmd",
				"helm.sh/helm/v4/pkg/kube",
			} {
				assert.NotEqual(t, forbidden, path)
				assert.Falsef(t, strings.HasPrefix(path, forbidden+"/"), "forbidden import %q in %s", path, entry.Name())
			}
		}
	}
}
