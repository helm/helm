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

	"helm.sh/helm/v4/pkg/cli"
)

func TestSetupEnv(t *testing.T) {
	name := "pequod"
	base := filepath.Join("testdata/helmhome/helm/plugins", name)

	s := cli.New()
	s.PluginsDirectory = "testdata/helmhome/helm/plugins"

	SetupPluginEnv(s, name, base)
	for _, tt := range []struct {
		name, expect string
	}{
		{"HELM_PLUGIN_NAME", name},
		{"HELM_PLUGIN_DIR", base},
	} {
		if got := os.Getenv(tt.name); got != tt.expect {
			t.Errorf("Expected $%s=%q, got %q", tt.name, tt.expect, got)
		}
	}
}

func TestSetupEnvWithSpace(t *testing.T) {
	name := "sureshdsk"
	base := filepath.Join("testdata/helm home/helm/plugins", name)

	s := cli.New()
	s.PluginsDirectory = "testdata/helm home/helm/plugins"

	SetupPluginEnv(s, name, base)
	for _, tt := range []struct {
		name, expect string
	}{
		{"HELM_PLUGIN_NAME", name},
		{"HELM_PLUGIN_DIR", base},
	} {
		if got := os.Getenv(tt.name); got != tt.expect {
			t.Errorf("Expected $%s=%q, got %q", tt.name, tt.expect, got)
		}
	}
}
