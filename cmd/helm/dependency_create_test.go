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
	"reflect"
	"strings"
	"testing"

	"k8s.io/helm/cmd/helm/helmpath"
	"k8s.io/helm/pkg/chartutil"
	"k8s.io/helm/pkg/proto/hapi/chart"
)

func TestDependencyCreateCmd(t *testing.T) {
	// Set up a testing helm home
	oldhome := helmHome
	hh, err := tempHelmHome(t)
	if err != nil {
		t.Fatal(err)
	}
	helmHome = hh
	defer func() {
		os.RemoveAll(hh)
		helmHome = oldhome
	}()

	chartname := "depcreate"
	if err := createChartWithoutDeps(hh, chartname); err != nil {
		t.Fatal(err)
	}

	out := bytes.NewBuffer(nil)
	dcc := &dependencyCreateCmd{out: out}
	dcc.name = "dep1"
	dcc.repository = "repo1"
	dcc.helmhome = helmpath.Home(hh)
	dcc.chartpath = filepath.Join(hh, chartname)
	dcc.version = "0.1.0"

	doesNotExist := filepath.Join(hh, chartname, "requirements.yaml")
	if _, err := os.Stat(doesNotExist); err == nil {
		t.Fatalf("Unexpected %q", doesNotExist)
	}

	// no requirements.yaml exists
	if err := dcc.run(); err != nil {
		output := out.String()
		t.Logf("Output: %s", output)
		t.Fatal(err)
	}

	output := out.String()
	if !strings.Contains(output, `charts, creating new requirements file`) {
		t.Errorf("New requirements.yaml did not get created")
	}
	expect := filepath.Join(hh, chartname, "requirements.yaml")
	if _, err := os.Stat(expect); err != nil {
		t.Fatal(err)
	}

	// add a dependency to an existing requirements.yaml
	dcc.name = "dep2"
	dcc.repository = "repo2"
	dcc.version = "1.0.0"

	if err := dcc.run(); err != nil {
		output := out.String()
		t.Logf("Output: %s", output)
		t.Fatal(err)
	}

	c, err := chartutil.LoadDir(dcc.chartpath)
	if err != nil {
		t.Fatal(err)
	}

	reqs, err := chartutil.LoadRequirements(c)
	if err != nil {
		t.Fatal(err)
	}

	// compare
	expectedReqCount := 2
	if len(reqs.Dependencies) != expectedReqCount {
		t.Errorf("Expected %d total requirements, actual count: %d", expectedReqCount, len(reqs.Dependencies))
	}

	expectedDeps := []chartutil.Dependency{
		{Name: "dep1", Version: "0.1.0", Repository: "repo1"},
		{Name: "dep2", Version: "1.0.0", Repository: "repo2"},
	}

	for i := 0; i < len(reqs.Dependencies); i++ {
		if !reflect.DeepEqual(expectedDeps[i], *reqs.Dependencies[i]) {
			t.Errorf("Expected deps: %+v\n Actual deps: %+v\n", expectedDeps[i], *reqs.Dependencies[i])
		}
	}

	// dep already exists
	dcc.name = "dep2"
	dcc.repository = "modifiedrepo"
	dcc.version = "1.0.1"

	if err := dcc.run(); err != nil {
		output := out.String()
		t.Logf("Output: %s", output)
		t.Fatal(err)
	}

	c, err = chartutil.LoadDir(dcc.chartpath)
	if err != nil {
		t.Fatal(err)
	}

	reqs, err = chartutil.LoadRequirements(c)
	if err != nil {
		t.Fatal(err)
	}

	expectedDeps[1] = chartutil.Dependency{
		Name: "dep2", Version: "1.0.1", Repository: "modifiedrepo",
	}

	for i := 0; i < len(reqs.Dependencies); i++ {
		if !reflect.DeepEqual(expectedDeps[i], *reqs.Dependencies[i]) {
			t.Errorf("Expected deps: %+v\n Actual deps: %+v\n", expectedDeps[i], *reqs.Dependencies[i])
		}
	}
}

func createChartWithoutDeps(dest, name string) error {
	cfile := &chart.Metadata{
		Name:    name,
		Version: "1.2.3",
	}
	_, err := chartutil.Create(cfile, dest)

	return err
}
