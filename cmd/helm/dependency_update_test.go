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
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ghodss/yaml"

	"k8s.io/helm/pkg/chartutil"
	"k8s.io/helm/pkg/proto/hapi/chart"
	"k8s.io/helm/pkg/provenance"
	"k8s.io/helm/pkg/repo"
)

func TestDependencyUpdateCmd(t *testing.T) {
	// Set up a testing helm home
	oldhome := helmHome
	hh, err := tempHelmHome()
	if err != nil {
		t.Fatal(err)
	}
	helmHome = hh // Shoot me now.
	defer func() {
		os.RemoveAll(hh)
		helmHome = oldhome
	}()

	srv := newTestingRepositoryServer(hh)
	defer srv.stop()
	copied, err := srv.copyCharts("testdata/testcharts/*.tgz")
	t.Logf("Copied charts %s", strings.Join(copied, "\n"))
	t.Logf("Listening for directory %s", srv.docroot)

	chartname := "depup"
	if err := createTestingChart(hh, chartname, srv.url()); err != nil {
		t.Fatal(err)
	}

	out := bytes.NewBuffer(nil)
	duc := &dependencyUpdateCmd{out: out}
	duc.helmhome = hh
	duc.chartpath = filepath.Join(hh, chartname)
	duc.repoFile = filepath.Join(duc.helmhome, "repository/repositories.yaml")
	duc.repopath = filepath.Join(duc.helmhome, "repository")

	if err := duc.run(); err != nil {
		t.Fatal(err)
	}

	output := out.String()
	t.Logf("Output: %s", output)
	// This is written directly to stdout, so we have to capture as is.
	if !strings.Contains(output, `update from the "test" chart repository`) {
		t.Errorf("Repo did not get updated\n%s", output)
	}

	// Make sure the actual file got downloaded.
	expect := filepath.Join(hh, chartname, "charts/reqtest-0.1.0.tgz")
	if _, err := os.Stat(expect); err != nil {
		t.Fatal(err)
	}

	hash, err := provenance.DigestFile(expect)
	if err != nil {
		t.Fatal(err)
	}

	i, err := repo.LoadIndexFile(cacheIndexFile("test"))
	if err != nil {
		t.Fatal(err)
	}

	if h := i.Entries["reqtest-0.1.0"].Digest; h != hash {
		t.Errorf("Failed hash match: expected %s, got %s", hash, h)
	}

	t.Logf("Results: %s", out.String())
}

// newTestingRepositoryServer creates a repository server for testing.
//
// docroot should be a temp dir managed by the caller.
//
// This will start the server, serving files off of the docroot.
//
// Use copyCharts to move charts into the repository and then index them
// for service.
func newTestingRepositoryServer(docroot string) *testingRepositoryServer {
	root, err := filepath.Abs(docroot)
	if err != nil {
		panic(err)
	}
	srv := &testingRepositoryServer{
		docroot: root,
	}
	srv.start()
	// Add the testing repository as the only repo.
	if err := setTestingRepository(docroot, "test", srv.url()); err != nil {
		panic(err)
	}
	return srv
}

type testingRepositoryServer struct {
	docroot string
	srv     *httptest.Server
}

// copyCharts takes a glob expression and copies those charts to the server root.
func (s *testingRepositoryServer) copyCharts(origin string) ([]string, error) {
	files, err := filepath.Glob(origin)
	if err != nil {
		return []string{}, err
	}
	copied := make([]string, len(files))
	for i, f := range files {
		base := filepath.Base(f)
		newname := filepath.Join(s.docroot, base)
		data, err := ioutil.ReadFile(f)
		if err != nil {
			return []string{}, err
		}
		if err := ioutil.WriteFile(newname, data, 0755); err != nil {
			return []string{}, err
		}
		copied[i] = newname
	}

	// generate the index
	index, err := repo.IndexDirectory(s.docroot, s.url())
	if err != nil {
		return copied, err
	}

	d, err := yaml.Marshal(index.Entries)
	if err != nil {
		return copied, err
	}

	ifile := filepath.Join(s.docroot, "index.yaml")
	err = ioutil.WriteFile(ifile, d, 0755)
	return copied, err
}

func (s *testingRepositoryServer) start() {
	s.srv = httptest.NewServer(http.FileServer(http.Dir(s.docroot)))
}

func (s *testingRepositoryServer) stop() {
	s.srv.Close()
}

func (s *testingRepositoryServer) url() string {
	return s.srv.URL
}

// setTestingRepository sets up a testing repository.yaml with only the given name/URL.
func setTestingRepository(helmhome, name, url string) error {
	// Oddly, there is no repo.Save function for this.
	data, err := yaml.Marshal(&map[string]string{name: url})
	if err != nil {
		return err
	}
	os.MkdirAll(filepath.Join(helmhome, "repository", name), 0755)
	dest := filepath.Join(helmhome, "repository/repositories.yaml")
	return ioutil.WriteFile(dest, data, 0666)
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
		},
	}
	data, err := yaml.Marshal(req)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(filepath.Join(dir, "requirements.yaml"), data, 0655)
}
