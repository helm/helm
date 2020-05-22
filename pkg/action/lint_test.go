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

package action

import (
	"testing"
)

var (
	values                  = make(map[string]interface{})
	namespace               = "testNamespace"
	strict                  = false
	chart1MultipleChartLint = "testdata/charts/multiplecharts-lint-chart-1"
	chart2MultipleChartLint = "testdata/charts/multiplecharts-lint-chart-2"
	corruptedTgzChart       = "testdata/charts/corrupted-compressed-chart.tgz"
	chartWithNoTemplatesDir = "testdata/charts/chart-with-no-templates-dir"
)

func TestLintChart(t *testing.T) {
	tests := []struct {
		name      string
		chartPath string
		err       bool
	}{
		{
			name:      "decompressed-chart",
			chartPath: "testdata/charts/decompressedchart/",
		},
		{
			name:      "archived-chart-path",
			chartPath: "testdata/charts/compressedchart-0.1.0.tgz",
		},
		{
			name:      "archived-chart-path-with-hyphens",
			chartPath: "testdata/charts/compressedchart-with-hyphens-0.1.0.tgz",
		},
		{
			name:      "archived-tar-gz-chart-path",
			chartPath: "testdata/charts/compressedchart-0.1.0.tar.gz",
		},
		{
			name:      "invalid-archived-chart-path",
			chartPath: "testdata/charts/invalidcompressedchart0.1.0.tgz",
			err:       true,
		},
		{
			name:      "chart-missing-manifest",
			chartPath: "testdata/charts/chart-missing-manifest",
			err:       true,
		},
		{
			name:      "chart-with-schema",
			chartPath: "testdata/charts/chart-with-schema",
		},
		{
			name:      "chart-with-schema-negative",
			chartPath: "testdata/charts/chart-with-schema-negative",
		},
		{
			name:      "pre-release-chart",
			chartPath: "testdata/charts/pre-release-chart-0.1.0-alpha.tgz",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := lintChart(tt.chartPath, map[string]interface{}{}, namespace, strict)
			switch {
			case err != nil && !tt.err:
				t.Errorf("%s", err)
			case err == nil && tt.err:
				t.Errorf("Expected a chart parsing error")
			}
		})
	}
}

func TestNonExistentChart(t *testing.T) {
	t.Run("should error out for non existent tgz chart", func(t *testing.T) {
		testCharts := []string{"non-existent-chart.tgz"}
		expectedError := "unable to open tarball: open non-existent-chart.tgz: no such file or directory"
		testLint := NewLint()

		result := testLint.Run(testCharts, values)
		if len(result.Errors) != 1 {
			t.Error("expected one error, but got", len(result.Errors))
		}

		actual := result.Errors[0].Error()
		if actual != expectedError {
			t.Errorf("expected '%s', but got '%s'", expectedError, actual)
		}
	})

	t.Run("should error out for corrupted tgz chart", func(t *testing.T) {
		testCharts := []string{corruptedTgzChart}
		expectedEOFError := "unable to extract tarball: EOF"
		testLint := NewLint()

		result := testLint.Run(testCharts, values)
		if len(result.Errors) != 1 {
			t.Error("expected one error, but got", len(result.Errors))
		}

		actual := result.Errors[0].Error()
		if actual != expectedEOFError {
			t.Errorf("expected '%s', but got '%s'", expectedEOFError, actual)
		}
	})
}

func TestLint_MultipleCharts(t *testing.T) {
	testCharts := []string{chart2MultipleChartLint, chart1MultipleChartLint}
	testLint := NewLint()
	if result := testLint.Run(testCharts, values); len(result.Errors) > 0 {
		t.Error(result.Errors)
	}
}

func TestLint_EmptyResultErrors(t *testing.T) {
	testCharts := []string{chart2MultipleChartLint}
	testLint := NewLint()
	if result := testLint.Run(testCharts, values); len(result.Errors) > 0 {
		t.Error("Expected no error, got more")
	}
}

func TestLint_ChartWithWarnings(t *testing.T) {
	t.Run("should pass when not strict", func(t *testing.T) {
		testCharts := []string{chartWithNoTemplatesDir}
		testLint := NewLint()
		testLint.Strict = false
		if result := testLint.Run(testCharts, values); len(result.Errors) > 0 {
			t.Error("Expected no error, got more")
		}
	})

	t.Run("should fail with errors when strict", func(t *testing.T) {
		testCharts := []string{chartWithNoTemplatesDir}
		testLint := NewLint()
		testLint.Strict = true
		if result := testLint.Run(testCharts, values); len(result.Errors) != 1 {
			t.Error("expected one error, but got", len(result.Errors))
		}
	})
}
