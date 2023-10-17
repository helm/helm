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

package chartutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExpand(t *testing.T) {
	dest := t.TempDir()

	reader, err := os.Open("testdata/frobnitz-1.2.3.tgz")
	if err != nil {
		t.Fatal(err)
	}

	if err := Expand(dest, reader); err != nil {
		t.Fatal(err)
	}

	expectedChartPath := filepath.Join(dest, "frobnitz")
	fi, err := os.Stat(expectedChartPath)
	if err != nil {
		t.Fatal(err)
	}
	if !fi.IsDir() {
		t.Fatalf("expected a chart directory at %s", expectedChartPath)
	}

	dir, err := os.Open(expectedChartPath)
	if err != nil {
		t.Fatal(err)
	}

	fis, err := dir.Readdir(0)
	if err != nil {
		t.Fatal(err)
	}

	expectLen := 11
	if len(fis) != expectLen {
		t.Errorf("Expected %d files, but got %d", expectLen, len(fis))
	}

	for _, fi := range fis {
		expect, err := os.Stat(filepath.Join("testdata", "frobnitz", fi.Name()))
		if err != nil {
			t.Fatal(err)
		}
		// os.Stat can return different values for directories, based on the OS
		// for Linux, for example, os.Stat alwaty returns the size of the directory
		// (value-4096) regardless of the size of the contents of the directory
		mode := expect.Mode()
		if !mode.IsDir() {
			if fi.Size() != expect.Size() {
				t.Errorf("Expected %s to have size %d, got %d", fi.Name(), expect.Size(), fi.Size())
			}
		}
	}
}

func TestExpandFile(t *testing.T) {
	dest := t.TempDir()

	if err := ExpandFile(dest, "testdata/frobnitz-1.2.3.tgz"); err != nil {
		t.Fatal(err)
	}

	expectedChartPath := filepath.Join(dest, "frobnitz")
	fi, err := os.Stat(expectedChartPath)
	if err != nil {
		t.Fatal(err)
	}
	if !fi.IsDir() {
		t.Fatalf("expected a chart directory at %s", expectedChartPath)
	}

	dir, err := os.Open(expectedChartPath)
	if err != nil {
		t.Fatal(err)
	}

	fis, err := dir.Readdir(0)
	if err != nil {
		t.Fatal(err)
	}

	expectLen := 11
	if len(fis) != expectLen {
		t.Errorf("Expected %d files, but got %d", expectLen, len(fis))
	}

	for _, fi := range fis {
		expect, err := os.Stat(filepath.Join("testdata", "frobnitz", fi.Name()))
		if err != nil {
			t.Fatal(err)
		}
		// os.Stat can return different values for directories, based on the OS
		// for Linux, for example, os.Stat alwaty returns the size of the directory
		// (value-4096) regardless of the size of the contents of the directory
		mode := expect.Mode()
		if !mode.IsDir() {
			if fi.Size() != expect.Size() {
				t.Errorf("Expected %s to have size %d, got %d", fi.Name(), expect.Size(), fi.Size())
			}
		}
	}
}
