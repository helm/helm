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
	"strings"
	"testing"

	"helm.sh/helm/internal/test/ensure"
	"helm.sh/helm/pkg/helmpath"
	"helm.sh/helm/pkg/repo"
	"helm.sh/helm/pkg/repo/repotest"
)

func TestRepoRemove(t *testing.T) {
	defer resetEnv()()

	ensure.HelmHome(t)
	defer ensure.CleanHomeDirs(t)

	ts, err := repotest.NewTempServer("testdata/testserver/*.*")
	if err != nil {
		t.Fatal(err)
	}
	defer ts.Stop()

	repoFile := helmpath.RepositoryFile()
	if _, err := os.Stat(repoFile); err != nil {
		rf := repo.NewFile()
		rf.Add(&repo.Entry{
			Name: "charts",
			URL:  "http://example.com/foo",
		})
		if err := rf.WriteFile(repoFile, 0644); err != nil {
			t.Fatal(err)
		}
	}
	if r, err := repo.LoadFile(repoFile); err == repo.ErrRepoOutOfDate {
		if err := r.WriteFile(repoFile, 0644); err != nil {
			t.Fatal(err)
		}
	}

	const testRepoName = "test-name"

	b := bytes.NewBuffer(nil)
	if err := removeRepoLine(b, testRepoName); err == nil {
		t.Errorf("Expected error removing %s, but did not get one.", testRepoName)
	}
	if err := addRepository(testRepoName, ts.URL(), "", "", "", "", "", true); err != nil {
		t.Error(err)
	}

	mf, _ := os.Create(helmpath.CacheIndex(testRepoName))
	mf.Close()

	b.Reset()
	if err := removeRepoLine(b, testRepoName); err != nil {
		t.Errorf("Error removing %s from repositories", testRepoName)
	}
	if !strings.Contains(b.String(), "has been removed") {
		t.Errorf("Unexpected output: %s", b.String())
	}

	if _, err := os.Stat(helmpath.CacheIndex(testRepoName)); err == nil {
		t.Errorf("Error cache file was not removed for repository %s", testRepoName)
	}

	f, err := repo.LoadFile(helmpath.RepositoryFile())
	if err != nil {
		t.Error(err)
	}

	if f.Has(testRepoName) {
		t.Errorf("%s was not successfully removed from repositories list", testRepoName)
	}
}
