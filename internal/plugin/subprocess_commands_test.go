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
	"reflect"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPrepareCommand(t *testing.T) {
	cmdMain := "sh"
	cmdArgs := []string{"-c", "echo \"test\""}

	platformCommand := []PlatformCommand{
		{OperatingSystem: "no-os", Architecture: "no-arch", Command: "pwsh", Args: []string{"-c", "echo \"error\""}},
		{OperatingSystem: runtime.GOOS, Architecture: "no-arch", Command: "pwsh", Args: []string{"-c", "echo \"error\""}},
		{OperatingSystem: runtime.GOOS, Architecture: "", Command: "pwsh", Args: []string{"-c", "echo \"error\""}},
		{OperatingSystem: runtime.GOOS, Architecture: runtime.GOARCH, Command: cmdMain, Args: cmdArgs},
	}

	env := map[string]string{}
	cmd, args, err := PrepareCommands(platformCommand, true, []string{}, env)
	if err != nil {
		t.Fatal(err)
	}
	if cmd != cmdMain {
		t.Fatalf("Expected %q, got %q", cmdMain, cmd)
	}
	if !reflect.DeepEqual(args, cmdArgs) {
		t.Fatalf("Expected %v, got %v", cmdArgs, args)
	}
}

func TestPrepareCommandExtraArgs(t *testing.T) {

	cmdMain := "sh"
	cmdArgs := []string{"-c", "echo \"test\""}
	platformCommand := []PlatformCommand{
		{OperatingSystem: "no-os", Architecture: "no-arch", Command: "pwsh", Args: []string{"-c", "echo \"error\""}},
		{OperatingSystem: runtime.GOOS, Architecture: runtime.GOARCH, Command: cmdMain, Args: cmdArgs},
		{OperatingSystem: runtime.GOOS, Architecture: "no-arch", Command: "pwsh", Args: []string{"-c", "echo \"error\""}},
		{OperatingSystem: runtime.GOOS, Architecture: "", Command: "pwsh", Args: []string{"-c", "echo \"error\""}},
	}

	extraArgs := []string{"--debug", "--foo", "bar"}

	type testCaseExpected struct {
		cmdMain string
		args    []string
	}

	testCases := map[string]struct {
		ignoreFlags bool
		expected    testCaseExpected
	}{
		"ignoreFlags false": {
			ignoreFlags: false,
			expected: testCaseExpected{
				cmdMain: cmdMain,
				args:    []string{"-c", "echo \"test\"", "--debug", "--foo", "bar"},
			},
		},
		"ignoreFlags true": {
			ignoreFlags: true,
			expected: testCaseExpected{
				cmdMain: cmdMain,
				args:    []string{"-c", "echo \"test\""},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// extra args are expected when ignoreFlags is unset or false
			testExtraArgs := extraArgs
			if tc.ignoreFlags {
				testExtraArgs = []string{}
			}

			env := map[string]string{}
			cmd, args, err := PrepareCommands(platformCommand, true, testExtraArgs, env)
			if err != nil {
				t.Fatal(err)
			}
			assert.Equal(t, tc.expected.cmdMain, cmd, "Expected command to match")
			assert.Equal(t, tc.expected.args, args, "Expected args to match")
		})
	}
}

func TestPrepareCommands(t *testing.T) {
	cmdMain := "sh"
	cmdArgs := []string{"-c", "echo \"test\""}

	cmds := []PlatformCommand{
		{OperatingSystem: "no-os", Architecture: "no-arch", Command: "pwsh", Args: []string{"-c", "echo \"error\""}},
		{OperatingSystem: runtime.GOOS, Architecture: runtime.GOARCH, Command: cmdMain, Args: cmdArgs},
		{OperatingSystem: runtime.GOOS, Architecture: "no-arch", Command: "pwsh", Args: []string{"-c", "echo \"error\""}},
		{OperatingSystem: runtime.GOOS, Architecture: "", Command: "pwsh", Args: []string{"-c", "echo \"error\""}},
	}

	env := map[string]string{}
	cmd, args, err := PrepareCommands(cmds, true, []string{}, env)
	if err != nil {
		t.Fatal(err)
	}
	if cmd != cmdMain {
		t.Fatalf("Expected %q, got %q", cmdMain, cmd)
	}
	if !reflect.DeepEqual(args, cmdArgs) {
		t.Fatalf("Expected %v, got %v", cmdArgs, args)
	}
}

func TestPrepareCommandsExtraArgs(t *testing.T) {
	cmdMain := "sh"
	cmdArgs := []string{"-c", "echo \"test\""}
	extraArgs := []string{"--debug", "--foo", "bar"}

	cmds := []PlatformCommand{
		{OperatingSystem: "no-os", Architecture: "no-arch", Command: "pwsh", Args: []string{"-c", "echo \"error\""}},
		{OperatingSystem: runtime.GOOS, Architecture: runtime.GOARCH, Command: "sh", Args: []string{"-c", "echo \"test\""}},
		{OperatingSystem: runtime.GOOS, Architecture: "no-arch", Command: "pwsh", Args: []string{"-c", "echo \"error\""}},
		{OperatingSystem: runtime.GOOS, Architecture: "", Command: "pwsh", Args: []string{"-c", "echo \"error\""}},
	}

	expectedArgs := append(cmdArgs, extraArgs...)

	env := map[string]string{}
	cmd, args, err := PrepareCommands(cmds, true, extraArgs, env)
	if err != nil {
		t.Fatal(err)
	}
	if cmd != cmdMain {
		t.Fatalf("Expected %q, got %q", cmdMain, cmd)
	}
	if !reflect.DeepEqual(args, expectedArgs) {
		t.Fatalf("Expected %v, got %v", expectedArgs, args)
	}
}

