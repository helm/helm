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

package installer

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeTestTarGz(t *testing.T, headers []tar.Header, bodies map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)
	for _, hdr := range headers {
		hdr.Size = int64(len(bodies[hdr.Name]))
		require.NoError(t, tw.WriteHeader(&hdr))
		_, err := tw.Write([]byte(bodies[hdr.Name]))
		require.NoError(t, err)
	}
	require.NoError(t, tw.Close())
	require.NoError(t, gw.Close())
	return buf.Bytes()
}

// extractEntryPoints are the two ways plugin archives are extracted: the
// Extractor used by the HTTP and local installers, and the OCI installer.
var extractEntryPoints = map[string]func(data []byte, targetDir string) error{
	"TarGzExtractor": func(data []byte, targetDir string) error {
		return (&TarGzExtractor{}).Extract(bytes.NewBuffer(data), targetDir)
	},
	"extractTarGz": func(data []byte, targetDir string) error {
		return extractTarGz(bytes.NewReader(data), targetDir)
	},
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

func TestExtractStaysInsideTarget(t *testing.T) {
	file := func(name string) tar.Header { return tar.Header{Name: name, Typeflag: tar.TypeReg, Mode: 0o644} }

	tests := map[string]struct {
		// setup may create symlinks in the target directory before extraction.
		setup   func(t *testing.T, target, outside string)
		headers func(outside string) []tar.Header
		wantErr string
	}{
		"parent directory": {
			headers: func(string) []tar.Header { return []tar.Header{file("bin/../../outside/escape")} },
			wantErr: "path contains '..'",
		},
		"absolute path": {
			headers: func(outside string) []tar.Header {
				return []tar.Header{file(filepath.ToSlash(filepath.Join(outside, "escape")))}
			},
			wantErr: "illegal",
		},
		"symlink entry followed by a write through it": {
			headers: func(outside string) []tar.Header {
				return []tar.Header{{Name: "lib", Typeflag: tar.TypeSymlink, Linkname: outside}, file("lib/escape")}
			},
			wantErr: "unknown type",
		},
		"existing directory symlink with absolute target": {
			setup: func(t *testing.T, target, outside string) {
				t.Helper()
				symlinkOrSkip(t, outside, filepath.Join(target, "lib"))
			},
			headers: func(string) []tar.Header { return []tar.Header{file("lib/escape")} },
		},
		"existing directory symlink with relative target": {
			setup: func(t *testing.T, target, _ string) {
				t.Helper()
				symlinkOrSkip(t, filepath.Join("..", "outside"), filepath.Join(target, "lib"))
			},
			headers: func(string) []tar.Header { return []tar.Header{file("lib/escape")} },
		},
		"directory entry through an existing symlink": {
			setup: func(t *testing.T, target, outside string) {
				t.Helper()
				symlinkOrSkip(t, outside, filepath.Join(target, "lib"))
			},
			headers: func(string) []tar.Header {
				return []tar.Header{{Name: "lib/sub/", Typeflag: tar.TypeDir, Mode: 0o755}}
			},
		},
		"existing file symlink": {
			setup: func(t *testing.T, target, outside string) {
				t.Helper()
				symlinkOrSkip(t, filepath.Join(outside, "victim"), filepath.Join(target, "plugin.yaml"))
			},
			headers: func(string) []tar.Header { return []tar.Header{file("plugin.yaml")} },
		},
	}

	for entryPoint, extract := range extractEntryPoints {
		for name, tt := range tests {
			t.Run(entryPoint+"/"+name, func(t *testing.T) {
				base := t.TempDir()
				target, outside := filepath.Join(base, "target"), filepath.Join(base, "outside")
				require.NoError(t, os.Mkdir(target, 0o755))
				require.NoError(t, os.Mkdir(outside, 0o755))
				if tt.setup != nil {
					tt.setup(t, target, outside)
				}

				err := extract(makeTestTarGz(t, tt.headers(outside), nil), target)
				require.ErrorContains(t, err, tt.wantErr)

				entries, err := os.ReadDir(outside)
				require.NoError(t, err)
				assert.Empty(t, entries, "nothing may be written outside the target directory")
			})
		}
	}
}

func TestExtractWellFormedArchive(t *testing.T) {
	headers := []tar.Header{
		{Name: "./", Typeflag: tar.TypeDir, Mode: 0o755},
		{Name: "plugin.yaml", Typeflag: tar.TypeReg, Mode: 0o644},
		{Name: "bin/", Typeflag: tar.TypeDir, Mode: 0o755},
		{Name: "bin/run.sh", Typeflag: tar.TypeReg, Mode: 0o755},
		{Name: "share/doc/README.md", Typeflag: tar.TypeReg, Mode: 0o600},
		// Only the permission bits are applied, as with os.OpenFile.
		{Name: "bin/setuid", Typeflag: tar.TypeReg, Mode: 0o4755},
		{Name: "bin/setgid", Typeflag: tar.TypeReg, Mode: 0o2755},
		{Name: "bin/sticky", Typeflag: tar.TypeReg, Mode: 0o1755},
		{Name: "bin/typed", Typeflag: tar.TypeReg, Mode: 0o100755},
		{Name: "share/typed.txt", Typeflag: tar.TypeReg, Mode: 0o100644},
		{Name: "pax_global_header", Typeflag: tar.TypeXGlobalHeader},
	}
	bodies := map[string]string{
		"plugin.yaml":         "name: example\n",
		"bin/run.sh":          "#!/bin/sh\n",
		"share/doc/README.md": "docs",
		"bin/setuid":          "setuid",
		"bin/setgid":          "setgid",
		"bin/sticky":          "sticky",
		"bin/typed":           "typed",
		"share/typed.txt":     "typed",
	}

	for entryPoint, extract := range extractEntryPoints {
		t.Run(entryPoint, func(t *testing.T) {
			target := t.TempDir()

			require.NoError(t, extract(makeTestTarGz(t, headers, bodies), target))

			for _, hdr := range headers {
				if hdr.Typeflag != tar.TypeReg {
					continue
				}
				p := filepath.Join(target, filepath.FromSlash(hdr.Name))
				content, err := os.ReadFile(p)
				require.NoError(t, err)
				assert.Equal(t, bodies[hdr.Name], string(content))

				if runtime.GOOS != "windows" {
					info, err := os.Stat(p)
					require.NoError(t, err)
					// The umask may clear group and other bits, but not the owner's.
					assert.Equal(t, os.FileMode(hdr.Mode)&0o700, info.Mode().Perm()&0o700, "mode of %s", hdr.Name)
					assert.Zero(t, info.Mode()&(os.ModeSetuid|os.ModeSetgid|os.ModeSticky), "special bits of %s", hdr.Name)
				}
			}
		})
	}
}

func TestExtractFollowsSymlinksInsideTarget(t *testing.T) {
	headers := []tar.Header{{Name: "lib/linked.txt", Typeflag: tar.TypeReg, Mode: 0o644}}
	bodies := map[string]string{"lib/linked.txt": "linked"}

	for entryPoint, extract := range extractEntryPoints {
		t.Run(entryPoint, func(t *testing.T) {
			target := t.TempDir()
			require.NoError(t, os.Mkdir(filepath.Join(target, "real"), 0o755))
			symlinkOrSkip(t, "real", filepath.Join(target, "lib"))

			require.NoError(t, extract(makeTestTarGz(t, headers, bodies), target))

			content, err := os.ReadFile(filepath.Join(target, "real", "linked.txt"))
			require.NoError(t, err)
			assert.Equal(t, "linked", string(content))
		})
	}
}
