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
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

const (
	testfile         = "testdata/frobnitz/Chart.yaml"
	testdir          = "testdata/frobnitz/"
	testarchive      = "testdata/frobnitz-0.0.1.tgz"
	testmember       = "templates/template.tpl"
	expectedTemplate = "Hello {{.Name | default \"world\"}}\n"
)

// Type canaries. If these fail, they will fail at compile time.
var _ chartLoader = &dirChart{}
var _ chartLoader = &tarChart{}

func TestLoadDir(t *testing.T) {

	c, err := LoadDir(testdir)
	if err != nil {
		t.Errorf("Failed to load chart: %s", err)
	}

	if c.Chartfile().Name != "frobnitz" {
		t.Errorf("Expected chart name to be 'frobnitz'. Got '%s'.", c.Chartfile().Name)
	}
}

func TestCreate(t *testing.T) {
	tdir, err := ioutil.TempDir("", "helm-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tdir)

	cf := &Chartfile{Name: "foo"}

	c, err := Create(cf, tdir)
	if err != nil {
		t.Fatal(err)
	}

	dir := filepath.Join(tdir, "foo")

	if c.Chartfile().Name != "foo" {
		t.Errorf("Expected name to be 'foo', got %q", c.Chartfile().Name)
	}

	for _, d := range []string{preTemplates, preCharts} {
		if fi, err := os.Stat(filepath.Join(dir, d)); err != nil {
			t.Errorf("Expected %s dir: %s", d, err)
		} else if !fi.IsDir() {
			t.Errorf("Expected %s to be a directory.", d)
		}
	}

	for _, f := range []string{ChartfileName, preValues} {
		if fi, err := os.Stat(filepath.Join(dir, f)); err != nil {
			t.Errorf("Expected %s file: %s", f, err)
		} else if fi.IsDir() {
			t.Errorf("Expected %s to be a fle.", f)
		}
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

	dir := c.Dir()
	d := c.ChartsDir()
	if d != filepath.Join(dir, preCharts) {
		t.Errorf("Unexpectedly, charts are in %s", d)
	}

	d = c.TemplatesDir()
	if d != filepath.Join(dir, preTemplates) {
		t.Errorf("Unexpectedly, templates are in %s", d)
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
		t.Errorf("Unexpected chart file\nwant:\n%v\nhave:\n%v\n", want, have)
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

func TestExpand(t *testing.T) {
	r, err := os.Open(testarchive)
	if err != nil {
		t.Errorf("Failed to read testarchive file: %s", err)
		return
	}

	td, err := ioutil.TempDir("", "helm-unittest-chart-")
	if err != nil {
		t.Errorf("Failed to create tempdir: %s", err)
		return
	}

	err = Expand(td, r)
	if err != nil {
		t.Errorf("Failed to expand testarchive file: %s", err)
	}

	fi, err := os.Lstat(td + "/frobnitz/Chart.yaml")
	if err != nil {
		t.Errorf("Failed to stat Chart.yaml from expanded archive: %s", err)
	}
	if fi.Name() != "Chart.yaml" {
		t.Errorf("Didn't get the right file name from stat, expected Chart.yaml, got: %s", fi.Name())
	}

	tr, err := os.Open(td + "/frobnitz/templates/template.tpl")
	if err != nil {
		t.Errorf("Failed to open template.tpl from expanded archive: %s", err)
	}
	c, err := ioutil.ReadAll(tr)
	if err != nil {
		t.Errorf("Failed to read contents of template.tpl from expanded archive: %s", err)
	}
	if string(c) != expectedTemplate {
		t.Errorf("Contents of the expanded template differ, wanted '%s' got '%s'", expectedTemplate, c)
	}
}
