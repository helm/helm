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
	releaseutil "helm.sh/helm/v4/internal/release/v2/util"
)

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
