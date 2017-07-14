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
	"os"
	"path/filepath"
	"strings"
	"testing"

	"k8s.io/helm/pkg/helm/helmpath"
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
