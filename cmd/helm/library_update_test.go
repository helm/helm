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
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"k8s.io/helm/pkg/chart"
	"k8s.io/helm/pkg/chartutil"
	"k8s.io/helm/pkg/provenance"
	"k8s.io/helm/pkg/repo"
	"k8s.io/helm/pkg/repo/repotest"
)

func TestLibraryUpdateCmd(t *testing.T) {
	defer resetEnv()()

	hh := testHelmHome(t)
	settings.Home = hh

	srv := repotest.NewServer(hh.String())
	defer srv.Stop()
	copied, err := srv.CopyCharts("testdata/testcharts/lib-charts/*.tgz")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Copied charts:\n%s", strings.Join(copied, "\n"))
	t.Logf("Listening on directory %s", srv.Root())

	chartname := "libup"
	md := createTestingMetadataLibRef(chartname, srv.URL())
	if _, err := chartutil.Create(md, hh.String()); err != nil {
		t.Fatal(err)
	}

	out, err := executeCommand(nil, fmt.Sprintf("--home='%s' library update '%s'", hh, hh.Path(chartname)))
	if err != nil {
		t.Logf("Output: %s", out)
		t.Fatal(err)
	}

	// This is written directly to stdout, so we have to capture as is.
	if !strings.Contains(out, `update from the "test" chart repository`) {
		t.Errorf("Repo did not get updated\n%s", out)
	}

	// Make sure the actual file got downloaded.
	expect := hh.Path(chartname, "library/common-0.0.5.tgz")
	if _, err := os.Stat(expect); err != nil {
		t.Fatal(err)
	}

	hash, err := provenance.DigestFile(expect)
	if err != nil {
		t.Fatal(err)
	}

	i, err := repo.LoadIndexFile(hh.CacheIndex("test"))
	if err != nil {
		t.Fatal(err)
	}

	reqver := i.Entries["common"][0]
	if h := reqver.Digest; h != hash {
		t.Errorf("Failed hash match: expected %s, got %s", hash, h)
	}

	// Now change the libraries and update. This verifies that on update,
	// old libraries are cleansed and new libraries are added.
	md.Libraries = []*chart.Dependency{
		{Name: "common", Version: "0.0.5", Repository: srv.URL()},
		{Name: "compressedchart", Version: "0.3.0", Repository: srv.URL()},
	}
	dir := hh.Path(chartname, "Chart.yaml")
	if err := chartutil.SaveChartfile(dir, md); err != nil {
		t.Fatal(err)
	}

	out, err = executeCommand(nil, fmt.Sprintf("--home='%s' library update '%s'", hh, hh.Path(chartname)))
	if err != nil {
		t.Logf("Output: %s", out)
		t.Fatal(err)
	}

	// In this second run, we should see compressedchart-0.3.0.tgz, and not
	// the 0.1.0 version.
	expect = hh.Path(chartname, "library/compressedchart-0.3.0.tgz")
	if _, err := os.Stat(expect); err != nil {
		t.Fatalf("Expected %q: %s", expect, err)
	}
	dontExpect := hh.Path(chartname, "library/compressedchart-0.1.0.tgz")
	if _, err := os.Stat(dontExpect); err == nil {
		t.Fatalf("Unexpected %q", dontExpect)
	}
}

func TestLibraryUpdateCmd_SkipRefresh(t *testing.T) {
	defer resetEnv()()

	hh := testHelmHome(t)
	settings.Home = hh

	srv := repotest.NewServer(hh.String())
	defer srv.Stop()
	copied, err := srv.CopyCharts("testdata/testcharts/lib-charts/*.tgz")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Copied charts:\n%s", strings.Join(copied, "\n"))
	t.Logf("Listening on directory %s", srv.Root())

	chartname := "libup"
	if err := createTestingChartLibRef(hh.String(), chartname, srv.URL()); err != nil {
		t.Fatal(err)
	}

	out, err := executeCommand(nil, fmt.Sprintf("--home='%s' library update --skip-refresh '%s'", hh, hh.Path(chartname)))
	if err == nil {
		t.Fatal("Expected failure to find the repo with skipRefresh")
	}

	// This is written directly to stdout, so we have to capture as is.
	if strings.Contains(out, `update from the "test" chart repository`) {
		t.Errorf("Repo was unexpectedly updated\n%s", out)
	}
}

func TestLibraryUpdateCmd_DontDeleteOldChartsOnError(t *testing.T) {
	defer resetEnv()()

	hh := testHelmHome(t)
	settings.Home = hh

	srv := repotest.NewServer(hh.String())
	defer srv.Stop()
	copied, err := srv.CopyCharts("testdata/testcharts/lib-charts/*.tgz")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Copied charts:\n%s", strings.Join(copied, "\n"))
	t.Logf("Listening on directory %s", srv.Root())

	chartname := "libupdelete"
	if err := createTestingChartLibRef(hh.String(), chartname, srv.URL()); err != nil {
		t.Fatal(err)
	}

	out := bytes.NewBuffer(nil)
	o := &refUpdateOptions{}
	o.helmhome = hh
	o.chartpath = hh.Path(chartname)

	if err := o.run(out, true); err != nil {
		output := out.String()
		t.Logf("Output: %s", output)
		t.Fatal(err)
	}

	// Chart repo is down
	srv.Stop()

	if err := o.run(out, true); err == nil {
		output := out.String()
		t.Logf("Output: %s", output)
		t.Fatal("Expected error, got nil")
	}

	// Make sure charts dir still has libraries
	files, err := ioutil.ReadDir(filepath.Join(o.chartpath, "library"))
	if err != nil {
		t.Fatal(err)
	}
	libraries := []string{"common-0.0.5.tgz", "compressedchart-0.1.0.tgz"}

	if len(libraries) != len(files) {
		t.Fatalf("Expected %d chart library, got %d", len(libraries), len(files))
	}
	for index, file := range files {
		if libraries[index] != file.Name() {
			t.Fatalf("Chart library %s not matching %s", libraries[index], file.Name())
		}
	}

	// Make sure tmpcharts is deleted
	if _, err := os.Stat(filepath.Join(o.chartpath, "tmpcharts")); !os.IsNotExist(err) {
		t.Fatalf("tmpcharts dir still exists")
	}
}

// createTestingMetadataLibRef creates a basic chart that depends on lib chart
// common-0.0.5
//
// The baseURL can be used to point to a particular repository server.
func createTestingMetadataLibRef(name, baseURL string) *chart.Metadata {
	return &chart.Metadata{
		Name:    name,
		Version: "1.2.3",
		Libraries: []*chart.Dependency{
			{Name: "common", Version: "0.0.5", Repository: baseURL},
			{Name: "compressedchart", Version: "0.1.0", Repository: baseURL},
		},
	}
}

// createTestingChartLibRef creates a basic chart that depends on
// lib chart common-0.0.5
//
// The baseURL can be used to point to a particular repository server.
func createTestingChartLibRef(dest, name, baseURL string) error {
	cfile := createTestingMetadataLibRef(name, baseURL)
	_, err := chartutil.Create(cfile, dest)
	return err
}
