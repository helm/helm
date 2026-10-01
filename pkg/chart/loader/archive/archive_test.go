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

package archive

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"testing"

	"github.com/stretchr/testify/require"
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
				t.Helper()
				require.EqualError(t, err, "no files in chart archive")
			},
		},
		{
			name: "should ignore files with XGlobalHeader type",
			generate: func(w *tar.Writer) {
				// simulate the presence of a `pax_global_header` file like you would get when
				// processing a GitHub release archive.
				require.NoError(t, w.WriteHeader(&tar.Header{
					Typeflag: tar.TypeXGlobalHeader,
					Name:     "pax_global_header",
				}))

				// we need to have at least one file, otherwise we'll get the "no files in chart archive" error
				require.NoError(t, w.WriteHeader(&tar.Header{
					Typeflag: tar.TypeReg,
					Name:     "dir/empty",
				}))
			},
			check: func(t *testing.T, files []*BufferedFile, err error) {
				t.Helper()
				require.NoErrorf(t, err, `got unwanted error for tar file with pax_global_header content`)
				require.Lenf(t, files, 1, `expected to get one file but got [%v]`, files)
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

func TestLoadArchiveFilesAtMaxDecompressedSize(t *testing.T) {
	const content = "apiVersion: v2\nname: mychart\nversion: 0.1.0\n"

	archiveOf := func(t *testing.T) *bytes.Buffer {
		t.Helper()
		buf := &bytes.Buffer{}
		gzw := gzip.NewWriter(buf)
		tw := tar.NewWriter(gzw)
		require.NoError(t, tw.WriteHeader(&tar.Header{
			Typeflag: tar.TypeReg,
			Name:     "mychart/Chart.yaml",
			Size:     int64(len(content)),
		}))
		_, err := tw.Write([]byte(content))
		require.NoError(t, err)
		require.NoError(t, tw.Close())
		require.NoError(t, gzw.Close())
		return buf
	}

	orig := MaxDecompressedChartSize
	t.Cleanup(func() { MaxDecompressedChartSize = orig })

	for _, tc := range []struct {
		name    string
		budget  int64
		wantErr bool
	}{
		{"one byte over the chart size", int64(len(content)) + 1, false},
		// A chart that exactly fills the budget is at the limit, not over it,
		// and BudgetedReader accepts it for directory loads.
		{"exactly the chart size", int64(len(content)), false},
		{"one byte under the chart size", int64(len(content)) - 1, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			MaxDecompressedChartSize = tc.budget
			files, err := LoadArchiveFiles(archiveOf(t))
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Len(t, files, 1)
		})
	}
}
