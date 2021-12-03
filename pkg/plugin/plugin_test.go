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
	"testing"

	"helm.sh/helm/v3/pkg/cli"
)

func checkCommand(p *Plugin, extraArgs []string, osStrCmp string, t *testing.T) {
	cmd, args, err := p.PrepareCommand(extraArgs)
	if err != nil {
		t.Fatal(err)
	}
	if cmd != "echo" {
		t.Fatalf("Expected echo, got %q", cmd)
	}

	if l := len(args); l != 5 {
		t.Fatalf("expected 5 args, got %d", l)
	}

	expect := []string{"-n", osStrCmp, "--debug", "--foo", "bar"}
	for i := 0; i < len(args); i++ {
		if expect[i] != args[i] {
			t.Errorf("Expected arg=%q, got %q", expect[i], args[i])
		}
	}

	// Test with IgnoreFlags. This should omit --debug, --foo, bar
	p.Metadata.IgnoreFlags = true
	cmd, args, err = p.PrepareCommand(extraArgs)
	if err != nil {
		t.Fatal(err)
	}
	if cmd != "echo" {
		t.Fatalf("Expected echo, got %q", cmd)
	}
	if l := len(args); l != 2 {
		t.Fatalf("expected 2 args, got %d", l)
	}
	expect = []string{"-n", osStrCmp}
	for i := 0; i < len(args); i++ {
		if expect[i] != args[i] {
			t.Errorf("Expected arg=%q, got %q", expect[i], args[i])
		}
	}
}

func TestPrepareCommand(t *testing.T) {
	p := &Plugin{
		Dir: "/tmp", // Unused
		Metadata: &Metadata{
			Name:    "test",
			Command: "echo -n foo",
		},
	}
	argv := []string{"--debug", "--foo", "bar"}

	checkCommand(p, argv, "foo", t)
}

func TestPlatformPrepareCommand(t *testing.T) {
	p := &Plugin{
		Dir: "/tmp", // Unused
		Metadata: &Metadata{
			Name:    "test",
			Command: "echo -n os-arch",
			PlatformCommand: []PlatformCommand{
				{OperatingSystem: "linux", Architecture: "i386", Command: "echo -n linux-i386"},
				{OperatingSystem: "linux", Architecture: "amd64", Command: "echo -n linux-amd64"},
				{OperatingSystem: "linux", Architecture: "arm64", Command: "echo -n linux-arm64"},
				{OperatingSystem: "linux", Architecture: "ppc64le", Command: "echo -n linux-ppc64le"},
				{OperatingSystem: "linux", Architecture: "s390x", Command: "echo -n linux-s390x"},
				{OperatingSystem: "windows", Architecture: "amd64", Command: "echo -n win-64"},
			},
		},
	}
	var osStrCmp string
	os := runtime.GOOS
	arch := runtime.GOARCH
	if os == "linux" && arch == "i386" {
		osStrCmp = "linux-i386"
	} else if os == "linux" && arch == "amd64" {
		osStrCmp = "linux-amd64"
	} else if os == "linux" && arch == "arm64" {
		osStrCmp = "linux-arm64"
	} else if os == "linux" && arch == "ppc64le" {
		osStrCmp = "linux-ppc64le"
	} else if os == "linux" && arch == "s390x" {
		osStrCmp = "linux-s390x"
	} else if os == "windows" && arch == "amd64" {
		osStrCmp = "win-64"
	} else {
		osStrCmp = "os-arch"
	}

	argv := []string{"--debug", "--foo", "bar"}
	checkCommand(p, argv, osStrCmp, t)
}

func TestPartialPlatformPrepareCommand(t *testing.T) {
	p := &Plugin{
		Dir: "/tmp", // Unused
		Metadata: &Metadata{
			Name:    "test",
			Command: "echo -n os-arch",
			PlatformCommand: []PlatformCommand{
				{OperatingSystem: "linux", Architecture: "i386", Command: "echo -n linux-i386"},
				{OperatingSystem: "windows", Architecture: "amd64", Command: "echo -n win-64"},
			},
		},
	}
	var osStrCmp string
	os := runtime.GOOS
	arch := runtime.GOARCH
	if os == "linux" {
		osStrCmp = "linux-i386"
	} else if os == "windows" && arch == "amd64" {
		osStrCmp = "win-64"
	} else {
		osStrCmp = "os-arch"
	}

	argv := []string{"--debug", "--foo", "bar"}
	checkCommand(p, argv, osStrCmp, t)
}

func TestNoPrepareCommand(t *testing.T) {
	p := &Plugin{
		Dir: "/tmp", // Unused
		Metadata: &Metadata{
			Name: "test",
		},
	}
	argv := []string{"--debug", "--foo", "bar"}

	_, _, err := p.PrepareCommand(argv)
	if err == nil {
		t.Fatalf("Expected error to be returned")
	}
}

func TestNoMatchPrepareCommand(t *testing.T) {
	p := &Plugin{
		Dir: "/tmp", // Unused
		Metadata: &Metadata{
			Name: "test",
			PlatformCommand: []PlatformCommand{
				{OperatingSystem: "no-os", Architecture: "amd64", Command: "echo -n linux-i386"},
			},
		},
	}
	argv := []string{"--debug", "--foo", "bar"}

	if _, _, err := p.PrepareCommand(argv); err == nil {
		t.Fatalf("Expected error to be returned")
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
		Command:     "$HELM_PLUGIN_DIR/hello.sh",
		IgnoreFlags: true,
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

func TestValidatePluginData(t *testing.T) {
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
