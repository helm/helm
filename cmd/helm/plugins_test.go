/*
Copyright 2016 The Kubernetes Authors All rights reserved.
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
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"k8s.io/helm/cmd/helm/helmpath"

	"github.com/spf13/cobra"
)

func TestManuallyProcessArgs(t *testing.T) {
	input := []string{
		"--debug",
		"--foo", "bar",
		"--host", "example.com",
		"--kube-context", "test1",
		"--home=/tmp",
		"--tiller-namespace=hello",
		"command",
	}

	expectKnown := []string{
		"--debug", "--host", "example.com", "--kube-context", "test1", "--home=/tmp", "--tiller-namespace=hello",
	}

	expectUnknown := []string{
		"--foo", "bar", "command",
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
	// Set helm home to point to testdata
	old := helmHome
	helmHome = "testdata/helmhome"
	defer func() {
		helmHome = old
	}()
	hh := helmpath.Home(homePath())

	out := bytes.NewBuffer(nil)
	cmd := &cobra.Command{}
	loadPlugins(cmd, hh, out)

	envs := strings.Join([]string{
		"fullenv",
		hh.Plugins() + "/fullenv",
		hh.Plugins(),
		hh.String(),
		hh.Repository(),
		hh.RepositoryFile(),
		hh.Cache(),
		hh.LocalRepository(),
		os.Args[0],
	}, "\n")

	// Test that the YAML file was correctly converted to a command.
	tests := []struct {
		use    string
		short  string
		long   string
		expect string
		args   []string
	}{
		{"args", "echo args", "This echos args", "-a -b -c\n", []string{"-a", "-b", "-c"}},
		{"echo", "echo stuff", "This echos stuff", "hello\n", []string{}},
		{"env", "env stuff", "show the env", hh.String() + "\n", []string{}},
		{"fullenv", "show env vars", "show all env vars", envs + "\n", []string{}},
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
				t.Errorf("Error running %s: %s", tt.use, err)
			}
			if out.String() != tt.expect {
				t.Errorf("Expected %s to output:\n%s\ngot\n%s", tt.use, tt.expect, out.String())
			}
		}
	}
}

func TestLoadPlugins_HelmNoPlugins(t *testing.T) {
	os.Setenv("HELM_NO_PLUGINS", "1")
	defer os.Setenv("HELM_NO_PLUGINS", "0")

	// Set helm home to point to testdata
	old := helmHome
	helmHome = "testdata/helmhome"
	defer func() {
		helmHome = old
	}()
	hh := helmpath.Home(homePath())

	out := bytes.NewBuffer(nil)
	cmd := &cobra.Command{}
	loadPlugins(cmd, hh, out)
	plugins := cmd.Commands()

	if len(plugins) != 0 {
		t.Fatalf("Expected 0 plugins, got %d", len(plugins))
	}
}

func TestSetupEnv(t *testing.T) {
	name := "pequod"
	hh := helmpath.Home("testdata/helmhome")
	base := filepath.Join(hh.Plugins(), name)
	plugdirs := hh.Plugins()
	flagDebug = true
	defer func() {
		flagDebug = false
	}()

	setupEnv(name, base, plugdirs, hh)
	for _, tt := range []struct {
		name   string
		expect string
	}{
		{"HELM_PLUGIN_NAME", name},
		{"HELM_PLUGIN_DIR", base},
		{"HELM_PLUGIN", hh.Plugins()},
		{"HELM_DEBUG", "1"},
		{"HELM_HOME", hh.String()},
		{"HELM_PATH_REPOSITORY", hh.Repository()},
		{"HELM_PATH_REPOSITORY_FILE", hh.RepositoryFile()},
		{"HELM_PATH_CACHE", hh.Cache()},
		{"HELM_PATH_LOCAL_REPOSITORY", hh.LocalRepository()},
		{"HELM_PATH_STARTER", hh.Starters()},
		{"TILLER_HOST", tillerHost},
		{"TILLER_NAMESPACE", tillerNamespace},
	} {
		if got := os.Getenv(tt.name); got != tt.expect {
			t.Errorf("Expected $%s=%q, got %q", tt.name, tt.expect, got)
		}
	}
}
