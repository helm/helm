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

func TestLibraryListCmd(t *testing.T) {
	noSuchChart := cmdTestCase{
		name:      "No such chart",
		cmd:       "library list /no/such/chart",
		golden:    "output/dependency-list-no-chart-linux.txt",
		wantError: true,
	}

	noDependencies := cmdTestCase{
		name:   "No libraries",
		cmd:    "library list testdata/testcharts/alpine",
		golden: "output/library-list-no-requirements-linux.txt",
	}

	if runtime.GOOS == "windows" {
		noSuchChart.golden = "output/dependency-list-no-chart-windows.txt"
		noDependencies.golden = "output/library-list-no-requirements-windows.txt"
	}

	tests := []cmdTestCase{noSuchChart,
		noDependencies, {
			name:   "Libraries in library dir",
			cmd:    "library list testdata/testcharts/libcharttest",
			golden: "output/dependency-list.txt",
		}, {
			name:   "Libraries in chart archive",
			cmd:    "library list testdata/testcharts/libcharttest-0.1.0.tgz",
			golden: "output/dependency-list-archive.txt",
		}}
	runTestCmd(t, tests)
}
