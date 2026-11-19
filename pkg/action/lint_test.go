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
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"helm.sh/helm/v4/pkg/chart/v2/lint/support"
)

var (
	values                  = make(map[string]any)
	namespace               = "testNamespace"
	chart1MultipleChartLint = "testdata/charts/multiplecharts-lint-chart-1"
	chart2MultipleChartLint = "testdata/charts/multiplecharts-lint-chart-2"
	corruptedTgzChart       = "testdata/charts/corrupted-compressed-chart.tgz"
	chartWithNoTemplatesDir = "testdata/charts/chart-with-no-templates-dir"
)

func TestLintChart(t *testing.T) {
	tests := []struct {
		name                 string
		chartPath            string
		err                  bool
		skipSchemaValidation bool
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
			name:                 "chart-with-schema-negative-skip-validation",
			chartPath:            "testdata/charts/chart-with-schema-negative",
			skipSchemaValidation: true,
		},
		{
			name:      "pre-release-chart",
			chartPath: "testdata/charts/pre-release-chart-0.1.0-alpha.tgz",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := lintChart(tt.chartPath, map[string]any{}, namespace, nil, tt.skipSchemaValidation)
			if tt.err {
				require.Error(t, err, "Expected a chart parsing error")
			} else {
				require.NoError(t, err)
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
		require.Len(t, result.Errors, 1, "expected one error, but got", len(result.Errors))
		assert.EqualError(t, result.Errors[0], expectedError)
	})

	t.Run("should error out for corrupted tgz chart", func(t *testing.T) {
		testCharts := []string{corruptedTgzChart}
		expectedEOFError := "unable to extract tarball: EOF"
		testLint := NewLint()

		result := testLint.Run(testCharts, values)
		require.Len(t, result.Errors, 1, "expected one error, but got", len(result.Errors))
		assert.EqualError(t, result.Errors[0], expectedEOFError)
	})
}

func TestLint_MultipleCharts(t *testing.T) {
	testCharts := []string{chart2MultipleChartLint, chart1MultipleChartLint}
	testLint := NewLint()
	result := testLint.Run(testCharts, values)
	assert.Empty(t, result.Errors)
}

func TestLint_EmptyResultErrors(t *testing.T) {
	testCharts := []string{chart2MultipleChartLint}
	testLint := NewLint()
	result := testLint.Run(testCharts, values)
	assert.Empty(t, result.Errors, "Expected no error, got more")
}

func TestLint_ChartWithWarnings(t *testing.T) {
	t.Run("should pass when not strict", func(t *testing.T) {
		testCharts := []string{chartWithNoTemplatesDir}
		testLint := NewLint()
		testLint.Strict = false
		result := testLint.Run(testCharts, values)
		assert.Empty(t, result.Errors, "Expected no error, got more")
	})

	t.Run("should fail with one error when strict", func(t *testing.T) {
		testCharts := []string{chartWithNoTemplatesDir}
		testLint := NewLint()
		testLint.Strict = true
		result := testLint.Run(testCharts, values)
		assert.Len(t, result.Errors, 1, "expected one error")
	})
}

func TestHasWarningsOrErrors(t *testing.T) {
	testError := errors.New("test-error")
	cases := []struct {
		name     string
		data     LintResult
		expected bool
	}{
		{
			name:     "has no warning messages and no errors",
			data:     LintResult{TotalChartsLinted: 1, Messages: make([]support.Message, 0), Errors: make([]error, 0)},
			expected: false,
		},
		{
			name:     "has error",
			data:     LintResult{TotalChartsLinted: 1, Messages: make([]support.Message, 0), Errors: []error{testError}},
			expected: true,
		},
		{
			name:     "has info message only",
			data:     LintResult{TotalChartsLinted: 1, Messages: []support.Message{{Severity: support.InfoSev, Path: "", Err: testError}}, Errors: make([]error, 0)},
			expected: false,
		},
		{
			name:     "has warning message",
			data:     LintResult{TotalChartsLinted: 1, Messages: []support.Message{{Severity: support.WarningSev, Path: "", Err: testError}}, Errors: make([]error, 0)},
			expected: true,
		},
		{
			name:     "has error message",
			data:     LintResult{TotalChartsLinted: 1, Messages: []support.Message{{Severity: support.ErrorSev, Path: "", Err: testError}}, Errors: make([]error, 0)},
			expected: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := HasWarningsOrErrors(&tc.data)
			assert.Equal(t, tc.expected, result)
		})
	}
}
