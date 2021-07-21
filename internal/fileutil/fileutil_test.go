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

package fileutil

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func TestAtomicWriteFile(t *testing.T) {
	dir := t.TempDir()

	testpath := filepath.Join(dir, "test")
	stringContent := "Test content"
	reader := bytes.NewReader([]byte(stringContent))
	mode := os.FileMode(0644)

	err := AtomicWriteFile(testpath, reader, mode)
	if err != nil {
		t.Errorf("AtomicWriteFile error: %s", err)
	}

	got, err := ioutil.ReadFile(testpath)
	if err != nil {
		t.Fatal(err)
	}

	if stringContent != string(got) {
		t.Fatalf("expected: %s, got: %s", stringContent, string(got))
	}

	gotinfo, err := os.Stat(testpath)
	if err != nil {
		t.Fatal(err)
	}

	if mode != gotinfo.Mode() {
		t.Fatalf("expected %s: to be the same mode as %s",
			mode, gotinfo.Mode())
	}
}

func TestCompressDirToTgz(t *testing.T) {

	testDataDir := "testdata"
	chartTestDir := "testdata/testdir"

	chartBytes, err := CompressDirToTgz(chartTestDir, testDataDir)
	if err != nil {
		t.Fatal(err)
	}

	// gzip read
	gr, err := gzip.NewReader(chartBytes)
	if err != nil {
		t.Fatal(err)
	}
	defer gr.Close()

	// tar read
	tr := tar.NewReader(gr)
	defer gr.Close()

	found := false
	fileBytes := bytes.NewBuffer(nil)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if hdr.Name == "testdir/testfile" {
			found = true
			_, err := io.Copy(fileBytes, tr)
			if err != nil {
				t.Fatal(err)
			}
		}
	}
	if !found {
		t.Fatal("testdir/testfile not found")
	}
	if !bytes.Equal(fileBytes.Bytes(), []byte("helm")) {
		t.Fatalf("testdir/testfile's content not match, excpcted %s, got %s", "helm", fileBytes.String())
	}
}
