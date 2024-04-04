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
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var (
	values                  = make(map[string]interface{})
	namespace               = "testNamespace"
	chart1MultipleChartLint = "testdata/charts/multiplecharts-lint-chart-1"
	chart2MultipleChartLint = "testdata/charts/multiplecharts-lint-chart-2"
	corruptedTgzChart       = "testdata/charts/corrupted-compressed-chart.tgz"
	chartWithNoTemplatesDir = "testdata/charts/chart-with-no-templates-dir"
)

func TestLintChart(t *testing.T) {
	workingDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to determine the current working directory")
	}

	// Actual directory that `helm lint` uses
	helmLintTempDirPrefix := `/tmp/helm-lint`

	tests := []struct {
		name              string
		chartPath         string
		chartName         string
		err               bool
		enableFullPath    bool
		isCompressedChart bool
	}{
		{
			name:      "decompressed-chart",
			chartPath: "testdata/charts/decompressedchart/",
		},
		{
			name:              "archived-chart-path",
			chartPath:         "testdata/charts/compressedchart-0.1.0.tgz",
			isCompressedChart: true,
		},
		{
			name:              "archived-chart-path-with-hyphens",
			chartPath:         "testdata/charts/compressedchart-with-hyphens-0.1.0.tgz",
			isCompressedChart: true,
		},
		{
			name:              "archived-tar-gz-chart-path",
			chartPath:         "testdata/charts/compressedchart-0.1.0.tar.gz",
			isCompressedChart: true,
		},
		{
			name:              "invalid-archived-chart-path",
			chartPath:         "testdata/charts/invalidcompressedchart0.1.0.tgz",
			isCompressedChart: true,
			err:               true,
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
			name:              "pre-release-chart",
			chartPath:         "testdata/charts/pre-release-chart-0.1.0-alpha.tgz",
			isCompressedChart: true,
		},
		{
			name:           "decompressed-chart-enable-full-path",
			chartPath:      "testdata/charts/decompressedchart/",
			enableFullPath: true,
		},
		{
			name:              "archived-chart-path-enable-full-path",
			chartPath:         "testdata/charts/compressedchart-0.1.0.tgz",
			chartName:         "compressedchart",
			enableFullPath:    true,
			isCompressedChart: true,
		},
		{
			name:              "archived-chart-path-with-hyphens-enable-full-path",
			chartPath:         "testdata/charts/compressedchart-with-hyphens-0.1.0.tgz",
			chartName:         "compressedchart-with-hyphens",
			enableFullPath:    true,
			isCompressedChart: true,
		},
		{
			name:              "archived-tar-gz-chart-path-enable-full-path",
			chartPath:         "testdata/charts/compressedchart-0.1.0.tar.gz",
			chartName:         "compressedchart",
			enableFullPath:    true,
			isCompressedChart: true,
		},
		{
			name:              "invalid-archived-chart-path-enable-full-path",
			chartPath:         "testdata/charts/invalidcompressedchart0.1.0.tgz",
			err:               true,
			enableFullPath:    true,
			isCompressedChart: true,
		},
		{
			name:           "chart-missing-manifest-enable-full-path",
			chartPath:      "testdata/charts/chart-missing-manifest",
			err:            true,
			enableFullPath: true,
		},
		{
			name:           "chart-with-schema-enable-full-path",
			chartPath:      "testdata/charts/chart-with-schema",
			enableFullPath: true,
		},
		{
			name:           "chart-with-schema-negative-enable-full-path",
			chartPath:      "testdata/charts/chart-with-schema-negative",
			enableFullPath: true,
		},
		{
			name:              "pre-release-chart-enable-full-path",
			chartPath:         "testdata/charts/pre-release-chart-0.1.0-alpha.tgz",
			chartName:         "pre-release-chart",
			enableFullPath:    true,
			isCompressedChart: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := lintChart(tt.chartPath, map[string]interface{}{}, namespace, nil, tt.enableFullPath)
			switch {
			case err != nil && !tt.err:
				t.Errorf("%s", err)
			case err == nil && tt.err:
				t.Errorf("Expected a chart parsing error")
			}

			// Check for full path if enabled
			for _, msg := range result.Messages {
				if tt.enableFullPath {
					if tt.isCompressedChart {
						tempPathPrefixPattern := fmt.Sprintf("^%s[0-9]*/%s/.*",
							helmLintTempDirPrefix, tt.chartName)

						re, err := regexp.Compile(tempPathPrefixPattern)
						if err != nil {
							t.Fatalf("Unexpected error parsing regex pattern %q: %v",
								tempPathPrefixPattern, err)
						}

						if !re.Match([]byte(msg.Path)) {
							t.Fatalf("Full path is missing or incorrect: %s\nExpected to match pattern: %s",
								msg.Path, tempPathPrefixPattern)
						}

						continue
					}

					pathPrefix := filepath.Join(workingDir, tt.chartPath)
					if !strings.HasPrefix(msg.Path, pathPrefix) {
						t.Fatalf("Full path is missing or incorrect: %s\nExpected to have prefix: %s",
							msg.Path, pathPrefix)
					}
				}
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
	chartFile := "Chart.yaml"
	chartFile1FullPath, err := filepath.Abs(
		filepath.Join(chart1MultipleChartLint, chartFile))
	if err != nil {
		t.Fatalf("Failed to determine the full path of %s in %s",
			chartFile, chart1MultipleChartLint)
	}
	chartFile2FullPath, err := filepath.Abs(
		filepath.Join(chart2MultipleChartLint, chartFile))
	if err != nil {
		t.Fatalf("Failed to determine the full path of %s in %s",
			chartFile, chart2MultipleChartLint)
	}

	t.Run("multiple charts", func(t *testing.T) {
		testLint := NewLint()
		if result := testLint.Run(testCharts, values); len(result.Errors) > 0 {
			t.Error(result.Errors)
		}
	})

	t.Run("multiple charts with --full-path enabled", func(t *testing.T) {
		testLint := NewLint()
		testLint.EnableFullPath = true

		result := testLint.Run(testCharts, values)
		if len(result.Errors) > 0 {
			t.Error(result.Errors)
		}

		var file1Found, file2Found bool
		for _, msg := range result.Messages {
			switch msg.Path {
			case chartFile1FullPath:
				file1Found = true
			case chartFile2FullPath:
				file2Found = true
			default:
				t.Errorf("Unexpected path in linter message: %s", msg.Path)
			}
		}

		if !file1Found || !file2Found {
			t.Errorf("Missing either of the chart's path (%s, %s)",
				chartFile1FullPath, chartFile2FullPath)
		}
	})
}

func TestLint_EmptyResultErrors(t *testing.T) {
	testCharts := []string{chart2MultipleChartLint}
	chartFile := "Chart.yaml"
	chartFileFullPath, err := filepath.Abs(
		filepath.Join(chart2MultipleChartLint, chartFile))
	if err != nil {
		t.Fatalf("Failed to determine the full path of %s in %s",
			chartFile, chart2MultipleChartLint)
	}

	t.Run("empty result errors", func(t *testing.T) {
		testLint := NewLint()
		if result := testLint.Run(testCharts, values); len(result.Errors) > 0 {
			t.Error("Expected no error, got more")
		}
	})

	t.Run("empty result errors with --full-path enabled", func(t *testing.T) {
		testLint := NewLint()
		testLint.EnableFullPath = true

		result := testLint.Run(testCharts, values)
		if len(result.Errors) > 0 {
			t.Errorf("Incorrect number of linter errors\nExpected: 0\nGot:      %d",
				len(result.Errors))
		}

		if len(result.Messages) != 1 {
			t.Errorf("Incorrect number of linter messages\nExpected: 1\nGot:      %d",
				len(result.Messages))
		}

		if result.Messages[0].Path != chartFileFullPath {
			t.Errorf("Mismatch of path in log message\nExpected: %s\nGot:      %s",
				chartFileFullPath, result.Messages[0].Path)
		}
	})
}

func TestLint_ChartWithWarnings(t *testing.T) {
	valuesFile := "values.yaml"
	valuesFileFullPath, err := filepath.Abs(
		filepath.Join(chartWithNoTemplatesDir, valuesFile))
	if err != nil {
		t.Fatalf("Failed to determine the full path of %s in %s",
			valuesFile, chartWithNoTemplatesDir)
	}

	t.Run("should pass when not strict", func(t *testing.T) {
		testCharts := []string{chartWithNoTemplatesDir}
		testLint := NewLint()
		testLint.Strict = false
		if result := testLint.Run(testCharts, values); len(result.Errors) > 0 {
			t.Error("Expected no error, got more")
		}
	})

	t.Run("should pass with no errors when strict", func(t *testing.T) {
		testCharts := []string{chartWithNoTemplatesDir}
		testLint := NewLint()
		testLint.Strict = true
		if result := testLint.Run(testCharts, values); len(result.Errors) != 0 {
			t.Error("expected no errors, but got", len(result.Errors))
		}
	})

	t.Run("should pass when not strict with --full-path enabled", func(t *testing.T) {
		testCharts := []string{chartWithNoTemplatesDir}
		testLint := NewLint()
		testLint.Strict = false
		testLint.EnableFullPath = true

		result := testLint.Run(testCharts, values)
		if len(result.Errors) > 0 {
			t.Error("Expected no error, got more")
		}

		if len(result.Messages) == 0 {
			t.Errorf("Missing linter message for values file")
		}

		if result.Messages[0].Path != valuesFileFullPath {
			t.Errorf("Mismatch of path in log message\nExpected: %s\nGot:      %s",
				valuesFileFullPath, result.Messages[0].Path)
		}
	})

	t.Run("should pass with no errors when strict with --full-path enabled", func(t *testing.T) {
		testCharts := []string{chartWithNoTemplatesDir}
		testLint := NewLint()
		testLint.Strict = true
		testLint.EnableFullPath = true

		result := testLint.Run(testCharts, values)
		if len(result.Errors) != 0 {
			t.Error("expected no errors, but got", len(result.Errors))
		}

		if len(result.Messages) == 0 {
			t.Errorf("Missing linter message for values file")
		}

		if result.Messages[0].Path != valuesFileFullPath {
			t.Errorf("Mismatch of path in log message\nExpected: %s\nGot:      %s",
				valuesFileFullPath, result.Messages[0].Path)
		}
	})
}
