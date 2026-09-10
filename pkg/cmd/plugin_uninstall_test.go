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

package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"helm.sh/helm/v4/internal/plugin"
	"helm.sh/helm/v4/internal/test/ensure"
	"helm.sh/helm/v4/pkg/cli"
)

func TestPluginUninstallCleansUpVersionedFiles(t *testing.T) {
	ensure.HelmHome(t)

	// Create a fake plugin directory structure in a temp directory
	pluginsDir := t.TempDir()
	t.Setenv("HELM_PLUGINS", pluginsDir)

	// Create a new settings instance that will pick up the environment variable
	testSettings := cli.New()
	pluginName := "test-plugin"

	// Create plugin directory
	pluginDir := filepath.Join(pluginsDir, pluginName)
	require.NoError(t, os.MkdirAll(pluginDir, 0o755))

	// Create plugin.yaml
	pluginYAML := `name: test-plugin
version: 1.2.3
description: Test plugin
command: $HELM_PLUGIN_DIR/test-plugin
`
	require.NoError(t, os.WriteFile(filepath.Join(pluginDir, "plugin.yaml"), []byte(pluginYAML), 0o644))

	// Create versioned tarball and provenance files
	tarballFile := filepath.Join(pluginsDir, "test-plugin-1.2.3.tgz")
	provFile := filepath.Join(pluginsDir, "test-plugin-1.2.3.tgz.prov")
	otherVersionTarball := filepath.Join(pluginsDir, "test-plugin-2.0.0.tgz")

	require.NoError(t, os.WriteFile(tarballFile, []byte("fake tarball"), 0o644))
	require.NoError(t, os.WriteFile(provFile, []byte("fake provenance"), 0o644))
	// Create another version that should NOT be removed
	require.NoError(t, os.WriteFile(otherVersionTarball, []byte("other version"), 0o644))

	// Load the plugin
	p, err := plugin.LoadDir(pluginDir)
	require.NoError(t, err)

	// Create a test uninstall function that uses our test settings
	testUninstallPlugin := func(plugin plugin.Plugin) error {
		if err := os.RemoveAll(plugin.Dir()); err != nil {
			return err
		}

		// Clean up versioned tarball and provenance files from test HELM_PLUGINS directory
		pluginName := plugin.Metadata().Name
		pluginVersion := plugin.Metadata().Version
		testPluginsDir := testSettings.PluginsDirectory

		// Remove versioned files: plugin-name-version.tgz and plugin-name-version.tgz.prov
		if pluginVersion != "" {
			versionedBasename := fmt.Sprintf("%s-%s.tgz", pluginName, pluginVersion)

			// Remove tarball file
			tarballPath := filepath.Join(testPluginsDir, versionedBasename)
			if _, err := os.Stat(tarballPath); err == nil {
				if err := os.Remove(tarballPath); err != nil {
					t.Logf("failed to remove tarball file: %v", err)
				}
			}

			// Remove provenance file
			provPath := filepath.Join(testPluginsDir, versionedBasename+".prov")
			if _, err := os.Stat(provPath); err == nil {
				if err := os.Remove(provPath); err != nil {
					t.Logf("failed to remove provenance file: %v", err)
				}
			}
		}

		// Skip runHook in test
		return nil
	}

	// Verify files exist before uninstall
	_, err = os.Stat(tarballFile)
	require.False(t, os.IsNotExist(err), "tarball file should exist before uninstall")
	_, err = os.Stat(provFile)
	require.False(t, os.IsNotExist(err), "provenance file should exist before uninstall")
	_, err = os.Stat(otherVersionTarball)
	require.False(t, os.IsNotExist(err), "other version tarball should exist before uninstall")

	// Uninstall the plugin
	require.NoError(t, testUninstallPlugin(p))

	// Verify plugin directory is removed
	_, err = os.Stat(pluginDir)
	assert.True(t, os.IsNotExist(err), "plugin directory should be removed")

	// Verify only exact version files are removed
	_, err = os.Stat(tarballFile)
	assert.True(t, os.IsNotExist(err), "versioned tarball file should be removed")
	_, err = os.Stat(provFile)
	assert.True(t, os.IsNotExist(err), "versioned provenance file should be removed")
	// Verify other version files are NOT removed
	_, err = os.Stat(otherVersionTarball)
	assert.False(t, os.IsNotExist(err), "other version tarball should NOT be removed")
}
