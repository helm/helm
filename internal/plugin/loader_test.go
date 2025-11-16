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
	"bytes"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"helm.sh/helm/v4/internal/plugin/schema"
)

func TestPeekAPIVersion(t *testing.T) {
	testCases := map[string]struct {
		data     []byte
		expected string
	}{
		"v1": {
			data: []byte(`---
apiVersion: v1
name: "test-plugin"
`),
			expected: "v1",
		},
		"legacy": { // No apiVersion field
			data: []byte(`---
name: "test-plugin"
`),
			expected: "",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			version, err := peekAPIVersion(bytes.NewReader(tc.data))
			require.NoError(t, err)
			assert.Equal(t, tc.expected, version)
		})
	}

	// invalid yaml
	{
		data := []byte(`bad yaml`)
		_, err := peekAPIVersion(bytes.NewReader(data))
		assert.Error(t, err)
	}
}

func TestLoadDir(t *testing.T) {

	makeMetadata := func(apiVersion string) Metadata {
		usage := "hello [params]..."
		if apiVersion == "legacy" {
			usage = "" // Legacy plugins don't have Usage field for command syntax
		}
		return Metadata{
			APIVersion: apiVersion,
			Name:       fmt.Sprintf("hello-%s", apiVersion),
			Version:    "0.1.0",
			Type:       "cli/v1",
			Runtime:    "subprocess",
			Config: &schema.ConfigCLIV1{
				Usage:       usage,
				ShortHelp:   "echo hello message",
				LongHelp:    "description",
				IgnoreFlags: true,
			},
			RuntimeConfig: &RuntimeConfigSubprocess{
				PlatformCommand: []PlatformCommand{
					{OperatingSystem: "linux", Architecture: "", Command: "sh", Args: []string{"-c", "${HELM_PLUGIN_DIR}/hello.sh"}},
					{OperatingSystem: "windows", Architecture: "", Command: "pwsh", Args: []string{"-c", "${HELM_PLUGIN_DIR}/hello.ps1"}},
				},
				PlatformHooks: map[string][]PlatformCommand{
					Install: {
						{OperatingSystem: "linux", Architecture: "", Command: "sh", Args: []string{"-c", "echo \"installing...\""}},
						{OperatingSystem: "windows", Architecture: "", Command: "pwsh", Args: []string{"-c", "echo \"installing...\""}},
					},
				},
				expandHookArgs: apiVersion == "legacy",
			},
		}
	}

	testCases := map[string]struct {
		dirname    string
		apiVersion string
		expect     Metadata
	}{
		"legacy": {
			dirname:    "testdata/plugdir/good/hello-legacy",
			apiVersion: "legacy",
			expect:     makeMetadata("legacy"),
		},
		"v1": {
			dirname:    "testdata/plugdir/good/hello-v1",
			apiVersion: "v1",
			expect:     makeMetadata("v1"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			plug, err := LoadDir(tc.dirname)
			require.NoError(t, err, "error loading plugin from %s", tc.dirname)

			assert.Equal(t, tc.dirname, plug.Dir())
			assert.EqualValues(t, tc.expect, plug.Metadata())
		})
	}
}

func TestLoadDirDuplicateEntries(t *testing.T) {
	testCases := map[string]string{
		"legacy": "testdata/plugdir/bad/duplicate-entries-legacy",
		"v1":     "testdata/plugdir/bad/duplicate-entries-v1",
	}
	for name, dirname := range testCases {
		t.Run(name, func(t *testing.T) {
			_, err := LoadDir(dirname)
			assert.Error(t, err)
		})
	}
}

func TestLoadDirGetter(t *testing.T) {
	dirname := "testdata/plugdir/good/getter"

	expect := Metadata{
		Name:       "getter",
		Version:    "1.2.3",
		Type:       "getter/v1",
		APIVersion: "v1",
		Runtime:    "subprocess",
		Config: &schema.ConfigGetterV1{
			Protocols: []string{"myprotocol", "myprotocols"},
		},
		RuntimeConfig: &RuntimeConfigSubprocess{
			ProtocolCommands: []SubprocessProtocolCommand{
				{
					Protocols:       []string{"myprotocol", "myprotocols"},
					PlatformCommand: []PlatformCommand{{Command: "echo getter"}},
				},
			},
		},
	}

	plug, err := LoadDir(dirname)
	require.NoError(t, err)
	assert.Equal(t, dirname, plug.Dir())
	assert.Equal(t, expect, plug.Metadata())
}

