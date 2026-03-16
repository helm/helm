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

package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCliPluginExitCode(t *testing.T) {
	if os.Getenv("RUN_MAIN_FOR_TESTING") == "1" {
		os.Args = []string{"helm", "exitwith", "43"}

		// We DO call helm's main() here. So this looks like a normal `helm` process.
		main()

		// As main calls os.Exit, we never reach this line.
		// But the test called this block of code catches and verifies the exit code.
		return
	}

	// Currently, plugins assume a Linux subsystem. Skip the execution
	// tests until this is fixed
	if runtime.GOOS != "windows" {
		// Do a second run of this specific test(TestPluginExitCode) with RUN_MAIN_FOR_TESTING=1 set,
		// So that the second run is able to run main() and this first run can verify the exit status returned by that.
		//
		// This technique originates from https://talks.golang.org/2014/testing.slide#23.
		cmd := exec.Command(os.Args[0], "-test.run=TestCliPluginExitCode")
		cmd.Env = append(
			os.Environ(),
			"RUN_MAIN_FOR_TESTING=1",
			// See pkg/cli/environment.go for which envvars can be used for configuring these passes
			// and also see plugin_test.go for how a plugin env can be set up.
			// This mimics the "exitwith" test case in TestLoadPlugins using envvars
			"HELM_PLUGINS=../../pkg/cmd/testdata/helmhome/helm/plugins",
		)
		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}
		cmd.Stdout = stdout
		cmd.Stderr = stderr
		err := cmd.Run()

		exiterr := &exec.ExitError{}
		ok := errors.As(err, &exiterr)
		if !ok {
			t.Fatalf("Unexpected error type returned by os.Exit: %T", err)
		}

		assert.Empty(t, stdout.String())

		expectedStderr := "Error: plugin \"exitwith\" exited with error\n"
		if stderr.String() != expectedStderr {
			t.Errorf("Expected %q written to stderr: Got %q", expectedStderr, stderr.String())
		}

		if exiterr.ExitCode() != 43 {
			t.Errorf("Expected exit code 43: Got %d", exiterr.ExitCode())
		}
	}
}
