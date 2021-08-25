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

package repo

import (
	"bufio"
	"bytes"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/helmpath"
)

const (
	testfile            = "testdata/local-index.yaml"
	annotationstestfile = "testdata/local-index-annotations.yaml"
	chartmuseumtestfile = "testdata/chartmuseum-index.yaml"
	unorderedTestfile   = "testdata/local-index-unordered.yaml"
	testRepo            = "test-repo"
	indexWithDuplicates = `
apiVersion: v1
entries:
  nginx:
    - urls:
        - https://charts.helm.sh/stable/nginx-0.2.0.tgz
      name: nginx
      description: string
      version: 0.2.0
      home: https://github.com/something/else
      digest: "sha256:1234567890abcdef"
  nginx:
    - urls:
        - https://charts.helm.sh/stable/alpine-1.0.0.tgz
        - http://storage2.googleapis.com/kubernetes-charts/alpine-1.0.0.tgz
      name: alpine
      description: string
      version: 1.0.0
      home: https://github.com/something
      digest: "sha256:1234567890abcdef"
`
)

func TestIndexFile(t *testing.T) {
	i := NewIndexFile()
	for _, x := range []struct {
		md       *chart.Metadata
		filename string
		baseURL  string
		digest   string
	}{
		{&chart.Metadata{APIVersion: "v2", Name: "clipper", Version: "0.1.0"}, "clipper-0.1.0.tgz", "http://example.com/charts", "sha256:1234567890"},
		{&chart.Metadata{APIVersion: "v2", Name: "cutter", Version: "0.1.1"}, "cutter-0.1.1.tgz", "http://example.com/charts", "sha256:1234567890abc"},
		{&chart.Metadata{APIVersion: "v2", Name: "cutter", Version: "0.1.0"}, "cutter-0.1.0.tgz", "http://example.com/charts", "sha256:1234567890abc"},
		{&chart.Metadata{APIVersion: "v2", Name: "cutter", Version: "0.2.0"}, "cutter-0.2.0.tgz", "http://example.com/charts", "sha256:1234567890abc"},
		{&chart.Metadata{APIVersion: "v2", Name: "setter", Version: "0.1.9+alpha"}, "setter-0.1.9+alpha.tgz", "http://example.com/charts", "sha256:1234567890abc"},
		{&chart.Metadata{APIVersion: "v2", Name: "setter", Version: "0.1.9+beta"}, "setter-0.1.9+beta.tgz", "http://example.com/charts", "sha256:1234567890abc"},
	} {
		if err := i.MustAdd(x.md, x.filename, x.baseURL, x.digest); err != nil {
			t.Errorf("unexpected error adding to index: %s", err)
		}
	}

	i.SortEntries()

	if i.APIVersion != APIVersionV1 {
		t.Error("Expected API version v1")
	}

	if len(i.Entries) != 3 {
		t.Errorf("Expected 3 charts. Got %d", len(i.Entries))
	}

	if i.Entries["clipper"][0].Name != "clipper" {
		t.Errorf("Expected clipper, got %s", i.Entries["clipper"][0].Name)
	}

	if len(i.Entries["cutter"]) != 3 {
		t.Error("Expected three cutters.")
	}

	// Test that the sort worked. 0.2 should be at the first index for Cutter.
	if v := i.Entries["cutter"][0].Version; v != "0.2.0" {
		t.Errorf("Unexpected first version: %s", v)
	}

	cv, err := i.Get("setter", "0.1.9")
	if err == nil && !strings.Contains(cv.Metadata.Version, "0.1.9") {
		t.Errorf("Unexpected version: %s", cv.Metadata.Version)
	}

	cv, err = i.Get("setter", "0.1.9+alpha")
	if err != nil || cv.Metadata.Version != "0.1.9+alpha" {
		t.Errorf("Expected version: 0.1.9+alpha")
	}
}

func TestLoadIndex(t *testing.T) {

	tests := []struct {
		Name     string
		Filename string
	}{
		{
			Name:     "regular index file",
			Filename: testfile,
		},
		{
			Name:     "chartmuseum index file",
			Filename: chartmuseumtestfile,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			i, err := LoadIndexFile(tc.Filename)
			if err != nil {
				t.Fatal(err)
			}
			verifyLocalIndex(t, i)
		})
	}
}

// TestLoadIndex_Duplicates is a regression to make sure that we don't non-deterministically allow duplicate packages.
func TestLoadIndex_Duplicates(t *testing.T) {
	if _, err := loadIndex([]byte(indexWithDuplicates), "indexWithDuplicates"); err == nil {
		t.Errorf("Expected an error when duplicate entries are present")
	}
}

func TestLoadIndex_Empty(t *testing.T) {
	if _, err := loadIndex([]byte(""), "indexWithEmpty"); err == nil {
		t.Errorf("Expected an error when index.yaml is empty.")
	}
}

