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
	"sort"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"helm.sh/helm/v3/pkg/release"
)

func TestManuallyProcessArgs(t *testing.T) {
	input := []string{
		"--debug",
		"--foo", "bar",
		"--kubeconfig=/home/foo",
		"--kubeconfig", "/home/foo",
		"--kube-context=test1",
		"--kube-context", "test1",
		"--kube-as-user", "pikachu",
		"--kube-as-group", "teatime",
		"--kube-as-group", "admins",
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
		"--kube-as-user", "pikachu",
		"--kube-as-group", "teatime",
		"--kube-as-group", "admins",
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

type staticCompletionDetails struct {
	use       string
	validArgs []string
	flags     []string
	next      []staticCompletionDetails
}

func TestLoadPluginsForCompletion(t *testing.T) {
	settings.PluginsDirectory = "testdata/helmhome/helm/plugins"

	var out bytes.Buffer

	cmd := &cobra.Command{
		Use: "completion",
	}

	loadPlugins(cmd, &out)

	tests := []staticCompletionDetails{
		{"args", []string{}, []string{}, []staticCompletionDetails{}},
		{"echo", []string{}, []string{}, []staticCompletionDetails{}},
		{"env", []string{}, []string{"global"}, []staticCompletionDetails{
			{"list", []string{}, []string{"a", "all", "log"}, []staticCompletionDetails{}},
			{"remove", []string{"all", "one"}, []string{}, []staticCompletionDetails{}},
		}},
		{"exitwith", []string{}, []string{}, []staticCompletionDetails{
			{"code", []string{}, []string{"a", "b"}, []staticCompletionDetails{}},
		}},
		{"fullenv", []string{}, []string{"q", "z"}, []staticCompletionDetails{
			{"empty", []string{}, []string{}, []staticCompletionDetails{}},
			{"full", []string{}, []string{}, []staticCompletionDetails{
				{"less", []string{}, []string{"a", "all"}, []staticCompletionDetails{}},
				{"more", []string{"one", "two"}, []string{"b", "ball"}, []staticCompletionDetails{}},
			}},
		}},
	}
	checkCommand(t, cmd.Commands(), tests)
}

func checkCommand(t *testing.T, plugins []*cobra.Command, tests []staticCompletionDetails) {
	if len(plugins) != len(tests) {
		t.Fatalf("Expected commands %v, got %v", tests, plugins)
	}

	for i := 0; i < len(plugins); i++ {
		pp := plugins[i]
		tt := tests[i]
		if pp.Use != tt.use {
			t.Errorf("%s: Expected Use=%q, got %q", pp.Name(), tt.use, pp.Use)
		}

		targs := tt.validArgs
		pargs := pp.ValidArgs
		if len(targs) != len(pargs) {
			t.Fatalf("%s: expected args %v, got %v", pp.Name(), targs, pargs)
		}

		sort.Strings(targs)
		sort.Strings(pargs)
		for j := range targs {
			if targs[j] != pargs[j] {
				t.Errorf("%s: expected validArg=%q, got %q", pp.Name(), targs[j], pargs[j])
			}
		}

		tflags := tt.flags
		var pflags []string
		pp.LocalFlags().VisitAll(func(flag *pflag.Flag) {
			pflags = append(pflags, flag.Name)
			if len(flag.Shorthand) > 0 && flag.Shorthand != flag.Name {
				pflags = append(pflags, flag.Shorthand)
			}
		})
		if len(tflags) != len(pflags) {
			t.Fatalf("%s: expected flags %v, got %v", pp.Name(), tflags, pflags)
		}

		sort.Strings(tflags)
		sort.Strings(pflags)
		for j := range tflags {
			if tflags[j] != pflags[j] {
				t.Errorf("%s: expected flag=%q, got %q", pp.Name(), tflags[j], pflags[j])
			}
		}
		// Check the next level
		checkCommand(t, pp.Commands(), tt.next)
	}
}

func TestPluginDynamicCompletion(t *testing.T) {

	tests := []cmdTestCase{{
		name:   "completion for plugin",
		cmd:    "__complete args ''",
		golden: "output/plugin_args_comp.txt",
		rels:   []*release.Release{},
	}, {
		name:   "completion for plugin with flag",
		cmd:    "__complete args --myflag ''",
		golden: "output/plugin_args_flag_comp.txt",
		rels:   []*release.Release{},
	}, {
		name:   "completion for plugin with global flag",
		cmd:    "__complete args --namespace mynamespace ''",
		golden: "output/plugin_args_ns_comp.txt",
		rels:   []*release.Release{},
	}, {
		name:   "completion for plugin with multiple args",
		cmd:    "__complete args --myflag --namespace mynamespace start",
		golden: "output/plugin_args_many_args_comp.txt",
		rels:   []*release.Release{},
	}, {
		name:   "completion for plugin no directive",
		cmd:    "__complete echo -n mynamespace ''",
		golden: "output/plugin_echo_no_directive.txt",
		rels:   []*release.Release{},
	}}
	for _, test := range tests {
		settings.PluginsDirectory = "testdata/helmhome/helm/plugins"
		runTestCmd(t, []cmdTestCase{test})
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

func TestPluginCmdsCompletion(t *testing.T) {

	tests := []cmdTestCase{{
		name:   "completion for plugin update",
		cmd:    "__complete plugin update ''",
		golden: "output/plugin_list_comp.txt",
		rels:   []*release.Release{},
	}, {
		name:   "completion for plugin update, no filter",
		cmd:    "__complete plugin update full",
		golden: "output/plugin_list_comp.txt",
		rels:   []*release.Release{},
	}, {
		name:   "completion for plugin update repetition",
		cmd:    "__complete plugin update args ''",
		golden: "output/plugin_repeat_comp.txt",
		rels:   []*release.Release{},
	}, {
		name:   "completion for plugin uninstall",
		cmd:    "__complete plugin uninstall ''",
		golden: "output/plugin_list_comp.txt",
		rels:   []*release.Release{},
	}, {
		name:   "completion for plugin uninstall, no filter",
		cmd:    "__complete plugin uninstall full",
		golden: "output/plugin_list_comp.txt",
		rels:   []*release.Release{},
	}, {
		name:   "completion for plugin uninstall repetition",
		cmd:    "__complete plugin uninstall args ''",
		golden: "output/plugin_repeat_comp.txt",
		rels:   []*release.Release{},
	}, {
		name:   "completion for plugin list",
		cmd:    "__complete plugin list ''",
		golden: "output/empty_nofile_comp.txt",
		rels:   []*release.Release{},
	}, {
		name:   "completion for plugin install no args",
		cmd:    "__complete plugin install ''",
		golden: "output/empty_default_comp.txt",
		rels:   []*release.Release{},
	}, {
		name:   "completion for plugin install one arg",
		cmd:    "__complete plugin list /tmp ''",
		golden: "output/empty_nofile_comp.txt",
		rels:   []*release.Release{},
	}, {}}
	for _, test := range tests {
		settings.PluginsDirectory = "testdata/helmhome/helm/plugins"
		runTestCmd(t, []cmdTestCase{test})
	}
}

func TestPluginFileCompletion(t *testing.T) {
	checkFileCompletion(t, "plugin", false)
}

func TestPluginInstallFileCompletion(t *testing.T) {
	checkFileCompletion(t, "plugin install", true)
	checkFileCompletion(t, "plugin install mypath", false)
}

func TestPluginListFileCompletion(t *testing.T) {
	checkFileCompletion(t, "plugin list", false)
}

func TestPluginUninstallFileCompletion(t *testing.T) {
	checkFileCompletion(t, "plugin uninstall", false)
	checkFileCompletion(t, "plugin uninstall myplugin", false)
}

func TestPluginUpdateFileCompletion(t *testing.T) {
	checkFileCompletion(t, "plugin update", false)
	checkFileCompletion(t, "plugin update myplugin", false)
}
