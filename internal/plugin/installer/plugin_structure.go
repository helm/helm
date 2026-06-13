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
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"helm.sh/helm/v4/internal/plugin"
)

// detectPluginRoot searches for plugin.yaml in the extracted directory
// and returns the path to the directory containing it.
// This handles cases where the tarball contains the plugin in a subdirectory.
func detectPluginRoot(extractDir string) (string, error) {
	// First check if plugin.yaml is at the root
	if _, err := os.Stat(filepath.Join(extractDir, plugin.PluginFileName)); err == nil {
		return extractDir, nil
	}

	// Otherwise, look for plugin.yaml in subdirectories (only one level deep)
	entries, err := os.ReadDir(extractDir)
	if err != nil {
		return "", err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			subdir := filepath.Join(extractDir, entry.Name())
			if _, err := os.Stat(filepath.Join(subdir, plugin.PluginFileName)); err == nil {
				return subdir, nil
			}
		}
	}

	return "", fmt.Errorf("plugin.yaml not found in %s or its immediate subdirectories", extractDir)
}

// validatePluginName checks if the plugin directory name matches the plugin name
// from plugin.yaml when the plugin is in a subdirectory.
func validatePluginName(pluginRoot string, expectedName string) error {
	// Only validate if plugin is in a subdirectory
	dirName := filepath.Base(pluginRoot)
	if dirName == expectedName {
		return nil
	}

	// Load plugin.yaml to get the actual name
	p, err := plugin.LoadDir(pluginRoot)
	if err != nil {
		return fmt.Errorf("failed to load plugin from %s: %w", pluginRoot, err)
	}

	m := p.Metadata()
	actualName := m.Name

	// For now, just log a warning if names don't match
	// In the future, we might want to enforce this more strictly
	if actualName != dirName && actualName != strings.TrimSuffix(expectedName, filepath.Ext(expectedName)) {
		// This is just informational - not an error
		return nil
	}

	return nil
}
