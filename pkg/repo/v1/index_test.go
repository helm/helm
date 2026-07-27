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
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	chart "helm.sh/helm/v4/pkg/chart/v2"
	"helm.sh/helm/v4/pkg/cli"
	"helm.sh/helm/v4/pkg/getter"
	"helm.sh/helm/v4/pkg/helmpath"
)

const (
	testfile            = "testdata/local-index.yaml"
	annotationstestfile = "testdata/local-index-annotations.yaml"
	chartmuseumtestfile = "testdata/chartmuseum-index.yaml"
	unorderedTestfile   = "testdata/local-index-unordered.yaml"
	jsonTestfile        = "testdata/local-index.json"
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
	indexWithEmptyEntry = `
apiVersion: v1
entries:
  grafana:
  - apiVersion: v2
    name: grafana
  - null
  foo:
  -
  bar:
  - digest: "sha256:1234567890abcdef"
    urls:
    - https://charts.helm.sh/stable/alpine-1.0.0.tgz
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
		{&chart.Metadata{APIVersion: "v2", Name: "setter", Version: "0.1.8"}, "setter-0.1.8.tgz", "http://example.com/charts", "sha256:1234567890abc"},
		{&chart.Metadata{APIVersion: "v2", Name: "setter", Version: "0.1.8+beta"}, "setter-0.1.8+beta.tgz", "http://example.com/charts", "sha256:1234567890abc"},
	} {
		require.NoErrorf(t, i.MustAdd(x.md, x.filename, x.baseURL, x.digest), "unexpected error adding to index")
	}

	i.SortEntries()

	assert.Equal(t, APIVersionV1, i.APIVersion, "Expected API version v1")

	assert.Lenf(t, i.Entries, 3, "Expected 3 charts. Got %d", len(i.Entries))

	assert.Equalf(t, "clipper", i.Entries["clipper"][0].Name, "Expected clipper, got %s", i.Entries["clipper"][0].Name)

	assert.Len(t, i.Entries["cutter"], 3, "Expected three cutters.")

	// Test that the sort worked. 0.2 should be at the first index for Cutter.
	v := i.Entries["cutter"][0].Version
	assert.Equalf(t, "0.2.0", v, "Unexpected first version: %s", v)

	cv, err := i.Get("setter", "0.1.9")
	require.NoError(t, err)
	assert.Contains(t, cv.Version, "0.1.9", "Unexpected version: %s", cv.Version)

	cv, err = i.Get("setter", "0.1.9+alpha")
	require.NoError(t, err, "Expected version: 0.1.9+alpha")
	assert.Equal(t, "0.1.9+alpha", cv.Version, "Expected version: 0.1.9+alpha")

	cv, err = i.Get("setter", "0.1.8")
	require.NoError(t, err, "Expected version: 0.1.8")
	assert.Equal(t, "0.1.8", cv.Version, "Expected version: 0.1.8")
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
		{
			Name:     "JSON index file",
			Filename: jsonTestfile,
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			i, err := LoadIndexFile(tc.Filename)
			require.NoError(t, err)
			verifyLocalIndex(t, i)
		})
	}
}

// TestLoadIndex_Duplicates is a regression to make sure that we don't non-deterministically allow duplicate packages.
func TestLoadIndex_Duplicates(t *testing.T) {
	_, err := loadIndex([]byte(indexWithDuplicates), "indexWithDuplicates")
	assert.Error(t, err, "Expected an error when duplicate entries are present")
}

func TestLoadIndex_EmptyEntry(t *testing.T) {
	_, err := loadIndex([]byte(indexWithEmptyEntry), "indexWithEmptyEntry")
	assert.NoError(t, err)
}

func TestLoadIndex_Empty(t *testing.T) {
	_, err := loadIndex([]byte(""), "indexWithEmpty")
	assert.Error(t, err, "Expected an error when index.yaml is empty.")
}

func TestLoadIndexFileAnnotations(t *testing.T) {
	i, err := LoadIndexFile(annotationstestfile)
	require.NoError(t, err)
	verifyLocalIndex(t, i)

	require.Lenf(t, i.Annotations, 1, "Expected 1 annotation but got %d", len(i.Annotations))
	assert.Equal(t, "foo bar", i.Annotations["helm.sh/test"], "Did not get expected value for helm.sh/test annotation")
}

func TestLoadUnorderedIndex(t *testing.T) {
	i, err := LoadIndexFile(unorderedTestfile)
	require.NoError(t, err)
	verifyLocalIndex(t, i)
}

func TestMerge(t *testing.T) {
	ind1 := NewIndexFile()

	require.NoError(t, ind1.MustAdd(&chart.Metadata{APIVersion: "v2", Name: "dreadnought", Version: "0.1.0"}, "dreadnought-0.1.0.tgz", "http://example.com", "aaaa"))

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
		require.NoError(t, ind2.MustAdd(x.md, x.filename, x.baseURL, x.digest))
	}

	ind1.Merge(ind2)

	assert.Lenf(t, ind1.Entries, 2, "Expected 2 entries, got %d", len(ind1.Entries))

	vs := ind1.Entries["dreadnought"]
	assert.Lenf(t, vs, 2, "Expected 2 versions, got %d", len(vs))

	v := vs[1]
	assert.Equalf(t, "0.2.0", v.Version, "Expected %q version to be 0.2.0, got %s", v.Name, v.Version)
}

func TestDownloadIndexFile(t *testing.T) {
	t.Run("should  download index file", func(t *testing.T) {
		srv, err := startLocalServerForTests(nil)
		require.NoError(t, err)
		defer srv.Close()

		r, err := NewChartRepository(&Entry{
			Name: testRepo,
			URL:  srv.URL,
		}, getter.All(&cli.EnvSettings{}))
		require.NoErrorf(t, err, "Problem creating chart repository from %s", testRepo)

		idx, err := r.DownloadIndexFile()
		require.NoErrorf(t, err, "Failed to download index file to %s", idx)

		_, err = os.Stat(idx)

		require.NoErrorf(t, err, "error finding created index file")

		i, err := LoadIndexFile(idx)
		require.NoErrorf(t, err, "Index %q failed to parse", testfile)
		verifyLocalIndex(t, i)

		// Check that charts file is also created
		idx = filepath.Join(r.CachePath, helmpath.CacheChartsFile(r.Config.Name))
		_, err = os.Stat(idx)
		require.NoErrorf(t, err, "error finding created charts file")

		b, err := os.ReadFile(idx)
		require.NoErrorf(t, err, "error reading charts file")
		verifyLocalChartsFile(t, b, i)
	})

	t.Run("should not decode the path in the repo url while downloading index", func(t *testing.T) {
		chartRepoURLPath := "/some%2Fpath/test"
		fileBytes, err := os.ReadFile("testdata/local-index.yaml")
		require.NoError(t, err)
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.RawPath == chartRepoURLPath+"/index.yaml" {
				w.Write(fileBytes)
			}
		})
		srv, err := startLocalServerForTests(handler)
		require.NoError(t, err)
		defer srv.Close()

		r, err := NewChartRepository(&Entry{
			Name: testRepo,
			URL:  srv.URL + chartRepoURLPath,
		}, getter.All(&cli.EnvSettings{}))
		require.NoErrorf(t, err, "Problem creating chart repository from %s", testRepo)

		idx, err := r.DownloadIndexFile()
		require.NoErrorf(t, err, "Failed to download index file to %s", idx)

		_, err = os.Stat(idx)
		require.NoErrorf(t, err, "error finding created index file")

		i, err := LoadIndexFile(idx)
		require.NoErrorf(t, err, "Index %q failed to parse", testfile)
		verifyLocalIndex(t, i)

		// Check that charts file is also created
		idx = filepath.Join(r.CachePath, helmpath.CacheChartsFile(r.Config.Name))
		_, err = os.Stat(idx)
		require.NoErrorf(t, err, "error finding created charts file")

		b, err := os.ReadFile(idx)
		require.NoErrorf(t, err, "error reading charts file")
		verifyLocalChartsFile(t, b, i)
	})
}

func verifyLocalIndex(t *testing.T, i *IndexFile) {
	t.Helper()
	numEntries := len(i.Entries)
	assert.Equalf(t, 3, numEntries, "Expected 3 entries in index file but got %d", numEntries)

	alpine, ok := i.Entries["alpine"]
	require.True(t, ok, "'alpine' section not found.")

	l := len(alpine)
	require.Equalf(t, 1, l, "'alpine' should have 1 chart, got %d", l)

	nginx, ok := i.Entries["nginx"]
	require.True(t, ok)
	require.Len(t, nginx, 2, "Expected 2 nginx entries")

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
		assert.Equalf(t, expect.Name, tt.Name, "Expected name %q, got %q", expect.Name, tt.Name)
		assert.Equalf(t, expect.Description, tt.Description, "Expected description %q, got %q", expect.Description, tt.Description)
		assert.Equalf(t, expect.Version, tt.Version, "Expected version %q, got %q", expect.Version, tt.Version)
		assert.Equalf(t, expect.Digest, tt.Digest, "Expected digest %q, got %q", expect.Digest, tt.Digest)
		assert.Equalf(t, expect.Home, tt.Home, "Expected home %q, got %q", expect.Home, tt.Home)

		for i, url := range tt.URLs {
			assert.Equalf(t, expect.URLs[i], url, "Expected URL %q, got %q", expect.URLs[i], url)
		}
		for i, kw := range tt.Keywords {
			assert.Equalf(t, expect.Keywords[i], kw, "Expected keywords %q, got %q", expect.Keywords[i], kw)
		}
	}
}

func verifyLocalChartsFile(t *testing.T, chartsContent []byte, indexContent *IndexFile) {
	t.Helper()
	var expected, reald []string
	for chart := range indexContent.Entries {
		expected = append(expected, chart)
	}
	sort.Strings(expected)

	scanner := bufio.NewScanner(bytes.NewReader(chartsContent))
	for scanner.Scan() {
		reald = append(reald, scanner.Text())
	}
	sort.Strings(reald)

	assert.Equalf(t, strings.Join(expected, " "), strings.Join(reald, " "), "Cached charts file content unexpected. Expected:\n%s\ngot:\n%s", expected, reald)
}

func TestIndexDirectory(t *testing.T) {
	dir := "testdata/repository"
	index, err := IndexDirectory(dir, "http://localhost:8080")
	require.NoError(t, err)

	l := len(index.Entries)
	require.Equalf(t, 3, l, "Expected 3 entries, got %d", l)

	// Other things test the entry generation more thoroughly. We just test a
	// few fields.

	corpus := []struct{ chartName, downloadLink string }{
		{"frobnitz", "http://localhost:8080/frobnitz-1.2.3.tgz"},
		{"zarthal", "http://localhost:8080/universe/zarthal-1.0.0.tgz"},
	}

	for _, test := range corpus {
		cname := test.chartName
		frobs, ok := index.Entries[cname]
		require.Truef(t, ok, "Could not read chart %s", cname)

		frob := frobs[0]
		assert.NotEmptyf(t, frob.Digest, "Missing digest of file %s.", frob.Name)
		assert.Equalf(t, test.downloadLink, frob.URLs[0], "Unexpected URLs: %v", frob.URLs)
		assert.Equalf(t, cname, frob.Name, "Expected %q, got %q", cname, frob.Name)
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
		require.NoErrorf(t, i.MustAdd(x.md, x.filename, x.baseURL, x.digest), "unexpected error adding to index")
	}

	assert.Equalf(t, "http://example.com/charts/clipper-0.1.0.tgz", i.Entries["clipper"][0].URLs[0], "Expected http://example.com/charts/clipper-0.1.0.tgz, got %s", i.Entries["clipper"][0].URLs[0])
	assert.Equalf(t, "http://example.com/charts/alpine-0.1.0.tgz", i.Entries["alpine"][0].URLs[0], "Expected http://example.com/charts/alpine-0.1.0.tgz, got %s", i.Entries["alpine"][0].URLs[0])
	assert.Equalf(t, "http://example.com/charts/deis-0.1.0.tgz", i.Entries["deis"][0].URLs[0], "Expected http://example.com/charts/deis-0.1.0.tgz, got %s", i.Entries["deis"][0].URLs[0])

	// test error condition
	require.Error(t, i.MustAdd(&chart.Metadata{}, "error-0.1.0.tgz", "", ""), "expected error adding to index")
}

func TestIndexWrite(t *testing.T) {
	i := NewIndexFile()
	require.NoError(t, i.MustAdd(&chart.Metadata{APIVersion: "v2", Name: "clipper", Version: "0.1.0"}, "clipper-0.1.0.tgz", "http://example.com/charts", "sha256:1234567890"))
	dir := t.TempDir()
	testpath := filepath.Join(dir, "test")
	i.WriteFile(testpath, 0o600)

	got, err := os.ReadFile(testpath)
	require.NoError(t, err)
	require.Contains(t, string(got), "clipper-0.1.0.tgz", "Index files doesn't contain expected content")
}

func TestIndexJSONWrite(t *testing.T) {
	i := NewIndexFile()
	require.NoError(t, i.MustAdd(&chart.Metadata{APIVersion: "v2", Name: "clipper", Version: "0.1.0"}, "clipper-0.1.0.tgz", "http://example.com/charts", "sha256:1234567890"))
	dir := t.TempDir()
	testpath := filepath.Join(dir, "test")
	i.WriteJSONFile(testpath, 0o600)

	got, err := os.ReadFile(testpath)
	require.NoError(t, err)
	require.True(t, json.Valid(got), "Index files doesn't contain valid JSON")
	require.Contains(t, string(got), "clipper-0.1.0.tgz", "Index files doesn't contain expected content")
}

func TestAddFileIndexEntriesNil(t *testing.T) {
	i := NewIndexFile()
	i.APIVersion = chart.APIVersionV1
	i.Entries = nil
	for _, x := range []struct {
		md       *chart.Metadata
		filename string
		baseURL  string
		digest   string
	}{
		{&chart.Metadata{APIVersion: "v2", Name: " ", Version: "8033-5.apinie+s.r"}, "setter-0.1.9+beta.tgz", "http://example.com/charts", "sha256:1234567890abc"},
	} {
		assert.Error(t, i.MustAdd(x.md, x.filename, x.baseURL, x.digest), "expected err to be non-nil when entries not initialized")
	}
}

func TestIgnoreSkippableChartValidationError(t *testing.T) {
	type TestCase struct {
		Input        error
		ErrorSkipped bool
	}
	testCases := map[string]TestCase{
		"nil": {
			Input: nil,
		},
		"generic_error": {
			Input: errors.New("foo"),
		},
		"non_skipped_validation_error": {
			Input: chart.ValidationError("chart.metadata.type must be application or library"),
		},
		"skipped_validation_error": {
			Input:        chart.ValidationErrorf("more than one dependency with name or alias %q", "foo"),
			ErrorSkipped: true,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			result := ignoreSkippableChartValidationError(tc.Input)
			switch {
			case tc.Input == nil:
				assert.NoError(t, result, "expected nil result for nil input")
			case tc.ErrorSkipped:
				assert.NoError(t, result, "expected nil result for skipped error")
			default:
				assert.ErrorIs(t, tc.Input, result, "expected the result equal to input")
			}
		})
	}
}

var indexWithDuplicatesInChartDeps = `
apiVersion: v1
entries:
  nginx:
    - urls:
        - https://charts.helm.sh/stable/alpine-1.0.0.tgz
        - http://storage2.googleapis.com/kubernetes-charts/alpine-1.0.0.tgz
      name: alpine
      description: string
      home: https://github.com/something
      digest: "sha256:1234567890abcdef"
    - urls:
        - https://charts.helm.sh/stable/nginx-0.2.0.tgz
      name: nginx
      description: string
      version: 0.2.0
      home: https://github.com/something/else
      digest: "sha256:1234567890abcdef"
`
var indexWithDuplicatesInLastChartDeps = `
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
    - urls:
        - https://charts.helm.sh/stable/alpine-1.0.0.tgz
        - http://storage2.googleapis.com/kubernetes-charts/alpine-1.0.0.tgz
      name: alpine
      description: string
      home: https://github.com/something
      digest: "sha256:111"
`

func TestLoadIndex_DuplicateChartDeps(t *testing.T) {
	tests := []struct {
		source string
		data   string
	}{
		{
			source: "indexWithDuplicatesInChartDeps",
			data:   indexWithDuplicatesInChartDeps,
		},
		{
			source: "indexWithDuplicatesInLastChartDeps",
			data:   indexWithDuplicatesInLastChartDeps,
		},
	}
	for _, tc := range tests {
		t.Run(tc.source, func(t *testing.T) {
			idx, err := loadIndex([]byte(tc.data), tc.source)
			require.NoError(t, err)
			cvs := idx.Entries["nginx"]
			assert.NotNil(t, cvs, "expected one chart version not to be filtered out")
			for _, v := range cvs {
				assert.NotEqual(t, "alpine", v.Name, "malformed version was not filtered out")
			}
		})
	}
}

func TestIsVersionRange(t *testing.T) {
	tests := []struct {
		version  string
		expected bool
	}{
		{"1.0.0", false},
		{"1.0.0+metadata", false},
		{"v1.19.2", false},
		{"v1", false},
		{"^1", true},
		{"^1.2.3", true},
		{"~1.10", true},
		{"~1.10.0", true},
		{">= 1.0.0", true},
		{"> 1.0.0", true},
		{"< 2.0.0", true},
		{"<= 2.0.0", true},
		{"!= 1.0.0", true},
		{"1.*", true},
		{"1.x", true},
		{"1.X", true},
		{"v1.x", true},
		{"v1.X", true},
		{"1.0.0 - 2.0.0", true},
		{"^1.0.0 || ^2.0.0", true},
		{">=1.0.0 <2.0.0", true},
		// Exact versions with 'x'/'X' in prerelease or build metadata
		{"1.0.0-fix", false},
		{"2.0.0-next", false},
		{"1.0.0+exp", false},
	}

	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			got := isVersionRange(tt.version)
			assert.Equalf(t, tt.expected, got, "isVersionRange(%q) = %v, want %v", tt.version, got, tt.expected)
		})
	}
}
