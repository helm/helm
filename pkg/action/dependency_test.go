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
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"helm.sh/helm/v4/internal/test"
	chart "helm.sh/helm/v4/pkg/chart/v2"
	chartutil "helm.sh/helm/v4/pkg/chart/v2/util"
)

func TestList(t *testing.T) {
	for _, tcase := range []struct {
		chart  string
		golden string
	}{
		{
			chart:  "testdata/charts/chart-with-compressed-dependencies",
			golden: "output/list-compressed-deps.txt",
		},
		{
			chart:  "testdata/charts/chart-with-compressed-dependencies-2.1.8.tgz",
			golden: "output/list-compressed-deps-tgz.txt",
		},
		{
			chart:  "testdata/charts/chart-with-uncompressed-dependencies",
			golden: "output/list-uncompressed-deps.txt",
		},
		{
			chart:  "testdata/charts/chart-with-uncompressed-dependencies-2.1.8.tgz",
			golden: "output/list-uncompressed-deps-tgz.txt",
		},
		{
			chart:  "testdata/charts/chart-missing-deps",
			golden: "output/list-missing-deps.txt",
		},
	} {
		buf := bytes.Buffer{}
		require.NoError(t, NewDependency().List(tcase.chart, &buf))
		test.AssertGoldenString(t, buf.String(), tcase.golden)
	}
}

// TestDependencyStatus_Dashes is a regression test to make sure that dashes in
// chart names do not cause resolution problems.
func TestDependencyStatus_Dashes(t *testing.T) {
	// Make a temp dir
	dir := t.TempDir()

	chartpath := filepath.Join(dir, "charts")
	require.NoError(t, os.MkdirAll(chartpath, 0o700))

	// Add some fake charts
	first := buildChart(withName("first-chart"))
	_, err := chartutil.Save(first, chartpath)
	require.NoError(t, err)

	second := buildChart(withName("first-chart-second-chart"))
	_, err = chartutil.Save(second, chartpath)
	require.NoError(t, err)

	dep := &chart.Dependency{
		Name:    "first-chart",
		Version: "0.1.0",
	}

	// Now try to get the deps
	stat := NewDependency().dependencyStatus(dir, dep, first)
	assert.Equal(t, "ok", stat, "Unexpected status: %q", stat)
}

func TestStatArchiveForStatus(t *testing.T) {
	// Make a temp dir
	dir := t.TempDir()

	chartpath := filepath.Join(dir, "charts")
	require.NoError(t, os.MkdirAll(chartpath, 0o700))

	// unsaved chart
	lilith := buildChart(withName("lilith"))

	// dep referring to chart
	dep := &chart.Dependency{
		Name:    "lilith",
		Version: "1.2.3",
	}

	is := assert.New(t)
	req := require.New(t)

	lilithpath := filepath.Join(chartpath, "lilith-1.2.3.tgz")
	is.Empty(statArchiveForStatus(lilithpath, dep))

	// save the chart (version 0.1.0, because that is the default)
	where, err := chartutil.Save(lilith, chartpath)
	req.NoError(err)

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

// TestPrintMissingBrokenSymlink ensures that a dangling symlink in a chart's
// charts/ directory is reported as a warning and skipped, rather than causing
// a nil-pointer dereference. os.Stat follows symlinks and returns a nil
// FileInfo for a broken link, so the subsequent fi.IsDir() call used to panic.
func TestPrintMissingBrokenSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("creating symlinks is not reliably permitted on Windows")
	}

	chartpath := t.TempDir()
	chartsDir := filepath.Join(chartpath, "charts")
	if err := os.MkdirAll(chartsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// A symlink whose target does not exist. filepath.Glob returns it, but
	// os.Stat (which follows the link) fails.
	if err := os.Symlink(filepath.Join(chartpath, "does-not-exist"), filepath.Join(chartsDir, "broken")); err != nil {
		t.Fatal(err)
	}

	buf := bytes.Buffer{}
	d := NewDependency()
	assert.NotPanics(t, func() {
		d.printMissing(chartpath, &buf, []*chart.Dependency{})
	})
	assert.Contains(t, buf.String(), "Warning")
}
