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
	"os"
	"path/filepath"
	"testing"

	"k8s.io/helm/pkg/getter"
	"k8s.io/helm/pkg/helm/environment"
	"k8s.io/helm/pkg/proto/hapi/chart"
)

const (
	testfile          = "testdata/local-index.yaml"
	unorderedTestfile = "testdata/local-index-unordered.yaml"
	testRepo          = "test-repo"
)

func TestIndexFile(t *testing.T) {
	i := NewIndexFile()
	i.Add(&chart.Metadata{Name: "clipper", Version: "0.1.0"}, "clipper-0.1.0.tgz", "http://example.com/charts", "sha256:1234567890")
	i.Add(&chart.Metadata{Name: "cutter", Version: "0.1.1"}, "cutter-0.1.1.tgz", "http://example.com/charts", "sha256:1234567890abc")
	i.Add(&chart.Metadata{Name: "cutter", Version: "0.1.0"}, "cutter-0.1.0.tgz", "http://example.com/charts", "sha256:1234567890abc")
	i.Add(&chart.Metadata{Name: "cutter", Version: "0.2.0"}, "cutter-0.2.0.tgz", "http://example.com/charts", "sha256:1234567890abc")
	i.SortEntries()

	if i.APIVersion != APIVersionV1 {
		t.Error("Expected API version v1")
	}

	if len(i.Entries) != 2 {
		t.Errorf("Expected 2 charts. Got %d", len(i.Entries))
	}

	if i.Entries["clipper"][0].Name != "clipper" {
		t.Errorf("Expected clipper, got %s", i.Entries["clipper"][0].Name)
	}

	if len(i.Entries["cutter"]) != 3 {
		t.Error("Expected two cutters.")
	}

	// Test that the sort worked. 0.2 should be at the first index for Cutter.
	if v := i.Entries["cutter"][0].Version; v != "0.2.0" {
		t.Errorf("Unexpected first version: %s", v)
	}
}

func TestLoadIndex(t *testing.T) {
	b, err := ioutil.ReadFile(testfile)
	if err != nil {
		t.Fatal(err)
	}
	i, err := loadIndex(b)
	if err != nil {
		t.Fatal(err)
	}
	verifyLocalIndex(t, i)
}

func TestLoadIndexFile(t *testing.T) {
	i, err := LoadIndexFile(testfile)
	if err != nil {
		t.Fatal(err)
	}
	verifyLocalIndex(t, i)
}

func TestLoadUnorderedIndex(t *testing.T) {
	b, err := ioutil.ReadFile(unorderedTestfile)
	if err != nil {
		t.Fatal(err)
	}
	i, err := loadIndex(b)
	if err != nil {
		t.Fatal(err)
	}
	verifyLocalIndex(t, i)
}

func TestMerge(t *testing.T) {
	ind1 := NewIndexFile()
	ind1.Add(&chart.Metadata{
		Name:    "dreadnought",
		Version: "0.1.0",
	}, "dreadnought-0.1.0.tgz", "http://example.com", "aaaa")

	ind2 := NewIndexFile()
	ind2.Add(&chart.Metadata{
		Name:    "dreadnought",
		Version: "0.2.0",
	}, "dreadnought-0.2.0.tgz", "http://example.com", "aaaabbbb")
	ind2.Add(&chart.Metadata{
		Name:    "doughnut",
		Version: "0.2.0",
	}, "doughnut-0.2.0.tgz", "http://example.com", "ccccbbbb")

	ind1.Merge(ind2)

	if len(ind1.Entries) != 2 {
		t.Errorf("Expected 2 entries, got %d", len(ind1.Entries))
		vs := ind1.Entries["dreadnaught"]
		if len(vs) != 2 {
			t.Errorf("Expected 2 versions, got %d", len(vs))
		}
		v := vs[0]
		if v.Version != "0.2.0" {
			t.Errorf("Expected %q version to be 0.2.0, got %s", v.Name, v.Version)
		}
	}

}

func TestDownloadIndexFile(t *testing.T) {
	srv, err := startLocalServerForTests(nil)
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()

	dirName, err := ioutil.TempDir("", "tmp")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dirName)

	indexFilePath := filepath.Join(dirName, testRepo+"-index.yaml")
	r, err := NewChartRepository(&Entry{
		Name:  testRepo,
		URL:   srv.URL,
		Cache: indexFilePath,
	}, getter.All(environment.EnvSettings{}))
	if err != nil {
		t.Errorf("Problem creating chart repository from %s: %v", testRepo, err)
	}

	if err := r.DownloadIndexFile(""); err != nil {
		t.Errorf("%#v", err)
	}

	if _, err := os.Stat(indexFilePath); err != nil {
		t.Errorf("error finding created index file: %#v", err)
	}

	b, err := ioutil.ReadFile(indexFilePath)
	if err != nil {
		t.Errorf("error reading index file: %#v", err)
	}

	i, err := loadIndex(b)
	if err != nil {
		t.Errorf("Index %q failed to parse: %s", testfile, err)
		return
	}

	verifyLocalIndex(t, i)
}

