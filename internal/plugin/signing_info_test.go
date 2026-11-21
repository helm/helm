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

package plugin

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"helm.sh/helm/v4/pkg/provenance"
)

// signTestTarball returns a valid clearsigned provenance for the given tarball
func signTestTarball(t *testing.T, tarballPath string) string {
	t.Helper()

	signer, err := provenance.NewFromKeyring(testKeyFile, "helm-test")
	require.NoError(t, err)
	require.NoError(t, signer.DecryptKey(func(_ string) ([]byte, error) {
		return []byte(""), nil
	}))

	tarballData, err := os.ReadFile(tarballPath)
	require.NoError(t, err)

	sig, err := SignPlugin(tarballData, filepath.Base(tarballPath), signer)
	require.NoError(t, err)
	return sig
}

// installTestPlugin lays out a plugin directory under pluginsDir, the way
// the installers do, and returns the plugin directory and the metadata
// that will be loaded from it
func installTestPlugin(t *testing.T, pluginsDir string) (string, Metadata) {
	t.Helper()

	pluginDir := filepath.Join(pluginsDir, "test-plugin")
	require.NoError(t, os.MkdirAll(pluginDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(pluginDir, "plugin.yaml"), []byte(testPluginYAML), 0o644))

	return pluginDir, Metadata{Name: "test-plugin", Version: "1.0.0"}
}

// writeTestTarball packages pluginDir into pluginsDir/<name>-<version>.tgz,
// as saved by the installers, and returns the tarball path
func writeTestTarball(t *testing.T, pluginsDir, pluginDir string, metadata Metadata) string {
	t.Helper()

	tarballPath := filepath.Join(pluginsDir, metadata.Name+"-"+metadata.Version+".tgz")
	tarFile, err := os.Create(tarballPath)
	require.NoError(t, err)
	if err := CreatePluginTarball(pluginDir, metadata.Name, tarFile); err != nil {
		tarFile.Close()
		t.Fatal(err)
	}
	tarFile.Close()
	return tarballPath
}

func TestGetPluginSigningInfo(t *testing.T) {
	tests := []struct {
		name         string
		setup        func(t *testing.T, pluginsDir string) (pluginDir string, metadata Metadata)
		expectStatus string
		expectSigned bool
	}{
		{
			name: "symlinked plugin is local dev",
			setup: func(t *testing.T, pluginsDir string) (string, Metadata) {
				t.Helper()
				srcDir, metadata := installTestPlugin(t, t.TempDir())
				pluginDir := filepath.Join(pluginsDir, "test-plugin")
				require.NoError(t, os.Symlink(srcDir, pluginDir))
				return pluginDir, metadata
			},
			expectStatus: "local dev",
		},
		{
			name:         "no tarball is unsigned",
			setup:        installTestPlugin,
			expectStatus: "unsigned",
		},
		{
			name: "tarball without provenance is unsigned",
			setup: func(t *testing.T, pluginsDir string) (string, Metadata) {
				t.Helper()
				pluginDir, metadata := installTestPlugin(t, pluginsDir)
				writeTestTarball(t, pluginsDir, pluginDir, metadata)
				return pluginDir, metadata
			},
			expectStatus: "unsigned",
		},
		{
			name: "malformed provenance is invalid",
			setup: func(t *testing.T, pluginsDir string) (string, Metadata) {
				t.Helper()
				pluginDir, metadata := installTestPlugin(t, pluginsDir)
				tarballPath := writeTestTarball(t, pluginsDir, pluginDir, metadata)
				require.NoError(t, os.WriteFile(tarballPath+".prov", []byte("not a clearsigned message"), 0o644))
				return pluginDir, metadata
			},
			expectStatus: "invalid provenance",
		},
		{
			name: "provenance for another tarball is mismatched",
			setup: func(t *testing.T, pluginsDir string) (string, Metadata) {
				t.Helper()
				pluginDir, metadata := installTestPlugin(t, pluginsDir)
				tarballPath := writeTestTarball(t, pluginsDir, pluginDir, metadata)
				sig := signTestTarball(t, tarballPath)
				// Tamper with the tarball after signing it
				require.NoError(t, os.WriteFile(tarballPath, []byte("tampered"), 0o644))
				require.NoError(t, os.WriteFile(tarballPath+".prov", []byte(sig), 0o644))
				return pluginDir, metadata
			},
			expectStatus: "mismatched provenance",
		},
		{
			name: "matching provenance is signed",
			setup: func(t *testing.T, pluginsDir string) (string, Metadata) {
				t.Helper()
				pluginDir, metadata := installTestPlugin(t, pluginsDir)
				tarballPath := writeTestTarball(t, pluginsDir, pluginDir, metadata)
				sig := signTestTarball(t, tarballPath)
				require.NoError(t, os.WriteFile(tarballPath+".prov", []byte(sig), 0o644))
				return pluginDir, metadata
			},
			expectStatus: "signed",
			expectSigned: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// The plugins directory is deliberately not the default Helm data
			// path: signing info must follow the plugin directory, wherever
			// HELM_PLUGINS points to
			pluginsDir := t.TempDir()
			pluginDir, metadata := tt.setup(t, pluginsDir)

			info, err := GetPluginSigningInfo(pluginDir, metadata)
			require.NoError(t, err)
			assert.Equal(t, tt.expectStatus, info.Status)
			assert.Equal(t, tt.expectSigned, info.IsSigned)
		})
	}
}

func TestGetPluginSigningInfoMissingPlugin(t *testing.T) {
	pluginDir := filepath.Join(t.TempDir(), "missing-plugin")

	_, err := GetPluginSigningInfo(pluginDir, Metadata{Name: "missing-plugin", Version: "1.0.0"})
	require.ErrorContains(t, err, "plugin missing-plugin not found")
}

func TestGetSigningInfoForPlugins(t *testing.T) {
	pluginsDir := t.TempDir()

	signedDir, signedMetadata := installTestPlugin(t, pluginsDir)
	tarballPath := writeTestTarball(t, pluginsDir, signedDir, signedMetadata)
	require.NoError(t, os.WriteFile(tarballPath+".prov", []byte(signTestTarball(t, tarballPath)), 0o644))
	signed, err := LoadDir(signedDir)
	require.NoError(t, err)

	// A plugin whose directory went away is reported as unknown rather than failing the whole listing
	goneDir := filepath.Join(pluginsDir, "gone-plugin")
	require.NoError(t, os.MkdirAll(goneDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(goneDir, "plugin.yaml"), []byte(`apiVersion: v1
name: gone-plugin
type: cli/v1
runtime: subprocess
version: 1.0.0
runtimeConfig:
  platformCommand:
    - command: echo`), 0o644))
	gone, err := LoadDir(goneDir)
	require.NoError(t, err)
	require.NoError(t, os.RemoveAll(goneDir))

	infos := GetSigningInfoForPlugins([]Plugin{signed, gone})

	require.Len(t, infos, 2)
	assert.Equal(t, "signed", infos["test-plugin"].Status)
	assert.True(t, infos["test-plugin"].IsSigned)
	assert.Equal(t, "unknown", infos["gone-plugin"].Status)
	assert.False(t, infos["gone-plugin"].IsSigned)
}
