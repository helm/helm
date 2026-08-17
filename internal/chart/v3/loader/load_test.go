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

package loader

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	chart "helm.sh/helm/v4/internal/chart/v3"
	"helm.sh/helm/v4/pkg/chart/common"
	"helm.sh/helm/v4/pkg/chart/loader/archive"
)

func TestLoadDir(t *testing.T) {
	l, err := Loader("testdata/frobnitz")
	require.NoError(t, err, "Failed to load testdata")
	c, err := l.Load()
	require.NoError(t, err, "Failed to load testdata")
	verifyFrobnitz(t, c)
	verifyChart(t, c)
	verifyDependencies(t, c)
	verifyDependenciesLock(t, c)
}

func TestLoadDirWithDevNull(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test only works on unix systems with /dev/null present")
	}

	l, err := Loader("testdata/frobnitz_with_dev_null")
	require.NoError(t, err, "Failed to load testdata")
	_, err = l.Load()
	assert.Error(t, err, "packages with an irregular file (/dev/null) should not load")
}

func TestLoadDirWithSymlink(t *testing.T) {
	sym := filepath.Join("..", "LICENSE")
	link := filepath.Join("testdata", "frobnitz_with_symlink", "LICENSE")

	require.NoError(t, os.Symlink(sym, link))

	defer os.Remove(link)

	l, err := Loader("testdata/frobnitz_with_symlink")
	require.NoError(t, err, "Failed to load testdata")

	c, err := l.Load()
	require.NoError(t, err, "Failed to load testdata")
	verifyFrobnitz(t, c)
	verifyChart(t, c)
	verifyDependencies(t, c)
	verifyDependenciesLock(t, c)
}

func TestBomTestData(t *testing.T) {
	testFiles := []string{"frobnitz_with_bom/.helmignore", "frobnitz_with_bom/templates/template.tpl", "frobnitz_with_bom/Chart.yaml"}
	for _, file := range testFiles {
		data, err := os.ReadFile("testdata/" + file)
		if err != nil || !bytes.HasPrefix(data, utf8bom) {
			t.Errorf("Test file has no BOM or is invalid: testdata/%s", file)
		}
	}

	archive, err := os.ReadFile("testdata/frobnitz_with_bom.tgz")
	require.NoErrorf(t, err, "Error reading archive frobnitz_with_bom.tgz")
	unzipped, err := gzip.NewReader(bytes.NewReader(archive))
	require.NoErrorf(t, err, "Error reading archive frobnitz_with_bom.tgz")
	defer unzipped.Close()
	for _, testFile := range testFiles {
		data := make([]byte, 3)
		require.NoErrorf(t, unzipped.Reset(bytes.NewReader(archive)), "Error reading archive frobnitz_with_bom.tgz")
		tr := tar.NewReader(unzipped)
		for {
			file, err := tr.Next()
			if errors.Is(err, io.EOF) {
				break
			}
			require.NoErrorf(t, err, "Error reading archive frobnitz_with_bom.tgz")
			if file != nil && strings.EqualFold(file.Name, testFile) {
				_, err := tr.Read(data)
				if err == nil {
					break
				}
				t.Fatalf("Error reading archive frobnitz_with_bom.tgz: %s", err)
			}
		}
		require.Truef(t, bytes.Equal(data, utf8bom), "Test file has no BOM or is invalid: frobnitz_with_bom.tgz/%s", testFile)
	}
}

func TestLoadDirWithUTFBOM(t *testing.T) {
	l, err := Loader("testdata/frobnitz_with_bom")
	require.NoError(t, err, "Failed to load testdata")
	c, err := l.Load()
	require.NoError(t, err, "Failed to load testdata")
	verifyFrobnitz(t, c)
	verifyChart(t, c)
	verifyDependencies(t, c)
	verifyDependenciesLock(t, c)
	verifyBomStripped(t, c.Files)
}

func TestLoadArchiveWithUTFBOM(t *testing.T) {
	l, err := Loader("testdata/frobnitz_with_bom.tgz")
	require.NoError(t, err, "Failed to load testdata")
	c, err := l.Load()
	require.NoError(t, err, "Failed to load testdata")
	verifyFrobnitz(t, c)
	verifyChart(t, c)
	verifyDependencies(t, c)
	verifyDependenciesLock(t, c)
	verifyBomStripped(t, c.Files)
}

