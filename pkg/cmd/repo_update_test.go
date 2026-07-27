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

package cmd

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"helm.sh/helm/v4/internal/test/ensure"
	"helm.sh/helm/v4/pkg/getter"
	"helm.sh/helm/v4/pkg/repo/v1"
	"helm.sh/helm/v4/pkg/repo/v1/repotest"
)

func TestUpdateCmd(t *testing.T) {
	var out bytes.Buffer
	// Instead of using the HTTP updater, we provide our own for this test.
	// The TestUpdateCharts test verifies the HTTP behavior independently.
	updater := func(repos []*repo.ChartRepository, out io.Writer) error {
		for _, re := range repos {
			fmt.Fprintln(out, re.Config.Name)
		}
		return nil
	}
	o := &repoUpdateOptions{
		update:   updater,
		repoFile: "testdata/repositories.yaml",
	}
	require.NoError(t, o.run(&out))

	if got := out.String(); !strings.Contains(got, "charts") ||
		!strings.Contains(got, "firstexample") ||
		!strings.Contains(got, "secondexample") {
		t.Errorf("Expected 'charts', 'firstexample' and 'secondexample' but got %q", got)
	}
}

func TestUpdateCmdMultiple(t *testing.T) {
	var out bytes.Buffer
	// Instead of using the HTTP updater, we provide our own for this test.
	// The TestUpdateCharts test verifies the HTTP behavior independently.
	updater := func(repos []*repo.ChartRepository, out io.Writer) error {
		for _, re := range repos {
			fmt.Fprintln(out, re.Config.Name)
		}
		return nil
	}
	o := &repoUpdateOptions{
		update:   updater,
		repoFile: "testdata/repositories.yaml",
		names:    []string{"firstexample", "charts"},
	}
	require.NoError(t, o.run(&out))

	if got := out.String(); !strings.Contains(got, "charts") ||
		!strings.Contains(got, "firstexample") ||
		strings.Contains(got, "secondexample") {
		t.Errorf("Expected 'charts' and 'firstexample' but not 'secondexample' but got %q", got)
	}
}

func TestUpdateCmdInvalid(t *testing.T) {
	var out bytes.Buffer
	// Instead of using the HTTP updater, we provide our own for this test.
	// The TestUpdateCharts test verifies the HTTP behavior independently.
	updater := func(repos []*repo.ChartRepository, out io.Writer) error {
		for _, re := range repos {
			fmt.Fprintln(out, re.Config.Name)
		}
		return nil
	}
	o := &repoUpdateOptions{
		update:   updater,
		repoFile: "testdata/repositories.yaml",
		names:    []string{"firstexample", "invalid"},
	}
	require.Error(t, o.run(&out), "expected error but did not get one")
}

func TestUpdateCustomCacheCmd(t *testing.T) {
	rootDir := t.TempDir()
	cachePath := filepath.Join(rootDir, "updcustomcache")
	os.Mkdir(cachePath, os.ModePerm)

	ts := repotest.NewTempServer(
		t,
		repotest.WithChartSourceGlob("testdata/testserver/*.*"),
	)

	defer ts.Stop()

	o := &repoUpdateOptions{
		update:    updateCharts,
		repoFile:  filepath.Join(ts.Root(), "repositories.yaml"),
		repoCache: cachePath,
	}
	b := io.Discard
	require.NoError(t, o.run(b))
	_, err := os.Stat(filepath.Join(cachePath, "test-index.yaml"))
	require.NoErrorf(t, err, "error finding created index file in custom cache")
}

func TestUpdateCharts(t *testing.T) {
	defer resetEnv()()
	ensure.HelmHome(t)

	ts := repotest.NewTempServer(t,
		repotest.WithChartSourceGlob("testdata/testserver/*.*"),
	)
	defer ts.Stop()

	r, err := repo.NewChartRepository(&repo.Entry{
		Name: "charts",
		URL:  ts.URL(),
	}, getter.All(settings))
	require.NoError(t, err)

	b := bytes.NewBuffer(nil)
	updateCharts([]*repo.ChartRepository{r}, b)

	got := b.String()
	assert.NotContains(t, got, "Unable to get an update", "Failed to get a repo: %q", got)
	assert.Contains(t, got, "Update Complete.", "Update was not successful")
}

func TestRepoUpdateFileCompletion(t *testing.T) {
	checkFileCompletion(t, "repo update", false)
	checkFileCompletion(t, "repo update repo1", false)
}

func TestUpdateChartsFailWithError(t *testing.T) {
	defer resetEnv()()
	ensure.HelmHome(t)

	ts := repotest.NewTempServer(
		t,
		repotest.WithChartSourceGlob("testdata/testserver/*.*"),
	)
	defer ts.Stop()

	var invalidURL = ts.URL() + "55"
	r1, err := repo.NewChartRepository(&repo.Entry{
		Name: "charts",
		URL:  invalidURL,
	}, getter.All(settings))
	require.NoError(t, err)
	r2, err := repo.NewChartRepository(&repo.Entry{
		Name: "charts",
		URL:  invalidURL,
	}, getter.All(settings))
	require.NoError(t, err)

	b := bytes.NewBuffer(nil)
	err = updateCharts([]*repo.ChartRepository{r1, r2}, b)
	require.Error(t, err, "Repo update should return error because update of repository fails and 'fail-on-repo-update-fail' flag set")
	require.ErrorContains(t, err, "failed to update the following repositories")
	require.ErrorContains(t, err, invalidURL)

	got := b.String()
	assert.Contains(t, got, "Unable to get an update", "Repo should have failed update but instead got: %q", got)
	assert.NotContains(t, got, "Update Complete.", "Update was not successful and should return error message because 'fail-on-repo-update-fail' flag set")
}
