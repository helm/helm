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
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"helm.sh/helm/v3/internal/test/ensure"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/repo"
	"helm.sh/helm/v3/pkg/repo/repotest"
)

func TestUpdateCmd(t *testing.T) {
	var out bytes.Buffer
	// Instead of using the HTTP updater, we provide our own for this test.
	// The TestUpdateCharts test verifies the HTTP behavior independently.
	updater := func(repos []*repo.ChartRepository, out io.Writer, hideValidationWarnings bool) {
		for _, re := range repos {
			fmt.Fprintln(out, re.Config.Name)
		}
	}
	o := &repoUpdateOptions{
		update:   updater,
		repoFile: "testdata/repositories.yaml",
	}
	if err := o.run(&out, []string{}); err != nil {
		t.Fatal(err)
	}

	if got := out.String(); !strings.Contains(got, "charts") {
		t.Errorf("Expected 'charts' got %q", got)
	}
}

func TestUpdateCustomCacheCmd(t *testing.T) {
	rootDir := ensure.TempDir(t)
	cachePath := filepath.Join(rootDir, "updcustomcache")
	os.Mkdir(cachePath, os.ModePerm)
	defer os.RemoveAll(cachePath)

	ts, err := repotest.NewTempServerWithCleanup(t, "testdata/testserver/*.*")
	if err != nil {
		t.Fatal(err)
	}
	defer ts.Stop()

	o := &repoUpdateOptions{
		update:    updateCharts,
		repoFile:  filepath.Join(ts.Root(), "repositories.yaml"),
		repoCache: cachePath,
	}
	b := ioutil.Discard
	if err := o.run(b, []string{}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(cachePath, "test-index.yaml")); err != nil {
		t.Fatalf("error finding created index file in custom cache: %v", err)
	}
}

func TestUpdateCharts(t *testing.T) {
	defer resetEnv()()
	defer ensure.HelmHome(t)()

	ts, err := repotest.NewTempServerWithCleanup(t, "testdata/testserver/*.*")
	if err != nil {
		t.Fatal(err)
	}
	defer ts.Stop()

	r, err := repo.NewChartRepository(&repo.Entry{
		Name: "charts",
		URL:  ts.URL(),
	}, getter.All(settings))
	if err != nil {
		t.Error(err)
	}

	b := bytes.NewBuffer(nil)
	updateCharts([]*repo.ChartRepository{r}, b, false)

	got := b.String()
	if strings.Contains(got, "Unable to get an update") {
		t.Errorf("Failed to get a repo: %q", got)
	}
	if !strings.Contains(got, "Update Complete.") {
		t.Error("Update was not successful")
	}
}

func TestRepoUpdateFileCompletion(t *testing.T) {
	checkFileCompletion(t, "repo update", false)
}
