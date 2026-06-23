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
	"path/filepath"
	"testing"
)

func TestPath(t *testing.T) {
	pluginsDir := filepath.FromSlash("/helm/data/plugins")
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
			source:         "https://github.com/jkroepke/helm-secrets",
			helmPluginsDir: pluginsDir,
			expectPath:     filepath.Join(pluginsDir, "helm-secrets"),
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

func TestPathMultiplePluginDirs(t *testing.T) {
	// When HELM_PLUGINS contains a list of paths, install into the first one.
	first := filepath.FromSlash("/helm/data/plugins")
	second := filepath.FromSlash("/helm/extra/plugins")
	multiPath := first + string(filepath.ListSeparator) + second

	t.Setenv("HELM_PLUGINS", multiPath)
	b := newBase("https://github.com/jkroepke/helm-secrets")
	got := b.Path()
	expected := filepath.Join(first, "helm-secrets")
	if got != expected {
		t.Errorf("expected path %s, got %s", expected, got)
	}
}

func TestPathEmptyPluginDir(t *testing.T) {
	// When HELM_PLUGINS is explicitly empty, newBase must not panic.
	t.Setenv("HELM_PLUGINS", "")
	b := newBase("https://github.com/jkroepke/helm-secrets")
	// Path() only returns "" when Source is ""; with an empty PluginsDirectory it
	// returns a relative path. Just verify no panic.
	_ = b.Path()
}

func TestPathSkipsEmptyPluginDirs(t *testing.T) {
	// A leading empty segment (e.g. ":/real/path") must be skipped so the plugin is
	// installed into the first real directory rather than a relative path.
	realDir := filepath.FromSlash("/helm/data/plugins")
	multiPath := string(filepath.ListSeparator) + realDir

	t.Setenv("HELM_PLUGINS", multiPath)
	b := newBase("https://github.com/jkroepke/helm-secrets")
	got := b.Path()
	expected := filepath.Join(realDir, "helm-secrets")
	if got != expected {
		t.Errorf("expected path %s, got %s", expected, got)
	}
}
