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

package plugin // import "helm.sh/helm/v3/pkg/plugin"

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"helm.sh/helm/v3/pkg/cli"
)

func TestPrepareCommand(t *testing.T) {
	p := &Plugin{
		Dir: "/tmp", // Unused
		Metadata: &Metadata{
			Name:    "test",
			Command: "echo \"error\"",
			PlatformCommand: []PlatformCommand{
				{OperatingSystem: "no-os", Architecture: "no-arch", Command: "pwsh", Args: []string{"-c", "echo \"error\""}},
				{OperatingSystem: runtime.GOOS, Architecture: runtime.GOARCH, Command: "sh", Args: []string{"-c", "echo \"test\""}},
				{OperatingSystem: runtime.GOOS, Architecture: "no-arch", Command: "pwsh", Args: []string{"-c", "echo \"error\""}},
				{OperatingSystem: runtime.GOOS, Architecture: "", Command: "pwsh", Args: []string{"-c", "echo \"error\""}},
			},
		},
	}

	expectedIndex := 1

	cmd, args, err := p.PrepareCommand([]string{})
	if err != nil {
		t.Fatal(err)
	}
	if cmd != p.Metadata.PlatformCommand[expectedIndex].Command {
		t.Fatalf("Expected %q, got %q", p.Metadata.PlatformCommand[expectedIndex].Command, cmd)
	}
	if !reflect.DeepEqual(args, p.Metadata.PlatformCommand[expectedIndex].Args) {
		t.Fatalf("Expected %v, got %v", p.Metadata.PlatformCommand[expectedIndex].Args, args)
	}
}

func TestPrepareCommandExtraArgs(t *testing.T) {
	p := &Plugin{
		Dir: "/tmp", // Unused
		Metadata: &Metadata{
			Name:    "test",
			Command: "echo \"error\"",
			PlatformCommand: []PlatformCommand{
				{OperatingSystem: "no-os", Architecture: "no-arch", Command: "pwsh", Args: []string{"-c", "echo \"error\""}},
				{OperatingSystem: runtime.GOOS, Architecture: runtime.GOARCH, Command: "sh", Args: []string{"-c", "echo \"test\""}},
				{OperatingSystem: runtime.GOOS, Architecture: "no-arch", Command: "pwsh", Args: []string{"-c", "echo \"error\""}},
				{OperatingSystem: runtime.GOOS, Architecture: "", Command: "pwsh", Args: []string{"-c", "echo \"error\""}},
			},
		},
	}
	extraArgs := []string{"--debug", "--foo", "bar"}

	expectedIndex := 1
	resultArgs := append(p.Metadata.PlatformCommand[expectedIndex].Args, extraArgs...)

	cmd, args, err := p.PrepareCommand(extraArgs)
	if err != nil {
		t.Fatal(err)
	}
	if cmd != p.Metadata.PlatformCommand[expectedIndex].Command {
		t.Fatalf("Expected %q, got %q", p.Metadata.PlatformCommand[expectedIndex].Command, cmd)
	}
	if !reflect.DeepEqual(args, resultArgs) {
		t.Fatalf("Expected %v, got %v", resultArgs, args)
	}
}

