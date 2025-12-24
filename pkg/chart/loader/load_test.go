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

package loader

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"maps"
	"path/filepath"
	"strings"
	"testing"
	"time"

	c3 "helm.sh/helm/v4/internal/chart/v3"
	"helm.sh/helm/v4/pkg/chart"
	c2 "helm.sh/helm/v4/pkg/chart/v2"
)

// createChartArchive is a helper function to create a gzipped tar archive in memory
func createChartArchive(t *testing.T, chartName, apiVersion string, extraFiles map[string][]byte, createChartYaml bool) io.Reader {
	t.Helper()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	files := make(map[string][]byte)
	maps.Copy(files, extraFiles)

	if createChartYaml {
		chartYAMLContent := fmt.Sprintf(`apiVersion: %s
name: %s
version: 0.1.0
description: A test chart
`, apiVersion, chartName)
		files["Chart.yaml"] = []byte(chartYAMLContent)
	}

	for name, data := range files {
		header := &tar.Header{
			Name:    filepath.Join(chartName, name),
			Mode:    0644,
			Size:    int64(len(data)),
			ModTime: time.Now(),
		}
		if err := tw.WriteHeader(header); err != nil {
			t.Fatalf("Failed to write tar header for %s: %v", name, err)
		}
		if _, err := tw.Write(data); err != nil {
			t.Fatalf("Failed to write tar data for %s: %v", name, err)
		}
	}

	if err := tw.Close(); err != nil {
		t.Fatalf("Failed to close tar writer: %v", err)
	}
	if err := gw.Close(); err != nil {
		t.Fatalf("Failed to close gzip writer: %v", err)
	}
	return &buf
}

func TestLoadArchive(t *testing.T) {
	testCases := []struct {
		name            string
		chartName       string
		apiVersion      string
		extraFiles      map[string][]byte
		inputReader     io.Reader
		expectedChart   chart.Charter
		expectedError   string
		createChartYaml bool
	}{
		{
			name:       "valid v2 chart archive",
			chartName:  "mychart-v2",
			apiVersion: c2.APIVersionV2,
			extraFiles: map[string][]byte{"templates/config.yaml": []byte("key: value")},
			expectedChart: &c2.Chart{
				Metadata: &c2.Metadata{APIVersion: c2.APIVersionV2, Name: "mychart-v2", Version: "0.1.0", Description: "A test chart"},
			},
			createChartYaml: true,
		},
		{
			name:       "valid v3 chart archive",
			chartName:  "mychart-v3",
			apiVersion: c3.APIVersionV3,
			extraFiles: map[string][]byte{"templates/config.yaml": []byte("key: value")},
			expectedChart: &c3.Chart{
				Metadata: &c3.Metadata{APIVersion: c3.APIVersionV3, Name: "mychart-v3", Version: "0.1.0", Description: "A test chart"},
			},
			createChartYaml: true,
		},
		{
			name:          "invalid gzip header",
			inputReader:   bytes.NewBufferString("not a gzip file"),
			expectedError: "stream does not appear to be a valid chart file (details: gzip: invalid header)",
		},
		{
			name:            "archive without Chart.yaml",
			chartName:       "no-chart-yaml",
			apiVersion:      c2.APIVersionV2, // This will be ignored as Chart.yaml is missing
			extraFiles:      map[string][]byte{"values.yaml": []byte("foo: bar")},
			expectedError:   "unable to detect chart version, no Chart.yaml found",
			createChartYaml: false,
		},
		{
			name:            "archive with malformed Chart.yaml",
			chartName:       "malformed-chart-yaml",
			apiVersion:      c2.APIVersionV2,
			extraFiles:      map[string][]byte{"Chart.yaml": []byte("apiVersion: v2\nname: mychart\nversion: 0.1.0\ndescription: A test chart\ninvalid: :")},
			expectedError:   "cannot load Chart.yaml: error converting YAML to JSON: yaml: line 5: mapping values are not allowed in this context",
			createChartYaml: false,
		},
		{
			name:            "unsupported API version",
			chartName:       "unsupported-api",
			apiVersion:      "v99",
			expectedError:   "unsupported chart version",
			createChartYaml: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var reader io.Reader
			if tc.inputReader != nil {
				reader = tc.inputReader
			} else {
				reader = createChartArchive(t, tc.chartName, tc.apiVersion, tc.extraFiles, tc.createChartYaml)
			}

			loadedChart, err := LoadArchive(reader)

			if tc.expectedError != "" {
				if err == nil || !strings.Contains(err.Error(), tc.expectedError) {
					t.Errorf("Expected error containing %q, but got %v", tc.expectedError, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			lac, err := chart.NewAccessor(loadedChart)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			eac, err := chart.NewAccessor(tc.expectedChart)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if lac.Name() != eac.Name() {
				t.Errorf("Expected chart name %q, got %q", eac.Name(), lac.Name())
			}

			var loadedAPIVersion string
			switch lc := loadedChart.(type) {
			case *c2.Chart:
				loadedAPIVersion = lc.Metadata.APIVersion
			case *c3.Chart:
				loadedAPIVersion = lc.Metadata.APIVersion
			}
			if loadedAPIVersion != tc.apiVersion {
				t.Errorf("Expected API version %q, got %q", tc.apiVersion, loadedAPIVersion)
			}
		})
	}
}
