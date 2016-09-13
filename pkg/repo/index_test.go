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
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v2"
)

const testfile = "testdata/local-index.yaml"

var (
	testRepo = "test-repo"
)

func TestDownloadIndexFile(t *testing.T) {
	fileBytes, err := ioutil.ReadFile("testdata/local-index.yaml")
	if err != nil {
		t.Errorf("%#v", err)
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "binary/octet-stream")
		fmt.Fprintln(w, string(fileBytes))
	}))

	dirName, err := ioutil.TempDir("", "tmp")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dirName)

	path := filepath.Join(dirName, testRepo+"-index.yaml")
	if err := DownloadIndexFile(testRepo, ts.URL, path); err != nil {
		t.Errorf("%#v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Errorf("error finding created index file: %#v", err)
	}

	b, err := ioutil.ReadFile(path)
	if err != nil {
		t.Errorf("error reading index file: %#v", err)
	}

	var i IndexFile
	if err = yaml.Unmarshal(b, &i); err != nil {
		t.Errorf("error unmarshaling index file: %#v", err)
	}

	numEntries := len(i.Entries)
	if numEntries != 2 {
		t.Errorf("Expected 2 entries in index file but got %v", numEntries)
	}
	os.Remove(path)
}

func TestLoadIndexFile(t *testing.T) {
	cf, err := LoadIndexFile(testfile)
	if err != nil {
		t.Errorf("Failed to load index file: %s", err)
	}
	if len(cf.Entries) != 2 {
		t.Errorf("Expected 2 entries in the index file, but got %d", len(cf.Entries))
	}
	nginx := false
	alpine := false
	for k, e := range cf.Entries {
		if k == "nginx-0.1.0" {
			if e.Name == "nginx" {
				if len(e.Chartfile.Keywords) == 3 {
					nginx = true
				}
			}
		}
		if k == "alpine-1.0.0" {
			if e.Name == "alpine" {
				if len(e.Chartfile.Keywords) == 4 {
					alpine = true
				}
			}
		}
	}
	if !nginx {
		t.Errorf("nginx entry was not decoded properly")
	}
	if !alpine {
		t.Errorf("alpine entry was not decoded properly")
	}
}

func TestIndexDirectory(t *testing.T) {
	dir := "testdata/repository"
	index, err := IndexDirectory(dir, "http://localhost:8080")
	if err != nil {
		t.Fatal(err)
	}

	if l := len(index.Entries); l != 2 {
		t.Fatalf("Expected 2 entries, got %d", l)
	}

	// Other things test the entry generation more thoroughly. We just test a
	// few fields.
	cname := "frobnitz-1.2.3"
	frob, ok := index.Entries[cname]
	if !ok {
		t.Fatalf("Could not read chart %s", cname)
	}
	if len(frob.Digest) == 0 {
		t.Errorf("Missing digest of file %s.", frob.Name)
	}
	if frob.Chartfile == nil {
		t.Fatalf("Chartfile %s not added to index.", cname)
	}
	if frob.URL != "http://localhost:8080/frobnitz-1.2.3.tgz" {
		t.Errorf("Unexpected URL: %s", frob.URL)
	}
	if frob.Chartfile.Name != "frobnitz" {
		t.Errorf("Expected frobnitz, got %q", frob.Chartfile.Name)
	}
}
