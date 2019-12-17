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
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"helm.sh/helm/v3/pkg/chart"
)

func TestLoadDir(t *testing.T) {
	l, err := Loader("testdata/frobnitz")
	if err != nil {
		t.Fatalf("Failed to load testdata: %s", err)
	}
	c, err := l.Load()
	if err != nil {
		t.Fatalf("Failed to load testdata: %s", err)
	}
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
	if err != nil {
		t.Fatalf("Failed to load testdata: %s", err)
	}
	if _, err := l.Load(); err == nil {
		t.Errorf("packages with an irregular file (/dev/null) should not load")
	}
}

func TestLoadDirWithSymlink(t *testing.T) {
	sym := filepath.Join("..", "LICENSE")
	link := filepath.Join("testdata", "frobnitz_with_symlink", "LICENSE")

	if err := os.Symlink(sym, link); err != nil {
		t.Fatal(err)
	}

	defer os.Remove(link)

	l, err := Loader("testdata/frobnitz_with_symlink")
	if err != nil {
		t.Fatalf("Failed to load testdata: %s", err)
	}

	c, err := l.Load()
	if err != nil {
		t.Fatalf("Failed to load testdata: %s", err)
	}
	verifyFrobnitz(t, c)
	verifyChart(t, c)
	verifyDependencies(t, c)
	verifyDependenciesLock(t, c)
}

func TestLoadV1(t *testing.T) {
	l, err := Loader("testdata/frobnitz.v1")
	if err != nil {
		t.Fatalf("Failed to load testdata: %s", err)
	}
	c, err := l.Load()
	if err != nil {
		t.Fatalf("Failed to load testdata: %s", err)
	}
	verifyDependencies(t, c)
	verifyDependenciesLock(t, c)
}

func TestLoadFileV1(t *testing.T) {
	l, err := Loader("testdata/frobnitz.v1.tgz")
	if err != nil {
		t.Fatalf("Failed to load testdata: %s", err)
	}
	c, err := l.Load()
	if err != nil {
		t.Fatalf("Failed to load testdata: %s", err)
	}
	verifyDependencies(t, c)
	verifyDependenciesLock(t, c)
}

func TestLoadFile(t *testing.T) {
	l, err := Loader("testdata/frobnitz-1.2.3.tgz")
	if err != nil {
		t.Fatalf("Failed to load testdata: %s", err)
	}
	c, err := l.Load()
	if err != nil {
		t.Fatalf("Failed to load testdata: %s", err)
	}
	verifyFrobnitz(t, c)
	verifyChart(t, c)
	verifyDependencies(t, c)
}

