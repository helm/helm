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

func TestDependencyListCmdNew(t *testing.T) {
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

	tests := []cmdTestCase{
		noSuchChart,
		noDependencies, {
			name:   "Dependencies in chart dir",
			cmd:    "dependency list testdata/testcharts/reqtest",
			golden: "output/dependency-list.txt",
		}, {
			name:   "Dependencies in chart archive",
			cmd:    "dependency list testdata/testcharts/reqtest-0.1.0.tgz",
			golden: "output/dependency-list-archive.txt",
		},
		{
			name:   "Dependency list with compressed dependencies from chart folder",
			cmd:    "dep list testdata/testcharts/chart-with-compressed-dependencies",
			golden: "output/dependency-list-compressed-deps.txt",
		},
		{
			name:   "Dependency list with compressed dependencies from chart folder with json output format",
			cmd:    "dep list testdata/testcharts/chart-with-compressed-dependencies --output json",
			golden: "output/dependency-list-compressed-deps.json",
		},
		{
			name:   "Dependency list with compressed dependencies from chart folder with yaml output format",
			cmd:    "dep list testdata/testcharts/chart-with-compressed-dependencies --output yaml",
			golden: "output/dependency-list-compressed-deps.yaml",
		},
		{
			name:   "Dependency list with compressed dependencies from chart archive",
			cmd:    "dep list testdata/testcharts/chart-with-compressed-dependencies-2.1.8.tgz",
			golden: "output/dependency-list-compressed-deps-tgz.txt",
		},
		{
			name:   "Dependency list with compressed dependencies from chart archive with json format output",
			cmd:    "dep list testdata/testcharts/chart-with-compressed-dependencies-2.1.8.tgz --output json",
			golden: "output/dependency-list-compressed-deps-tgz.json",
		},
		{
			name:   "Dependency list with compressed dependencies from chart archive with yaml format output",
			cmd:    "dep list testdata/testcharts/chart-with-compressed-dependencies-2.1.8.tgz --output yaml",
			golden: "output/dependency-list-compressed-deps-tgz.yaml",
		},
		{
			name:   "Dependency list with uncompressed dependencies from chart folder",
			cmd:    "dep list testdata/testcharts/chart-with-uncompressed-dependencies",
			golden: "output/dependency-list-uncompressed-deps.txt",
		},
		{
			name:   "Dependency list with uncompressed dependencies from chart folder with json output format",
			cmd:    "dep list testdata/testcharts/chart-with-uncompressed-dependencies --output json",
			golden: "output/dependency-list-uncompressed-deps.json",
		},
		{
			name:   "Dependency list with uncompressed dependencies from chart folder with yaml output format",
			cmd:    "dep list testdata/testcharts/chart-with-uncompressed-dependencies --output yaml",
			golden: "output/dependency-list-uncompressed-deps.yaml",
		},
		{
			name:   "Dependency list with uncompressed dependencies from chart archive",
			cmd:    "dep list testdata/testcharts/chart-with-uncompressed-dependencies-2.1.8.tgz",
			golden: "output/dependency-list-uncompressed-deps-tgz.txt",
		},
		{
			name:   "Dependency list with uncompressed dependencies from chart archive with json output format",
			cmd:    "dep list testdata/testcharts/chart-with-uncompressed-dependencies-2.1.8.tgz --output json",
			golden: "output/dependency-list-uncompressed-deps-tgz.json",
		},
		{
			name:   "Dependency list with uncompressed dependencies from chart archive with yaml output format",
			cmd:    "dep list testdata/testcharts/chart-with-uncompressed-dependencies-2.1.8.tgz --output yaml",
			golden: "output/dependency-list-uncompressed-deps-tgz.yaml",
		},
		{
			name:   "Dependency list of chart with missing dependencies",
			cmd:    "dep list testdata/testcharts/chart-missing-dep",
			golden: "output/dependency-list-missing-dep.txt",
		},
		{
			name:   "Dependency list of chart with missing dependencies with json output format",
			cmd:    "dep list testdata/testcharts/chart-missing-dep --output json",
			golden: "output/dependency-list-missing-dep.json",
		},
		{
			name:   "Dependency list of chart with missing dependencies with yaml output format",
			cmd:    "dep list testdata/testcharts/chart-missing-dep --output yaml",
			golden: "output/dependency-list-missing-dep.yaml",
		},
	}
	runTestCmd(t, tests)
}
