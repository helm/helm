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

package v2

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

func TestDependencyDependsOnJSONOmitEmpty(t *testing.T) {
	dependency := Dependency{
		Name:       "app",
		Repository: "https://example.com/charts",
	}

	data, err := json.Marshal(dependency)
	require.NoError(t, err)

	assert.NotContains(t, string(data), "depends-on")
}

func TestDependencyDependsOnJSONTag(t *testing.T) {
	dependency := Dependency{
		Name:       "app",
		Repository: "https://example.com/charts",
		DependsOn:  []string{"database"},
	}

	data, err := json.Marshal(dependency)
	require.NoError(t, err)

	assert.Contains(t, string(data), `"depends-on":["database"]`)
}

func TestDependencyDependsOnYAMLBackwardCompatibility(t *testing.T) {
	input := []byte(`
name: app
repository: https://example.com/charts
`)

	var dependency Dependency
	err := yaml.Unmarshal(input, &dependency)
	require.NoError(t, err)

	assert.Nil(t, dependency.DependsOn)
}