func TestLoadFile(t *testing.T) {
	l, err := Loader("testdata/frobnitz-1.2.3.tgz")
	require.NoError(t, err, "Failed to load testdata")
	c, err := l.Load()
	require.NoError(t, err, "Failed to load testdata")
	verifyFrobnitz(t, c)
	verifyChart(t, c)
	verifyDependencies(t, c)
}

func TestLoadFiles(t *testing.T) {
	modTime := time.Now()
	goodFiles := []*archive.BufferedFile{
		{
			Name:    "Chart.yaml",
			ModTime: modTime,
			Data: []byte(`apiVersion: v3
name: frobnitz
description: This is a frobnitz.
version: "1.2.3"
keywords:
  - frobnitz
  - sprocket
  - dodad
maintainers:
  - name: The Helm Team
    email: helm@example.com
  - name: Someone Else
    email: nobody@example.com
sources:
  - https://example.com/foo/bar
home: http://example.com
icon: https://example.com/64x64.png
`),
		},
		{
			Name:    "values.yaml",
			ModTime: modTime,
			Data:    []byte("var: some values"),
		},
		{
			Name:    "values.schema.json",
			ModTime: modTime,
			Data:    []byte("type: Values"),
		},
		{
			Name:    "templates/deployment.yaml",
			ModTime: modTime,
			Data:    []byte("some deployment"),
		},
		{
			Name:    "templates/service.yaml",
			ModTime: modTime,
			Data:    []byte("some service"),
		},
	}

	c, err := LoadFiles(goodFiles)
	require.NoError(t, err, "Expected good files to be loaded")
	assert.Equal(t, "frobnitz", c.Name(), "Expected chart name to be 'frobnitz', got %s", c.Name())
	assert.Equal(t, "some values", c.Values["var"], "Expected chart values to be populated with default values")
	assert.Len(t, c.Raw, 5, "Expected 5 files")
	assert.True(t, bytes.Equal(c.Schema, []byte("type: Values")), "Expected chart schema to be populated with default values")
	assert.Len(t, c.Templates, 2, "Expected 2 templates")

	_, err = LoadFiles([]*archive.BufferedFile{})
	require.Error(t, err, "Expected err to be non-nil")
	assert.EqualError(t, err, "Chart.yaml file is missing", "Expected chart metadata missing error, got '%s'", err.Error())
}

// Test the order of file loading. The Chart.yaml file needs to come first for
// later comparison checks. See https://github.com/helm/helm/pull/8948
func TestLoadFilesOrder(t *testing.T) {
	modTime := time.Now()
	goodFiles := []*archive.BufferedFile{
		{
			Name:    "requirements.yaml",
			ModTime: modTime,
			Data:    []byte("dependencies:"),
		},
		{
			Name:    "values.yaml",
			ModTime: modTime,
			Data:    []byte("var: some values"),
		},

		{
			Name:    "templates/deployment.yaml",
			ModTime: modTime,
			Data:    []byte("some deployment"),
		},
		{
			Name:    "templates/service.yaml",
			ModTime: modTime,
			Data:    []byte("some service"),
		},
		{
			Name:    "Chart.yaml",
			ModTime: modTime,
			Data: []byte(`apiVersion: v3
name: frobnitz
description: This is a frobnitz.
version: "1.2.3"
keywords:
  - frobnitz
  - sprocket
  - dodad
maintainers:
  - name: The Helm Team
    email: helm@example.com
  - name: Someone Else
    email: nobody@example.com
sources:
  - https://example.com/foo/bar
home: http://example.com
icon: https://example.com/64x64.png
`),
		},
	}

	// Capture stderr to make sure message about Chart.yaml handle dependencies
	// is not present
	r, w, err := os.Pipe()
	require.NoError(t, err, "Unable to create pipe")
	stderr := log.Writer()
	log.SetOutput(w)
	defer func() {
		log.SetOutput(stderr)
	}()

	_, err = LoadFiles(goodFiles)
	require.NoError(t, err, "Expected good files to be loaded")
	w.Close()

	var text bytes.Buffer
	io.Copy(&text, r)
	assert.Empty(t, text.String(), "Expected no message to Stderr, got %s", text.String())
}

