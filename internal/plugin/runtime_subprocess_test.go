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
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
	ok := errors.As(err, &ieerr)
	require.True(t, ok, "expected InvokeExecError, got %T", err)
	assert.Equal(t, 56, ieerr.ExitCode)

	assert.Nil(t, output)
}

func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	fmt.Fprint(os.Stderr, "plugin error\n")
	os.Exit(1)
}

func helperStderrCmd(t *testing.T) *exec.Cmd {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=TestHelperProcess", "--")
	cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")
	return cmd
}

func TestExecuteCmdCapturesStderr(t *testing.T) {
	cmd := helperStderrCmd(t)
	err := executeCmd(cmd, "test-plugin")

	require.Error(t, err)
	var ieerr *InvokeExecError
	require.ErrorAs(t, err, &ieerr)
	assert.Equal(t, 1, ieerr.ExitCode)
	assert.Equal(t, []byte("plugin error"), ieerr.Stderr)
	assert.Equal(t, `plugin "test-plugin" exited with error: "plugin error"`, ieerr.Error())
}

func TestExecuteCmdTeesStderr(t *testing.T) {
	var existing bytes.Buffer
	cmd := helperStderrCmd(t)
	cmd.Stderr = &existing

	err := executeCmd(cmd, "test-plugin")

	require.Error(t, err)
	var ieerr *InvokeExecError
	require.ErrorAs(t, err, &ieerr)
	assert.Equal(t, 1, ieerr.ExitCode)
	assert.Equal(t, []byte("plugin error"), ieerr.Stderr)
	assert.Equal(t, "plugin error\n", existing.String())
}
