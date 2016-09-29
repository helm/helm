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
	"testing"

	"k8s.io/helm/cmd/helm/helmpath"
	"k8s.io/helm/pkg/repo"
	"k8s.io/helm/pkg/repo/repotest"
)

var testName = "test-name"

func TestRepoAddCmd(t *testing.T) {
	srv := repotest.NewServer("testdata/testserver")
	defer srv.Stop()

	thome, err := tempHelmHome(t)
	if err != nil {
		t.Fatal(err)
	}
	oldhome := homePath()
	helmHome = thome
	defer func() {
		helmHome = oldhome
		os.Remove(thome)
	}()

	tests := []releaseCase{
		{
			name:     "add a repository",
			args:     []string{testName, srv.URL()},
			expected: testName + " has been added to your repositories",
		},
	}

	for _, tt := range tests {
		buf := bytes.NewBuffer(nil)
		c := newRepoAddCmd(buf)
		if err := c.RunE(c, tt.args); err != nil {
			t.Errorf("%q: expected %q, got %q", tt.name, tt.expected, err)
		}
	}
}

func TestRepoAdd(t *testing.T) {
	ts := repotest.NewServer("testdata/testserver")
	defer ts.Stop()

	thome, err := tempHelmHome(t)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(thome)
	hh := helmpath.Home(thome)

	if err := addRepository(testName, ts.URL(), hh); err != nil {
		t.Error(err)
	}

	f, err := repo.LoadRepositoriesFile(hh.RepositoryFile())
	if err != nil {
		t.Error(err)
	}

	if !f.Has(testName) {
		t.Errorf("%s was not successfully inserted into %s", testName, hh.RepositoryFile())
	}
}
