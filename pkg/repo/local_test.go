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

package repo

import (
	"io/ioutil"
	"net/http"
	"strings"
	"testing"
)

func TestRepositoryServer(t *testing.T) {
	expectedIndexYAML, err := ioutil.ReadFile("testdata/server/index.yaml")
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name   string
		path   string
		expect string
	}{
		{"index YAML", "/charts/index.yaml", string(expectedIndexYAML)},
		{"index HTML", "/charts/index.html", "<html>"},
		{"charts root", "/charts/", "<html>"},
		{"root", "/", "<html>"},
		{"file", "/test.txt", "Hello World"},
	}

	s := &RepositoryServer{RepoPath: "testdata/server"}
	srv, err := startLocalServerForTests(s)
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()

	for _, tt := range tests {
		res, err := http.Get(srv.URL + tt.path)
		if err != nil {
			t.Errorf("%s: error getting %s: %s", tt.name, tt.path, err)
			continue
		}
		body, err := ioutil.ReadAll(res.Body)
		if err != nil {
			t.Errorf("%s: error reading %s: %s", tt.name, tt.path, err)
		}
		res.Body.Close()
		if !strings.Contains(string(body), tt.expect) {
			t.Errorf("%s: expected to find %q in %q", tt.name, tt.expect, string(body))
		}
	}

}