func TestPrepareCommandExtraArgsIgnored(t *testing.T) {
	p := &Plugin{
		Dir: "/tmp", // Unused
		Metadata: &Metadata{
			Name:    "test",
			Command: "echo \"error\"",
			PlatformCommand: []PlatformCommand{
				{OperatingSystem: "no-os", Architecture: "no-arch", Command: "pwsh", Args: []string{"-c", "echo \"error\""}},
				{OperatingSystem: runtime.GOOS, Architecture: runtime.GOARCH, Command: "sh", Args: []string{"-c", "echo \"test\""}},
				{OperatingSystem: runtime.GOOS, Architecture: "no-arch", Command: "pwsh", Args: []string{"-c", "echo \"error\""}},
				{OperatingSystem: runtime.GOOS, Architecture: "", Command: "pwsh", Args: []string{"-c", "echo \"error\""}},
			},
			IgnoreFlags: true,
		},
	}
	extraArgs := []string{"--debug", "--foo", "bar"}

	expectedIndex := 1

	cmd, args, err := p.PrepareCommand(extraArgs)
	if err != nil {
		t.Fatal(err)
	}
	if cmd != p.Metadata.PlatformCommand[expectedIndex].Command {
		t.Fatalf("Expected %q, got %q", p.Metadata.PlatformCommand[expectedIndex].Command, cmd)
	}
	if !reflect.DeepEqual(args, p.Metadata.PlatformCommand[expectedIndex].Args) {
		t.Fatalf("Expected %v, got %v", p.Metadata.PlatformCommand[expectedIndex].Args, args)
	}
}

func TestPrepareCommandNoArgs(t *testing.T) {
	command := "echo"
	commandArgs := []string{"-n", "\"test\""}

	p := &Plugin{
		Dir: "/tmp", // Unused
		Metadata: &Metadata{
			Name:    "test",
			Command: "echo \"error\"",
			PlatformCommand: []PlatformCommand{
				{OperatingSystem: "no-os", Architecture: "no-arch", Command: "echo \"error\""},
				{OperatingSystem: runtime.GOOS, Architecture: runtime.GOARCH, Command: strings.Join(append([]string{command}, commandArgs...), " ")},
				{OperatingSystem: runtime.GOOS, Architecture: "no-arch", Command: "echo \"error\""},
				{OperatingSystem: runtime.GOOS, Architecture: "", Command: "echo \"error\""},
			},
		},
	}

	cmd, args, err := p.PrepareCommand([]string{})
	if err != nil {
		t.Fatal(err)
	}
	if cmd != command {
		t.Fatalf("Expected %q, got %q", command, cmd)
	}
	if !reflect.DeepEqual(args, commandArgs) {
		t.Fatalf("Expected %v, got %v", commandArgs, args)
	}
}

func TestPrepareCommandFallback(t *testing.T) {
	command := "echo"
	commandArgs := []string{"-n", "foo"}

	p := &Plugin{
		Dir: "/tmp", // Unused
		Metadata: &Metadata{
			Name:    "test",
			Command: strings.Join(append([]string{command}, commandArgs...), " "),
		},
	}

	cmd, args, err := p.PrepareCommand([]string{})
	if err != nil {
		t.Fatal(err)
	}
	if cmd != command {
		t.Fatalf("Expected %q, got %q", command, cmd)
	}
	if !reflect.DeepEqual(commandArgs, args) {
		t.Fatalf("Expected %v, got %v", args, args)
	}
}

func TestPrepareCommandFallbackExtraArgs(t *testing.T) {
	command := "echo"
	commandArgs := []string{"-n", "foo"}

	p := &Plugin{
		Dir: "/tmp", // Unused
		Metadata: &Metadata{
			Name:    "test",
			Command: strings.Join(append([]string{command}, commandArgs...), " "),
		},
	}
	extraArgs := []string{"--debug", "--foo", "bar"}

	resultArgs := append(commandArgs, extraArgs...)

	cmd, args, err := p.PrepareCommand(extraArgs)
	if err != nil {
		t.Fatal(err)
	}
	if cmd != command {
		t.Fatalf("Expected %q, got %q", command, cmd)
	}
	if !reflect.DeepEqual(args, resultArgs) {
		t.Fatalf("Expected %v, got %v", resultArgs, args)
	}
}