// Packaging the chart on a Windows machine will produce an
// archive that has \\ as delimiters. Test that we support these archives
func TestLoadFileBackslash(t *testing.T) {
	c, err := Load("testdata/frobnitz_backslash-1.2.3.tgz")
	require.NoError(t, err, "Failed to load testdata")
	verifyChartFileAndTemplate(t, c, "frobnitz_backslash")
	verifyChart(t, c)
	verifyDependencies(t, c)
}

func TestLoadV3WithReqs(t *testing.T) {
	l, err := Loader("testdata/frobnitz.v3.reqs")
	require.NoError(t, err, "Failed to load testdata")
	c, err := l.Load()
	require.NoError(t, err, "Failed to load testdata")
	verifyDependencies(t, c)
	verifyDependenciesLock(t, c)
}

func TestLoadInvalidArchive(t *testing.T) {
	tmpdir := t.TempDir()

	writeTar := func(filename, internalPath string, body []byte) {
		dest, err := os.Create(filename)
		require.NoError(t, err)
		zipper := gzip.NewWriter(dest)
		tw := tar.NewWriter(zipper)

		h := &tar.Header{
			Name:    internalPath,
			Mode:    0o755,
			Size:    int64(len(body)),
			ModTime: time.Now(),
		}
		require.NoError(t, tw.WriteHeader(h))
		_, err = tw.Write(body)
		require.NoError(t, err)
		tw.Close()
		zipper.Close()
		dest.Close()
	}

	for _, tt := range []struct {
		chartname   string
		internal    string
		expectError string
	}{
		{"illegal-dots.tgz", "../../malformed-helm-test", "chart illegally references parent directory"},
		{"illegal-dots2.tgz", "/foo/../../malformed-helm-test", "chart illegally references parent directory"},
		{"illegal-dots3.tgz", "/../../malformed-helm-test", "chart illegally references parent directory"},
		{"illegal-dots4.tgz", "./../../malformed-helm-test", "chart illegally references parent directory"},
		{"illegal-name.tgz", "./.", "chart illegally contains content outside the base directory"},
		{"illegal-name2.tgz", "/./.", "chart illegally contains content outside the base directory"},
		{"illegal-name3.tgz", "missing-leading-slash", "chart illegally contains content outside the base directory"},
		{"illegal-name4.tgz", "/missing-leading-slash", "Chart.yaml file is missing"},
		{"illegal-abspath.tgz", "//foo", "chart illegally contains absolute paths"},
		{"illegal-abspath2.tgz", "///foo", "chart illegally contains absolute paths"},
		{"illegal-abspath3.tgz", "\\\\foo", "chart illegally contains absolute paths"},
		{"illegal-abspath3.tgz", "\\..\\..\\foo", "chart illegally references parent directory"},

		// Under special circumstances, this can get normalized to things that look like absolute Windows paths
		{"illegal-abspath4.tgz", "\\.\\c:\\\\foo", "chart contains illegally named files"},
		{"illegal-abspath5.tgz", "/./c://foo", "chart contains illegally named files"},
		{"illegal-abspath6.tgz", "\\\\?\\Some\\windows\\magic", "chart illegally contains absolute paths"},
	} {
		illegalChart := filepath.Join(tmpdir, tt.chartname)
		writeTar(illegalChart, tt.internal, []byte("hello: world"))
		_, err := Load(illegalChart)
		require.Error(t, err, "expected error when unpacking illegal files")
		require.ErrorContains(t, err, tt.expectError, "Expected error to contain %q, got %q for %s", tt.expectError, err.Error(), tt.chartname)
	}

	// Make sure that absolute path gets interpreted as relative
	illegalChart := filepath.Join(tmpdir, "abs-path.tgz")
	writeTar(illegalChart, "/Chart.yaml", []byte("hello: world"))
	_, err := Load(illegalChart)
	require.EqualError(t, err, "validation: chart.metadata.name is required")

	// And just to validate that the above was not spurious
	illegalChart = filepath.Join(tmpdir, "abs-path2.tgz")
	writeTar(illegalChart, "files/whatever.yaml", []byte("hello: world"))
	_, err = Load(illegalChart)
	require.EqualError(t, err, "Chart.yaml file is missing")

	// Finally, test that drive letter gets stripped off on Windows
	illegalChart = filepath.Join(tmpdir, "abs-winpath.tgz")
	writeTar(illegalChart, "c:\\Chart.yaml", []byte("hello: world"))
	_, err = Load(illegalChart)
	assert.EqualError(t, err, "validation: chart.metadata.name is required")
}

