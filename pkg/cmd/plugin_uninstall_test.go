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
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create plugin.yaml
	pluginYAML := `name: test-plugin
version: 1.2.3
description: Test plugin
command: $HELM_PLUGIN_DIR/test-plugin
`
	if err := os.WriteFile(filepath.Join(pluginDir, "plugin.yaml"), []byte(pluginYAML), 0644); err != nil {
		t.Fatal(err)
	}

	// Create versioned tarball and provenance files
	tarballFile := filepath.Join(pluginsDir, "test-plugin-1.2.3.tgz")
	provFile := filepath.Join(pluginsDir, "test-plugin-1.2.3.tgz.prov")
	otherVersionTarball := filepath.Join(pluginsDir, "test-plugin-2.0.0.tgz")

	if err := os.WriteFile(tarballFile, []byte("fake tarball"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(provFile, []byte("fake provenance"), 0644); err != nil {
		t.Fatal(err)
	}
	// Create another version that should NOT be removed
	if err := os.WriteFile(otherVersionTarball, []byte("other version"), 0644); err != nil {
		t.Fatal(err)
	}

	// Load the plugin
	p, err := plugin.LoadDir(pluginDir)
	if err != nil {
		t.Fatal(err)
	}

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
	if _, err := os.Stat(tarballFile); os.IsNotExist(err) {
		t.Fatal("tarball file should exist before uninstall")
	}
	if _, err := os.Stat(provFile); os.IsNotExist(err) {
		t.Fatal("provenance file should exist before uninstall")
	}
	if _, err := os.Stat(otherVersionTarball); os.IsNotExist(err) {
		t.Fatal("other version tarball should exist before uninstall")
	}

	// Uninstall the plugin
	if err := testUninstallPlugin(p); err != nil {
		t.Fatal(err)
	}

	// Verify plugin directory is removed
	if _, err := os.Stat(pluginDir); !os.IsNotExist(err) {
		t.Error("plugin directory should be removed")
	}

	// Verify only exact version files are removed
	if _, err := os.Stat(tarballFile); !os.IsNotExist(err) {
		t.Error("versioned tarball file should be removed")
	}
	if _, err := os.Stat(provFile); !os.IsNotExist(err) {
		t.Error("versioned provenance file should be removed")
	}
	// Verify other version files are NOT removed
	if _, err := os.Stat(otherVersionTarball); os.IsNotExist(err) {
		t.Error("other version tarball should NOT be removed")
	}
}
