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

package installer // import "helm.sh/helm/v4/internal/plugin/installer"

import (
	"fmt"
	"path/filepath"
	"testing"
)

func TestPath(t *testing.T) {
	pluginsDir := filepath.Join(string(filepath.Separator), "helm", "data", "plugins")
	systemPluginsDir := filepath.Join(string(filepath.Separator), "helm", "system", "plugins")
	pluginSource := "https://github.com/jkroepke/helm-secrets"

	tests := []struct {
		source         string
		helmPluginsDir string
		expectPath     string
	}{
		{
			source:         "",
			helmPluginsDir: pluginsDir,
			expectPath:     "",
		}, {
			source:         pluginSource,
			helmPluginsDir: pluginsDir,
			expectPath:     filepath.Join(pluginsDir, filepath.Base(pluginSource)),
		}, {
			source:         pluginSource,
			helmPluginsDir: fmt.Sprintf("%s%c%s", pluginsDir, filepath.ListSeparator, systemPluginsDir),
			expectPath:     filepath.Join(pluginsDir, filepath.Base(pluginSource)),
		},
	}

	for _, tt := range tests {
		t.Setenv("HELM_PLUGINS", tt.helmPluginsDir)
		baseIns := newBase(tt.source)
		baseInsPath := baseIns.Path()
		if baseInsPath != tt.expectPath {
			t.Errorf("expected name %s, got %s", tt.expectPath, baseInsPath)
		}
	}
}
