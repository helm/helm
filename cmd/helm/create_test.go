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
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"k8s.io/helm/pkg/chartutil"
	"k8s.io/helm/pkg/proto/hapi/chart"
)

func TestCreateCmd(t *testing.T) {
	cname := "testchart"
	// Make a temp dir
	tdir, err := ioutil.TempDir("", "helm-create-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tdir)

	// CD into it
	pwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tdir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(pwd)

	// Run a create
	cmd := newCreateCmd(os.Stdout)
	if err := cmd.RunE(cmd, []string{cname}); err != nil {
		t.Errorf("Failed to run create: %s", err)
		return
	}

	// Test that the chart is there
	if fi, err := os.Stat(cname); err != nil {
		t.Fatalf("no chart directory: %s", err)
	} else if !fi.IsDir() {
		t.Fatalf("chart is not directory")
	}

	c, err := chartutil.LoadDir(cname)
	if err != nil {
		t.Fatal(err)
	}

	if c.Metadata.Name != cname {
		t.Errorf("Expected %q name, got %q", cname, c.Metadata.Name)
	}
	if c.Metadata.ApiVersion != chartutil.ApiVersionV1 {
		t.Errorf("Wrong API version: %q", c.Metadata.ApiVersion)
	}
}

func TestCreateStarterCmd(t *testing.T) {
	cname := "testchart"
	// Make a temp dir
	tdir, err := ioutil.TempDir("", "helm-create-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tdir)

	thome, err := tempHelmHome(t)
	if err != nil {
		t.Fatal(err)
	}
	cleanup := resetEnv()
	defer func() {
		os.RemoveAll(thome.String())
		cleanup()
	}()

	settings.Home = thome

	// Create a starter.
	starterchart := filepath.Join(thome.String(), "starters")
	os.Mkdir(starterchart, 0755)
	if dest, err := chartutil.Create(&chart.Metadata{Name: "starterchart"}, starterchart); err != nil {
		t.Fatalf("Could not create chart: %s", err)
	} else {
		t.Logf("Created %s", dest)
	}
	tplpath := filepath.Join(starterchart, "starterchart", "templates", "foo.tpl")
	if err := ioutil.WriteFile(tplpath, []byte("test"), 0755); err != nil {
		t.Fatalf("Could not write template: %s", err)
	}

	// CD into it
	pwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tdir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(pwd)

	// Run a create
	cmd := newCreateCmd(os.Stdout)
	cmd.ParseFlags([]string{"--starter", "starterchart"})
	if err := cmd.RunE(cmd, []string{cname}); err != nil {
		t.Errorf("Failed to run create: %s", err)
		return
	}

	// Test that the chart is there
	if fi, err := os.Stat(cname); err != nil {
		t.Fatalf("no chart directory: %s", err)
	} else if !fi.IsDir() {
		t.Fatalf("chart is not directory")
	}

	c, err := chartutil.LoadDir(cname)
	if err != nil {
		t.Fatal(err)
	}

	if c.Metadata.Name != cname {
		t.Errorf("Expected %q name, got %q", cname, c.Metadata.Name)
	}
	if c.Metadata.ApiVersion != chartutil.ApiVersionV1 {
		t.Errorf("Wrong API version: %q", c.Metadata.ApiVersion)
	}

	if l := len(c.Templates); l != 6 {
		t.Errorf("Expected 5 templates, got %d", l)
	}

	found := false
	for _, tpl := range c.Templates {
		if tpl.Name == "templates/foo.tpl" {
			found = true
			data := tpl.Data
			if string(data) != "test" {
				t.Errorf("Expected template 'test', got %q", string(data))
			}
		}
	}
	if !found {
		t.Error("Did not find foo.tpl")
	}

}
