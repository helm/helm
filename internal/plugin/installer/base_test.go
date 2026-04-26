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
	"testing"
)

func TestPath(t *testing.T) {
	tests := []struct {
		source         string
		helmPluginsDir string
		expectPath     string
	}{
		{
			source:         "",
			helmPluginsDir: "/helm/data/plugins",
			expectPath:     "",
		}, {
			source:         "https://github.com/jkroepke/helm-secrets",
			helmPluginsDir: "/helm/data/plugins",
			expectPath:     "/helm/data/plugins/helm-secrets",
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

func TestPluginInstallDir(t *testing.T) {
	tests := []struct {
		name        string
		helmPlugins string
		want        string
	}{
		{
			name:        "empty string returns empty string",
			helmPlugins: "",
			want:        "",
		},
		{
			name:        "single path returned as-is",
			helmPlugins: "/tmp/plugins",
			want:        "/tmp/plugins",
		},
		{
			name:        "two paths returns first",
			helmPlugins: "/tmp/abc:/tmp/xyz",
			want:        "/tmp/abc",
		},
		{
			name:        "three paths returns first",
			helmPlugins: "/tmp/a:/tmp/b:/tmp/c",
			want:        "/tmp/a",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pluginInstallDir(tt.helmPlugins)
			if got != tt.want {
				t.Errorf("pluginInstallDir(%q) = %q, want %q", tt.helmPlugins, got, tt.want)
			}
		})
	}
}
