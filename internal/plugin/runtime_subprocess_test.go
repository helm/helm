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
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"

	"helm.sh/helm/v4/internal/plugin/schema"
)

func mockSubprocessCLIPluginErrorExit(t *testing.T, pluginName string, exitCode uint8) *SubprocessPluginRuntime {
	t.Helper()

	rc := RuntimeConfigSubprocess{
		PlatformCommand: []PlatformCommand{
			{Command: "sh", Args: []string{"-c", fmt.Sprintf("echo \"mock plugin $@\"; exit %d", exitCode)}},
		},
	}

	pluginDir := t.TempDir()

	md := Metadata{
		Name:       pluginName,
		Version:    "0.1.2",
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
	}

	data, err := yaml.Marshal(md)
	require.NoError(t, err)
	os.WriteFile(filepath.Join(pluginDir, "plugin.yaml"), data, 0o644)

	return &SubprocessPluginRuntime{
		metadata:      md,
		pluginDir:     pluginDir,
		RuntimeConfig: rc,
	}
}

func TestSubprocessPluginRuntime(t *testing.T) {
	p := mockSubprocessCLIPluginErrorExit(t, "foo", 56)

	output, err := p.Invoke(t.Context(), &Input{
		Message: schema.InputMessageCLIV1{
			ExtraArgs: []string{"arg1", "arg2"},
			// Env:       []string{"FOO=bar"},
		},
	})

	require.Error(t, err)
	ieerr := &InvokeExecError{}
	require.ErrorAs(t, err, &ieerr, "expected InvokeExecError")
	assert.Equal(t, 56, ieerr.ExitCode)

	assert.Nil(t, output)
}

func TestSubprocessPluginRuntimeGetterCommand(t *testing.T) {
	if runtime.GOOS == "windows" {
		// the getter command is a shell script, so skip this test on windows
		t.Skip("skipping on windows")
	}

	testCases := map[string]struct {
		command string
	}{
		"relative to plugin dir": {
			command: "get.sh",
		},
		"HELM_PLUGIN_DIR env var": {
			command: "${HELM_PLUGIN_DIR}/get.sh",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			pluginDir := t.TempDir()
			require.NoError(t, os.WriteFile(filepath.Join(pluginDir, "get.sh"), []byte("#!/bin/sh\nprintf 'getter output'\n"), 0o755))

			rc := RuntimeConfigSubprocess{
				ProtocolCommands: []SubprocessProtocolCommand{
					{
						Protocols:       []string{"myprotocol"},
						PlatformCommand: []PlatformCommand{{Command: tc.command}},
					},
				},
			}
			p := &SubprocessPluginRuntime{
				metadata: Metadata{
					Name:          "getter",
					Version:       "0.1.2",
					Type:          "getter/v1",
					APIVersion:    "v1",
					Runtime:       "subprocess",
					Config:        &schema.ConfigGetterV1{Protocols: []string{"myprotocol"}},
					RuntimeConfig: &rc,
				},
				pluginDir:     pluginDir,
				RuntimeConfig: rc,
			}

			output, err := p.Invoke(t.Context(), &Input{
				Message: schema.InputMessageGetterV1{
					Href:     "myprotocol://example.com/chart-0.1.0.tgz",
					Protocol: "myprotocol",
				},
			})
			require.NoError(t, err)

			msg, ok := output.Message.(schema.OutputMessageGetterV1)
			require.True(t, ok)
			assert.Equal(t, "getter output", string(msg.Data))
		})
	}
}
