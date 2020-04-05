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
	"os"
	"path/filepath"
	"strings"
	"testing"

	"helm.sh/helm/v3/internal/test/ensure"
	"helm.sh/helm/v3/pkg/helmpath"
	"helm.sh/helm/v3/pkg/repo"
	"helm.sh/helm/v3/pkg/repo/repotest"
)

func TestRepoRemove(t *testing.T) {
	ts, err := repotest.NewTempServer("testdata/testserver/*.*")
	if err != nil {
		t.Fatal(err)
	}
	defer ts.Stop()

	rootDir := ensure.TempDir(t)
	repoFile := filepath.Join(rootDir, "repositories.yaml")

	const testRepoName = "test-name"

	b := bytes.NewBuffer(nil)

	rmOpts := repoRemoveOptions{
		names:     []string{testRepoName},
		repoFile:  repoFile,
		repoCache: rootDir,
	}

	if err := rmOpts.run(os.Stderr); err == nil {
		t.Errorf("Expected error removing %s, but did not get one.", testRepoName)
	}
	o := &repoAddOptions{
		name:     testRepoName,
		url:      ts.URL(),
		repoFile: repoFile,
	}

	if err := o.run(os.Stderr); err != nil {
		t.Error(err)
	}

	idx, idx2 := indexing(b, rootDir, testRepoName)

	if err := rmOpts.run(b); err != nil {
		t.Errorf("Error removing %s from repositories", testRepoName)
	}
	if !strings.Contains(b.String(), "has been removed") {
		t.Errorf("Unexpected output: %s", b.String())
	}

	testIndices(t, idx, idx2, testRepoName)

	f, err := repo.LoadFile(repoFile)
	if err != nil {
		t.Error(err)
	}

	if f.Has(testRepoName) {
		t.Errorf("%s was not successfully removed from repositories list", testRepoName)
	}
}

func indexing(buf *bytes.Buffer, rootDir string, repoName string) (cacheIndexFile string, cacheChartsFile string) {
	idx := filepath.Join(rootDir, helmpath.CacheIndexFile(repoName))
	mf, _ := os.Create(idx)
	mf.Close()

	idx2 := filepath.Join(rootDir, helmpath.CacheChartsFile(repoName))
	mf, _ = os.Create(idx2)
	mf.Close()

	buf.Reset()

	return idx, idx2
}

func testIndices(t *testing.T, cacheIndexFile string, cacheChartsFile string, repoName string) {
	if _, err := os.Stat(cacheIndexFile); err == nil {
		t.Errorf("Error cache index file was not removed for repository %s", repoName)
	}
	if _, err := os.Stat(cacheChartsFile); err == nil {
		t.Errorf("Error cache chart file was not removed for repository %s", repoName)
	}
}
