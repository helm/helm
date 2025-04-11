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
	"os"
	"testing"
)

func TestLoadArchiveFiles(t *testing.T) {
	tcs := []struct {
		name     string
		generate func(w *tar.Writer)
		check    func(t *testing.T, files []*BufferedFile, err error)
	}{
		{
			name:     "empty input should return no files",
			generate: func(_ *tar.Writer) {},
			check: func(t *testing.T, _ []*BufferedFile, err error) {
				if err.Error() != "no files in chart archive" {
					t.Fatalf(`expected "no files in chart archive", got [%#v]`, err)
				}
			},
		},
		{
			name: "should ignore files with XGlobalHeader type",
			generate: func(w *tar.Writer) {
				// simulate the presence of a `pax_global_header` file like you would get when
				// processing a GitHub release archive.
				err := w.WriteHeader(&tar.Header{
					Typeflag: tar.TypeXGlobalHeader,
					Name:     "pax_global_header",
				})
				if err != nil {
					t.Fatal(err)
				}

				// we need to have at least one file, otherwise we'll get the "no files in chart archive" error
				err = w.WriteHeader(&tar.Header{
					Typeflag: tar.TypeReg,
					Name:     "dir/empty",
				})
				if err != nil {
					t.Fatal(err)
				}
			},
			check: func(t *testing.T, files []*BufferedFile, err error) {
				if err != nil {
					t.Fatalf(`got unwanted error [%#v] for tar file with pax_global_header content`, err)
				}

				if len(files) != 1 {
					t.Fatalf(`expected to get one file but got [%v]`, files)
				}
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			gzw := gzip.NewWriter(buf)
			tw := tar.NewWriter(gzw)

			tc.generate(tw)

			_ = tw.Close()
			_ = gzw.Close()

			files, err := LoadArchiveFiles(buf)
			tc.check(t, files, err)
		})
	}
}

func TestEnvVarOverrides(t *testing.T) {
	originalChartSize := MaxDecompressedChartSize
	originalFileSize := MaxDecompressedFileSize

	originalChartEnv, chartEnvExists := os.LookupEnv("HELM_MAX_DECOMPRESSED_CHART_SIZE")
	originalFileEnv, fileEnvExists := os.LookupEnv("HELM_MAX_DECOMPRESSED_FILE_SIZE")

	// Restore everything when test completes
	defer func() {
		MaxDecompressedChartSize = originalChartSize
		MaxDecompressedFileSize = originalFileSize

		if chartEnvExists {
			os.Setenv("HELM_MAX_DECOMPRESSED_CHART_SIZE", originalChartEnv)
		} else {
			os.Unsetenv("HELM_MAX_DECOMPRESSED_CHART_SIZE")
		}

		if fileEnvExists {
			os.Setenv("HELM_MAX_DECOMPRESSED_FILE_SIZE", originalFileEnv)
		} else {
			os.Unsetenv("HELM_MAX_DECOMPRESSED_FILE_SIZE")
		}
	}()

	os.Setenv("HELM_MAX_DECOMPRESSED_CHART_SIZE", "50000000") // ~50MB
	os.Setenv("HELM_MAX_DECOMPRESSED_FILE_SIZE", "3000000")   // ~3MB

	// Reset to default values before testing
	MaxDecompressedChartSize = 100 * 1024 * 1024
	MaxDecompressedFileSize = 5 * 1024 * 1024

	parseEnvSettings()

	if MaxDecompressedChartSize != 50000000 {
		t.Errorf("Expected MaxDecompressedChartSize = 50000000, got %d", MaxDecompressedChartSize)
	}

	if MaxDecompressedFileSize != 3000000 {
		t.Errorf("Expected MaxDecompressedFileSize = 3000000, got %d", MaxDecompressedFileSize)
	}
}
