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

func TestLintCmdWithQuietFlag(t *testing.T) {
	testChart1 := "testdata/testcharts/alpine"
	testChart2 := "testdata/testcharts/chart-bad-requirements"
	tests := []cmdTestCase{{
		name:   "lint good chart using --quiet flag",
		cmd:    fmt.Sprintf("lint --quiet %s", testChart1),
		golden: "output/lint-quiet.txt",
	}, {
		name:      "lint two charts, one with error using --quiet flag",
		cmd:       fmt.Sprintf("lint --quiet %s %s", testChart1, testChart2),
		golden:    "output/lint-quiet-with-error.txt",
		wantError: true,
	}, {
		name:   "lint chart with warning using --quiet flag",
		cmd:    "lint --quiet testdata/testcharts/chart-with-only-crds",
		golden: "output/lint-quiet-with-warning.txt",
	}, {
		name:      "lint non-existent chart using --quiet flag",
		cmd:       "lint --quiet thischartdoesntexist/",
		golden:    "",
		wantError: true,
	}}
	runTestCmd(t, tests)

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

func TestLintCmdRequiresArgs(t *testing.T) {
	tests := []cmdTestCase{{
		name:      "lint without arguments should fail",
		cmd:       "lint",
		wantError: true,
	}}
	runTestCmd(t, tests)
}

func TestLintFileCompletion(t *testing.T) {
	checkFileCompletion(t, "lint", true)
	checkFileCompletion(t, "lint mypath", true) // Multiple paths can be given
}
