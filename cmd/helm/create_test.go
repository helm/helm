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

	"helm.sh/helm/internal/test/ensure"
	"helm.sh/helm/pkg/chart"
	"helm.sh/helm/pkg/chart/loader"
	"helm.sh/helm/pkg/chartutil"
	"helm.sh/helm/pkg/helmpath"
)

func TestCreateCmd(t *testing.T) {
	cname := "testchart"
	ensure.HelmHome(t)
	defer ensure.CleanHomeDirs(t)
	defer testChdir(t, helmpath.CachePath())()

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
	if c.Metadata.APIVersion != chart.APIVersionV2 {
		t.Errorf("Wrong API version: %q", c.Metadata.APIVersion)
	}
}

func TestCreateStarterCmd(t *testing.T) {
	cname := "testchart"
	defer resetEnv()()
	ensure.HelmHome(t)
	defer ensure.CleanHomeDirs(t)
	defer testChdir(t, helmpath.CachePath())()

	// Create a starter.
	starterchart := helmpath.Starters()
	os.Mkdir(starterchart, 0755)
	if dest, err := chartutil.Create("starterchart", starterchart); err != nil {
		t.Fatalf("Could not create chart: %s", err)
	} else {
		t.Logf("Created %s", dest)
	}
	tplpath := filepath.Join(starterchart, "starterchart", "templates", "foo.tpl")
	if err := ioutil.WriteFile(tplpath, []byte("test"), 0755); err != nil {
		t.Fatalf("Could not write template: %s", err)
	}

	// Run a create
	if _, _, err := executeActionCommand(fmt.Sprintf("create --starter=starterchart %s", cname)); err != nil {
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
	if c.Metadata.APIVersion != chart.APIVersionV2 {
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

func TestCreateStarterAbsoluteCmd(t *testing.T) {
	defer resetEnv()()
	ensure.HelmHome(t)
	defer ensure.CleanHomeDirs(t)
	cname := "testchart"

	// Create a starter.
	starterchart := helmpath.Starters()
	os.Mkdir(starterchart, 0755)
	if dest, err := chartutil.Create("starterchart", starterchart); err != nil {
		t.Fatalf("Could not create chart: %s", err)
	} else {
		t.Logf("Created %s", dest)
	}
	tplpath := filepath.Join(starterchart, "starterchart", "templates", "foo.tpl")
	if err := ioutil.WriteFile(tplpath, []byte("test"), 0755); err != nil {
		t.Fatalf("Could not write template: %s", err)
	}

	defer testChdir(t, helmpath.CachePath())()

	starterChartPath := filepath.Join(starterchart, "starterchart")

	// Run a create
	if _, _, err := executeActionCommand(fmt.Sprintf("create --starter=%s %s", starterChartPath, cname)); err != nil {
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
	if c.Metadata.APIVersion != chart.APIVersionV2 {
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
