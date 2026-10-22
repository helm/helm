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
	"strings"
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

// Once GetVerificationData has read the archive and its provenance file, the
// installer must install those bytes rather than re-reading the source, as the
// HTTP and OCI installers already do. Otherwise the plugin installed need not
// be the one that was verified.
func TestLocalInstallerTarballInstallsCachedBytes(t *testing.T) {
	ensure.HelmHome(t)

	pluginDir := createTestPluginDir(t)
	tarballPath := createTarballFromPluginDir(t, pluginDir)
	provPath := tarballPath + ".prov"
	require.NoError(t, os.WriteFile(provPath, []byte("first provenance"), 0o644))

	i, err := NewLocalInstaller(tarballPath)
	require.NoError(t, err)
	_, _, _, err = i.GetVerificationData()
	require.NoError(t, err)

	// Replace the archive and provenance file after they have been read.
	pluginYAML := filepath.Join(pluginDir, "plugin.yaml")
	orig, err := os.ReadFile(pluginYAML)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(pluginYAML, []byte(strings.Replace(string(orig), "version: 1.0.0", "version: 2.0.0", 1)), 0o644))
	require.Equal(t, tarballPath, createTarballFromPluginDir(t, pluginDir))
	require.NoError(t, os.WriteFile(provPath, []byte("second provenance"), 0o644))

	require.NoError(t, i.Install())

	installed, err := os.ReadFile(filepath.Join(i.Path(), "plugin.yaml"))
	require.NoError(t, err)
	require.Contains(t, string(installed), "version: 1.0.0")

	require.FileExists(t, helmpath.DataPath("plugins", "test-plugin-1.0.0.tgz"))
	require.NoFileExists(t, helmpath.DataPath("plugins", "test-plugin-2.0.0.tgz"))

	prov, err := os.ReadFile(helmpath.DataPath("plugins", "test-plugin-1.0.0.tgz.prov"))
	require.NoError(t, err)
	require.Equal(t, "first provenance", string(prov))
}
