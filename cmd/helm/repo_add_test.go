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
	"fmt"
	"os"
	"testing"

	"k8s.io/helm/pkg/repo"
	"k8s.io/helm/pkg/repo/repotest"
)

func TestRepoAddCmd(t *testing.T) {
	srv, thome, err := repotest.NewTempServer("testdata/testserver/*.*")
	if err != nil {
		t.Fatal(err)
	}

	cleanup := resetEnv()
	defer func() {
		srv.Stop()
		os.RemoveAll(thome.String())
		cleanup()
	}()
	if err := ensureTestHome(t, thome); err != nil {
		t.Fatal(err)
	}

	settings.Home = thome

	tests := []releaseCase{{
		name:   "add a repository",
		cmd:    fmt.Sprintf("repo add test-name %s --home %s", srv.URL(), thome),
		golden: "output/repo-add.txt",
	}}

	testReleaseCmd(t, tests)
}

func TestRepoAdd(t *testing.T) {
	ts, thome, err := repotest.NewTempServer("testdata/testserver/*.*")
	if err != nil {
		t.Fatal(err)
	}

	cleanup := resetEnv()
	hh := thome
	defer func() {
		ts.Stop()
		os.RemoveAll(thome.String())
		cleanup()
	}()
	if err := ensureTestHome(t, hh); err != nil {
		t.Fatal(err)
	}

	settings.Home = thome

	const testRepoName = "test-name"

	if err := addRepository(testRepoName, ts.URL(), "", "", hh, "", "", "", true); err != nil {
		t.Error(err)
	}

	f, err := repo.LoadRepositoriesFile(hh.RepositoryFile())
	if err != nil {
		t.Error(err)
	}

	if !f.Has(testRepoName) {
		t.Errorf("%s was not successfully inserted into %s", testRepoName, hh.RepositoryFile())
	}

	if err := addRepository(testRepoName, ts.URL(), "", "", hh, "", "", "", false); err != nil {
		t.Errorf("Repository was not updated: %s", err)
	}

	if err := addRepository(testRepoName, ts.URL(), "", "", hh, "", "", "", false); err != nil {
		t.Errorf("Duplicate repository name was added")
	}
}
