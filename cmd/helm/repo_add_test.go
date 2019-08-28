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
	"io/ioutil"
	"path/filepath"
	"testing"

	"helm.sh/helm/internal/test/ensure"
	"helm.sh/helm/pkg/repo"
	"helm.sh/helm/pkg/repo/repotest"
)

func TestRepoAddCmd(t *testing.T) {
	srv, err := repotest.NewTempServer("testdata/testserver/*.*")
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Stop()

	repoFile := filepath.Join(ensure.TempDir(t), "repositories.yaml")

	tests := []cmdTestCase{{
		name:   "add a repository",
		cmd:    fmt.Sprintf("repo add test-name %s --repository-config %s", srv.URL(), repoFile),
		golden: "output/repo-add.txt",
	}}

	runTestCmd(t, tests)
}

func TestRepoAdd(t *testing.T) {
	ts, err := repotest.NewTempServer("testdata/testserver/*.*")
	if err != nil {
		t.Fatal(err)
	}
	defer ts.Stop()

	repoFile := filepath.Join(ensure.TempDir(t), "repositories.yaml")

	const testRepoName = "test-name"

	o := &repoAddOptions{
		name:     testRepoName,
		url:      ts.URL(),
		noUpdate: true,
		repoFile: repoFile,
	}

	if err := o.run(ioutil.Discard); err != nil {
		t.Error(err)
	}

	f, err := repo.LoadFile(repoFile)
	if err != nil {
		t.Fatal(err)
	}

	if !f.Has(testRepoName) {
		t.Errorf("%s was not successfully inserted into %s", testRepoName, repoFile)
	}

	o.noUpdate = false

	if err := o.run(ioutil.Discard); err != nil {
		t.Errorf("Repository was not updated: %s", err)
	}

	if err := o.run(ioutil.Discard); err != nil {
		t.Errorf("Duplicate repository name was added")
	}
}
