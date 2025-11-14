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
)

func TestMetadataLegacyValidate_PluginName(t *testing.T) {
	tests := []struct {
		name       string
		pluginName string
		shouldPass bool
	}{
		// Valid names
		{"lowercase", "myplugin", true},
		{"uppercase", "MYPLUGIN", true},
		{"mixed case", "MyPlugin", true},
		{"with digits", "plugin123", true},
		{"with hyphen", "my-plugin", true},
		{"with underscore", "my_plugin", true},
		{"mixed chars", "my-awesome_plugin_123", true},

		// Invalid names
		{"empty", "", false},
		{"space", "my plugin", false},
		{"colon", "Name:", false},
		{"period", "my.plugin", false},
		{"slash", "my/plugin", false},
		{"dollar", "$plugin", false},
		{"unicode", "plügîn", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metadata := MetadataLegacy{
				Name:    tt.pluginName,
				Version: "1.0.0",
			}

			err := metadata.Validate()

			if tt.shouldPass && err != nil {
				t.Errorf("expected %q to be valid, got error: %v", tt.pluginName, err)
			}
			if !tt.shouldPass && err == nil {
				t.Errorf("expected %q to be invalid, but passed", tt.pluginName)
			}
		})
	}
}
