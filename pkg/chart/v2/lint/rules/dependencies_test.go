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
package rules

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	chart "helm.sh/helm/v4/pkg/chart/v2"
	"helm.sh/helm/v4/pkg/chart/v2/lint/support"
	chartutil "helm.sh/helm/v4/pkg/chart/v2/util"
)

func chartWithBadDependencies() chart.Chart {
	badChartDeps := chart.Chart{
		Metadata: &chart.Metadata{
			Name:       "badchart",
			Version:    "0.1.0",
			APIVersion: "v2",
			Dependencies: []*chart.Dependency{
				{
					Name: "sub2",
				},
				{
					Name: "sub3",
				},
			},
		},
	}

	badChartDeps.SetDependencies(
		&chart.Chart{
			Metadata: &chart.Metadata{
				Name:       "sub1",
				Version:    "0.1.0",
				APIVersion: "v2",
			},
		},
		&chart.Chart{
			Metadata: &chart.Metadata{
				Name:       "sub2",
				Version:    "0.1.0",
				APIVersion: "v2",
			},
		},
	)
	return badChartDeps
}

func TestValidateDependencyInChartsDir(t *testing.T) {
	c := chartWithBadDependencies()
	assert.Error(t, validateDependencyInChartsDir(&c), "chart should have been flagged for missing deps in chart directory")
}

func TestValidateDependencyInMetadata(t *testing.T) {
	c := chartWithBadDependencies()
	assert.Error(t, validateDependencyInMetadata(&c), "chart should have been flagged for missing deps in chart metadata")
}

func TestValidateDependenciesUnique(t *testing.T) {
	tests := []struct {
		chart chart.Chart
	}{
		{chart.Chart{
			Metadata: &chart.Metadata{
				Name:       "badchart",
				Version:    "0.1.0",
				APIVersion: "v2",
				Dependencies: []*chart.Dependency{
					{
						Name: "foo",
					},
					{
						Name: "foo",
					},
				},
			},
		}},
		{chart.Chart{
			Metadata: &chart.Metadata{
				Name:       "badchart",
				Version:    "0.1.0",
				APIVersion: "v2",
				Dependencies: []*chart.Dependency{
					{
						Name:  "foo",
						Alias: "bar",
					},
					{
						Name: "bar",
					},
				},
			},
		}},
		{chart.Chart{
			Metadata: &chart.Metadata{
				Name:       "badchart",
				Version:    "0.1.0",
				APIVersion: "v2",
				Dependencies: []*chart.Dependency{
					{
						Name:  "foo",
						Alias: "baz",
					},
					{
						Name:  "bar",
						Alias: "baz",
					},
				},
			},
		}},
	}

	for _, tt := range tests {
		assert.Error(t, validateDependenciesUnique(&tt.chart), "chart should have been flagged for dependency shadowing")
	}
}

func TestDependencies(t *testing.T) {
	tmp := t.TempDir()

	c := chartWithBadDependencies()
	require.NoError(t, chartutil.SaveDir(&c, tmp))
	linter := support.Linter{ChartDir: filepath.Join(tmp, c.Metadata.Name)}

	Dependencies(&linter)
	if !assert.Len(t, linter.Messages, 2, "expected 2 linter errors for bad chart dependencies") {
		for i, msg := range linter.Messages {
			t.Logf("Message: %d, Error: %#v", i, msg)
		}
	}
}
