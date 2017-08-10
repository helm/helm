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
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ghodss/yaml"

	"k8s.io/helm/pkg/chartutil"
	"k8s.io/helm/pkg/helm/helmpath"
	"k8s.io/helm/pkg/proto/hapi/chart"
	"k8s.io/helm/pkg/provenance"
	"k8s.io/helm/pkg/repo"
	"k8s.io/helm/pkg/repo/repotest"
)

func TestDependencyUpdateCmd(t *testing.T) {
	hh, err := tempHelmHome(t)
	if err != nil {
		t.Fatal(err)
	}
	cleanup := resetEnv()
	defer func() {
		os.RemoveAll(hh.String())
		cleanup()
	}()

	settings.Home = hh

	srv := repotest.NewServer(hh.String())
	defer srv.Stop()
	copied, err := srv.CopyCharts("testdata/testcharts/*.tgz")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Copied charts:\n%s", strings.Join(copied, "\n"))
	t.Logf("Listening on directory %s", srv.Root())

	chartname := "depup"
	if err := createTestingChart(hh.String(), chartname, srv.URL()); err != nil {
		t.Fatal(err)
	}

	out := bytes.NewBuffer(nil)
	duc := &dependencyUpdateCmd{out: out}
	duc.helmhome = helmpath.Home(hh)
	duc.chartpath = filepath.Join(hh.String(), chartname)

	if err := duc.run(); err != nil {
		output := out.String()
		t.Logf("Output: %s", output)
		t.Fatal(err)
	}

	output := out.String()
	// This is written directly to stdout, so we have to capture as is.
	if !strings.Contains(output, `update from the "test" chart repository`) {
		t.Errorf("Repo did not get updated\n%s", output)
	}

	// Make sure the actual file got downloaded.
	expect := filepath.Join(hh.String(), chartname, "charts/reqtest-0.1.0.tgz")
	if _, err := os.Stat(expect); err != nil {
		t.Fatal(err)
	}

	hash, err := provenance.DigestFile(expect)
	if err != nil {
		t.Fatal(err)
	}

	i, err := repo.LoadIndexFile(duc.helmhome.CacheIndex("test"))
	if err != nil {
		t.Fatal(err)
	}

	reqver := i.Entries["reqtest"][0]
	if h := reqver.Digest; h != hash {
		t.Errorf("Failed hash match: expected %s, got %s", hash, h)
	}

	// Now change the dependencies and update. This verifies that on update,
	// old dependencies are cleansed and new dependencies are added.
	reqfile := &chartutil.Requirements{
		Dependencies: []*chartutil.Dependency{
			{Name: "reqtest", Version: "0.1.0", Repository: srv.URL()},
			{Name: "compressedchart", Version: "0.3.0", Repository: srv.URL()},
		},
	}
	dir := filepath.Join(hh.String(), chartname)
	if err := writeRequirements(dir, reqfile); err != nil {
		t.Fatal(err)
	}
	if err := duc.run(); err != nil {
		output := out.String()
		t.Logf("Output: %s", output)
		t.Fatal(err)
	}

	// In this second run, we should see compressedchart-0.3.0.tgz, and not
	// the 0.1.0 version.
	expect = filepath.Join(hh.String(), chartname, "charts/compressedchart-0.3.0.tgz")
	if _, err := os.Stat(expect); err != nil {
		t.Fatalf("Expected %q: %s", expect, err)
	}
	dontExpect := filepath.Join(hh.String(), chartname, "charts/compressedchart-0.1.0.tgz")
	if _, err := os.Stat(dontExpect); err == nil {
		t.Fatalf("Unexpected %q", dontExpect)
	}
}

