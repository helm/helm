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

package main

import (
	"bytes"
	"fmt"
	"testing"
)

func TestLintChart(t *testing.T) {
	tests := []struct {
		name      string
		chartPath string
		err       bool
	}{
		{
			name:      "decompressed-chart",
			chartPath: "testdata/testcharts/decompressedchart/",
		},
		{
			name:      "archived-chart-path",
			chartPath: "testdata/testcharts/compressedchart-0.1.0.tgz",
		},
		{
			name:      "archived-chart-path-with-hyphens",
			chartPath: "testdata/testcharts/compressedchart-with-hyphens-0.1.0.tgz",
		},
		{
			name:      "pre-release-chart",
			chartPath: "testdata/testcharts/pre-release-chart-0.1.0-alpha.tgz",
		},
		{
			name:      "invalid-archived-chart-path",
			chartPath: "testdata/testcharts/invalidcompressedchart0.1.0.tgz",
			err:       true,
		},
		{
			name:      "chart-missing-manifest",
			chartPath: "testdata/testcharts/chart-missing-manifest",
			err:       true,
		},
	}

	values := []byte{}
	namespace := "testNamespace"
	strict := false

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := lintChart(tt.chartPath, values, namespace, strict)
			switch {
			case err != nil && !tt.err:
				t.Errorf("%s", err)
			case err == nil && tt.err:
				t.Errorf("Expected a chart parsing error")
			}
		})
	}
}

func TestLinRunForNonExistentChart(t *testing.T) {
	t.Run("should error out for non existent tgz chart", func(t *testing.T) {
		testCharts := []string{"non-existent-chart.tgz"}
		testLint := &lintCmd{
			paths: testCharts,
			out:   bytes.NewBufferString(""),
		}
		expectedErr := fmt.Errorf("0 chart(s) linted, 1 chart(s) failed")
		err := testLint.run()
		if err == nil {
			t.Errorf("expected error but got no error")
		}
		if err != nil && (err.Error() != expectedErr.Error()) {
			t.Errorf("expected: \"%v\" , but got: \"%v\"", expectedErr, err)
		}
	})

	t.Run("should error out for corrupted tgz chart", func(t *testing.T) {
		var corruptedTgzChart = "testdata/testcharts/corrupted-compressed-chart.tgz"
		testCharts := []string{corruptedTgzChart}
		testLint := &lintCmd{
			paths: testCharts,
			out:   bytes.NewBufferString(""),
		}
		expectedErr := fmt.Errorf("0 chart(s) linted, 1 chart(s) failed")
		err := testLint.run()
		if err == nil {
			t.Errorf("expected error but got no error")
		}
		if err != nil && (err.Error() != expectedErr.Error()) {
			t.Errorf("expected: \"%v\" , but got: \"%v\"", expectedErr, err)
		}
	})
}
