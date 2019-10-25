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
	"io/ioutil"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"

	shellwords "github.com/mattn/go-shellwords"
	"github.com/spf13/cobra"

	"helm.sh/helm/v3/internal/test"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chartutil"
	kubefake "helm.sh/helm/v3/pkg/kube/fake"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/storage"
	"helm.sh/helm/v3/pkg/storage/driver"
	"helm.sh/helm/v3/pkg/time"
)

func testTimestamper() time.Time { return time.Unix(242085845, 0).UTC() }

func init() {
	action.Timestamper = testTimestamper
}

func runTestCmd(t *testing.T, tests []cmdTestCase) {
	t.Helper()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer resetEnv()()

			storage := storageFixture()
			for _, rel := range tt.rels {
				if err := storage.Create(rel); err != nil {
					t.Fatal(err)
				}
			}
			t.Log("running cmd: ", tt.cmd)
			_, out, err := executeActionCommandC(storage, tt.cmd)
			if (err != nil) != tt.wantError {
				t.Errorf("expected error, got '%v'", err)
			}
			if tt.golden != "" {
				test.AssertGoldenString(t, out, tt.golden)
			}
		})
	}
}

func runTestActionCmd(t *testing.T, tests []cmdTestCase) {
	t.Helper()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer resetEnv()()

			store := storageFixture()
			for _, rel := range tt.rels {
				store.Create(rel)
			}
			_, out, err := executeActionCommandC(store, tt.cmd)
			if (err != nil) != tt.wantError {
				t.Errorf("expected error, got '%v'", err)
			}
			if tt.golden != "" {
				test.AssertGoldenString(t, out, tt.golden)
			}
		})
	}
}

func storageFixture() *storage.Storage {
	return storage.Init(driver.NewMemory())
}

func executeActionCommandC(store *storage.Storage, cmd string) (*cobra.Command, string, error) {
	args, err := shellwords.Parse(cmd)
	if err != nil {
		return nil, "", err
	}
	buf := new(bytes.Buffer)

	actionConfig := &action.Configuration{
		Releases:     store,
		KubeClient:   &kubefake.PrintingKubeClient{Out: ioutil.Discard},
		Capabilities: chartutil.DefaultCapabilities,
		Log:          func(format string, v ...interface{}) {},
	}

	root := newRootCmd(actionConfig, buf, args)
	root.SetOutput(buf)
	root.SetArgs(args)

	c, err := root.ExecuteC()

	return c, buf.String(), err
}

// cmdTestCase describes a test case that works with releases.
type cmdTestCase struct {
	name      string
	cmd       string
	golden    string
	wantError bool
	// Rels are the available releases at the start of the test.
	rels []*release.Release
}

func executeActionCommand(cmd string) (*cobra.Command, string, error) {
	return executeActionCommandC(storageFixture(), cmd)
}

func resetEnv() func() {
	origSettings, origEnv := settings, os.Environ()
	return func() {
		os.Clearenv()
		settings = origSettings
		for _, pair := range origEnv {
			kv := strings.SplitN(pair, "=", 2)
			os.Setenv(kv[0], kv[1])
		}
	}
}

func testChdir(t *testing.T, dir string) func() {
	t.Helper()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	return func() { os.Chdir(old) }
}

func TestPluginExitCode(t *testing.T) {
	if os.Getenv("RUN_MAIN_FOR_TESTING") == "1" {
		os.Args = []string{"helm", "exitwith", "2"}

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
		cmd := exec.Command(os.Args[0], "-test.run=TestPluginExitCode")
		cmd.Env = append(
			os.Environ(),
			"RUN_MAIN_FOR_TESTING=1",
			// See pkg/cli/environment.go for which envvars can be used for configuring these passes
			// and also see plugin_test.go for how a plugin env can be set up.
			// We just does the same setup as plugin_test.go via envvars
			"HELM_PLUGINS=testdata/helmhome/helm/plugins",
			"HELM_REPOSITORY_CONFIG=testdata/helmhome/helm/repositories.yaml",
			"HELM_REPOSITORY_CACHE=testdata/helmhome/helm/repository",
		)
		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}
		cmd.Stdout = stdout
		cmd.Stderr = stderr
		err := cmd.Run()
		exiterr, ok := err.(*exec.ExitError)

		if !ok {
			t.Fatalf("Unexpected error returned by os.Exit: %T", err)
		}

		if stdout.String() != "" {
			t.Errorf("Expected no write to stdout: Got %q", stdout.String())
		}

		expectedStderr := "Error: plugin \"exitwith\" exited with error\n"
		if stderr.String() != expectedStderr {
			t.Errorf("Expected %q written to stderr: Got %q", expectedStderr, stderr.String())
		}

		if exiterr.ExitCode() != 2 {
			t.Errorf("Expected exit code 2: Got %d", exiterr.ExitCode())
		}
	}
}
