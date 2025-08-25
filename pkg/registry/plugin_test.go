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

package registry

import (
	"testing"
)

func TestGetPluginName(t *testing.T) {
	tests := []struct {
		name      string
		source    string
		expected  string
		expectErr bool
	}{
		{
			name:     "valid OCI reference with tag",
			source:   "oci://ghcr.io/user/plugin-name:v1.0.0",
			expected: "plugin-name",
		},
		{
			name:     "valid OCI reference with digest",
			source:   "oci://ghcr.io/user/plugin-name@sha256:1234567890abcdef",
			expected: "plugin-name",
		},
		{
			name:     "valid OCI reference without tag",
			source:   "oci://ghcr.io/user/plugin-name",
			expected: "plugin-name",
		},
		{
			name:     "valid OCI reference with multiple path segments",
			source:   "oci://registry.example.com/org/team/plugin-name:latest",
			expected: "plugin-name",
		},
		{
			name:     "valid OCI reference with plus signs in tag",
			source:   "oci://registry.example.com/user/plugin-name:v1.0.0+build.1",
			expected: "plugin-name",
		},
		{
			name:     "valid OCI reference - single path segment",
			source:   "oci://registry.example.com/plugin",
			expected: "plugin",
		},
		{
			name:      "invalid OCI reference - no repository",
			source:    "oci://registry.example.com",
			expectErr: true,
		},
		{
			name:      "invalid OCI reference - malformed",
			source:    "not-an-oci-reference",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pluginName, err := GetPluginName(tt.source)

			if tt.expectErr {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if pluginName != tt.expected {
				t.Errorf("expected plugin name %q, got %q", tt.expected, pluginName)
			}
		})
	}
}