func TestPrepareCommandFallbackExtraArgsIgnored(t *testing.T) {
	command := "echo"
	commandArgs := []string{"-n", "foo"}

	p := &Plugin{
		Dir: "/tmp", // Unused
		Metadata: &Metadata{
			Name:        "test",
			Command:     strings.Join(append([]string{command}, commandArgs...), " "),
			IgnoreFlags: true,
		},
	}
	extraArgs := []string{"--debug", "--foo", "bar"}

	cmd, args, err := p.PrepareCommand(extraArgs)
	if err != nil {
		t.Fatal(err)
	}
	if cmd != command {
		t.Fatalf("Expected %q, got %q", command, cmd)
	}
	if !reflect.DeepEqual(args, commandArgs) {
		t.Fatalf("Expected %v, got %v", commandArgs, args)
	}
}

func TestPrepareCommandNoMatch(t *testing.T) {
	p := &Plugin{
		Dir: "/tmp", // Unused
		Metadata: &Metadata{
			Name: "test",
			PlatformCommand: []PlatformCommand{
				{OperatingSystem: "no-os", Architecture: "no-arch", Command: "sh", Args: []string{"-c", "echo \"test\""}},
				{OperatingSystem: runtime.GOOS, Architecture: "no-arch", Command: "sh", Args: []string{"-c", "echo \"test\""}},
				{OperatingSystem: "no-os", Architecture: runtime.GOARCH, Command: "sh", Args: []string{"-c", "echo \"test\""}},
			},
		},
	}

	_, _, err := p.PrepareCommand([]string{})
	if err == nil {
		t.Fatalf("Expected error to be returned")
	}
}

func TestPrepareCommandNoCommand(t *testing.T) {
	p := &Plugin{
		Dir: "/tmp", // Unused
		Metadata: &Metadata{
			Name: "test",
		},
	}

	_, _, err := p.PrepareCommand([]string{})
	if err == nil {
		t.Fatalf("Expected error to be returned")
	}
}

func TestPrepareCommands(t *testing.T) {
	cmds := []PlatformCommand{
		{OperatingSystem: "no-os", Architecture: "no-arch", Command: "pwsh", Args: []string{"-c", "echo \"error\""}},
		{OperatingSystem: runtime.GOOS, Architecture: runtime.GOARCH, Command: "sh", Args: []string{"-c", "echo \"test\""}},
		{OperatingSystem: runtime.GOOS, Architecture: "no-arch", Command: "pwsh", Args: []string{"-c", "echo \"error\""}},
		{OperatingSystem: runtime.GOOS, Architecture: "", Command: "pwsh", Args: []string{"-c", "echo \"error\""}},
	}

	expectedIndex := 1

	cmd, args, err := PrepareCommands(cmds, "pwsh", []string{"-c", "echo \"error\""}, true, []string{})
	if err != nil {
		t.Fatal(err)
	}
	if cmd != cmds[expectedIndex].Command {
		t.Fatalf("Expected %q, got %q", cmds[expectedIndex].Command, cmd)
	}
	if !reflect.DeepEqual(args, cmds[expectedIndex].Args) {
		t.Fatalf("Expected %v, got %v", cmds[expectedIndex].Args, args)
	}
}

func TestPrepareCommandsExtraArgs(t *testing.T) {
	cmds := []PlatformCommand{
		{OperatingSystem: "no-os", Architecture: "no-arch", Command: "pwsh", Args: []string{"-c", "echo \"error\""}},
		{OperatingSystem: runtime.GOOS, Architecture: runtime.GOARCH, Command: "sh", Args: []string{"-c", "echo \"test\""}},
		{OperatingSystem: runtime.GOOS, Architecture: "no-arch", Command: "pwsh", Args: []string{"-c", "echo \"error\""}},
		{OperatingSystem: runtime.GOOS, Architecture: "", Command: "pwsh", Args: []string{"-c", "echo \"error\""}},
	}
	extraArgs := []string{"--debug", "--foo", "bar"}

	expectedIndex := 1
	resultArgs := append(cmds[expectedIndex].Args, extraArgs...)

	cmd, args, err := PrepareCommands(cmds, "pwsh", []string{"-c", "echo \"error\""}, true, extraArgs)
	if err != nil {
		t.Fatal(err)
	}
	if cmd != cmds[expectedIndex].Command {
		t.Fatalf("Expected %q, got %q", cmds[expectedIndex].Command, cmd)
	}
	if !reflect.DeepEqual(args, resultArgs) {
		t.Fatalf("Expected %v, got %v", resultArgs, args)
	}
}

