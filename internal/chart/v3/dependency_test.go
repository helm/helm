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
package v3

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

func TestValidateDependency(t *testing.T) {
	dep := &Dependency{
		Name: "example",
	}
	for value, shouldFail := range map[string]bool{
		"abcdefghijklmenopQRSTUVWXYZ-0123456780_": false,
		"-okay":      false,
		"_okay":      false,
		"- bad":      true,
		" bad":       true,
		"bad\nvalue": true,
		"bad ":       true,
		"bad$":       true,
	} {
		dep.Alias = value
		res := dep.Validate()
		if shouldFail {
			assert.Errorf(t, res, "Expected failure for %q", dep.Alias)
		} else {
			assert.NoErrorf(t, res, "Failed on case %q", dep.Alias)
		}
	}
}

func TestValidateDependencySanitizesDependsOn(t *testing.T) {
	dep := &Dependency{
		Name:      "example",
		DependsOn: []string{"data\abase", "cache\rqueue"},
	}

	require.NoError(t, dep.Validate())
	assert.Equal(t, []string{"database", "cache queue"}, dep.DependsOn)
}

func TestDependencyDependsOnJSON(t *testing.T) {
	tests := []struct {
		name       string
		dependsOn  []string
		wantJSON   string
		unexpected string
	}{
		{
			name:       "omits empty field",
			unexpected: "depends-on",
		},
		{
			name:      "uses depends-on tag",
			dependsOn: []string{"database"},
			wantJSON:  `"depends-on":["database"]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dependency := Dependency{
				Name:       "app",
				Repository: "https://example.com/charts",
				DependsOn:  tt.dependsOn,
			}

			data, err := json.Marshal(dependency)
			require.NoError(t, err)
			if tt.wantJSON != "" {
				assert.Contains(t, string(data), tt.wantJSON)
			}
			if tt.unexpected != "" {
				assert.NotContains(t, string(data), tt.unexpected)
			}
		})
	}
}

func TestDependencyDependsOnYAML(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		dependsOn []string
	}{
		{
			name: "populates field",
			input: `
name: app
repository: https://example.com/charts
depends-on:
  - database
  - cache
`,
			dependsOn: []string{"database", "cache"},
		},
		{
			name: "absent field remains nil",
			input: `
name: app
repository: https://example.com/charts
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var dependency Dependency
			require.NoError(t, yaml.Unmarshal([]byte(tt.input), &dependency))
			assert.Equal(t, tt.dependsOn, dependency.DependsOn)
		})
	}
}
