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
		{"valid name", "my-plugin", true},
		{"invalid with space", "my plugin", false},
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