func TestPrepareCommandsNoArch(t *testing.T) {
	cmdMain := "sh"
	cmdArgs := []string{"-c", "echo \"test\""}

	cmds := []PlatformCommand{
		{OperatingSystem: "no-os", Architecture: "no-arch", Command: "pwsh", Args: []string{"-c", "echo \"error\""}},
		{OperatingSystem: runtime.GOOS, Architecture: "", Command: "sh", Args: []string{"-c", "echo \"test\""}},
		{OperatingSystem: runtime.GOOS, Architecture: "no-arch", Command: "pwsh", Args: []string{"-c", "echo \"error\""}},
	}

	env := map[string]string{}
	cmd, args, err := PrepareCommands(cmds, true, []string{}, env)
	if err != nil {
		t.Fatal(err)
	}
	if cmd != cmdMain {
		t.Fatalf("Expected %q, got %q", cmdMain, cmd)
	}
	if !reflect.DeepEqual(args, cmdArgs) {
		t.Fatalf("Expected %v, got %v", cmdArgs, args)
	}
}

func TestPrepareCommandsNoOsNoArch(t *testing.T) {
	cmdMain := "sh"
	cmdArgs := []string{"-c", "echo \"test\""}

	cmds := []PlatformCommand{
		{OperatingSystem: "no-os", Architecture: "no-arch", Command: "pwsh", Args: []string{"-c", "echo \"error\""}},
		{OperatingSystem: "", Architecture: "", Command: "sh", Args: []string{"-c", "echo \"test\""}},
		{OperatingSystem: runtime.GOOS, Architecture: "no-arch", Command: "pwsh", Args: []string{"-c", "echo \"error\""}},
	}

	env := map[string]string{}
	cmd, args, err := PrepareCommands(cmds, true, []string{}, env)
	if err != nil {
		t.Fatal(err)
	}
	if cmd != cmdMain {
		t.Fatalf("Expected %q, got %q", cmdMain, cmd)
	}
	if !reflect.DeepEqual(args, cmdArgs) {
		t.Fatalf("Expected %v, got %v", cmdArgs, args)
	}
}

func TestPrepareCommandsNoMatch(t *testing.T) {
	cmds := []PlatformCommand{
		{OperatingSystem: "no-os", Architecture: "no-arch", Command: "sh", Args: []string{"-c", "echo \"test\""}},
		{OperatingSystem: runtime.GOOS, Architecture: "no-arch", Command: "sh", Args: []string{"-c", "echo \"test\""}},
		{OperatingSystem: "no-os", Architecture: runtime.GOARCH, Command: "sh", Args: []string{"-c", "echo \"test\""}},
	}

	env := map[string]string{}
	if _, _, err := PrepareCommands(cmds, true, []string{}, env); err == nil {
		t.Fatalf("Expected error to be returned")
	}
}

func TestPrepareCommandsNoCommands(t *testing.T) {
	cmds := []PlatformCommand{}

	env := map[string]string{}
	if _, _, err := PrepareCommands(cmds, true, []string{}, env); err == nil {
		t.Fatalf("Expected error to be returned")
	}
}

func TestPrepareCommandsExpand(t *testing.T) {
	cmdMain := "sh"
	cmdArgs := []string{"-c", "echo \"${TESTX}${TESTY}\""}
	cmds := []PlatformCommand{
		{OperatingSystem: "", Architecture: "", Command: cmdMain, Args: cmdArgs},
	}

	expectedArgs := []string{"-c", "echo \"testxtesty\""}

	env := map[string]string{
		"TESTX": "testx",
		"TESTY": "testy",
	}

	cmd, args, err := PrepareCommands(cmds, true, []string{}, env)
	if err != nil {
		t.Fatal(err)
	}
	if cmd != cmdMain {
		t.Fatalf("Expected %q, got %q", cmdMain, cmd)
	}
	if !reflect.DeepEqual(args, expectedArgs) {
		t.Fatalf("Expected %v, got %v", expectedArgs, args)
	}
}

func TestPrepareCommandsNoExpand(t *testing.T) {
	cmdMain := "sh"
	cmdArgs := []string{"-c", "echo \"${TEST}\""}
	cmds := []PlatformCommand{
		{OperatingSystem: "", Architecture: "", Command: cmdMain, Args: cmdArgs},
	}

	env := map[string]string{
		"TEST": "test",
	}

	cmd, args, err := PrepareCommands(cmds, false, []string{}, env)
	if err != nil {
		t.Fatal(err)
	}
	if cmd != cmdMain {
		t.Fatalf("Expected %q, got %q", cmdMain, cmd)
	}
	if !reflect.DeepEqual(args, cmdArgs) {
		t.Fatalf("Expected %v, got %v", cmdArgs, args)
	}
}