func TestLoadValues(t *testing.T) {
	testCases := map[string]struct {
		data          []byte
		expctedValues map[string]any
	}{
		"It should load values correctly": {
			data: []byte(`
foo:
  image: foo:v1
bar:
  version: v2
`),
			expctedValues: map[string]any{
				"foo": map[string]any{
					"image": "foo:v1",
				},
				"bar": map[string]any{
					"version": "v2",
				},
			},
		},
		"It should load values correctly with multiple documents in one file": {
			data: []byte(`
foo:
  image: foo:v1
bar:
  version: v2
---
foo:
  image: foo:v2
`),
			expctedValues: map[string]any{
				"foo": map[string]any{
					"image": "foo:v2",
				},
				"bar": map[string]any{
					"version": "v2",
				},
			},
		},
	}
	for testName, testCase := range testCases {
		t.Run(testName, func(tt *testing.T) {
			values, err := LoadValues(bytes.NewReader(testCase.data))
			require.NoError(tt, err)
			assert.Equalf(tt, testCase.expctedValues, values, "Expected values: %v, got %v", testCase.expctedValues, values)
		})
	}
}

func TestMergeValuesV3(t *testing.T) {
	nestedMap := map[string]any{
		"foo": "bar",
		"baz": map[string]string{
			"cool": "stuff",
		},
	}
	anotherNestedMap := map[string]any{
		"foo": "bar",
		"baz": map[string]string{
			"cool":    "things",
			"awesome": "stuff",
		},
	}
	flatMap := map[string]any{
		"foo": "bar",
		"baz": "stuff",
	}
	anotherFlatMap := map[string]any{
		"testing": "fun",
	}

	testMap := MergeMaps(flatMap, nestedMap)
	assert.Equal(t, testMap, nestedMap, "Expected a nested map to overwrite a flat value. Expected: %v, got %v", nestedMap, testMap)

	testMap = MergeMaps(nestedMap, flatMap)
	assert.Equal(t, testMap, flatMap, "Expected a flat value to overwrite a map. Expected: %v, got %v", flatMap, testMap)

	testMap = MergeMaps(nestedMap, anotherNestedMap)
	assert.Equal(t, testMap, anotherNestedMap, "Expected a nested map to overwrite another nested map. Expected: %v, got %v", anotherNestedMap, testMap)

	testMap = MergeMaps(anotherFlatMap, anotherNestedMap)
	expectedMap := map[string]any{
		"testing": "fun",
		"foo":     "bar",
		"baz": map[string]string{
			"cool":    "things",
			"awesome": "stuff",
		},
	}
	assert.Equal(t, expectedMap, testMap, "Expected a map with different keys to merge properly with another map. Expected: %v, got %v", expectedMap, testMap)
}

func verifyChart(t *testing.T, c *chart.Chart) {
	t.Helper()
	require.NotEmpty(t, c.Name(), "No chart metadata found on %v", c)
	t.Logf("Verifying chart %s", c.Name())
	assert.Len(t, c.Templates, 1, "Expected 1 template")

	numfiles := 6
	if !assert.Len(t, c.Files, numfiles, "Expected %d extra files", numfiles) {
		for _, n := range c.Files {
			t.Logf("\t%s", n.Name)
		}
	}

	if !assert.Len(t, c.Dependencies(), 2, "Expected 2 dependencies") {
		for _, d := range c.Dependencies() {
			t.Logf("\tSubchart: %s\n", d.Name())
		}
	}

	expect := map[string]map[string]string{
		"alpine": {
			"version": "0.1.0",
		},
		"mariner": {
			"version": "4.3.2",
		},
	}

	for _, dep := range c.Dependencies() {
		require.NotNil(t, dep.Metadata, "expected metadata on dependency: %v", dep)
		exp, ok := expect[dep.Name()]
		require.True(t, ok, "Unknown dependency %s", dep.Name())
		assert.Equal(t, exp["version"], dep.Metadata.Version, "Expected %s version %s, got %s", dep.Name(), exp["version"], dep.Metadata.Version)
	}
}

