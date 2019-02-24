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
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"k8s.io/helm/pkg/chartutil"
	"k8s.io/helm/pkg/helm/helmpath"
	"k8s.io/helm/pkg/proto/hapi/chart"
	"k8s.io/helm/pkg/provenance"
	"k8s.io/helm/pkg/repo"
	"k8s.io/helm/pkg/repo/repotest"
)

func TestDependencyBuildCmd(t *testing.T) {
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
	_, err = srv.CopyCharts("testdata/testcharts/*.tgz")
	if err != nil {
		t.Fatal(err)
	}

	chartname := "depbuild"
	if err := createTestingChart(hh.String(), chartname, srv.URL()); err != nil {
		t.Fatal(err)
	}

	out := bytes.NewBuffer(nil)
	dbc := &dependencyBuildCmd{out: out}
	dbc.helmhome = helmpath.Home(hh)
	dbc.chartpath = filepath.Join(hh.String(), chartname)

	// In the first pass, we basically want the same results as an update.
	if err := dbc.run(); err != nil {
		output := out.String()
		t.Logf("Output: %s", output)
		t.Fatal(err)
	}

	output := out.String()
	if !strings.Contains(output, `update from the "test" chart repository`) {
		t.Errorf("Repo did not get updated\n%s", output)
	}

	// Make sure the actual file got downloaded.
	expect := filepath.Join(hh.String(), chartname, "charts/reqtest-0.1.0.tgz")
	if _, err := os.Stat(expect); err != nil {
		t.Fatal(err)
	}

	// In the second pass, we want to remove the chart's request dependency,
	// then see if it restores from the lock.
	lockfile := filepath.Join(hh.String(), chartname, "requirements.lock")
	if _, err := os.Stat(lockfile); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(expect); err != nil {
		t.Fatal(err)
	}

	if err := dbc.run(); err != nil {
		output := out.String()
		t.Logf("Output: %s", output)
		t.Fatal(err)
	}

	// Now repeat the test that the dependency exists.
	expect = filepath.Join(hh.String(), chartname, "charts/reqtest-0.1.0.tgz")
	if _, err := os.Stat(expect); err != nil {
		t.Fatal(err)
	}

	// Make sure that build is also fetching the correct version.
	hash, err := provenance.DigestFile(expect)
	if err != nil {
		t.Fatal(err)
	}

	i, err := repo.LoadIndexFile(dbc.helmhome.CacheIndex("test"))
	if err != nil {
		t.Fatal(err)
	}

	reqver := i.Entries["reqtest"][0]
	if h := reqver.Digest; h != hash {
		t.Errorf("Failed hash match: expected %s, got %s", hash, h)
	}
	if v := reqver.Version; v != "0.1.0" {
		t.Errorf("mismatched versions. Expected %q, got %q", "0.1.0", v)
	}
}

func TestDependencyRecursiveBuildCmd(t *testing.T) {
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

	chartname := "rectest"
	if err := createTestingChartWithRecursiveDep(hh.String(), chartname, srv); err != nil {
		t.Fatal(err)
	}

	out := bytes.NewBuffer(nil)
	dbc := &dependencyBuildCmd{out: out}
	dbc.helmhome = helmpath.Home(hh)
	dbc.chartpath = filepath.Join(hh.String(), chartname)
	dbc.recursive = true

	if err := dbc.run(); err != nil {
		output := out.String()
		t.Logf("Output: %s", output)
		t.Fatal(err)
	}

	expect := filepath.Join(hh.String(), chartname, "charts/rectest1-0.1.0.tgz")
	if _, err := os.Stat(expect); err != nil {
		t.Fatal(err)
	}

	expect = filepath.Join(hh.String(), chartname, "charts/rectest2-0.1.0.tgz")
	if _, err := os.Stat(expect); err != nil {
		t.Fatal(err)
	}
}

func createTestingChartWithRecursiveDep(dest string, name string, srv *repotest.Server) error {
	// Create the deepest chart without any dependency
	rectestChart2 := "rectest2"
	rectestChart2Version := "0.1.0"
	err := createAndSaveTestingChart(dest, rectestChart2, rectestChart2Version, nil)
	if err != nil {
		return err
	}
	rectestChart1 := "rectest1"
	rectestChart1Version := "0.1.0"
	err = createAndSaveTestingChart(dest, rectestChart1, rectestChart1Version,
		[]*chartutil.Dependency{
			{Name: rectestChart2, Version: rectestChart2Version, Repository: srv.URL()}},
	)
	if err != nil {
		return err
	}
	_, err = srv.CopyCharts(filepath.Join(dest, "/*.tgz"))
	if err != nil {
		return err
	}
	return createAndSaveTestingChart(dest, name, "0.1.0",
		[]*chartutil.Dependency{
			{Name: rectestChart1, Version: rectestChart1Version, Repository: srv.URL()}},
	)
}

func createAndSaveTestingChart(dest string, name, version string, deps []*chartutil.Dependency) error {
	cfile := &chart.Metadata{
		Name:    name,
		Version: version,
	}
	dir := filepath.Join(dest, name)
	_, err := chartutil.Create(cfile, dest)
	if err != nil {
		return err
	}

	if len(deps) > 0 {
		req := &chartutil.Requirements{
			Dependencies: deps,
		}
		err := writeRequirements(dir, req)
		if err != nil {
			return err
		}
	}

	archiveFile := filepath.Join(dest, fmt.Sprintf("%s-%s.tgz", name, version))
	f, err := os.Create(archiveFile)
	if err != nil {
		return err
	}
	defer f.Close()
	zipper := gzip.NewWriter(f)
	defer zipper.Close()
	tarball := tar.NewWriter(zipper)
	defer tarball.Close()
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		header, err := tar.FileInfoHeader(info, info.Name())
		if err != nil {
			return err
		}

		header.Name = filepath.Join(filepath.Base(dir), strings.TrimPrefix(path, dir))
		if err := tarball.WriteHeader(header); err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()
		_, err = io.Copy(tarball, file)
		return err
	})
}