func TestLoadIndexFileAnnotations(t *testing.T) {
	i, err := LoadIndexFile(annotationstestfile)
	if err != nil {
		t.Fatal(err)
	}
	verifyLocalIndex(t, i)

	if len(i.Annotations) != 1 {
		t.Fatalf("Expected 1 annotation but got %d", len(i.Annotations))
	}
	if i.Annotations["helm.sh/test"] != "foo bar" {
		t.Error("Did not get expected value for helm.sh/test annotation")
	}
}

func TestLoadUnorderedIndex(t *testing.T) {
	i, err := LoadIndexFile(unorderedTestfile)
	if err != nil {
		t.Fatal(err)
	}
	verifyLocalIndex(t, i)
}

func TestMerge(t *testing.T) {
	ind1 := NewIndexFile()

	if err := ind1.MustAdd(&chart.Metadata{APIVersion: "v2", Name: "dreadnought", Version: "0.1.0"}, "dreadnought-0.1.0.tgz", "http://example.com", "aaaa"); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	ind2 := NewIndexFile()

	for _, x := range []struct {
		md       *chart.Metadata
		filename string
		baseURL  string
		digest   string
	}{
		{&chart.Metadata{APIVersion: "v2", Name: "dreadnought", Version: "0.2.0"}, "dreadnought-0.2.0.tgz", "http://example.com", "aaaabbbb"},
		{&chart.Metadata{APIVersion: "v2", Name: "doughnut", Version: "0.2.0"}, "doughnut-0.2.0.tgz", "http://example.com", "ccccbbbb"},
	} {
		if err := ind2.MustAdd(x.md, x.filename, x.baseURL, x.digest); err != nil {
			t.Errorf("unexpected error: %s", err)
		}
	}

	ind1.Merge(ind2)

	if len(ind1.Entries) != 2 {
		t.Errorf("Expected 2 entries, got %d", len(ind1.Entries))
		vs := ind1.Entries["dreadnought"]
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
	t.Run("should  download index file", func(t *testing.T) {
		srv, err := startLocalServerForTests(nil)
		if err != nil {
			t.Fatal(err)
		}
		defer srv.Close()

		r, err := NewChartRepository(&Entry{
			Name: testRepo,
			URL:  srv.URL,
		}, getter.All(&cli.EnvSettings{}))
		if err != nil {
			t.Errorf("Problem creating chart repository from %s: %v", testRepo, err)
		}

		idx, err := r.DownloadIndexFile()
		if err != nil {
			t.Fatalf("Failed to download index file to %s: %#v", idx, err)
		}

		if _, err := os.Stat(idx); err != nil {
			t.Fatalf("error finding created index file: %#v", err)
		}

		i, err := LoadIndexFile(idx)
		if err != nil {
			t.Fatalf("Index %q failed to parse: %s", testfile, err)
		}
		verifyLocalIndex(t, i)

		// Check that charts file is also created
		idx = filepath.Join(r.CachePath, helmpath.CacheChartsFile(r.Config.Name))
		if _, err := os.Stat(idx); err != nil {
			t.Fatalf("error finding created charts file: %#v", err)
		}

		b, err := ioutil.ReadFile(idx)
		if err != nil {
			t.Fatalf("error reading charts file: %#v", err)
		}
		verifyLocalChartsFile(t, b, i)
	})

	t.Run("should not decode the path in the repo url while downloading index", func(t *testing.T) {
		chartRepoURLPath := "/some%2Fpath/test"
		fileBytes, err := ioutil.ReadFile("testdata/local-index.yaml")
		if err != nil {
			t.Fatal(err)
		}
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.RawPath == chartRepoURLPath+"/index.yaml" {
				w.Write(fileBytes)
			}
		})
		srv, err := startLocalServerForTests(handler)
		if err != nil {
			t.Fatal(err)
		}
		defer srv.Close()

		r, err := NewChartRepository(&Entry{
			Name: testRepo,
			URL:  srv.URL + chartRepoURLPath,
		}, getter.All(&cli.EnvSettings{}))
		if err != nil {
			t.Errorf("Problem creating chart repository from %s: %v", testRepo, err)
		}

		idx, err := r.DownloadIndexFile()
		if err != nil {
			t.Fatalf("Failed to download index file to %s: %#v", idx, err)
		}

		if _, err := os.Stat(idx); err != nil {
			t.Fatalf("error finding created index file: %#v", err)
		}

		i, err := LoadIndexFile(idx)
		if err != nil {
			t.Fatalf("Index %q failed to parse: %s", testfile, err)
		}
		verifyLocalIndex(t, i)

		// Check that charts file is also created
		idx = filepath.Join(r.CachePath, helmpath.CacheChartsFile(r.Config.Name))
		if _, err := os.Stat(idx); err != nil {
			t.Fatalf("error finding created charts file: %#v", err)
		}

		b, err := ioutil.ReadFile(idx)
		if err != nil {
			t.Fatalf("error reading charts file: %#v", err)
		}
		verifyLocalChartsFile(t, b, i)
	})
}

