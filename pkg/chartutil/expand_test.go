package chartutil

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func TestExpand(t *testing.T) {
	dest, err := ioutil.TempDir("", "helm-testing-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dest)

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

	expectLen := 12
	if len(fis) != expectLen {
		t.Errorf("Expected %d files, but got %d", expectLen, len(fis))
	}

	for _, fi := range fis {
		expect, err := os.Stat(filepath.Join("testdata", "frobnitz", fi.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if fi.Size() != expect.Size() {
			t.Errorf("Expected %s to have size %d, got %d", fi.Name(), expect.Size(), fi.Size())
		}
	}
}

func TestExpandFile(t *testing.T) {
	dest, err := ioutil.TempDir("", "helm-testing-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dest)

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

	expectLen := 12
	if len(fis) != expectLen {
		t.Errorf("Expected %d files, but got %d", expectLen, len(fis))
	}

	for _, fi := range fis {
		expect, err := os.Stat(filepath.Join("testdata", "frobnitz", fi.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if fi.Size() != expect.Size() {
			t.Errorf("Expected %s to have size %d, got %d", fi.Name(), expect.Size(), fi.Size())
		}
	}
}