func TestDependencyUpdateCmd_SkipRefresh(t *testing.T) {
	hh, err := tempHelmHome(t)
	if err != nil {
		t.Fatal(err)
	}
	cleanup := resetEnv()
	defer func() {
		os.RemoveAll(hh.String())
		cleanup()
	}()

	settings.Home = hh

	srv := repotest.NewServer(hh.String())
	defer srv.Stop()
	copied, err := srv.CopyCharts("testdata/testcharts/*.tgz")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Copied charts:\n%s", strings.Join(copied, "\n"))
	t.Logf("Listening on directory %s", srv.Root())

	chartname := "depup"
	if err := createTestingChart(hh.String(), chartname, srv.URL()); err != nil {
		t.Fatal(err)
	}

	out := bytes.NewBuffer(nil)
	duc := &dependencyUpdateCmd{out: out}
	duc.helmhome = helmpath.Home(hh)
	duc.chartpath = filepath.Join(hh.String(), chartname)
	duc.skipRefresh = true

	if err := duc.run(); err == nil {
		t.Fatal("Expected failure to find the repo with skipRefresh")
	}

	output := out.String()
	// This is written directly to stdout, so we have to capture as is.
	if strings.Contains(output, `update from the "test" chart repository`) {
		t.Errorf("Repo was unexpectedly updated\n%s", output)
	}
}

func TestDependencyUpdateCmd_DontDeleteOldChartsOnError(t *testing.T) {
	hh, err := tempHelmHome(t)
	if err != nil {
		t.Fatal(err)
	}
	cleanup := resetEnv()
	defer func() {
		os.RemoveAll(hh.String())
		cleanup()
	}()

	settings.Home = hh

	srv := repotest.NewServer(hh.String())
	defer srv.Stop()
	copied, err := srv.CopyCharts("testdata/testcharts/*.tgz")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Copied charts:\n%s", strings.Join(copied, "\n"))
	t.Logf("Listening on directory %s", srv.Root())

	chartname := "depupdelete"
	if err := createTestingChart(hh.String(), chartname, srv.URL()); err != nil {
		t.Fatal(err)
	}

	out := bytes.NewBuffer(nil)
	duc := &dependencyUpdateCmd{out: out}
	duc.helmhome = helmpath.Home(hh)
	duc.chartpath = filepath.Join(hh.String(), chartname)

	if err := duc.run(); err != nil {
		output := out.String()
		t.Logf("Output: %s", output)
		t.Fatal(err)
	}

	// Chart repo is down
	srv.Stop()

	if err := duc.run(); err == nil {
		output := out.String()
		t.Logf("Output: %s", output)
		t.Fatal("Expected error, got nil")
	}

	// Make sure charts dir still has dependencies
	files, err := ioutil.ReadDir(filepath.Join(duc.chartpath, "charts"))
	if err != nil {
		t.Fatal(err)
	}
	dependencies := []string{"compressedchart-0.1.0.tgz", "reqtest-0.1.0.tgz"}

	if len(dependencies) != len(files) {
		t.Fatalf("Expected %d chart dependencies, got %d", len(dependencies), len(files))
	}
	for index, file := range files {
		if dependencies[index] != file.Name() {
			t.Fatalf("Chart dependency %s not matching %s", dependencies[index], file.Name())
		}
	}

	// Make sure tmpcharts is deleted
	if _, err := os.Stat(filepath.Join(duc.chartpath, "tmpcharts")); !os.IsNotExist(err) {
		t.Fatalf("tmpcharts dir still exists")
	}
}

// createTestingChart creates a basic chart that depends on reqtest-0.1.0
//
// The baseURL can be used to point to a particular repository server.
func createTestingChart(dest, name, baseURL string) error {
	cfile := &chart.Metadata{
		Name:    name,
		Version: "1.2.3",
	}
	dir := filepath.Join(dest, name)
	_, err := chartutil.Create(cfile, dest)
	if err != nil {
		return err
	}
	req := &chartutil.Requirements{
		Dependencies: []*chartutil.Dependency{
			{Name: "reqtest", Version: "0.1.0", Repository: baseURL},
			{Name: "compressedchart", Version: "0.1.0", Repository: baseURL},
		},
	}
	return writeRequirements(dir, req)
}

func writeRequirements(dir string, req *chartutil.Requirements) error {
	data, err := yaml.Marshal(req)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(filepath.Join(dir, "requirements.yaml"), data, 0655)
}
