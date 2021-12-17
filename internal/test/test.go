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
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

// UpdateGolden writes out the golden files with the latest values, rather than failing the test.
var updateGolden = flag.Bool("update", false, "update golden files")

// TestingT describes a testing object compatible with the critical functions from the testing.T type
type TestingT interface {
	Errorf(format string, args ...interface{})
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

// AssertGoldenStringWithCustomLineValidation asserts that the given string matches the contents of the given file, using the given function to check each line.
// It is useful when the output is expected to contain information that cannot be predicted, such as timestamps.
//
// The line validation function must return a pair of values representing, respectively,
//
// 1. whether the expected line is "special" or not, and
//
// 2. if the expected line is special, the validity of the actual line.
//
// "Not special" means that the actual line must be exactly equal to the expected line to be considered valid.
func AssertGoldenStringWithCustomLineValidation(t TestingT, checkLine func(expected, actual string) (bool, error)) func(actualOutput, expectedFilename string) {
	t.Helper()
	is := assert.New(t)
	return func(actualOutput, expectedFilename string) {
		expectedOutput, err := ioutil.ReadFile(path(expectedFilename))
		if err != nil {
			t.Fatalf("%v", err)
		}
		expectedLines := lines(expectedOutput)
		actualLines := lines([]byte(actualOutput))
		expectedLineCount := len(expectedLines)
		actualLineCount := len(actualLines)
		for i := 0; i < max(expectedLineCount, actualLineCount); i++ {
			lineNumber := i + 1
			// We need to prevent index-out-of-range errors if the number of lines doesn't match between the expected and the actual output.
			// But we cannot just use the empty string as a default value, because that's equivalent to downright ignoring trailing empty lines.
			if lineNumber > expectedLineCount {
				t.Errorf("Output should only have %d line(s), but has %d. Line %d is: %q", expectedLineCount, actualLineCount, lineNumber, actualLines[i])
			} else if lineNumber > actualLineCount {
				t.Errorf("Output should have %d line(s), but has only %d. Line %d should have been: %q", expectedLineCount, actualLineCount, lineNumber, expectedLines[i])
			} else {
				actualLine := actualLines[i]
				expectedLine := expectedLines[i]
				if isSpecialLine, err := checkLine(expectedLine, actualLine); isSpecialLine {
					if err != nil {
						t.Errorf("Unexpected content on line %d (%v): %s", lineNumber, err.Error(), actualLine)
					}
				} else {
					is.Equal(expectedLine, actualLine, fmt.Sprintf("Line %d in the actual output does not match line %d in the expected output (%s).", lineNumber, lineNumber, expectedFilename))
				}
			}
		}
	}
}

func lines(raw []byte) []string {
	return strings.Split(strings.TrimSuffix(string(normalize(raw)), "\n"), "\n") // We first remove the final newline (if any), so that e.g. the 2-line string "a\nb\n" is mapped to ["a", "b"] and not ["a", "b", ""].
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

func max(x, y int) int {
	if x > y {
		return x
	}
	return y
}
