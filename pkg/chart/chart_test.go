/*
Copyright 2015 The Kubernetes Authors All rights reserved.

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

package chart

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/kubernetes/helm/pkg/log"
	"github.com/kubernetes/helm/pkg/util"
)

const (
	testfile    = "testdata/frobnitz/Chart.yaml"
	testdir     = "testdata/frobnitz/"
	testarchive = "testdata/frobnitz-0.0.1.tgz"
	testill     = "testdata/ill-1.2.3.tgz"
	testnochart = "testdata/nochart.tgz"
	testmember  = "templates/wordpress.jinja"
)

// Type canaries. If these fail, they will fail at compile time.
var _ chartLoader = &dirChart{}
var _ chartLoader = &tarChart{}

func init() {
	log.IsDebugging = true
}

func TestLoadDir(t *testing.T) {

	c, err := LoadDir(testdir)
	if err != nil {
		t.Errorf("Failed to load chart: %s", err)
	}

	if c.Chartfile().Name != "frobnitz" {
		t.Errorf("Expected chart name to be 'frobnitz'. Got '%s'.", c.Chartfile().Name)
	}

	if c.Chartfile().Dependencies[0].Version != "^3" {
		d := c.Chartfile().Dependencies[0].Version
		t.Errorf("Expected dependency 0 to have version '^3'. Got '%s'.", d)
	}
}

func TestLoad(t *testing.T) {
	c, err := Load(testarchive)
	if err != nil {
		t.Errorf("Failed to load chart: %s", err)
		return
	}
	defer c.Close()

	if c.Chartfile() == nil {
		t.Error("No chartfile was loaded.")
		return
	}

	if c.Chartfile().Name != "frobnitz" {
		t.Errorf("Expected name to be frobnitz, got %q", c.Chartfile().Name)
	}
}

func TestLoadData(t *testing.T) {
	data, err := ioutil.ReadFile(testarchive)
	if err != nil {
		t.Errorf("Failed to read testarchive file: %s", err)
		return
	}
	c, err := LoadData(data)
	if err != nil {
		t.Errorf("Failed to load chart: %s", err)
		return
	}
	if c.Chartfile() == nil {
		t.Error("No chartfile was loaded.")
		return
	}

	if c.Chartfile().Name != "frobnitz" {
		t.Errorf("Expected name to be frobnitz, got %q", c.Chartfile().Name)
	}
}

func TestLoadIll(t *testing.T) {
	c, err := Load(testill)
	if err != nil {
		t.Errorf("Failed to load chart: %s", err)
		return
	}
	defer c.Close()

	if c.Chartfile() == nil {
		t.Error("No chartfile was loaded.")
		return
	}

	// Ill does not have an icon.
	if i, err := c.Icon(); err == nil {
		t.Errorf("Expected icon to be missing. Got %s", i)
	}
}

func TestLoadNochart(t *testing.T) {
	_, err := Load(testnochart)
	if err == nil {
		t.Error("Nochart should not have loaded at all.")
	}
}

func TestChart(t *testing.T) {
	c, err := LoadDir(testdir)
	if err != nil {
		t.Errorf("Failed to load chart: %s", err)
	}
	defer c.Close()

	if c.Dir() != c.loader.dir() {
		t.Errorf("Unexpected location for directory: %s", c.Dir())
	}

	if c.Chartfile().Name != c.loader.chartfile().Name {
		t.Errorf("Unexpected chart file name: %s", c.Chartfile().Name)
	}

	d := c.DocsDir()
	if d != filepath.Join(testdir, preDocs) {
		t.Errorf("Unexpectedly, docs are in %s", d)
	}

	d = c.TemplatesDir()
	if d != filepath.Join(testdir, preTemplates) {
		t.Errorf("Unexpectedly, templates are in %s", d)
	}

	d = c.HooksDir()
	if d != filepath.Join(testdir, preHooks) {
		t.Errorf("Unexpectedly, hooks are in %s", d)
	}

	i, err := c.Icon()
	if err != nil {
		t.Errorf("No icon found in test chart: %s", err)
	}
	if i != filepath.Join(testdir, preIcon) {
		t.Errorf("Unexpectedly, icon is in %s", i)
	}
}

func TestLoadTemplates(t *testing.T) {
	c, err := LoadDir(testdir)
	if err != nil {
		t.Errorf("Failed to load chart: %s", err)
	}

	members, err := c.LoadTemplates()
	if members == nil {
		t.Fatalf("Cannot load templates: unknown error")
	}

	if err != nil {
		t.Fatalf("Cannot load templates: %s", err)
	}

	dir := c.TemplatesDir()
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		t.Fatalf("Cannot read template directory: %s", err)
	}

	if len(members) != len(files) {
		t.Fatalf("Expected %d templates, got %d", len(files), len(members))
	}

	root := c.loader.dir()
	for _, file := range files {
		path := filepath.Join(preTemplates, file.Name())
		if err := findMember(root, path, members); err != nil {
			t.Fatal(err)
		}
	}
}

func findMember(root, path string, members []*Member) error {
	for _, member := range members {
		if member.Path == path {
			filename := filepath.Join(root, path)
			if err := compareContent(filename, member.Content); err != nil {
				return err
			}

			return nil
		}
	}

	return fmt.Errorf("Template not found: %s", path)
}

func TestLoadMember(t *testing.T) {
	c, err := LoadDir(testdir)
	if err != nil {
		t.Errorf("Failed to load chart: %s", err)
	}

	member, err := c.LoadMember(testmember)
	if member == nil {
		t.Fatalf("Cannot load member %s: unknown error", testmember)
	}

	if err != nil {
		t.Fatalf("Cannot load member %s: %s", testmember, err)
	}

	if member.Path != testmember {
		t.Errorf("Expected member path %s, got %s", testmember, member.Path)
	}

	filename := filepath.Join(c.loader.dir(), testmember)
	if err := compareContent(filename, member.Content); err != nil {
		t.Fatal(err)
	}
}

func TestLoadContent(t *testing.T) {
	c, err := LoadDir(testdir)
	if err != nil {
		t.Errorf("Failed to load chart: %s", err)
	}

	content, err := c.LoadContent()
	if err != nil {
		t.Errorf("Failed to load chart content: %s", err)
	}

	want := c.Chartfile()
	have := content.Chartfile
	if !reflect.DeepEqual(want, have) {
		t.Errorf("Unexpected chart file\nwant:\n%s\nhave:\n%s\n",
			util.ToYAMLOrError(want), util.ToYAMLOrError(have))
	}

	for _, member := range content.Members {
		have := member.Content
		wantMember, err := c.LoadMember(member.Path)
		if err != nil {
			t.Errorf("Failed to load chart member: %s", err)
		}

		t.Logf("%s:\n%s\n\n", member.Path, member.Content)
		want := wantMember.Content
		if !reflect.DeepEqual(want, have) {
			t.Errorf("Unexpected chart member %s\nwant:\n%v\nhave:\n%v\n", member.Path, want, have)
		}
	}
}

func compareContent(filename string, content []byte) error {
	compare, err := ioutil.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("Cannot read test file %s: %s", filename, err)
	}

	if !reflect.DeepEqual(compare, content) {
		return fmt.Errorf("Expected member content\n%v\ngot\n%v", compare, content)
	}

	return nil
}
