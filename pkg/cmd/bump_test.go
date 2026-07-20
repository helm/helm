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

package cmd

import (
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestBump(t *testing.T) {
	tempChartDir := t.TempDir()

	testChartFile := filepath.Join("testdata", "testcharts", "empty", "Chart.yaml")

	destFile := filepath.Join(tempChartDir, "Chart.yaml")

	srcFile, err := os.Open(testChartFile)
	if err != nil {
		t.Fatalf("error on opening test file: %v", err)
	}
	defer srcFile.Close()

	destFileHandle, err := os.Create(destFile)
	if err != nil {
		t.Fatalf("error on creating test file: %v", err)
	}
	defer destFileHandle.Close()

	_, err = io.Copy(destFileHandle, srcFile)
	if err != nil {
		t.Fatalf("error on copying test file: %v", err)
	}

	tests := []cmdTestCase{{
		name:   "default",
		cmd:    "bump " + tempChartDir,
		golden: "output/bump-default.txt",
	}, {
		name:   "with bump type",
		cmd:    "bump minor " + tempChartDir,
		golden: "output/bump-minor.txt",
	}, {
		name:   "with explicit version",
		cmd:    "bump 2.0.0 " + tempChartDir,
		golden: "output/bump-explicit.txt",
	}}
	runTestCmd(t, tests)
}
