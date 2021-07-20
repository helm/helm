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
	"bytes"
	"crypto/md5"
	"encoding/hex"
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
	chartTestDir := "testdata/helmchart"
	expectMd5Value := "218a88e89fa53efc6dd56aab27159880"
	expectChartBytesLen := 3998

	chartBytes, err := CompressDirToTgz(chartTestDir, testDataDir)
	if err != nil {
		t.Fatal(err)
	}
	chartBytesLen := chartBytes.Len()
	hash := md5.Sum(chartBytes.Bytes())
	currentMd5Value := hex.EncodeToString(hash[:])
	if currentMd5Value != expectMd5Value {
		t.Fatalf("Expect md5 %s, but get md5 %s, len %d", expectMd5Value, currentMd5Value, chartBytesLen)
	}
	if chartBytesLen != expectChartBytesLen {
		t.Fatalf("Expect chartBytesLen %d, but get %d", expectChartBytesLen, chartBytesLen)
	}
}
