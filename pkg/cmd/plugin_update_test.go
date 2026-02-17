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

package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParsePluginVersion(t *testing.T) {
	tests := []struct {
		arg         string
		wantName    string
		wantVersion string
	}{
		{"myplugin", "myplugin", ""},
		{"myplugin@1.2.3", "myplugin", "1.2.3"},
		{"myplugin@v1.2.3", "myplugin", "v1.2.3"},
		{"myplugin@", "myplugin", ""},
		{"@version", "", "version"},
		{"", "", ""},
		// LastIndex ensures the last @ is used as delimiter
		{"weird@name@1.0", "weird@name", "1.0"},
	}
	for _, tt := range tests {
		t.Run(tt.arg, func(t *testing.T) {
			name, version := parsePluginVersion(tt.arg)
			assert.Equal(t, tt.wantName, name)
			assert.Equal(t, tt.wantVersion, version)
		})
	}
}

func TestPluginUpdateComplete(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		wantPlugins map[string]string
		wantErr     string
	}{
		{
			name:    "no args",
			args:    []string{},
			wantErr: "please provide plugin name to update",
		},
		{
			name:        "single plugin no version",
			args:        []string{"myplugin"},
			wantPlugins: map[string]string{"myplugin": ""},
		},
		{
			name:        "single plugin with inline version",
			args:        []string{"myplugin@1.2.3"},
			wantPlugins: map[string]string{"myplugin": "1.2.3"},
		},
		{
			name:        "multiple plugins no versions",
			args:        []string{"plugin-a", "plugin-b"},
			wantPlugins: map[string]string{"plugin-a": "", "plugin-b": ""},
		},
		{
			name:        "multiple plugins with inline versions",
			args:        []string{"plugin-a@1.0.0", "plugin-b@2.0.0"},
			wantPlugins: map[string]string{"plugin-a": "1.0.0", "plugin-b": "2.0.0"},
		},
		{
			name:        "multiple plugins each with different exact versions",
			args:        []string{"plugin-a@1.2.3", "plugin-b@2.0.0", "plugin-c@3.0.0"},
			wantPlugins: map[string]string{"plugin-a": "1.2.3", "plugin-b": "2.0.0", "plugin-c": "3.0.0"},
		},
		{
			name:        "multiple plugins mixed versions",
			args:        []string{"plugin-a@1.0.0", "plugin-b"},
			wantPlugins: map[string]string{"plugin-a": "1.0.0", "plugin-b": ""},
		},
		{
			name:        "multiple plugins mixed with latest in the middle",
			args:        []string{"plugin-a@1.0.0", "plugin-b", "plugin-c@3.0.0"},
			wantPlugins: map[string]string{"plugin-a": "1.0.0", "plugin-b": "", "plugin-c": "3.0.0"},
		},
		{
			name:    "duplicate plugin name errors",
			args:    []string{"myplugin@1.0.0", "myplugin@2.0.0"},
			wantErr: `plugin "myplugin" specified more than once`,
		},
		{
			name:    "empty plugin name errors",
			args:    []string{"@1.0.0"},
			wantErr: `invalid plugin reference "@1.0.0": plugin name must not be empty`,
		},
		{
			name:    "v-prefixed version rejected",
			args:    []string{"myplugin@v1.2.3"},
			wantErr: `invalid version "v1.2.3" for plugin "myplugin": must be an exact semver version (e.g. 1.2.3); the "v" prefix is not allowed`,
		},
		{
			name:    "tilde range version rejected",
			args:    []string{"myplugin@~1.2"},
			wantErr: `invalid version "~1.2" for plugin "myplugin": must be an exact semver version (e.g. 1.2.3); the "v" prefix is not allowed`,
		},
		{
			name:    "caret range version rejected",
			args:    []string{"myplugin@^1.2.3"},
			wantErr: `invalid version "^1.2.3" for plugin "myplugin": must be an exact semver version (e.g. 1.2.3); the "v" prefix is not allowed`,
		},
		{
			name:    "gte constraint rejected",
			args:    []string{"myplugin@>=1.0.0"},
			wantErr: `invalid version ">=1.0.0" for plugin "myplugin": must be an exact semver version (e.g. 1.2.3); the "v" prefix is not allowed`,
		},
		{
			name:    "wildcard version rejected",
			args:    []string{"myplugin@1.x"},
			wantErr: `invalid version "1.x" for plugin "myplugin": must be an exact semver version (e.g. 1.2.3); the "v" prefix is not allowed`,
		},
		{
			name:    "range constraint rejected",
			args:    []string{"myplugin@>=1.0.0, <2.0.0"},
			wantErr: `invalid version ">=1.0.0, <2.0.0" for plugin "myplugin": must be an exact semver version (e.g. 1.2.3); the "v" prefix is not allowed`,
		},
		{
			name:    "garbage version rejected",
			args:    []string{"myplugin@notaversion"},
			wantErr: `invalid version "notaversion" for plugin "myplugin": must be an exact semver version (e.g. 1.2.3); the "v" prefix is not allowed`,
		},
		{
			name:    "range rejected among multiple plugins",
			args:    []string{"plugin-a@1.0.0", "plugin-b@~2.0"},
			wantErr: `invalid version "~2.0" for plugin "plugin-b": must be an exact semver version (e.g. 1.2.3); the "v" prefix is not allowed`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := &pluginUpdateOptions{}
			err := o.complete(tt.args)
			if tt.wantErr != "" {
				require.EqualError(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantPlugins, o.plugins)
		})
	}
}