func TestPrepareCommandsNoArch(t *testing.T) {
	cmds := []PlatformCommand{
		{OperatingSystem: "no-os", Architecture: "no-arch", Command: "pwsh", Args: []string{"-c", "echo \"error\""}},
		{OperatingSystem: runtime.GOOS, Architecture: "", Command: "sh", Args: []string{"-c", "echo \"test\""}},
		{OperatingSystem: runtime.GOOS, Architecture: "no-arch", Command: "pwsh", Args: []string{"-c", "echo \"error\""}},
	}

	expectedIndex := 1

	cmd, args, err := PrepareCommands(cmds, "pwsh", []string{"-c", "echo \"error\""}, true, []string{})
	if err != nil {
		t.Fatal(err)
	}
	if cmd != cmds[expectedIndex].Command {
		t.Fatalf("Expected %q, got %q", cmds[expectedIndex].Command, cmd)
	}
	if !reflect.DeepEqual(args, cmds[expectedIndex].Args) {
		t.Fatalf("Expected %v, got %v", cmds[expectedIndex].Args, args)
	}
}

func TestPrepareCommandsFallback(t *testing.T) {
	cmds := []PlatformCommand{
		{OperatingSystem: "no-os", Architecture: "no-arch", Command: "sh", Args: []string{"-c", "echo \"test\""}},
		{OperatingSystem: runtime.GOOS, Architecture: "no-arch", Command: "sh", Args: []string{"-c", "echo \"test\""}},
		{OperatingSystem: "no-os", Architecture: runtime.GOARCH, Command: "sh", Args: []string{"-c", "echo \"test\""}},
	}

	command := "sh"
	commandArgs := []string{"-c", "echo \"test\""}

	cmd, args, err := PrepareCommands(cmds, command, commandArgs, true, []string{})
	if err != nil {
		t.Fatal(err)
	}
	if cmd != command {
		t.Fatalf("Expected %q, got %q", command, cmd)
	}
	if !reflect.DeepEqual(args, commandArgs) {
		t.Fatalf("Expected %v, got %v", commandArgs, args)
	}
}

func TestPrepareCommandsNoMatch(t *testing.T) {
	cmds := []PlatformCommand{
		{OperatingSystem: "no-os", Architecture: "no-arch", Command: "sh", Args: []string{"-c", "echo \"test\""}},
		{OperatingSystem: runtime.GOOS, Architecture: "no-arch", Command: "sh", Args: []string{"-c", "echo \"test\""}},
		{OperatingSystem: "no-os", Architecture: runtime.GOARCH, Command: "sh", Args: []string{"-c", "echo \"test\""}},
	}

	if _, _, err := PrepareCommands(cmds, "", []string{}, true, []string{}); err == nil {
		t.Fatalf("Expected error to be returned")
	}
}

func TestPrepareCommandsNoCommands(t *testing.T) {
	cmds := []PlatformCommand{}

	if _, _, err := PrepareCommands(cmds, "", []string{}, true, []string{}); err == nil {
		t.Fatalf("Expected error to be returned")
	}
}

func TestPrepareCommandsExpand(t *testing.T) {
	cmds := []PlatformCommand{}

	command := "echo"
	commandArgs := []string{"${TEST}"}
	commandArgsExpanded := []string{""}

	cmd, args, err := PrepareCommands(cmds, command, commandArgs, true, []string{})
	if err != nil {
		t.Fatal(err)
	}
	if cmd != command {
		t.Fatalf("Expected %q, got %q", command, cmd)
	}
	if !reflect.DeepEqual(args, commandArgsExpanded) {
		t.Fatalf("Expected %v, got %v", commandArgsExpanded, args)
	}
}

