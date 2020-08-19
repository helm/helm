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
	"testing"
)

func TestLintCmdWithSubchartsFlag(t *testing.T) {
	testChart := "testdata/testcharts/chart-with-bad-subcharts"
	tests := []cmdTestCase{{
		name:      "lint good chart with bad subcharts",
		cmd:       fmt.Sprintf("lint %s", testChart),
		golden:    "output/lint-chart-with-bad-subcharts.txt",
		wantError: true,
	}, {
		name:      "lint good chart with bad subcharts using --with-subcharts flag",
		cmd:       fmt.Sprintf("lint --with-subcharts %s", testChart),
		golden:    "output/lint-chart-with-bad-subcharts-with-subcharts.txt",
		wantError: true,
	}}
	runTestCmd(t, tests)
}

func TestLintFileCompletion(t *testing.T) {
	checkFileCompletion(t, "lint", true)
	checkFileCompletion(t, "lint mypath", true) // Multiple paths can be given
}
