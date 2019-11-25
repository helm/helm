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
	"os"
	"runtime"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestManuallyProcessArgs(t *testing.T) {
	input := []string{
		"--debug",
		"--foo", "bar",
		"--kubeconfig=/home/foo",
		"--kubeconfig", "/home/foo",
		"--kube-context=test1",
		"--kube-context", "test1",
		"-n=test2",
		"-n", "test2",
		"--namespace=test2",
		"--namespace", "test2",
		"--home=/tmp",
		"command",
	}

	expectKnown := []string{
		"--debug",
		"--kubeconfig=/home/foo",
		"--kubeconfig", "/home/foo",
		"--kube-context=test1",
		"--kube-context", "test1",
		"-n=test2",
		"-n", "test2",
		"--namespace=test2",
		"--namespace", "test2",
	}

	expectUnknown := []string{
		"--foo", "bar", "--home=/tmp", "command",
	}

	known, unknown := manuallyProcessArgs(input)

	for i, k := range known {
		if k != expectKnown[i] {
			t.Errorf("expected known flag %d to be %q, got %q", i, expectKnown[i], k)
		}
	}
	for i, k := range unknown {
		if k != expectUnknown[i] {
			t.Errorf("expected unknown flag %d to be %q, got %q", i, expectUnknown[i], k)
		}
	}

}

func TestLoadPlugins(t *testing.T) {
	settings.PluginsDirectory = "testdata/helmhome/helm/plugins"
	settings.RepositoryConfig = "testdata/helmhome/helm/repositories.yaml"
	settings.RepositoryCache = "testdata/helmhome/helm/repository"

	var (
		out bytes.Buffer
		cmd cobra.Command
	)
	loadPlugins(&cmd, &out)

	envs := strings.Join([]string{
		"fullenv",
		"testdata/helmhome/helm/plugins/fullenv",
		"testdata/helmhome/helm/plugins",
		"testdata/helmhome/helm/repositories.yaml",
		"testdata/helmhome/helm/repository",
		os.Args[0],
	}, "\n")

	// Test that the YAML file was correctly converted to a command.
	tests := []struct {
		use    string
		short  string
		long   string
		expect string
		args   []string
		code   int
	}{
		{"args", "echo args", "This echos args", "-a -b -c\n", []string{"-a", "-b", "-c"}, 0},
		{"echo", "echo stuff", "This echos stuff", "hello\n", []string{}, 0},
		{"env", "env stuff", "show the env", "env\n", []string{}, 0},
		{"exitwith", "exitwith code", "This exits with the specified exit code", "", []string{"2"}, 2},
		{"fullenv", "show env vars", "show all env vars", envs + "\n", []string{}, 0},
	}

	plugins := cmd.Commands()

	if len(plugins) != len(tests) {
		t.Fatalf("Expected %d plugins, got %d", len(tests), len(plugins))
	}

	for i := 0; i < len(plugins); i++ {
		out.Reset()
		tt := tests[i]
		pp := plugins[i]
		if pp.Use != tt.use {
			t.Errorf("%d: Expected Use=%q, got %q", i, tt.use, pp.Use)
		}
		if pp.Short != tt.short {
			t.Errorf("%d: Expected Use=%q, got %q", i, tt.short, pp.Short)
		}
		if pp.Long != tt.long {
			t.Errorf("%d: Expected Use=%q, got %q", i, tt.long, pp.Long)
		}

		// Currently, plugins assume a Linux subsystem. Skip the execution
		// tests until this is fixed
		if runtime.GOOS != "windows" {
			if err := pp.RunE(pp, tt.args); err != nil {
				if tt.code > 0 {
					perr, ok := err.(pluginError)
					if !ok {
						t.Errorf("Expected %s to return pluginError: got %v(%T)", tt.use, err, err)
					}
					if perr.code != tt.code {
						t.Errorf("Expected %s to return %d: got %d", tt.use, tt.code, perr.code)
					}
				} else {
					t.Errorf("Error running %s: %+v", tt.use, err)
				}
			}
			if out.String() != tt.expect {
				t.Errorf("Expected %s to output:\n%s\ngot\n%s", tt.use, tt.expect, out.String())
			}
		}
	}
}

func TestLoadPlugins_HelmNoPlugins(t *testing.T) {
	settings.PluginsDirectory = "testdata/helmhome/helm/plugins"
	settings.RepositoryConfig = "testdata/helmhome/helm/repository"

	os.Setenv("HELM_NO_PLUGINS", "1")

	out := bytes.NewBuffer(nil)
	cmd := &cobra.Command{}
	loadPlugins(cmd, out)
	plugins := cmd.Commands()

	if len(plugins) != 0 {
		t.Fatalf("Expected 0 plugins, got %d", len(plugins))
	}
}