func TestPrepareCommandsNoExpand(t *testing.T) {
	cmds := []PlatformCommand{}

	command := "echo"
	commandArgs := []string{"${TEST}"}

	cmd, args, err := PrepareCommands(cmds, command, commandArgs, false, []string{})
	if err != nil {
		t.Fatal(err)
	}
	if cmd != command {
		t.Fatalf("Expected %q, got %q", command, cmd)
	}
	if !reflect.DeepEqual(args, commandArgs) {
		t.Fatalf("Expected %v, got %v", commandArgs, args)
	}
}

func TestLoadDir(t *testing.T) {
	dirname := "testdata/plugdir/good/hello"
	plug, err := LoadDir(dirname)
	if err != nil {
		t.Fatalf("error loading Hello plugin: %s", err)
	}

	if plug.Dir != dirname {
		t.Fatalf("Expected dir %q, got %q", dirname, plug.Dir)
	}

	expect := &Metadata{
		Name:        "hello",
		Version:     "0.1.0",
		Usage:       "usage",
		Description: "description",
		PlatformCommand: []PlatformCommand{
			{OperatingSystem: "linux", Architecture: "", Command: "sh", Args: []string{"-c", "${HELM_PLUGIN_DIR}/hello.sh"}},
			{OperatingSystem: "windows", Architecture: "", Command: "pwsh", Args: []string{"-c", "${HELM_PLUGIN_DIR}/hello.ps1"}},
		},
		Command:     "${HELM_PLUGIN_DIR}/hello.sh",
		IgnoreFlags: true,
		PlatformHooks: map[string][]PlatformCommand{
			Install: {
				{OperatingSystem: "linux", Architecture: "", Command: "sh", Args: []string{"-c", "echo \"installing...\""}},
				{OperatingSystem: "windows", Architecture: "", Command: "pwsh", Args: []string{"-c", "echo \"installing...\""}},
			},
		},
		Hooks: map[string]string{
			Install: "echo installing...",
		},
	}

	if !reflect.DeepEqual(expect, plug.Metadata) {
		t.Fatalf("Expected plugin metadata %v, got %v", expect, plug.Metadata)
	}
}

func TestLoadDirDuplicateEntries(t *testing.T) {
	dirname := "testdata/plugdir/bad/duplicate-entries"
	if _, err := LoadDir(dirname); err == nil {
		t.Errorf("successfully loaded plugin with duplicate entries when it should've failed")
	}
}

func TestDownloader(t *testing.T) {
	dirname := "testdata/plugdir/good/downloader"
	plug, err := LoadDir(dirname)
	if err != nil {
		t.Fatalf("error loading Hello plugin: %s", err)
	}

	if plug.Dir != dirname {
		t.Fatalf("Expected dir %q, got %q", dirname, plug.Dir)
	}

	expect := &Metadata{
		Name:        "downloader",
		Version:     "1.2.3",
		Usage:       "usage",
		Description: "download something",
		Command:     "echo Hello",
		Downloaders: []Downloaders{
			{
				Protocols: []string{"myprotocol", "myprotocols"},
				Command:   "echo Download",
			},
		},
	}

	if !reflect.DeepEqual(expect, plug.Metadata) {
		t.Fatalf("Expected metadata %v, got %v", expect, plug.Metadata)
	}
}

