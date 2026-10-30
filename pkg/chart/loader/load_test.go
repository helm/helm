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
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
			Mode:    0o644,
			Size:    int64(len(data)),
			ModTime: time.Now(),
		}
		require.NoErrorf(t, tw.WriteHeader(header), "Failed to write tar header for %s", name)
		_, err := tw.Write(data)
		require.NoErrorf(t, err, "Failed to write tar data for %s", name)
	}

	require.NoErrorf(t, tw.Close(), "Failed to close tar writer")
	require.NoErrorf(t, gw.Close(), "Failed to close gzip writer")
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
				require.Errorf(t, err, "Expected error containing %q, but got no error", tc.expectedError)
				require.ErrorContains(t, err, tc.expectedError)
			} else {
				require.NoError(t, err)

				lac, err := chart.NewAccessor(loadedChart)
				require.NoError(t, err)

				eac, err := chart.NewAccessor(tc.expectedChart)
				require.NoError(t, err)
				assert.Equalf(t, lac.Name(), eac.Name(), "Expected chart name %q, got %q", eac.Name(), lac.Name())

				var loadedAPIVersion string
				switch lc := loadedChart.(type) {
				case *c2.Chart:
					loadedAPIVersion = lc.Metadata.APIVersion
				case *c3.Chart:
					loadedAPIVersion = lc.Metadata.APIVersion
				}
				assert.Equalf(t, loadedAPIVersion, tc.apiVersion, "Expected API version %q, got %q", tc.apiVersion, loadedAPIVersion)
			}
		})
	}
}

type archiveEntry struct {
	path string
	body string
}

// archiveBytes packs the given entries into a gzipped tar archive, keeping the
// entry paths verbatim so that malformed archives can be exercised.
func archiveBytes(t *testing.T, entries []archiveEntry) []byte {
	t.Helper()

	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)
	for _, e := range entries {
		hdr := &tar.Header{
			Name:    e.path,
			Mode:    0o644,
			Size:    int64(len(e.body)),
			ModTime: time.Now(),
		}
		require.NoErrorf(t, tw.WriteHeader(hdr), "Failed to write tar header for %s", e.path)
		_, err := tw.Write([]byte(e.body))
		require.NoErrorf(t, err, "Failed to write tar data for %s", e.path)
	}
	require.NoError(t, tw.Close())
	require.NoError(t, gw.Close())
	return buf.Bytes()
}

// gzipBytes returns a gzipped stream that has no tar framing at all.
func gzipBytes(t *testing.T, payload string) []byte {
	t.Helper()

	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	_, err := gw.Write([]byte(payload))
	require.NoError(t, err)
	require.NoError(t, gw.Close())
	return buf.Bytes()
}

// writeArchiveFile stores data as a chart archive in a temporary directory.
func writeArchiveFile(t *testing.T, data []byte) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "chart.tgz")
	require.NoError(t, os.WriteFile(path, data, 0o600))
	return path
}

func TestLoadFileArchiveErrorReason(t *testing.T) {
	const chartYAML = "apiVersion: v2\nname: mychart\nversion: 0.1.0\n"

	truncated := archiveBytes(t, []archiveEntry{{"mychart/Chart.yaml", chartYAML}})
	truncated = truncated[:len(truncated)/3]

	tests := []struct {
		name    string
		data    []byte
		wantErr string
	}{
		{
			name:    "archive referencing the parent directory",
			data:    archiveBytes(t, []archiveEntry{{"mychart/Chart.yaml", chartYAML}, {"mychart/../../evil.yaml", "x: 1\n"}}),
			wantErr: "chart illegally references parent directory",
		},
		{
			name:    "archive with an absolute path",
			data:    archiveBytes(t, []archiveEntry{{"mychart/Chart.yaml", chartYAML}, {"mychart//etc/passwd", "x: 1\n"}}),
			wantErr: "chart illegally contains absolute paths",
		},
		{
			name:    "Chart.yaml outside of the base directory",
			data:    archiveBytes(t, []archiveEntry{{"Chart.yaml", chartYAML}}),
			wantErr: `chart illegally contains content outside the base directory: "Chart.yaml"`,
		},
		{
			name:    "truncated archive",
			data:    truncated,
			wantErr: "unexpected EOF",
		},
		{
			name:    "gzip stream without tar framing",
			data:    gzipBytes(t, strings.Repeat("helm ", 200)),
			wantErr: "archive/tar: invalid tar header",
		},
		{
			name: "valid archive",
			data: archiveBytes(t, []archiveEntry{{"mychart/Chart.yaml", chartYAML}}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chartPath := writeArchiveFile(t, tt.data)

			loaded, err := LoadFile(chartPath)
			if tt.wantErr == "" {
				require.NoError(t, err)
				require.NotNil(t, loaded)
				return
			}

			require.Error(t, err)
			require.ErrorContains(t, err, tt.wantErr)
			require.ErrorContains(t, err, "unable to load chart archive "+chartPath)
		})
	}
}
