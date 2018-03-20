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
	"strings"
	"testing"

	"k8s.io/helm/pkg/helm/helmpath"
	"k8s.io/helm/pkg/repo"
	"k8s.io/helm/pkg/repo/repotest"
)

func TestRepoRemove(t *testing.T) {
	ts, thome, err := repotest.NewTempServer("testdata/testserver/*.*")
	if err != nil {
		t.Fatal(err)
	}

	hh := helmpath.Home(thome)
	cleanup := resetEnv()
	defer func() {
		ts.Stop()
		os.RemoveAll(thome.String())
		cleanup()
	}()
	if err := ensureTestHome(hh, t); err != nil {
		t.Fatal(err)
	}

	settings.Home = thome

	b := bytes.NewBuffer(nil)

	if err := removeRepoLine(b, testName, hh); err == nil {
		t.Errorf("Expected error removing %s, but did not get one.", testName)
	}
	if err := addRepository(testName, ts.URL(), "", "", hh, "", "", "", true); err != nil {
		t.Error(err)
	}

	mf, _ := os.Create(hh.CacheIndex(testName))
	mf.Close()

	b.Reset()
	if err := removeRepoLine(b, testName, hh); err != nil {
		t.Errorf("Error removing %s from repositories", testName)
	}
	if !strings.Contains(b.String(), "has been removed") {
		t.Errorf("Unexpected output: %s", b.String())
	}

	if _, err := os.Stat(hh.CacheIndex(testName)); err == nil {
		t.Errorf("Error cache file was not removed for repository %s", testName)
	}

	f, err := repo.LoadRepositoriesFile(hh.RepositoryFile())
	if err != nil {
		t.Error(err)
	}

	if f.Has(testName) {
		t.Errorf("%s was not successfully removed from repositories list", testName)
	}
}