func TestLoadFiles(t *testing.T) {
	goodFiles := []*BufferedFile{
		{
			Name: "Chart.yaml",
			Data: []byte(`apiVersion: v1
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
			Name: "values.yaml",
			Data: []byte("var: some values"),
		},
		{
			Name: "values.schema.json",
			Data: []byte("type: Values"),
		},
		{
			Name: "templates/deployment.yaml",
			Data: []byte("some deployment"),
		},
		{
			Name: "templates/service.yaml",
			Data: []byte("some service"),
		},
	}

	c, err := LoadFiles(goodFiles)
	if err != nil {
		t.Errorf("Expected good files to be loaded, got %v", err)
	}

	if c.Name() != "frobnitz" {
		t.Errorf("Expected chart name to be 'frobnitz', got %s", c.Name())
	}

	if c.Values["var"] != "some values" {
		t.Error("Expected chart values to be populated with default values")
	}

	if len(c.Raw) != 5 {
		t.Errorf("Expected %d files, got %d", 5, len(c.Raw))
	}

	if !bytes.Equal(c.Schema, []byte("type: Values")) {
		t.Error("Expected chart schema to be populated with default values")
	}

	if len(c.Templates) != 2 {
		t.Errorf("Expected number of templates == 2, got %d", len(c.Templates))
	}

	if _, err = LoadFiles([]*BufferedFile{}); err == nil {
		t.Fatal("Expected err to be non-nil")
	}
	if err.Error() != "validation: chart.metadata is required" {
		t.Errorf("Expected chart metadata missing error, got '%s'", err.Error())
	}
}

// Packaging the chart on a Windows machine will produce an
// archive that has \\ as delimiters. Test that we support these archives
func TestLoadFileBackslash(t *testing.T) {
	c, err := Load("testdata/frobnitz_backslash-1.2.3.tgz")
	if err != nil {
		t.Fatalf("Failed to load testdata: %s", err)
	}
	verifyChartFileAndTemplate(t, c, "frobnitz_backslash")
	verifyChart(t, c)
	verifyDependencies(t, c)
}

func TestLoadV2WithReqs(t *testing.T) {
	l, err := Loader("testdata/frobnitz.v2.reqs")
	if err != nil {
		t.Fatalf("Failed to load testdata: %s", err)
	}
	c, err := l.Load()
	if err != nil {
		t.Fatalf("Failed to load testdata: %s", err)
	}
	verifyDependencies(t, c)
	verifyDependenciesLock(t, c)
}

func TestLoadInvalidArchive(t *testing.T) {
	tmpdir, err := ioutil.TempDir("", "helm-test-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpdir)

	writeTar := func(filename, internalPath string, body []byte) {
		dest, err := os.Create(filename)
		if err != nil {
			t.Fatal(err)
		}
		zipper := gzip.NewWriter(dest)
		tw := tar.NewWriter(zipper)

		h := &tar.Header{
			Name:    internalPath,
			Mode:    0755,
			Size:    int64(len(body)),
			ModTime: time.Now(),
		}
		if err := tw.WriteHeader(h); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(body); err != nil {
			t.Fatal(err)
		}
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
		{"illegal-name4.tgz", "/missing-leading-slash", "validation: chart.metadata is required"},
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
		_, err = Load(illegalChart)
		if err == nil {
			t.Fatal("expected error when unpacking illegal files")
		}
		if !strings.Contains(err.Error(), tt.expectError) {
			t.Errorf("Expected error to contain %q, got %q for %s", tt.expectError, err.Error(), tt.chartname)
		}
	}

	// Make sure that absolute path gets interpreted as relative
	illegalChart := filepath.Join(tmpdir, "abs-path.tgz")
	writeTar(illegalChart, "/Chart.yaml", []byte("hello: world"))
	_, err = Load(illegalChart)
	if err.Error() != "validation: chart.metadata.name is required" {
		t.Error(err)
	}

	// And just to validate that the above was not spurious
	illegalChart = filepath.Join(tmpdir, "abs-path2.tgz")
	writeTar(illegalChart, "files/whatever.yaml", []byte("hello: world"))
	_, err = Load(illegalChart)
	if err.Error() != "validation: chart.metadata is required" {
		t.Error(err)
	}

	// Finally, test that drive letter gets stripped off on Windows
	illegalChart = filepath.Join(tmpdir, "abs-winpath.tgz")
	writeTar(illegalChart, "c:\\Chart.yaml", []byte("hello: world"))
	_, err = Load(illegalChart)
	if err.Error() != "validation: chart.metadata.name is required" {
		t.Error(err)
	}
}

func verifyChart(t *testing.T, c *chart.Chart) {
	t.Helper()
	if c.Name() == "" {
		t.Fatalf("No chart metadata found on %v", c)
	}
	t.Logf("Verifying chart %s", c.Name())
	if len(c.Templates) != 1 {
		t.Errorf("Expected 1 template, got %d", len(c.Templates))
	}

	numfiles := 6
	if len(c.Files) != numfiles {
		t.Errorf("Expected %d extra files, got %d", numfiles, len(c.Files))
		for _, n := range c.Files {
			t.Logf("\t%s", n.Name)
		}
	}

	if len(c.Dependencies()) != 2 {
		t.Errorf("Expected 2 dependencies, got %d (%v)", len(c.Dependencies()), c.Dependencies())
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
		if dep.Metadata == nil {
			t.Fatalf("expected metadata on dependency: %v", dep)
		}
		exp, ok := expect[dep.Name()]
		if !ok {
			t.Fatalf("Unknown dependency %s", dep.Name())
		}
		if exp["version"] != dep.Metadata.Version {
			t.Errorf("Expected %s version %s, got %s", dep.Name(), exp["version"], dep.Metadata.Version)
		}
	}

}

func verifyDependencies(t *testing.T, c *chart.Chart) {
	if len(c.Metadata.Dependencies) != 2 {
		t.Errorf("Expected 2 dependencies, got %d", len(c.Metadata.Dependencies))
	}
	tests := []*chart.Dependency{
		{Name: "alpine", Version: "0.1.0", Repository: "https://example.com/charts"},
		{Name: "mariner", Version: "4.3.2", Repository: "https://example.com/charts"},
	}
	for i, tt := range tests {
		d := c.Metadata.Dependencies[i]
		if d.Name != tt.Name {
			t.Errorf("Expected dependency named %q, got %q", tt.Name, d.Name)
		}
		if d.Version != tt.Version {
			t.Errorf("Expected dependency named %q to have version %q, got %q", tt.Name, tt.Version, d.Version)
		}
		if d.Repository != tt.Repository {
			t.Errorf("Expected dependency named %q to have repository %q, got %q", tt.Name, tt.Repository, d.Repository)
		}
	}
}

func verifyDependenciesLock(t *testing.T, c *chart.Chart) {
	if len(c.Metadata.Dependencies) != 2 {
		t.Errorf("Expected 2 dependencies, got %d", len(c.Metadata.Dependencies))
	}
	tests := []*chart.Dependency{
		{Name: "alpine", Version: "0.1.0", Repository: "https://example.com/charts"},
		{Name: "mariner", Version: "4.3.2", Repository: "https://example.com/charts"},
	}
	for i, tt := range tests {
		d := c.Metadata.Dependencies[i]
		if d.Name != tt.Name {
			t.Errorf("Expected dependency named %q, got %q", tt.Name, d.Name)
		}
		if d.Version != tt.Version {
			t.Errorf("Expected dependency named %q to have version %q, got %q", tt.Name, tt.Version, d.Version)
		}
		if d.Repository != tt.Repository {
			t.Errorf("Expected dependency named %q to have repository %q, got %q", tt.Name, tt.Repository, d.Repository)
		}
	}
}

func verifyFrobnitz(t *testing.T, c *chart.Chart) {
	verifyChartFileAndTemplate(t, c, "frobnitz")
}

func verifyChartFileAndTemplate(t *testing.T, c *chart.Chart, name string) {
	if c.Metadata == nil {
		t.Fatal("Metadata is nil")
	}
	if c.Name() != name {
		t.Errorf("Expected %s, got %s", name, c.Name())
	}
	if len(c.Templates) != 1 {
		t.Fatalf("Expected 1 template, got %d", len(c.Templates))
	}
	if c.Templates[0].Name != "templates/template.tpl" {
		t.Errorf("Unexpected template: %s", c.Templates[0].Name)
	}
	if len(c.Templates[0].Data) == 0 {
		t.Error("No template data.")
	}
	if len(c.Files) != 6 {
		t.Fatalf("Expected 6 Files, got %d", len(c.Files))
	}
	if len(c.Dependencies()) != 2 {
		t.Fatalf("Expected 2 Dependency, got %d", len(c.Dependencies()))
	}
	if len(c.Metadata.Dependencies) != 2 {
		t.Fatalf("Expected 2 Dependencies.Dependency, got %d", len(c.Metadata.Dependencies))
	}
	if len(c.Lock.Dependencies) != 2 {
		t.Fatalf("Expected 2 Lock.Dependency, got %d", len(c.Lock.Dependencies))
	}

	for _, dep := range c.Dependencies() {
		switch dep.Name() {
		case "mariner":
		case "alpine":
			if len(dep.Templates) != 1 {
				t.Fatalf("Expected 1 template, got %d", len(dep.Templates))
			}
			if dep.Templates[0].Name != "templates/alpine-pod.yaml" {
				t.Errorf("Unexpected template: %s", dep.Templates[0].Name)
			}
			if len(dep.Templates[0].Data) == 0 {
				t.Error("No template data.")
			}
			if len(dep.Files) != 1 {
				t.Fatalf("Expected 1 Files, got %d", len(dep.Files))
			}
			if len(dep.Dependencies()) != 2 {
				t.Fatalf("Expected 2 Dependency, got %d", len(dep.Dependencies()))
			}
		default:
			t.Errorf("Unexpected dependeny %s", dep.Name())
		}
	}
}