func TestLoadAll(t *testing.T) {

	// Verify that empty dir loads:
	if plugs, err := LoadAll("testdata"); err != nil {
		t.Fatalf("error loading dir with no plugins: %s", err)
	} else if len(plugs) > 0 {
		t.Fatalf("expected empty dir to have 0 plugins")
	}

	basedir := "testdata/plugdir/good"
	plugs, err := LoadAll(basedir)
	if err != nil {
		t.Fatalf("Could not load %q: %s", basedir, err)
	}

	if l := len(plugs); l != 3 {
		t.Fatalf("expected 3 plugins, found %d", l)
	}

	if plugs[0].Metadata.Name != "downloader" {
		t.Errorf("Expected first plugin to be echo, got %q", plugs[0].Metadata.Name)
	}
	if plugs[1].Metadata.Name != "echo" {
		t.Errorf("Expected first plugin to be echo, got %q", plugs[0].Metadata.Name)
	}
	if plugs[2].Metadata.Name != "hello" {
		t.Errorf("Expected second plugin to be hello, got %q", plugs[1].Metadata.Name)
	}
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
			expected: 3,
		},
	}
	for _, c := range cases {
		t.Run(t.Name(), func(t *testing.T) {
			plugin, _ := FindPlugins(c.plugdirs)
			if len(plugin) != c.expected {
				t.Errorf("expected: %v, got: %v", c.expected, len(plugin))
			}
		})
	}
}

func TestSetupEnv(t *testing.T) {
	name := "pequod"
	base := filepath.Join("testdata/helmhome/helm/plugins", name)

	s := cli.New()
	s.PluginsDirectory = "testdata/helmhome/helm/plugins"

	SetupPluginEnv(s, name, base)
	for _, tt := range []struct {
		name, expect string
	}{
		{"HELM_PLUGIN_NAME", name},
		{"HELM_PLUGIN_DIR", base},
	} {
		if got := os.Getenv(tt.name); got != tt.expect {
			t.Errorf("Expected $%s=%q, got %q", tt.name, tt.expect, got)
		}
	}
}

func TestSetupEnvWithSpace(t *testing.T) {
	name := "sureshdsk"
	base := filepath.Join("testdata/helm home/helm/plugins", name)

	s := cli.New()
	s.PluginsDirectory = "testdata/helm home/helm/plugins"

	SetupPluginEnv(s, name, base)
	for _, tt := range []struct {
		name, expect string
	}{
		{"HELM_PLUGIN_NAME", name},
		{"HELM_PLUGIN_DIR", base},
	} {
		if got := os.Getenv(tt.name); got != tt.expect {
			t.Errorf("Expected $%s=%q, got %q", tt.name, tt.expect, got)
		}
	}
}

func TestValidatePluginData(t *testing.T) {
	// A mock plugin missing any metadata.
	mockMissingMeta := &Plugin{
		Dir: "no-such-dir",
	}

	for i, item := range []struct {
		pass bool
		plug *Plugin
	}{
		{true, mockPlugin("abcdefghijklmnopqrstuvwxyz0123456789_-ABC")},
		{true, mockPlugin("foo-bar-FOO-BAR_1234")},
		{false, mockPlugin("foo -bar")},
		{false, mockPlugin("$foo -bar")}, // Test leading chars
		{false, mockPlugin("foo -bar ")}, // Test trailing chars
		{false, mockPlugin("foo\nbar")},  // Test newline
		{false, mockMissingMeta},         // Test if the metadata section missing
	} {
		err := validatePluginData(item.plug, fmt.Sprintf("test-%d", i))
		if item.pass && err != nil {
			t.Errorf("failed to validate case %d: %s", i, err)
		} else if !item.pass && err == nil {
			t.Errorf("expected case %d to fail", i)
		}
	}
}

func TestDetectDuplicates(t *testing.T) {
	plugs := []*Plugin{
		mockPlugin("foo"),
		mockPlugin("bar"),
	}
	if err := detectDuplicates(plugs); err != nil {
		t.Error("no duplicates in the first set")
	}
	plugs = append(plugs, mockPlugin("foo"))
	if err := detectDuplicates(plugs); err == nil {
		t.Error("duplicates in the second set")
	}
}

func mockPlugin(name string) *Plugin {
	return &Plugin{
		Metadata: &Metadata{
			Name:        name,
			Version:     "v0.1.2",
			Usage:       "Mock plugin",
			Description: "Mock plugin for testing",
			Command:     "echo mock plugin",
		},
		Dir: "no-such-dir",
	}
}
