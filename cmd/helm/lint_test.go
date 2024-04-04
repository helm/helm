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

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type lintTestCase struct {
	cmdTestCase                cmdTestCase
	updateFullPathInOutputFile bool
}

func TestLintCmdWithSubchartsFlag(t *testing.T) {
	testChart := "testdata/testcharts/chart-with-bad-subcharts"
	tests := []lintTestCase{
		{
			cmdTestCase: cmdTestCase{
				name:      "lint good chart with bad subcharts",
				cmd:       fmt.Sprintf("lint %s", testChart),
				golden:    "output/lint-chart-with-bad-subcharts.txt",
				wantError: true,
			},
		},
		{
			cmdTestCase: cmdTestCase{
				name:      "lint good chart with bad subcharts using --full-path flag",
				cmd:       fmt.Sprintf("lint --full-path %s", testChart),
				golden:    "output/lint-chart-with-bad-subcharts-with-full-paths-enabled.txt",
				wantError: true,
			},
			updateFullPathInOutputFile: true,
		},
		{
			cmdTestCase: cmdTestCase{
				name:      "lint good chart with bad subcharts using --with-subcharts flag",
				cmd:       fmt.Sprintf("lint --with-subcharts %s", testChart),
				golden:    "output/lint-chart-with-bad-subcharts-with-subcharts.txt",
				wantError: true,
			},
		},
		{
			cmdTestCase: cmdTestCase{
				name:      "lint good chart with bad subcharts using --with-subcharts and --full-path flags",
				cmd:       fmt.Sprintf("lint --with-subcharts --full-path %s", testChart),
				golden:    "output/lint-chart-with-bad-subcharts-with-subcharts-with-full-paths-enabled.txt",
				wantError: true,
			},
			updateFullPathInOutputFile: true,
		},
	}

	workingDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to determine present working directory: '%v'", err)
	}

	placeholder := `<full-path>`
	testCases := []cmdTestCase{}

	for _, test := range tests {
		if test.updateFullPathInOutputFile {
			// Copy the content of golden file to a temporary file
			// and replace the placeholder with working directory's
			// path. Replace the golden file's name with it.
			test.cmdTestCase.golden = tempFileWithUpdatedPlaceholder(
				t,
				filepath.Join("testdata", test.cmdTestCase.golden),
				placeholder,
				workingDir,
			)
		}

		testCases = append(testCases, test.cmdTestCase)
	}

	runTestCmd(t, testCases)
}

func TestLintCmdWithQuietFlag(t *testing.T) {
	testChart1 := "testdata/testcharts/alpine"
	testChart2 := "testdata/testcharts/chart-bad-requirements"
	tests := []lintTestCase{
		{
			cmdTestCase: cmdTestCase{
				name:   "lint good chart using --quiet flag",
				cmd:    fmt.Sprintf("lint --quiet %s", testChart1),
				golden: "output/lint-quiet.txt",
			},
		},
		{
			cmdTestCase: cmdTestCase{
				name:      "lint two charts, one with error using --quiet flag",
				cmd:       fmt.Sprintf("lint --quiet %s %s", testChart1, testChart2),
				golden:    "output/lint-quiet-with-error.txt",
				wantError: true,
			},
		},
		{
			cmdTestCase: cmdTestCase{
				name:      "lint two charts, one with error using --quiet & --full-path flags",
				cmd:       fmt.Sprintf("lint --quiet --full-path %s %s", testChart1, testChart2),
				golden:    "output/lint-quiet-with-error-with-full-paths.txt",
				wantError: true,
			},
			updateFullPathInOutputFile: true,
		},
		{
			cmdTestCase: cmdTestCase{
				name:   "lint chart with warning using --quiet flag",
				cmd:    "lint --quiet testdata/testcharts/chart-with-only-crds",
				golden: "output/lint-quiet-with-warning.txt",
			},
		},
		{
			cmdTestCase: cmdTestCase{
				name:      "lint non-existent chart using --quiet flag",
				cmd:       "lint --quiet thischartdoesntexist/",
				golden:    "",
				wantError: true,
			},
		},
	}

	workingDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to determine present working directory: '%v'", err)
	}

	placeholder := `<full-path>`
	testCases := []cmdTestCase{}

	for _, test := range tests {
		if test.updateFullPathInOutputFile {
			// Copy the content of golden file to a temporary file
			// and replace the placeholder with working directory's
			// path. Replace the golden file's name with it.
			test.cmdTestCase.golden = tempFileWithUpdatedPlaceholder(
				t,
				filepath.Join("testdata", test.cmdTestCase.golden),
				placeholder,
				workingDir,
			)
		}

		testCases = append(testCases, test.cmdTestCase)
	}

	runTestCmd(t, testCases)

}

func TestLintCmdWithKubeVersionFlag(t *testing.T) {
	testChart := "testdata/testcharts/chart-with-deprecated-api"
	tests := []cmdTestCase{{
		name:      "lint chart with deprecated api version using kube version flag",
		cmd:       fmt.Sprintf("lint --kube-version 1.22.0 %s", testChart),
		golden:    "output/lint-chart-with-deprecated-api.txt",
		wantError: false,
	}, {
		name:      "lint chart with deprecated api version using kube version and strict flag",
		cmd:       fmt.Sprintf("lint --kube-version 1.22.0 --strict %s", testChart),
		golden:    "output/lint-chart-with-deprecated-api-strict.txt",
		wantError: true,
	}, {
		// the test builds will use the default k8sVersionMinor const in deprecations.go and capabilities.go
		// which is "20"
		name:      "lint chart with deprecated api version without kube version",
		cmd:       fmt.Sprintf("lint %s", testChart),
		golden:    "output/lint-chart-with-deprecated-api-old-k8s.txt",
		wantError: false,
	}, {
		name:      "lint chart with deprecated api version with older kube version",
		cmd:       fmt.Sprintf("lint --kube-version 1.21.0 --strict %s", testChart),
		golden:    "output/lint-chart-with-deprecated-api-old-k8s.txt",
		wantError: false,
	}}
	runTestCmd(t, tests)
}

func TestLintFileCompletion(t *testing.T) {
	checkFileCompletion(t, "lint", true)
	checkFileCompletion(t, "lint mypath", true) // Multiple paths can be given
}

// tempFileWithUpdatedPlaceholder creates a temporary file, copies
// the content of source file to it, and replaces the placeholder
// with input value.
//
// The temporary file automatically gets deleted during test clean-up.
func tempFileWithUpdatedPlaceholder(t *testing.T, src string, placeholder,
	value string) string {
	// Create the temporary file in test's temporary directory.
	// This lets the test delete the directory along with file
	// during the test clean-up step.
	dst, err := os.CreateTemp(t.TempDir(), filepath.Base(src))
	if err != nil {
		t.Fatalf("failed to create temporary destination file: '%v'", err)
	}

	// Read source file's content
	srcData, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("failed to read source file %q: '%v'", src, err)
	}

	// Replace placeholder with input value
	dstData := strings.ReplaceAll(string(srcData), placeholder, value)

	// Write to destination (temporary) file
	_, err = dst.WriteString(dstData)
	if err != nil {
		t.Fatalf("failed to write to temporary destination file %q: '%v'",
			dst.Name(), err)
	}

	// Return temporary file's name
	return dst.Name()
}
