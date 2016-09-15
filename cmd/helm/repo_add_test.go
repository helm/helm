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
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"k8s.io/helm/pkg/repo"
)

var (
	testName = "test-name"
	testURL  = "test-url"
)

func TestRepoAdd(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprintln(w, "OK")
	}))

	helmHome, _ = ioutil.TempDir("", "helm_home")
	defer os.Remove(helmHome)
	os.Mkdir(filepath.Join(helmHome, repositoryDir), 0755)
	os.Mkdir(cacheDirectory(), 0755)

	if err := ioutil.WriteFile(repositoriesFile(), []byte("example-repo: http://exampleurl.com"), 0666); err != nil {
		t.Errorf("%#v", err)
	}

	if err := addRepository(testName, ts.URL); err != nil {
		t.Errorf("%s", err)
	}

	f, err := repo.LoadRepositoriesFile(repositoriesFile())
	if err != nil {
		t.Errorf("%s", err)
	}
	_, ok := f.Repositories[testName]
	if !ok {
		t.Errorf("%s was not successfully inserted into %s", testName, repositoriesFile())
	}

	if err := insertRepoLine(testName, ts.URL); err == nil {
		t.Errorf("Duplicate repository name was added")
	}

}