func verifyDependencies(t *testing.T, c *chart.Chart) {
	t.Helper()
	assert.Len(t, c.Metadata.Dependencies, 2, "Expected 2 dependencies")
	tests := []*chart.Dependency{
		{Name: "alpine", Version: "0.1.0", Repository: "https://example.com/charts"},
		{Name: "mariner", Version: "4.3.2", Repository: "https://example.com/charts"},
	}
	for i, tt := range tests {
		d := c.Metadata.Dependencies[i]
		assert.Equal(t, tt.Name, d.Name, "Expected dependency named %q, got %q", tt.Name, d.Name)
		assert.Equal(t, tt.Version, d.Version, "Expected dependency named %q to have version %q, got %q", tt.Name, tt.Version, d.Version)
		assert.Equal(t, tt.Repository, d.Repository, "Expected dependency named %q to have repository %q, got %q", tt.Name, tt.Repository, d.Repository)
	}
}

func verifyDependenciesLock(t *testing.T, c *chart.Chart) {
	t.Helper()
	assert.Len(t, c.Metadata.Dependencies, 2, "Expected 2 dependencies, got %d", len(c.Metadata.Dependencies))
	tests := []*chart.Dependency{
		{Name: "alpine", Version: "0.1.0", Repository: "https://example.com/charts"},
		{Name: "mariner", Version: "4.3.2", Repository: "https://example.com/charts"},
	}
	for i, tt := range tests {
		d := c.Metadata.Dependencies[i]
		assert.Equal(t, tt.Name, d.Name, "Expected dependency named %q, got %q", tt.Name, d.Name)
		assert.Equal(t, tt.Version, d.Version, "Expected dependency named %q to have version %q, got %q", tt.Name, tt.Version, d.Version)
		assert.Equal(t, tt.Repository, d.Repository, "Expected dependency named %q to have repository %q, got %q", tt.Name, tt.Repository, d.Repository)
	}
}

func verifyFrobnitz(t *testing.T, c *chart.Chart) {
	t.Helper()
	verifyChartFileAndTemplate(t, c, "frobnitz")
}

func verifyChartFileAndTemplate(t *testing.T, c *chart.Chart, name string) {
	t.Helper()
	require.NotNil(t, c.Metadata, "Metadata is nil")
	assert.Equal(t, name, c.Name(), "Expected %s, got %s", name, c.Name())
	require.Len(t, c.Templates, 1, "Expected 1 template, got %d", len(c.Templates))
	assert.Equal(t, "templates/template.tpl", c.Templates[0].Name, "Unexpected template: %s", c.Templates[0].Name)
	assert.NotEmpty(t, c.Templates[0].Data, "No template data.")
	require.Len(t, c.Files, 6, "Expected 6 Files, got %d", len(c.Files))
	require.Len(t, c.Dependencies(), 2, "Expected 2 Dependency, got %d", len(c.Dependencies()))
	require.Len(t, c.Metadata.Dependencies, 2, "Expected 2 Dependencies.Dependency, got %d", len(c.Metadata.Dependencies))
	require.Len(t, c.Lock.Dependencies, 2, "Expected 2 Lock.Dependency, got %d", len(c.Lock.Dependencies))

	for _, dep := range c.Dependencies() {
		switch dep.Name() {
		case "mariner":
		case "alpine":
			require.Len(t, dep.Templates, 1, "Expected 1 template, got %d", len(dep.Templates))
			assert.Equal(t, "templates/alpine-pod.yaml", dep.Templates[0].Name, "Unexpected template: %s", dep.Templates[0].Name)
			assert.NotEmpty(t, dep.Templates[0].Data, "No template data.")
			require.Len(t, dep.Files, 1, "Expected 1 Files, got %d", len(dep.Files))
			require.Len(t, dep.Dependencies(), 2, "Expected 2 Dependency, got %d", len(dep.Dependencies()))
		default:
			t.Errorf("Unexpected dependency %s", dep.Name())
		}
	}
}

func verifyBomStripped(t *testing.T, files []*common.File) {
	t.Helper()
	for _, file := range files {
		assert.Falsef(t, bytes.HasPrefix(file.Data, utf8bom), "Byte Order Mark still present in processed file %s", file.Name)
	}
}
