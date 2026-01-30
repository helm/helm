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

	"helm.sh/helm/v4/pkg/helmpath"
)

func Test_Path(t *testing.T) {
	tests := []struct {
		name           string
		source         string
		helmPluginsDir string
		expectPath     string
	}{
		{
			name:           "empty source default helm plugins dir",
			source:         "",
			helmPluginsDir: "",
			expectPath:     "",
		}, {
			name:           "default helm plugins dir",
			source:         "https://github.com/adamreese/helm-env",
			helmPluginsDir: "",
			expectPath:     helmpath.DataPath("plugins", "helm-env"),
		}, {
			name:           "empty source custom helm plugins dir",
			source:         "",
			helmPluginsDir: "/foo/bar",
			expectPath:     "",
		}, {
			name:           "custom helm plugins dir",
			source:         "https://github.com/adamreese/helm-env",
			helmPluginsDir: "/foo/bar",
			expectPath:     "/foo/bar/helm-env",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.helmPluginsDir != "" {
				t.Setenv("HELM_PLUGINS", tt.helmPluginsDir)
			}
			installer := newBase(tt.source)
			path := installer.Path()
			if path != tt.expectPath {
				t.Errorf("expected path %s, got %s", tt.expectPath, path)
			}
		})
	}
}
