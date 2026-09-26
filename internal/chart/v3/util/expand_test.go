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

package util

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// makeTestChartArchive builds a gzipped tar archive from the given sourceDir directory, file entries are prefixed with the given chartName
func makeTestChartArchive(t *testing.T, chartName, sourceDir string) *bytes.Buffer {
	t.Helper()

	var result bytes.Buffer
	gw := gzip.NewWriter(&result)
	tw := tar.NewWriter(gw)

	dir := os.DirFS(sourceDir)

	writeFile := func(relPath string) {
		t.Helper()
		f, err := dir.Open(relPath)
		require.NoError(t, err)

		fStat, err := f.Stat()
		require.NoError(t, err)

		require.NoError(t, tw.WriteHeader(&tar.Header{
			Name: filepath.Join(chartName, relPath),
			Mode: int64(fStat.Mode()),
			Size: fStat.Size(),
		}))

		data, err := fs.ReadFile(dir, relPath)
		require.NoError(t, err)
		_, err = tw.Write(data)
		require.NoError(t, err)
	}

	require.NoError(t, fs.WalkDir(dir, ".", func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if d.IsDir() {
			return nil
		}

		writeFile(path)

		return nil
	}))

	require.NoError(t, tw.Close())
	require.NoError(t, gw.Close())

	return &result
}

func TestExpand(t *testing.T) {
	dest := t.TempDir()

	reader, err := os.Open("testdata/frobnitz-1.2.3.tgz")
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, reader.Close()) })

	require.NoError(t, Expand(dest, reader))

	expectedChartPath := filepath.Join(dest, "frobnitz")
	fi, err := os.Stat(expectedChartPath)
	require.NoError(t, err)
	require.Truef(t, fi.IsDir(), "expected a chart directory at %s", expectedChartPath)

	dir, err := os.Open(expectedChartPath)
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, dir.Close()) })

	fis, err := dir.Readdir(0)
	require.NoError(t, err)

	expectLen := 11
	assert.Len(t, fis, expectLen, "Expected %d files", expectLen)

	for _, fi := range fis {
		expect, err := os.Stat(filepath.Join("testdata", "frobnitz", fi.Name()))
		require.NoError(t, err)
		// os.Stat can return different values for directories, based on the OS
		// for Linux, for example, os.Stat always returns the size of the directory
		// (value-4096) regardless of the size of the contents of the directory
		mode := expect.Mode()
		if !mode.IsDir() {
			assert.Equal(t, expect.Size(), fi.Size(), "Expected %s to have size %d, got %d", fi.Name(), expect.Size(), fi.Size())
		}
	}
}

func TestExpandError(t *testing.T) {
	tests := map[string]struct {
		chartName string
		chartDir  string
		wantErr   string
	}{
		"dot name":      {"dotname", "testdata/dotname", "not allowed"},
		"dotdot name":   {"dotdotname", "testdata/dotdotname", "not allowed"},
		"slash in name": {"slashinname", "testdata/slashinname", "must not contain path separators"},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			archive := makeTestChartArchive(t, tt.chartName, tt.chartDir)

			dest := t.TempDir()
			assert.ErrorContains(t, Expand(dest, archive), tt.wantErr)
		})
	}
}

func TestExpandFile(t *testing.T) {
	dest := t.TempDir()

	require.NoError(t, ExpandFile(dest, "testdata/frobnitz-1.2.3.tgz"))

	expectedChartPath := filepath.Join(dest, "frobnitz")
	fi, err := os.Stat(expectedChartPath)
	require.NoError(t, err)
	require.Truef(t, fi.IsDir(), "expected a chart directory at %s", expectedChartPath)

	dir, err := os.Open(expectedChartPath)
	require.NoError(t, err)

	fis, err := dir.Readdir(0)
	require.NoError(t, err)

	expectLen := 11
	assert.Len(t, fis, expectLen, "Expected %d files", expectLen)

	for _, fi := range fis {
		expect, err := os.Stat(filepath.Join("testdata", "frobnitz", fi.Name()))
		require.NoError(t, err)
		// os.Stat can return different values for directories, based on the OS
		// for Linux, for example, os.Stat always returns the size of the directory
		// (value-4096) regardless of the size of the contents of the directory
		mode := expect.Mode()
		if !mode.IsDir() {
			assert.Equal(t, expect.Size(), fi.Size(), "Expected %s to have size %d, got %d", fi.Name(), expect.Size(), fi.Size())
		}
	}
}
