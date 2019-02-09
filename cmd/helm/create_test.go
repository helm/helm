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
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"k8s.io/helm/pkg/chart"
	"k8s.io/helm/pkg/chart/loader"
	"k8s.io/helm/pkg/chartutil"
)

func TestCreateCmd(t *testing.T) {
	tdir := testTempDir(t)
	defer testChdir(t, tdir)()

	cname := "testchart"

	// Run a create
	if _, _, err := executeActionCommand("create " + cname); err != nil {
		t.Errorf("Failed to run create: %s", err)
		return
	}

	// Test that the chart is there
	if fi, err := os.Stat(cname); err != nil {
		t.Fatalf("no chart directory: %s", err)
	} else if !fi.IsDir() {
		t.Fatalf("chart is not directory")
	}

	c, err := loader.LoadDir(cname)
	if err != nil {
		t.Fatal(err)
	}

	if c.Name() != cname {
		t.Errorf("Expected %q name, got %q", cname, c.Name())
	}
	if c.Metadata.APIVersion != chart.APIVersionv1 {
		t.Errorf("Wrong API version: %q", c.Metadata.APIVersion)
	}
}

func TestCreateStarterCmd(t *testing.T) {
	defer resetEnv()()

	cname := "testchart"
	// Make a temp dir
	tdir := testTempDir(t)

	hh := testHelmHome(t)
	settings.Home = hh

	// Create a starter.
	starterchart := hh.Starters()
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

	defer testChdir(t, tdir)()

	// Run a create
	if _, _, err := executeActionCommand(fmt.Sprintf("--home='%s' create --starter=starterchart %s", hh.String(), cname)); err != nil {
		t.Errorf("Failed to run create: %s", err)
		return
	}

	// Test that the chart is there
	if fi, err := os.Stat(cname); err != nil {
		t.Fatalf("no chart directory: %s", err)
	} else if !fi.IsDir() {
		t.Fatalf("chart is not directory")
	}

	c, err := loader.LoadDir(cname)
	if err != nil {
		t.Fatal(err)
	}

	if c.Name() != cname {
		t.Errorf("Expected %q name, got %q", cname, c.Name())
	}
	if c.Metadata.APIVersion != chart.APIVersionv1 {
		t.Errorf("Wrong API version: %q", c.Metadata.APIVersion)
	}

	if l := len(c.Templates); l != 6 {
		t.Errorf("Expected 5 templates, got %d", l)
	}

	found := false
	for _, tpl := range c.Templates {
		if tpl.Name == "templates/foo.tpl" {
			found = true
			if data := string(tpl.Data); data != "test" {
				t.Errorf("Expected template 'test', got %q", data)
			}
		}
	}
	if !found {
		t.Error("Did not find foo.tpl")
	}

}
