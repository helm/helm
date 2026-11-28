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
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"helm.sh/helm/v4/pkg/helmpath"
)

// setPluginsDir points HELM_PLUGINS to dir, or makes sure it is unset when
// dir is empty, so that the default cases do not depend on the environment
// the tests run in
func setPluginsDir(t *testing.T, dir string) {
	t.Helper()
	t.Setenv("HELM_PLUGINS", dir)
	if dir == "" {
		os.Unsetenv("HELM_PLUGINS")
	}
}

func TestPath(t *testing.T) {
	customPluginsDir := "/foo/bar"

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
			helmPluginsDir: customPluginsDir,
			expectPath:     "",
		}, {
			name:           "custom helm plugins dir",
			source:         "https://github.com/adamreese/helm-env",
			helmPluginsDir: customPluginsDir,
			expectPath:     filepath.Join(customPluginsDir, "helm-env"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setPluginsDir(t, tt.helmPluginsDir)
			installer := newBase(tt.source)
			path := installer.Path()
			assert.Equal(t, tt.expectPath, path)
		})
	}
}