func TestPostRenderer(t *testing.T) {
	dirname := "testdata/plugdir/good/postrenderer-v1"

	expect := Metadata{
		Name:       "postrenderer-v1",
		Version:    "1.2.3",
		Type:       "postrenderer/v1",
		APIVersion: "v1",
		Runtime:    "subprocess",
		Config:     &schema.ConfigPostRendererV1{},
		RuntimeConfig: &RuntimeConfigSubprocess{
			PlatformCommand: []PlatformCommand{
				{
					Command: "${HELM_PLUGIN_DIR}/sed-test.sh",
				},
			},
		},
	}

	plug, err := LoadDir(dirname)
	require.NoError(t, err)
	assert.Equal(t, dirname, plug.Dir())
	assert.Equal(t, expect, plug.Metadata())
}

func TestDetectDuplicates(t *testing.T) {
	plugs := []Plugin{
		mockSubprocessCLIPlugin(t, "foo"),
		mockSubprocessCLIPlugin(t, "bar"),
	}
	if err := detectDuplicates(plugs); err != nil {
		t.Error("no duplicates in the first set")
	}
	plugs = append(plugs, mockSubprocessCLIPlugin(t, "foo"))
	if err := detectDuplicates(plugs); err == nil {
		t.Error("duplicates in the second set")
	}
}

func TestLoadAll(t *testing.T) {
	// Verify that empty dir loads:
	{
		plugs, err := LoadAll("testdata")
		require.NoError(t, err)
		assert.Len(t, plugs, 0)
	}

	basedir := "testdata/plugdir/good"
	plugs, err := LoadAll(basedir)
	require.NoError(t, err)
	require.NotEmpty(t, plugs, "expected plugins to be loaded from %s", basedir)

	plugsMap := map[string]Plugin{}
	for _, p := range plugs {
		plugsMap[p.Metadata().Name] = p
	}

	assert.Len(t, plugsMap, 7)
	assert.Contains(t, plugsMap, "downloader")
	assert.Contains(t, plugsMap, "echo-legacy")
	assert.Contains(t, plugsMap, "echo-v1")
	assert.Contains(t, plugsMap, "getter")
	assert.Contains(t, plugsMap, "hello-legacy")
	assert.Contains(t, plugsMap, "hello-v1")
	assert.Contains(t, plugsMap, "postrenderer-v1")
}

func TestFindPlugins(t *testing.T) {
	cases := []struct {
		name     string
		plugdirs string
		expected int
	}{
		{
			name:     "plugdirs is empty",
			plugdirs: "",
			expected: 0,
		},
		{
			name:     "plugdirs isn't dir",
			plugdirs: "./plugin_test.go",
			expected: 0,
		},
		{
			name:     "plugdirs doesn't have plugin",
			plugdirs: ".",
			expected: 0,
		},
		{
			name:     "normal",
			plugdirs: "./testdata/plugdir/good",
			expected: 7,
		},
	}
	for _, c := range cases {
		t.Run(t.Name(), func(t *testing.T) {
			plugin, err := LoadAll(c.plugdirs)
			require.NoError(t, err)
			assert.Len(t, plugin, c.expected, "expected %d plugins, got %d", c.expected, len(plugin))
		})
	}
}

func TestLoadMetadataLegacy(t *testing.T) {
	testCases := map[string]struct {
		yaml          string
		expectError   bool
		errorContains string
		expectedName  string
		logNote       string
	}{
		"capital name field": {
			yaml: `Name: my-plugin
version: 1.0.0
usage: test plugin
description: test description
command: echo test`,
			expectError:   true,
			errorContains: `invalid plugin name "": must contain only a-z, A-Z, 0-9, _ and -`,
			// Legacy plugins: No strict unmarshalling (backwards compatibility)
			// YAML decoder silently ignores "Name:", then validation catches empty name
			logNote: "NOTE: V1 plugins use strict unmarshalling and would get: yaml: field Name not found",
		},
		"correct name field": {
			yaml: `name: my-plugin
version: 1.0.0
usage: test plugin
description: test description
command: echo test`,
			expectError:  false,
			expectedName: "my-plugin",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			m, err := loadMetadataLegacy([]byte(tc.yaml))

			if tc.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errorContains)
				t.Logf("Legacy error (validation catches empty name): %v", err)
				if tc.logNote != "" {
					t.Log(tc.logNote)
				}
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.expectedName, m.Name)
			}
		})
	}
}

func TestLoadMetadataV1(t *testing.T) {
	testCases := map[string]struct {
		yaml          string
		expectError   bool
		errorContains string
		expectedName  string
	}{
		"capital name field": {
			yaml: `apiVersion: v1
Name: my-plugin
type: cli/v1
runtime: subprocess
`,
			expectError:   true,
			errorContains: "field Name not found in type plugin.MetadataV1",
		},
		"correct name field": {
			yaml: `apiVersion: v1
name: my-plugin
type: cli/v1
runtime: subprocess
`,
			expectError:  false,
			expectedName: "my-plugin",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			m, err := loadMetadataV1([]byte(tc.yaml))

			if tc.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errorContains)
				t.Logf("V1 error (strict unmarshalling): %v", err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.expectedName, m.Name)
			}
		})
	}
}