func verifyLocalIndex(t *testing.T, i *IndexFile) {
	numEntries := len(i.Entries)
	if numEntries != 3 {
		t.Errorf("Expected 3 entries in index file but got %d", numEntries)
	}

	alpine, ok := i.Entries["alpine"]
	if !ok {
		t.Fatalf("'alpine' section not found.")
	}

	if l := len(alpine); l != 1 {
		t.Fatalf("'alpine' should have 1 chart, got %d", l)
	}

	nginx, ok := i.Entries["nginx"]
	if !ok || len(nginx) != 2 {
		t.Fatalf("Expected 2 nginx entries")
	}

	expects := []*ChartVersion{
		{
			Metadata: &chart.Metadata{
				APIVersion:  "v2",
				Name:        "alpine",
				Description: "string",
				Version:     "1.0.0",
				Keywords:    []string{"linux", "alpine", "small", "sumtin"},
				Home:        "https://github.com/something",
			},
			URLs: []string{
				"https://charts.helm.sh/stable/alpine-1.0.0.tgz",
				"http://storage2.googleapis.com/kubernetes-charts/alpine-1.0.0.tgz",
			},
			Digest: "sha256:1234567890abcdef",
		},
		{
			Metadata: &chart.Metadata{
				APIVersion:  "v2",
				Name:        "nginx",
				Description: "string",
				Version:     "0.2.0",
				Keywords:    []string{"popular", "web server", "proxy"},
				Home:        "https://github.com/something/else",
			},
			URLs: []string{
				"https://charts.helm.sh/stable/nginx-0.2.0.tgz",
			},
			Digest: "sha256:1234567890abcdef",
		},
		{
			Metadata: &chart.Metadata{
				APIVersion:  "v2",
				Name:        "nginx",
				Description: "string",
				Version:     "0.1.0",
				Keywords:    []string{"popular", "web server", "proxy"},
				Home:        "https://github.com/something",
			},
			URLs: []string{
				"https://charts.helm.sh/stable/nginx-0.1.0.tgz",
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

func verifyLocalChartsFile(t *testing.T, chartsContent []byte, indexContent *IndexFile) {
	var expected, real []string
	for chart := range indexContent.Entries {
		expected = append(expected, chart)
	}
	sort.Strings(expected)

	scanner := bufio.NewScanner(bytes.NewReader(chartsContent))
	for scanner.Scan() {
		real = append(real, scanner.Text())
	}
	sort.Strings(real)

	if strings.Join(expected, " ") != strings.Join(real, " ") {
		t.Errorf("Cached charts file content unexpected. Expected:\n%s\ngot:\n%s", expected, real)
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
		if frob.Digest == "" {
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

func TestIndexAdd(t *testing.T) {
	i := NewIndexFile()

	for _, x := range []struct {
		md       *chart.Metadata
		filename string
		baseURL  string
		digest   string
	}{

		{&chart.Metadata{APIVersion: "v2", Name: "clipper", Version: "0.1.0"}, "clipper-0.1.0.tgz", "http://example.com/charts", "sha256:1234567890"},
		{&chart.Metadata{APIVersion: "v2", Name: "alpine", Version: "0.1.0"}, "/home/charts/alpine-0.1.0.tgz", "http://example.com/charts", "sha256:1234567890"},
		{&chart.Metadata{APIVersion: "v2", Name: "deis", Version: "0.1.0"}, "/home/charts/deis-0.1.0.tgz", "http://example.com/charts/", "sha256:1234567890"},
	} {
		if err := i.MustAdd(x.md, x.filename, x.baseURL, x.digest); err != nil {
			t.Errorf("unexpected error adding to index: %s", err)
		}
	}

	if i.Entries["clipper"][0].URLs[0] != "http://example.com/charts/clipper-0.1.0.tgz" {
		t.Errorf("Expected http://example.com/charts/clipper-0.1.0.tgz, got %s", i.Entries["clipper"][0].URLs[0])
	}
	if i.Entries["alpine"][0].URLs[0] != "http://example.com/charts/alpine-0.1.0.tgz" {
		t.Errorf("Expected http://example.com/charts/alpine-0.1.0.tgz, got %s", i.Entries["alpine"][0].URLs[0])
	}
	if i.Entries["deis"][0].URLs[0] != "http://example.com/charts/deis-0.1.0.tgz" {
		t.Errorf("Expected http://example.com/charts/deis-0.1.0.tgz, got %s", i.Entries["deis"][0].URLs[0])
	}

	// test error condition
	if err := i.MustAdd(&chart.Metadata{}, "error-0.1.0.tgz", "", ""); err == nil {
		t.Fatal("expected error adding to index")
	}
}

func TestIndexWrite(t *testing.T) {
	i := NewIndexFile()
	if err := i.MustAdd(&chart.Metadata{APIVersion: "v2", Name: "clipper", Version: "0.1.0"}, "clipper-0.1.0.tgz", "http://example.com/charts", "sha256:1234567890"); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	dir, err := ioutil.TempDir("", "helm-tmp")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	testpath := filepath.Join(dir, "test")
	i.WriteFile(testpath, 0600)

	got, err := ioutil.ReadFile(testpath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "clipper-0.1.0.tgz") {
		t.Fatal("Index files doesn't contain expected content")
	}
}
