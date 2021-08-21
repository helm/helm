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

package test

import (
	"bytes"
	"flag"
	"io/ioutil"
	"path/filepath"

	"github.com/pkg/errors"
)

// UpdateGolden writes out the golden files with the latest values, rather than failing the test.
var updateGolden = flag.Bool("update", false, "update golden files")

// TestingT describes a testing object compatible with the critical functions from the testing.T type
type TestingT interface {
	Fatal(...interface{})
	Fatalf(string, ...interface{})
	HelperT
}

// HelperT describes a test with a helper function
type HelperT interface {
	Helper()
}

// AssertGoldenBytes asserts that the give actual content matches the contents of the given filename
func AssertGoldenBytes(t TestingT, actual []byte, filename string) {
	t.Helper()

	if err := compare(actual, path(filename)); err != nil {
		t.Fatalf("%v", err)
	}
}

// AssertGoldenString asserts that the given string matches the contents of the given file.
func AssertGoldenString(t TestingT, actual, filename string) {
	t.Helper()

	if err := compare([]byte(actual), path(filename)); err != nil {
		t.Fatalf("%v", err)
	}
}

// AssertGoldenFile asserts that the content of the actual file matches the contents of the expected file
func AssertGoldenFile(t TestingT, actualFileName string, expectedFilename string) {
	t.Helper()

	actual, err := ioutil.ReadFile(actualFileName)
	if err != nil {
		t.Fatalf("%v", err)
	}
	AssertGoldenBytes(t, actual, expectedFilename)
}

func path(filename string) string {
	if filepath.IsAbs(filename) {
		return filename
	}
	return filepath.Join("testdata", filename)
}

func compare(actual []byte, filename string) error {
	actual = normalize(actual)
	if err := update(filename, actual); err != nil {
		return err
	}

	expected, err := ioutil.ReadFile(filename)
	if err != nil {
		return errors.Wrapf(err, "unable to read testdata %s", filename)
	}
	expected = normalize(expected)
	if !bytes.Equal(expected, actual) {
		return errors.Errorf("does not match golden file %s\n\nWANT:\n'%s'\n\nGOT:\n'%s'\n", filename, expected, actual)
	}
	return nil
}

func update(filename string, in []byte) error {
	if !*updateGolden {
		return nil
	}
	return ioutil.WriteFile(filename, normalize(in), 0666)
}

func normalize(in []byte) []byte {
	return bytes.Replace(in, []byte("\r\n"), []byte("\n"), -1)
}
