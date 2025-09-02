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
	"testing"

	"helm.sh/helm/v4/internal/plugin/schema"
)

func mockSubprocessCLIPlugin(t *testing.T, pluginName string) *SubprocessPluginRuntime {
	t.Helper()

	rc := RuntimeConfigSubprocess{
		PlatformCommand: []PlatformCommand{
			{OperatingSystem: "darwin", Architecture: "", Command: "sh", Args: []string{"-c", "echo \"mock plugin\""}},
			{OperatingSystem: "linux", Architecture: "", Command: "sh", Args: []string{"-c", "echo \"mock plugin\""}},
			{OperatingSystem: "windows", Architecture: "", Command: "pwsh", Args: []string{"-c", "echo \"mock plugin\""}},
		},
		PlatformHooks: map[string][]PlatformCommand{
			Install: {
				{OperatingSystem: "darwin", Architecture: "", Command: "sh", Args: []string{"-c", "echo \"installing...\""}},
				{OperatingSystem: "linux", Architecture: "", Command: "sh", Args: []string{"-c", "echo \"installing...\""}},
				{OperatingSystem: "windows", Architecture: "", Command: "pwsh", Args: []string{"-c", "echo \"installing...\""}},
			},
		},
	}

	pluginDir := t.TempDir()

	return &SubprocessPluginRuntime{
		metadata: Metadata{
			Name:       pluginName,
			Version:    "v0.1.2",
			Type:       "cli/v1",
			APIVersion: "v1",
			Runtime:    "subprocess",
			Config: &schema.ConfigCLIV1{
				Usage:       "Mock plugin",
				ShortHelp:   "Mock plugin",
				LongHelp:    "Mock plugin for testing",
				IgnoreFlags: false,
			},
			RuntimeConfig: &rc,
		},
		pluginDir:     pluginDir, // NOTE: dir is empty (ie. plugin.yaml is not present)
		RuntimeConfig: rc,
	}
}
