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
	"io"
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
	"helm.sh/helm/v3/pkg/cli"
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
		for i := 0; i <= tt.repeat; i++ {
			t.Run(tt.name, func(t *testing.T) {
				defer resetEnv()()

				storage := storageFixture()
				for _, rel := range tt.rels {
					if err := storage.Create(rel); err != nil {
						t.Fatal(err)
					}
				}
				t.Logf("running cmd (attempt %d): %s", i+1, tt.cmd)
				_, out, err := executeActionCommandC(
					storage,
					tt.cmd,
					tt.restClientGetter,
					tt.kubeClientOpts,
				)
				if tt.wantError && err == nil {
					t.Errorf("expected error, got success with the following output:\n%s", out)
				}
				if !tt.wantError && err != nil {
					t.Errorf("expected no error, got: '%v'", err)
				}
				if tt.golden != "" {
					test.AssertGoldenString(t, out, tt.golden)
				}
			})
		}
	}
}

func storageFixture() *storage.Storage {
	return storage.Init(driver.NewMemory())
}

func executeActionCommandC(
	store *storage.Storage,
	cmd string,
	restClientGetter action.RESTClientGetter,
	kubeClientOpts *kubefake.Options,
) (*cobra.Command, string, error) {
	return executeActionCommandStdinC(store, nil, cmd, restClientGetter, kubeClientOpts)
}

func executeActionCommandStdinC(
	store *storage.Storage,
	in *os.File,
	cmd string,
	restClientGetter action.RESTClientGetter,
	kubeClientOpts *kubefake.Options,
) (*cobra.Command, string, error) {
	args, err := shellwords.Parse(cmd)
	if err != nil {
		return nil, "", err
	}

	buf := new(bytes.Buffer)

	actionConfig := &action.Configuration{
		Releases: store,
		KubeClient: &kubefake.PrintingKubeClient{
			Out:     io.Discard,
			Options: kubeClientOpts,
		},
		Capabilities:     chartutil.DefaultCapabilities,
		Log:              func(_ string, _ ...interface{}) {},
		RESTClientGetter: restClientGetter,
	}

	root, err := newRootCmd(actionConfig, buf, args)
	if err != nil {
		return nil, "", err
	}

	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs(args)

	oldStdin := os.Stdin
	if in != nil {
		root.SetIn(in)
		os.Stdin = in
	}

	if mem, ok := store.Driver.(*driver.Memory); ok {
		mem.SetNamespace(settings.Namespace())
	}
	c, err := root.ExecuteC()

	result := buf.String()

	os.Stdin = oldStdin

	return c, result, err
}

// cmdTestCase describes a test case that works with releases.
type cmdTestCase struct {
	name      string
	cmd       string
	golden    string
	wantError bool
	// Rels are the available releases at the start of the test.
	rels []*release.Release
	// Number of repeats (in case a feature was previously flaky and the test checks
	// it's now stably producing identical results). 0 means test is run exactly once.
	repeat int
	// REST client getter to be used with Helm action config
	restClientGetter action.RESTClientGetter
	// Kube client options for be used with fake printer kube client
	kubeClientOpts *kubefake.Options
}

func executeActionCommand(
	cmd string,
	restClientGetter action.RESTClientGetter,
	kubeClintOpts *kubefake.Options,
) (*cobra.Command, string, error) {
	return executeActionCommandC(storageFixture(), cmd, restClientGetter, kubeClintOpts)
}

func resetEnv() func() {
	origEnv := os.Environ()
	return func() {
		os.Clearenv()
		for _, pair := range origEnv {
			kv := strings.SplitN(pair, "=", 2)
			os.Setenv(kv[0], kv[1])
		}
		settings = cli.New()
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
