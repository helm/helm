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
	"github.com/stretchr/testify/assert"
)

// UpdateGolden writes out the golden files with the latest values, rather than failing the test.
var updateGolden = flag.Bool("update", false, "update golden files")

// TestingT describes a testing object compatible with the critical functions from the testing.T type
type TestingT interface {
	Fatal(...interface{})
	Fatalf(string, ...interface{})
	Errorf(string, ...interface{})
	HelperT
}

// HelperT describes a test with a helper function
type HelperT interface {
	Helper()
}

// GoldenFile executes tests that run against canonical "golden" test files
type GoldenFile struct {
	t        TestingT
	isUpdate bool
	is       *assert.Assertions
}

func New(t TestingT) *GoldenFile {
	return &GoldenFile{
		t:        t,
		isUpdate: *updateGolden,
		is:       assert.New(t),
	}
}

func (g *GoldenFile) AssertString(actual, filename string) {
	g.t.Helper()
	g.compareStr(actual, path(filename))
}

func (g *GoldenFile) compareStr(actual string, filename string) {
	if err := g.update(filename, []byte(actual)); err != nil {
		g.t.Fatal(err)
	}

	expected, err := ioutil.ReadFile(filename)
	if err != nil {
		g.t.Fatalf("unable to read testdata %s: %s", filename, err)
	}
	g.is.Equal(string(expected), actual, "does not match golden file %s", filename)
}

func (g *GoldenFile) update(filename string, in []byte) error {
	if !*updateGolden {
		return nil
	}
	return ioutil.WriteFile(filename, normalize(in), 0666)
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
	New(t).AssertString(actual, filename)
	/*
		t.Helper()

		if err := compare([]byte(actual), path(filename)); err != nil {
			t.Fatalf("%v", err)
		}
	*/
}

// AssertGolden compares the given actual data with the contents of a file.AssertGolden//
//
// Mismatches produce diffs of actual vs desired.
func AssertGolden(t TestingT, actual, filename string) {
	t.Helper()
	is := assert.New(t)

	expected, err := ioutil.ReadFile(filename)
	if err != nil {
		t.Fatalf("unable to read testdata %s: %s", filename, err)
	}
	// This generates diffs and similarly helpful output for us.
	is.Equal(string(expected), actual, "does not match golden file %s", filename)
}

func path(filename string) string {
	if filepath.IsAbs(filename) {
		return filename
	}
	return filepath.Join("testdata", filename)
}

func compare(actual []byte, filename string) error {
	if err := update(filename, actual); err != nil {
		return err
	}

	expected, err := ioutil.ReadFile(filename)
	if err != nil {
		return errors.Wrapf(err, "unable to read testdata %s", filename)
	}
	if !bytes.Equal(expected, actual) {
		return errors.Errorf("does not match golden file %s\n\nWANT:\n%s\n\nGOT:\n%s\n", filename, expected, actual)
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
