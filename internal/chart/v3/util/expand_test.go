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
	"runtime"
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

const testChartYAML = "apiVersion: v3\nname: frobnitz\nversion: 1.2.3\n"

// makeRawTestChartArchive builds a gzipped chart archive from the given
// headers, written exactly as given.
func makeRawTestChartArchive(t *testing.T, headers []tar.Header, bodies map[string]string) *bytes.Buffer {
	t.Helper()
	var result bytes.Buffer
	gw := gzip.NewWriter(&result)
	tw := tar.NewWriter(gw)
	for _, hdr := range headers {
		hdr.Size = int64(len(bodies[hdr.Name]))
		require.NoError(t, tw.WriteHeader(&hdr))
		_, err := tw.Write([]byte(bodies[hdr.Name]))
		require.NoError(t, err)
	}
	require.NoError(t, tw.Close())
	require.NoError(t, gw.Close())
	return &result
}

func symlinkOrSkip(t *testing.T, oldname, newname string) {
	t.Helper()
	if err := os.Symlink(oldname, newname); err != nil {
		if runtime.GOOS == "windows" {
			t.Skipf("creating symlinks is not permitted: %v", err)
		}
		require.NoError(t, err)
	}
}

func TestExpandStaysInsideChartDirectory(t *testing.T) {
	file := func(name string) tar.Header { return tar.Header{Name: name, Typeflag: tar.TypeReg, Mode: 0o644} }

	tests := map[string]struct {
		// setup may create symlinks in the destination before expanding.
		setup   func(t *testing.T, dest, outside string)
		headers []tar.Header
		wantErr string
	}{
		"parent directory": {
			headers: []tar.Header{file("frobnitz/../../outside/escape")},
			wantErr: "chart illegally references parent directory",
		},
		"absolute path": {
			headers: []tar.Header{file("frobnitz//outside/escape")},
			wantErr: "chart illegally contains absolute paths",
		},
		"symlink entry followed by a write through it": {
			headers: []tar.Header{{Name: "frobnitz/lib", Typeflag: tar.TypeSymlink, Linkname: "../../outside"}, file("frobnitz/lib/escape")},
		},
		"existing chart directory symlink": {
			setup: func(t *testing.T, dest, outside string) {
				t.Helper()
				symlinkOrSkip(t, outside, filepath.Join(dest, "frobnitz"))
			},
			headers: []tar.Header{file("frobnitz/values.yaml")},
		},
		"existing subdirectory symlink": {
			setup: func(t *testing.T, dest, _ string) {
				t.Helper()
				require.NoError(t, os.Mkdir(filepath.Join(dest, "frobnitz"), 0o755))
				symlinkOrSkip(t, filepath.Join("..", "..", "outside"), filepath.Join(dest, "frobnitz", "templates"))
			},
			headers: []tar.Header{file("frobnitz/templates/escape.yaml")},
		},
		"existing file symlink": {
			setup: func(t *testing.T, dest, outside string) {
				t.Helper()
				require.NoError(t, os.Mkdir(filepath.Join(dest, "frobnitz"), 0o755))
				symlinkOrSkip(t, filepath.Join(outside, "victim.yaml"), filepath.Join(dest, "frobnitz", "values.yaml"))
			},
			headers: []tar.Header{file("frobnitz/values.yaml")},
		},
		"existing chart directory symlink to the destination": {
			setup: func(t *testing.T, dest, _ string) {
				t.Helper()
				symlinkOrSkip(t, ".", filepath.Join(dest, "frobnitz"))
			},
			wantErr: "resolves to the extraction root",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			base := t.TempDir()
			dest, outside := filepath.Join(base, "dest"), filepath.Join(base, "outside")
			require.NoError(t, os.Mkdir(dest, 0o755))
			require.NoError(t, os.Mkdir(outside, 0o755))
			if tt.setup != nil {
				tt.setup(t, dest, outside)
			}
			headers := append([]tar.Header{file("frobnitz/Chart.yaml")}, tt.headers...)
			archive := makeRawTestChartArchive(t, headers, map[string]string{"frobnitz/Chart.yaml": testChartYAML})

			err := Expand(dest, archive)
			require.ErrorContains(t, err, tt.wantErr)

			entries, err := os.ReadDir(outside)
			require.NoError(t, err)
			assert.Empty(t, entries, "nothing may be written outside the destination")
			_, err = os.Lstat(filepath.Join(dest, "Chart.yaml"))
			assert.ErrorIs(t, err, fs.ErrNotExist, "nothing may be written to the destination itself")
		})
	}
}

func TestExpandWellFormedChart(t *testing.T) {
	headers := []tar.Header{
		{Name: "frobnitz/", Typeflag: tar.TypeDir, Mode: 0o755},
		{Name: "frobnitz/Chart.yaml", Typeflag: tar.TypeReg, Mode: 0o644},
		{Name: "frobnitz/templates/deployment.yaml", Typeflag: tar.TypeReg, Mode: 0o644},
		{Name: "frobnitz/charts/sub/templates/service.yaml", Typeflag: tar.TypeReg, Mode: 0o600},
	}
	bodies := map[string]string{
		"frobnitz/Chart.yaml":                        testChartYAML,
		"frobnitz/templates/deployment.yaml":         "kind: Deployment\n",
		"frobnitz/charts/sub/templates/service.yaml": "kind: Service\n",
	}
	// The destination does not exist yet; Expand creates it.
	dest := filepath.Join(t.TempDir(), "new", "dest")

	require.NoError(t, Expand(dest, makeRawTestChartArchive(t, headers, bodies)))

	for name, body := range bodies {
		p := filepath.Join(dest, filepath.FromSlash(name))
		content, err := os.ReadFile(p)
		require.NoError(t, err)
		assert.Equal(t, body, string(content))

		if runtime.GOOS != "windows" {
			info, err := os.Stat(p)
			require.NoError(t, err)
			// Files are always written 0644; the umask may clear group and other bits.
			assert.Equal(t, os.FileMode(0o600), info.Mode().Perm()&0o700, "mode of %s", name)
		}
	}
}

func TestExpandFollowsSymlinksInsideChart(t *testing.T) {
	dest := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dest, "frobnitz", "real"), 0o755))
	symlinkOrSkip(t, "real", filepath.Join(dest, "frobnitz", "files"))
	headers := []tar.Header{
		{Name: "frobnitz/Chart.yaml", Typeflag: tar.TypeReg, Mode: 0o644},
		{Name: "frobnitz/files/linked.txt", Typeflag: tar.TypeReg, Mode: 0o644},
	}
	bodies := map[string]string{"frobnitz/Chart.yaml": testChartYAML, "frobnitz/files/linked.txt": "linked"}

	require.NoError(t, Expand(dest, makeRawTestChartArchive(t, headers, bodies)))

	content, err := os.ReadFile(filepath.Join(dest, "frobnitz", "real", "linked.txt"))
	require.NoError(t, err)
	assert.Equal(t, "linked", string(content))
}

func TestExpandEmptyDestination(t *testing.T) {
	// An empty destination means the current directory.
	t.Chdir(t.TempDir())
	headers := []tar.Header{{Name: "frobnitz/Chart.yaml", Typeflag: tar.TypeReg, Mode: 0o644}}

	require.NoError(t, Expand("", makeRawTestChartArchive(t, headers, map[string]string{"frobnitz/Chart.yaml": testChartYAML})))

	info, err := os.Stat(filepath.Join("frobnitz", "Chart.yaml"))
	require.NoError(t, err)
	assert.Equal(t, int64(len(testChartYAML)), info.Size())
}
