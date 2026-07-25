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
	"os"
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

func TestValidateDependencyConditions(t *testing.T) {
	newChart := func(deps []*chart.Dependency, values map[string]any, subcharts ...*chart.Chart) *chart.Chart {
		c := &chart.Chart{
			Metadata: &chart.Metadata{
				Name:         "parentchart",
				Version:      "0.1.0",
				APIVersion:   "v2",
				Dependencies: deps,
			},
			Values: values,
		}
		c.SetDependencies(subcharts...)
		return c
	}
	subchart := func(values map[string]any) *chart.Chart {
		return &chart.Chart{
			Metadata: &chart.Metadata{
				Name:       "sub",
				Version:    "0.1.0",
				APIVersion: "v2",
			},
			Values: values,
		}
	}

	tests := []struct {
		name           string
		chart          *chart.Chart
		valueOverrides map[string]any
		wantErr        string
	}{
		{
			name:  "dependency without condition",
			chart: newChart([]*chart.Dependency{{Name: "sub"}}, nil, subchart(nil)),
		},
		{
			name: "condition resolving to a parent chart value",
			chart: newChart(
				[]*chart.Dependency{{Name: "sub", Condition: "sub.enabled"}},
				map[string]any{"sub": map[string]any{"enabled": false}},
				subchart(nil),
			),
		},
		{
			name: "condition resolving to a dependency default value",
			chart: newChart(
				[]*chart.Dependency{{Name: "sub", Condition: "sub.enabled"}},
				nil,
				subchart(map[string]any{"enabled": true}),
			),
		},
		{
			name: "condition resolving to a value override",
			chart: newChart(
				[]*chart.Dependency{{Name: "sub", Condition: "sub.enabled"}},
				nil,
				subchart(nil),
			),
			valueOverrides: map[string]any{"sub": map[string]any{"enabled": false}},
		},
		{
			name: "second condition path resolving to a value",
			chart: newChart(
				[]*chart.Dependency{{Name: "sub", Condition: "sub.enabled,global.sub.enabled"}},
				map[string]any{"global": map[string]any{"sub": map[string]any{"enabled": false}}},
				subchart(nil),
			),
		},
		{
			name: "condition not resolving to a value",
			chart: newChart(
				[]*chart.Dependency{{Name: "sub", Condition: "sub.enabled"}},
				map[string]any{"sub": map[string]any{"nested": false}},
				subchart(nil),
			),
			wantErr: `sub (condition "sub.enabled")`,
		},
		{
			name: "condition of aliased dependency resolving to a dependency default value",
			chart: newChart(
				[]*chart.Dependency{{Name: "sub", Alias: "other", Condition: "other.enabled"}},
				nil,
				subchart(map[string]any{"enabled": false}),
			),
		},
		{
			name: "condition of aliased dependency not resolving to a value",
			chart: newChart(
				[]*chart.Dependency{{Name: "sub", Alias: "other", Condition: "other.enabled"}},
				nil,
				subchart(nil),
			),
			wantErr: `sub (condition "other.enabled")`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateDependencyConditions(tt.chart, tt.valueOverrides)
			if tt.wantErr == "" {
				assert.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, tt.wantErr)
			}
		})
	}
}

func TestDependencies(t *testing.T) {
	tmp := t.TempDir()

	c := chartWithBadDependencies()
	require.NoError(t, chartutil.SaveDir(&c, tmp))
	linter := support.Linter{ChartDir: filepath.Join(tmp, c.Metadata.Name)}

	Dependencies(&linter)
	if l := len(linter.Messages); l != 2 {
		t.Errorf("expected 2 linter errors for bad chart dependencies. Got %d.", l)
		for i, msg := range linter.Messages {
			t.Logf("Message: %d, Error: %#v", i, msg)
		}
	}
}

func TestDependenciesWithValues(t *testing.T) {
	tmp := t.TempDir()

	c := chart.Chart{
		Metadata: &chart.Metadata{
			Name:       "condchart",
			Version:    "0.1.0",
			APIVersion: "v2",
			Dependencies: []*chart.Dependency{
				{Name: "sub", Version: "0.1.0", Condition: "sub.enabled"},
			},
		},
	}
	c.SetDependencies(&chart.Chart{
		Metadata: &chart.Metadata{
			Name:       "sub",
			Version:    "0.1.0",
			APIVersion: "v2",
		},
	})
	require.NoError(t, chartutil.SaveDir(&c, tmp))
	chartDir := filepath.Join(tmp, c.Metadata.Name)

	// The condition does not resolve to any value, warn about it
	linter := support.Linter{ChartDir: chartDir}
	Dependencies(&linter)
	require.Len(t, linter.Messages, 1)
	assert.Equal(t, support.WarningSev, linter.Messages[0].Severity)
	require.ErrorContains(t, linter.Messages[0].Err, `sub (condition "sub.enabled")`)

	// The condition resolves to a value override, no warning
	linter = support.Linter{ChartDir: chartDir}
	DependenciesWithValues(&linter, map[string]any{"sub": map[string]any{"enabled": true}})
	assert.Empty(t, linter.Messages)

	// Conditions are not checked when dependencies are missing from the
	// charts directory, as their default values cannot be resolved
	require.NoError(t, os.RemoveAll(filepath.Join(chartDir, "charts")))
	linter = support.Linter{ChartDir: chartDir}
	Dependencies(&linter)
	require.Len(t, linter.Messages, 1)
	assert.ErrorContains(t, linter.Messages[0].Err, "chart directory is missing these dependencies")
}
