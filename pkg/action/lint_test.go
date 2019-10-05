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
	values                       = make(map[string]interface{})
	namespace                    = "testNamespace"
	strict                       = false
	archivedChartPath            = "../../cmd/helm/testdata/testcharts/compressedchart-0.1.0.tgz"
	archivedChartPathWithHyphens = "../../cmd/helm/testdata/testcharts/compressedchart-with-hyphens-0.1.0.tgz"
	invalidArchivedChartPath     = "../../cmd/helm/testdata/testcharts/invalidcompressedchart0.1.0.tgz"
	chartDirPath                 = "../../cmd/helm/testdata/testcharts/decompressedchart/"
	chartMissingManifest         = "../../cmd/helm/testdata/testcharts/chart-missing-manifest"
	chartSchema                  = "../../cmd/helm/testdata/testcharts/chart-with-schema"
	chartSchemaNegative          = "../../cmd/helm/testdata/testcharts/chart-with-schema-negative"
	chart1MultipleChartLint      = "../../cmd/helm/testdata/testcharts/multiplecharts-lint-chart-1"
	chart2MultipleChartLint      = "../../cmd/helm/testdata/testcharts/multiplecharts-lint-chart-2"
	corruptedTgzChart            = "../../cmd/helm/testdata/testcharts/corrupted-compressed-chart.tgz"
	chartWithNoTemplatesDir      = "../../cmd/helm/testdata/testcharts/chart-with-no-templates-dir"
)

func TestLintChart(t *testing.T) {
	if _, err := lintChart(chartDirPath, values, namespace, strict); err != nil {
		t.Error(err)
	}
	if _, err := lintChart(archivedChartPath, values, namespace, strict); err != nil {
		t.Error(err)
	}
	if _, err := lintChart(archivedChartPathWithHyphens, values, namespace, strict); err != nil {
		t.Error(err)
	}
	if _, err := lintChart(invalidArchivedChartPath, values, namespace, strict); err == nil {
		t.Error("Expected a chart parsing error")
	}
	if _, err := lintChart(chartMissingManifest, values, namespace, strict); err == nil {
		t.Error("Expected a chart parsing error")
	}
	if _, err := lintChart(chartSchema, values, namespace, strict); err != nil {
		t.Error(err)
	}
	if _, err := lintChart(chartSchemaNegative, values, namespace, strict); err != nil {
		t.Error(err)
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
