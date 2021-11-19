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

package action

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chartutil"
)

// TestDependencyStatus_Dashes is a regression test to make sure that dashes in
// chart names do not cause resolution problems.
func TestDependencyStatus_Dashes(t *testing.T) {
	// Make a temp dir
	dir, err := ioutil.TempDir("", "helmtest-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	chartpath := filepath.Join(dir, "charts")
	if err := os.MkdirAll(chartpath, 0700); err != nil {
		t.Fatal(err)
	}

	// Add some fake charts
	first := buildChart(withName("first-chart"))
	_, err = chartutil.Save(first, chartpath)
	if err != nil {
		t.Fatal(err)
	}

	second := buildChart(withName("first-chart-second-chart"))
	_, err = chartutil.Save(second, chartpath)
	if err != nil {
		t.Fatal(err)
	}

	dep := &chart.Dependency{
		Name:    "first-chart",
		Version: "0.1.0",
	}

	// Now try to get the deps
	stat := DependencyStatus(dir, dep, first)
	if stat != "ok" {
		t.Errorf("Unexpected status: %q", stat)
	}
}

func TestStatArchiveForStatus(t *testing.T) {
	// Make a temp dir
	dir, err := ioutil.TempDir("", "helmtest-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	chartpath := filepath.Join(dir, "charts")
	if err := os.MkdirAll(chartpath, 0700); err != nil {
		t.Fatal(err)
	}

	// unsaved chart
	lilith := buildChart(withName("lilith"))

	// dep referring to chart
	dep := &chart.Dependency{
		Name:    "lilith",
		Version: "1.2.3",
	}

	is := assert.New(t)

	lilithpath := filepath.Join(chartpath, "lilith-1.2.3.tgz")
	is.Empty(statArchiveForStatus(lilithpath, dep))

	// save the chart (version 0.1.0, because that is the default)
	where, err := chartutil.Save(lilith, chartpath)
	is.NoError(err)

	// Should get "wrong version" because we asked for 1.2.3 and got 0.1.0
	is.Equal("wrong version", statArchiveForStatus(where, dep))

	// Break version on dep
	dep = &chart.Dependency{
		Name:    "lilith",
		Version: "1.2.3.4.5",
	}
	is.Equal("invalid version", statArchiveForStatus(where, dep))

	// Break the name
	dep = &chart.Dependency{
		Name:    "lilith2",
		Version: "1.2.3",
	}
	is.Equal("misnamed", statArchiveForStatus(where, dep))

	// Now create the right version
	dep = &chart.Dependency{
		Name:    "lilith",
		Version: "0.1.0",
	}
	is.Equal("ok", statArchiveForStatus(where, dep))
}
