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

package repotest

import (
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/ghodss/yaml"

	"k8s.io/helm/pkg/repo"
)

// Young'n, in these here parts, we test our tests.

func TestServer(t *testing.T) {
	docroot, err := ioutil.TempDir("", "helm-repotest-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(docroot)

	srv := NewServer(docroot)
	defer srv.Stop()

	c, err := srv.CopyCharts("testdata/*.tgz")
	if err != nil {
		// Some versions of Go don't correctly fire defer on Fatal.
		t.Error(err)
		return
	}

	if len(c) != 1 {
		t.Errorf("Unexpected chart count: %d", len(c))
	}

	if filepath.Base(c[0]) != "examplechart-0.1.0.tgz" {
		t.Errorf("Unexpected chart: %s", c[0])
	}

	res, err := http.Get(srv.URL() + "/examplechart-0.1.0.tgz")
	if err != nil {
		t.Error(err)
		return
	}

	if res.ContentLength < 500 {
		t.Errorf("Expected at least 500 bytes of data, got %d", res.ContentLength)
	}

	res, err = http.Get(srv.URL() + "/index.yaml")
	if err != nil {
		t.Error(err)
		return
	}

	data, err := ioutil.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		t.Error(err)
		return
	}

	m := repo.NewIndexFile()
	if err := yaml.Unmarshal(data, m); err != nil {
		t.Error(err)
		return
	}

	if l := len(m.Entries); l != 1 {
		t.Errorf("Expected 1 entry, got %d", l)
		return
	}

	expect := "examplechart"
	if !m.Has(expect, "0.1.0") {
		t.Errorf("missing %q", expect)
	}

	res, err = http.Get(srv.URL() + "/index.yaml-nosuchthing")
	if err != nil {
		t.Error(err)
		return
	}
	if res.StatusCode != 404 {
		t.Errorf("Expected 404, got %d", res.StatusCode)
	}
}

func TestNewTempServer(t *testing.T) {
	srv, tdir, err := NewTempServer("testdata/examplechart-0.1.0.tgz")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		srv.Stop()
		os.RemoveAll(tdir.String())
	}()

	if _, err := os.Stat(tdir.String()); err != nil {
		t.Fatal(err)
	}

	res, err := http.Head(srv.URL() + "/examplechart-0.1.0.tgz")
	if err != nil {
		t.Error(err)
	}
	if res.StatusCode != 200 {
		t.Errorf("Expected 200, got %d", res.StatusCode)
	}
}
