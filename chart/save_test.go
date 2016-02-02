/*
Copyright 2015 The Kubernetes Authors All rights reserved.

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

package chart

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

const sprocketdir = "testdata/sprocket"

func TestSave(t *testing.T) {

	tmpdir, err := ioutil.TempDir("", "helm-")
	if err != nil {
		t.Fatal("Could not create temp directory")
	}
	t.Logf("Temp: %s", tmpdir)
	// Because of the defer, don't call t.Fatal in the remainder of this
	// function.
	defer os.RemoveAll(tmpdir)

	c, err := LoadDir(sprocketdir)
	if err != nil {
		t.Errorf("Failed to load %s: %s", sprocketdir, err)
		return
	}

	tfile, err := Save(c, tmpdir)
	if err != nil {
		t.Errorf("Failed to save %s to %s: %s", c.Chartfile().Name, tmpdir, err)
		return
	}

	b := filepath.Base(tfile)
	expectname := "sprocket-1.2.3-alpha.1+12345.tgz"
	if b != expectname {
		t.Errorf("Expected %q, got %q", expectname, b)
	}

	files, err := getAllFiles(tfile)
	if err != nil {
		t.Errorf("Could not extract files: %s", err)
	}

	// Files should come back in order.
	expect := []string{
		"sprocket",
		"sprocket/Chart.yaml",
		"sprocket/LICENSE",
		"sprocket/README.md",
		"sprocket/docs",
		"sprocket/docs/README.md",
		"sprocket/hooks",
		"sprocket/hooks/pre-install.py",
		"sprocket/icon.svg",
		"sprocket/templates",
		"sprocket/templates/placeholder.txt",
	}
	if len(expect) != len(files) {
		t.Errorf("Expected %d files, found %d", len(expect), len(files))
		return
	}
	for i := 0; i < len(expect); i++ {
		if expect[i] != files[i] {
			t.Errorf("Expected file %q, got %q", expect[i], files[i])
		}
	}
}

func getAllFiles(tfile string) ([]string, error) {
	f1, err := os.Open(tfile)
	if err != nil {
		return []string{}, err
	}
	f2, err := gzip.NewReader(f1)
	if err != nil {
		f1.Close()
		return []string{}, err
	}

	if f2.Header.Comment != "Helm" {
		return []string{}, fmt.Errorf("Expected header Helm. Got %s", f2.Header.Comment)
	}
	if string(f2.Header.Extra) != string(headerBytes) {
		return []string{}, fmt.Errorf("Expected header signature. Got %v", f2.Header.Extra)
	}

	f3 := tar.NewReader(f2)

	files := []string{}
	var e error
	var hdr *tar.Header
	for e == nil {
		hdr, e = f3.Next()
		if e == nil {
			files = append(files, hdr.Name)
		}
	}

	f2.Close()
	f1.Close()
	return files, nil
}
