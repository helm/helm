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
	"fmt"
	"os"
	"testing"

	"helm.sh/helm/internal/test/ensure"
	"helm.sh/helm/pkg/helmpath"
	"helm.sh/helm/pkg/repo"
	"helm.sh/helm/pkg/repo/repotest"
)

func TestRepoAddCmd(t *testing.T) {
	defer resetEnv()()

	ensure.HelmHome(t)
	defer ensure.CleanHomeDirs(t)

	srv, err := repotest.NewTempServer("testdata/testserver/*.*")
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Stop()

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

	tests := []cmdTestCase{{
		name:   "add a repository",
		cmd:    fmt.Sprintf("repo add test-name %s", srv.URL()),
		golden: "output/repo-add.txt",
	}}

	runTestCmd(t, tests)
}

func TestRepoAdd(t *testing.T) {
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

	if err := addRepository(testRepoName, ts.URL(), "", "", "", "", "", true); err != nil {
		t.Error(err)
	}

	f, err := repo.LoadFile(helmpath.RepositoryFile())
	if err != nil {
		t.Error(err)
	}

	if !f.Has(testRepoName) {
		t.Errorf("%s was not successfully inserted into %s", testRepoName, helmpath.RepositoryFile())
	}

	if err := addRepository(testRepoName, ts.URL(), "", "", "", "", "", false); err != nil {
		t.Errorf("Repository was not updated: %s", err)
	}

	if err := addRepository(testRepoName, ts.URL(), "", "", "", "", "", false); err != nil {
		t.Errorf("Duplicate repository name was added")
	}
}
