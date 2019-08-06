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
	"strings"
	"testing"

	"helm.sh/helm/internal/test/ensure"
	"helm.sh/helm/pkg/chart"
	"helm.sh/helm/pkg/chartutil"
	"helm.sh/helm/pkg/helmpath"
	"helm.sh/helm/pkg/provenance"
	"helm.sh/helm/pkg/repo"
	"helm.sh/helm/pkg/repo/repotest"
)

func TestDependencyUpdateCmd(t *testing.T) {
	defer resetEnv()()

	ensure.HelmHome(t)
	defer ensure.CleanHomeDirs(t)

	srv := repotest.NewServer(helmpath.ConfigPath())
	defer srv.Stop()
	copied, err := srv.CopyCharts("testdata/testcharts/*.tgz")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Copied charts:\n%s", strings.Join(copied, "\n"))
	t.Logf("Listening on directory %s", srv.Root())

	chartname := "depup"
	ch := createTestingMetadata(chartname, srv.URL())
	md := ch.Metadata
	if err := chartutil.SaveDir(ch, helmpath.DataPath()); err != nil {
		t.Fatal(err)
	}

	_, out, err := executeActionCommand(fmt.Sprintf("dependency update '%s'", filepath.Join(helmpath.DataPath(), chartname)))
	if err != nil {
		t.Logf("Output: %s", out)
		t.Fatal(err)
	}

	// This is written directly to stdout, so we have to capture as is.
	if !strings.Contains(out, `update from the "test" chart repository`) {
		t.Errorf("Repo did not get updated\n%s", out)
	}

	// Make sure the actual file got downloaded.
	expect := filepath.Join(helmpath.DataPath(), chartname, "charts/reqtest-0.1.0.tgz")
	if _, err := os.Stat(expect); err != nil {
		t.Fatal(err)
	}

	hash, err := provenance.DigestFile(expect)
	if err != nil {
		t.Fatal(err)
	}

	i, err := repo.LoadIndexFile(helmpath.CacheIndex("test"))
	if err != nil {
		t.Fatal(err)
	}

	reqver := i.Entries["reqtest"][0]
	if h := reqver.Digest; h != hash {
		t.Errorf("Failed hash match: expected %s, got %s", hash, h)
	}

	// Now change the dependencies and update. This verifies that on update,
	// old dependencies are cleansed and new dependencies are added.
	md.Dependencies = []*chart.Dependency{
		{Name: "reqtest", Version: "0.1.0", Repository: srv.URL()},
		{Name: "compressedchart", Version: "0.3.0", Repository: srv.URL()},
	}
	dir := filepath.Join(helmpath.DataPath(), chartname, "Chart.yaml")
	if err := chartutil.SaveChartfile(dir, md); err != nil {
		t.Fatal(err)
	}

	_, out, err = executeActionCommand(fmt.Sprintf("dependency update '%s'", filepath.Join(helmpath.DataPath(), chartname)))
	if err != nil {
		t.Logf("Output: %s", out)
		t.Fatal(err)
	}

	// In this second run, we should see compressedchart-0.3.0.tgz, and not
	// the 0.1.0 version.
	expect = filepath.Join(helmpath.DataPath(), chartname, "charts/compressedchart-0.3.0.tgz")
	if _, err := os.Stat(expect); err != nil {
		t.Fatalf("Expected %q: %s", expect, err)
	}
	dontExpect := filepath.Join(helmpath.DataPath(), chartname, "charts/compressedchart-0.1.0.tgz")
	if _, err := os.Stat(dontExpect); err == nil {
		t.Fatalf("Unexpected %q", dontExpect)
	}
}

func TestDependencyUpdateCmd_SkipRefresh(t *testing.T) {
	defer resetEnv()()

	ensure.HelmHome(t)
	defer ensure.CleanHomeDirs(t)

	srv := repotest.NewServer(helmpath.ConfigPath())
	defer srv.Stop()
	copied, err := srv.CopyCharts("testdata/testcharts/*.tgz")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Copied charts:\n%s", strings.Join(copied, "\n"))
	t.Logf("Listening on directory %s", srv.Root())

	chartname := "depup"
	if err := createTestingChart(helmpath.DataPath(), chartname, srv.URL()); err != nil {
		t.Fatal(err)
	}

	_, out, err := executeActionCommand(fmt.Sprintf("dependency update --skip-refresh %s", filepath.Join(helmpath.DataPath(), chartname)))
	if err == nil {
		t.Fatal("Expected failure to find the repo with skipRefresh")
	}

	// This is written directly to stdout, so we have to capture as is.
	if strings.Contains(out, `update from the "test" chart repository`) {
		t.Errorf("Repo was unexpectedly updated\n%s", out)
	}
}

func TestDependencyUpdateCmd_DontDeleteOldChartsOnError(t *testing.T) {
	defer resetEnv()()

	ensure.HelmHome(t)
	defer ensure.CleanHomeDirs(t)

	srv := repotest.NewServer(helmpath.ConfigPath())
	defer srv.Stop()
	copied, err := srv.CopyCharts("testdata/testcharts/*.tgz")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Copied charts:\n%s", strings.Join(copied, "\n"))
	t.Logf("Listening on directory %s", srv.Root())

	chartname := "depupdelete"
	if err := createTestingChart(helmpath.DataPath(), chartname, srv.URL()); err != nil {
		t.Fatal(err)
	}

	_, output, err := executeActionCommand(fmt.Sprintf("dependency update %s", filepath.Join(helmpath.DataPath(), chartname)))
	if err != nil {
		t.Logf("Output: %s", output)
		t.Fatal(err)
	}

	// Chart repo is down
	srv.Stop()

	_, output, err = executeActionCommand(fmt.Sprintf("dependency update %s", filepath.Join(helmpath.DataPath(), chartname)))
	if err == nil {
		t.Logf("Output: %s", output)
		t.Fatal("Expected error, got nil")
	}

	// Make sure charts dir still has dependencies
	files, err := ioutil.ReadDir(filepath.Join(filepath.Join(helmpath.DataPath(), chartname), "charts"))
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
	if _, err := os.Stat(filepath.Join(filepath.Join(helmpath.DataPath(), chartname), "tmpcharts")); !os.IsNotExist(err) {
		t.Fatalf("tmpcharts dir still exists")
	}
}

// createTestingMetadata creates a basic chart that depends on reqtest-0.1.0
//
// The baseURL can be used to point to a particular repository server.
func createTestingMetadata(name, baseURL string) *chart.Chart {
	return &chart.Chart{
		Metadata: &chart.Metadata{
			APIVersion: chart.APIVersionV1,
			Name:       name,
			Version:    "1.2.3",
			Dependencies: []*chart.Dependency{
				{Name: "reqtest", Version: "0.1.0", Repository: baseURL},
				{Name: "compressedchart", Version: "0.1.0", Repository: baseURL},
			},
		},
	}
}

// createTestingChart creates a basic chart that depends on reqtest-0.1.0
//
// The baseURL can be used to point to a particular repository server.
func createTestingChart(dest, name, baseURL string) error {
	cfile := createTestingMetadata(name, baseURL)
	return chartutil.SaveDir(cfile, dest)
}