func verifyLocalIndex(t *testing.T, i *IndexFile) {
	numEntries := len(i.Entries)
	if numEntries != 3 {
		t.Errorf("Expected 3 entries in index file but got %d", numEntries)
	}

	alpine, ok := i.Entries["alpine"]
	if !ok {
		t.Errorf("'alpine' section not found.")
		return
	}

	if l := len(alpine); l != 1 {
		t.Errorf("'alpine' should have 1 chart, got %d", l)
		return
	}

	nginx, ok := i.Entries["nginx"]
	if !ok || len(nginx) != 2 {
		t.Error("Expected 2 nginx entries")
		return
	}

	expects := []*ChartVersion{
		{
			Metadata: &chart.Metadata{
				Name:        "alpine",
				Description: "string",
				Version:     "1.0.0",
				Keywords:    []string{"linux", "alpine", "small", "sumtin"},
				Home:        "https://github.com/something",
			},
			URLs: []string{
				"https://kubernetes-charts.storage.googleapis.com/alpine-1.0.0.tgz",
				"http://storage2.googleapis.com/kubernetes-charts/alpine-1.0.0.tgz",
			},
			Digest: "sha256:1234567890abcdef",
		},
		{
			Metadata: &chart.Metadata{
				Name:        "nginx",
				Description: "string",
				Version:     "0.2.0",
				Keywords:    []string{"popular", "web server", "proxy"},
				Home:        "https://github.com/something/else",
			},
			URLs: []string{
				"https://kubernetes-charts.storage.googleapis.com/nginx-0.2.0.tgz",
			},
			Digest: "sha256:1234567890abcdef",
		},
		{
			Metadata: &chart.Metadata{
				Name:        "nginx",
				Description: "string",
				Version:     "0.1.0",
				Keywords:    []string{"popular", "web server", "proxy"},
				Home:        "https://github.com/something",
			},
			URLs: []string{
				"https://kubernetes-charts.storage.googleapis.com/nginx-0.1.0.tgz",
			},
			Digest: "sha256:1234567890abcdef",
		},
	}
	tests := []*ChartVersion{alpine[0], nginx[0], nginx[1]}

	for i, tt := range tests {
		expect := expects[i]
		if tt.Name != expect.Name {
			t.Errorf("Expected name %q, got %q", expect.Name, tt.Name)
		}
		if tt.Description != expect.Description {
			t.Errorf("Expected description %q, got %q", expect.Description, tt.Description)
		}
		if tt.Version != expect.Version {
			t.Errorf("Expected version %q, got %q", expect.Version, tt.Version)
		}
		if tt.Digest != expect.Digest {
			t.Errorf("Expected digest %q, got %q", expect.Digest, tt.Digest)
		}
		if tt.Home != expect.Home {
			t.Errorf("Expected home %q, got %q", expect.Home, tt.Home)
		}

		for i, url := range tt.URLs {
			if url != expect.URLs[i] {
				t.Errorf("Expected URL %q, got %q", expect.URLs[i], url)
			}
		}
		for i, kw := range tt.Keywords {
			if kw != expect.Keywords[i] {
				t.Errorf("Expected keywords %q, got %q", expect.Keywords[i], kw)
			}
		}
	}
}

func TestIndexDirectory(t *testing.T) {
	dir := "testdata/repository"
	index, err := IndexDirectory(dir, "http://localhost:8080")
	if err != nil {
		t.Fatal(err)
	}

	if l := len(index.Entries); l != 3 {
		t.Fatalf("Expected 3 entries, got %d", l)
	}

	// Other things test the entry generation more thoroughly. We just test a
	// few fields.

	corpus := []struct{ chartName, downloadLink string }{
		{"frobnitz", "http://localhost:8080/frobnitz-1.2.3.tgz"},
		{"zarthal", "http://localhost:8080/universe/zarthal-1.0.0.tgz"},
	}

	for _, test := range corpus {
		cname := test.chartName
		frobs, ok := index.Entries[cname]
		if !ok {
			t.Fatalf("Could not read chart %s", cname)
		}

		frob := frobs[0]
		if len(frob.Digest) == 0 {
			t.Errorf("Missing digest of file %s.", frob.Name)
		}
		if frob.URLs[0] != test.downloadLink {
			t.Errorf("Unexpected URLs: %v", frob.URLs)
		}
		if frob.Name != cname {
			t.Errorf("Expected %q, got %q", cname, frob.Name)
		}
	}
}

func TestLoadUnversionedIndex(t *testing.T) {
	data, err := ioutil.ReadFile("testdata/unversioned-index.yaml")
	if err != nil {
		t.Fatal(err)
	}

	ind, err := loadUnversionedIndex(data)
	if err != nil {
		t.Fatal(err)
	}

	if l := len(ind.Entries); l != 2 {
		t.Fatalf("Expected 2 entries, got %d", l)
	}

	if l := len(ind.Entries["mysql"]); l != 3 {
		t.Fatalf("Expected 3 mysql versions, got %d", l)
	}
}

func TestIndexAdd(t *testing.T) {
	i := NewIndexFile()
	i.Add(&chart.Metadata{Name: "clipper", Version: "0.1.0"}, "clipper-0.1.0.tgz", "http://example.com/charts", "sha256:1234567890")

	if i.Entries["clipper"][0].URLs[0] != "http://example.com/charts/clipper-0.1.0.tgz" {
		t.Errorf("Expected http://example.com/charts/clipper-0.1.0.tgz, got %s", i.Entries["clipper"][0].URLs[0])
	}

	i.Add(&chart.Metadata{Name: "alpine", Version: "0.1.0"}, "/home/charts/alpine-0.1.0.tgz", "http://example.com/charts", "sha256:1234567890")

	if i.Entries["alpine"][0].URLs[0] != "http://example.com/charts/alpine-0.1.0.tgz" {
		t.Errorf("Expected http://example.com/charts/alpine-0.1.0.tgz, got %s", i.Entries["alpine"][0].URLs[0])
	}

	i.Add(&chart.Metadata{Name: "deis", Version: "0.1.0"}, "/home/charts/deis-0.1.0.tgz", "http://example.com/charts/", "sha256:1234567890")

	if i.Entries["deis"][0].URLs[0] != "http://example.com/charts/deis-0.1.0.tgz" {
		t.Errorf("Expected http://example.com/charts/deis-0.1.0.tgz, got %s", i.Entries["deis"][0].URLs[0])
	}
}
