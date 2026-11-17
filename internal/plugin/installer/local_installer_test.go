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
	"testing"

	"github.com/stretchr/testify/require"

	"helm.sh/helm/v4/internal/test/ensure"
	"helm.sh/helm/v4/pkg/helmpath"
)

var _ Installer = new(LocalInstaller)

func TestLocalInstaller(t *testing.T) {
	ensure.HelmHome(t)
	// Make a temp dir
	tdir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tdir, "plugin.yaml"), []byte{}, 0o644))

	source := "../testdata/plugdir/good/echo-v1"
	i, err := NewForSource(source, "")
	require.NoError(t, err)

	require.NoError(t, Install(i))

	require.Equal(t, helmpath.DataPath("plugins", "echo-v1"), i.Path(), "expected path '$XDG_CONFIG_HOME/helm/plugins/helm-env', got %q", i.Path())
	os.RemoveAll(filepath.Dir(helmpath.DataPath())) // helmpath.DataPath is like /tmp/helm013130971/helm
}

func TestLocalInstallerNotAFolder(t *testing.T) {
	source := "../testdata/plugdir/good/echo-v1/plugin.yaml"
	i, err := NewForSource(source, "")
	require.NoError(t, err)
	require.ErrorIs(t, Install(i), ErrPluginNotADirectory)
}

func TestLocalInstallerTarball(t *testing.T) {
	ensure.HelmHome(t)

	// Create a test tarball
	tempDir := t.TempDir()
	tarballPath := filepath.Join(tempDir, "test-plugin-1.0.0.tar.gz")

	// Create tarball content
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	files := []struct {
		Name string
		Body string
		Mode int64
	}{
		{"test-plugin/plugin.yaml", "name: test-plugin\napiVersion: v1\ntype: cli/v1\nruntime: subprocess\nversion: 1.0.0\nconfig:\n  shortHelp: test\n  longHelp: test\nruntimeConfig:\n  platformCommand:\n  - command: echo", 0o644},
		{"test-plugin/bin/test-plugin", "#!/usr/bin/env sh\necho test", 0o755},
	}

	for _, file := range files {
		hdr := &tar.Header{
			Name: file.Name,
			Mode: file.Mode,
			Size: int64(len(file.Body)),
		}
		require.NoError(t, tw.WriteHeader(hdr))
		_, err := tw.Write([]byte(file.Body))
		require.NoError(t, err)
	}

	require.NoError(t, tw.Close())
	require.NoError(t, gw.Close())

	// Write tarball to file
	require.NoError(t, os.WriteFile(tarballPath, buf.Bytes(), 0o644))

	// Test installation
	i, err := NewForSource(tarballPath, "")
	require.NoError(t, err)

	// Verify it's detected as LocalInstaller
	localInstaller, ok := i.(*LocalInstaller)
	require.True(t, ok, "expected LocalInstaller")
	require.True(t, localInstaller.isArchive, "expected isArchive to be true")
	require.NoError(t, Install(i))

	expectedPath := helmpath.DataPath("plugins", "test-plugin")
	require.Equal(t, expectedPath, i.Path(), "expected path %q, got %q", expectedPath, i.Path())

	// Verify plugin was installed
	_, err = os.Stat(i.Path())
	require.NoErrorf(t, err, "plugin not found at %s", i.Path())
}
