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
	"runtime"
	"testing"
)

func TestDependencyListCmd(t *testing.T) {
	noSuchChart := cmdTestCase{
		name:      "No such chart",
		cmd:       "dependency list /no/such/chart",
		golden:    "output/dependency-list-no-chart-linux.txt",
		wantError: true,
	}

	noDependencies := cmdTestCase{
		name:   "No dependencies",
		cmd:    "dependency list testdata/testcharts/alpine",
		golden: "output/dependency-list-no-requirements-linux.txt",
	}

	if runtime.GOOS == "windows" {
		noSuchChart.golden = "output/dependency-list-no-chart-windows.txt"
		noDependencies.golden = "output/dependency-list-no-requirements-windows.txt"
	}

	tests := []cmdTestCase{noSuchChart,
		noDependencies, {
			name:   "Dependencies in chart dir",
			cmd:    "dependency list testdata/testcharts/reqtest",
			golden: "output/dependency-list.txt",
		}, {
			name:   "Dependencies in chart archive",
			cmd:    "dependency list testdata/testcharts/reqtest-0.1.0.tgz",
			golden: "output/dependency-list-archive.txt",
		}}
	runTestCmd(t, tests)
}
func TestList(t *testing.T) {
	tests := []cmdTestCase{{
		name:   "list deps of chart with compressed dependencies",
		cmd:    "dep list testdata/testcharts/chart-with-compressed-dependencies",
		golden: "output/dependency-list-compressed.txt",
	}, {
		name:   "list deps of archived chart with compressed dependencies",
		cmd:    "dep list testdata/testcharts/chart-with-compressed-dependencies-2.1.8.tgz",
		golden: "output/dependency-list-compressed-tgz.txt",
	}, {
		name:   "list deps of chart with uncompressed dependencies",
		cmd:    "dep list testdata/testcharts/chart-with-uncompressed-dependencies",
		golden: "output/dependency-list-uncompressed.txt",
	}, {
		name:   "list deps of archived chart with uncompressed dependencies",
		cmd:    "dep list testdata/testcharts/chart-with-uncompressed-dependencies-2.1.8.tgz",
		golden: "output/dependency-list-uncompressed-tgz.txt",
	}, {
		name:   "list deps of chart with missing dependencies",
		cmd:    "dep list testdata/testcharts/chart-missing-deps",
		golden: "output/dependency-list-missing.txt",
	}}

	runTestCmd(t, tests)
}
