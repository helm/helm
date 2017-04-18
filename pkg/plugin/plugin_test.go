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

package plugin // import "k8s.io/helm/pkg/plugin"

import (
	"reflect"
	"testing"
)

func TestPrepareCommand(t *testing.T) {
	p := &Plugin{
		Dir: "/tmp", // Unused
		Metadata: &Metadata{
			Name:    "test",
			Command: "echo -n foo",
		},
	}
	argv := []string{"--debug", "--foo", "bar"}

	cmd, args := p.PrepareCommand(argv)
	if cmd != "echo" {
		t.Errorf("Expected echo, got %q", cmd)
	}

	if l := len(args); l != 5 {
		t.Errorf("expected 5 args, got %d", l)
	}

	expect := []string{"-n", "foo", "--debug", "--foo", "bar"}
	for i := 0; i < len(args); i++ {
		if expect[i] != args[i] {
			t.Errorf("Expected arg=%q, got %q", expect[i], args[i])
		}
	}

	// Test with IgnoreFlags. This should omit --debug, --foo, bar
	p.Metadata.IgnoreFlags = true
	cmd, args = p.PrepareCommand(argv)
	if cmd != "echo" {
		t.Errorf("Expected echo, got %q", cmd)
	}
	if l := len(args); l != 2 {
		t.Errorf("expected 2 args, got %d", l)
	}
	expect = []string{"-n", "foo"}
	for i := 0; i < len(args); i++ {
		if expect[i] != args[i] {
			t.Errorf("Expected arg=%q, got %q", expect[i], args[i])
		}
	}
}

func TestLoadDir(t *testing.T) {
	dirname := "testdata/plugdir/hello"
	plug, err := LoadDir(dirname)
	if err != nil {
		t.Fatalf("error loading Hello plugin: %s", err)
	}

	if plug.Dir != dirname {
		t.Errorf("Expected dir %q, got %q", dirname, plug.Dir)
	}

	expect := &Metadata{
		Name:        "hello",
		Version:     "0.1.0",
		Usage:       "usage",
		Description: "description",
		Command:     "$HELM_PLUGIN_SELF/hello.sh",
		UseTunnel:   true,
		IgnoreFlags: true,
		Hooks: map[string]string{
			Install: "echo installing...",
		},
	}

	if !reflect.DeepEqual(expect, plug.Metadata) {
		t.Errorf("Expected plugin metadata %v, got %v", expect, plug.Metadata)
	}
}

func TestDownloader(t *testing.T) {
	dirname := "testdata/plugdir/downloader"
	plug, err := LoadDir(dirname)
	if err != nil {
		t.Fatalf("error loading Hello plugin: %s", err)
	}

	if plug.Dir != dirname {
		t.Errorf("Expected dir %q, got %q", dirname, plug.Dir)
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
		t.Errorf("Expected metadata %v, got %v", expect, plug.Metadata)
	}
}

func TestLoadAll(t *testing.T) {

	// Verify that empty dir loads:
	if plugs, err := LoadAll("testdata"); err != nil {
		t.Fatalf("error loading dir with no plugins: %s", err)
	} else if len(plugs) > 0 {
		t.Fatalf("expected empty dir to have 0 plugins")
	}

	basedir := "testdata/plugdir"
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
