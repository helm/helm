/*
Copyright 2016 The Kubernetes Authors All rights reserved.
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
	"testing"
)

func TestDependencyListCmd(t *testing.T) {

	tests := []releaseCase{
		{
			name:      "No such chart",
			cmd:       "dependency list /no/such/chart",
			wantError: true,
		},
		{
			name:    "No requirements.yaml",
			cmd:     "dependency list testdata/testcharts/alpine",
			matches: "WARNING: no requirements at ",
		},
		{
			name: "Requirements in chart dir",
			cmd:  "dependency list testdata/testcharts/reqtest",
			matches: "NAME        \tVERSION\tREPOSITORY                \tSTATUS  \n" +
				"reqsubchart \t0.1.0  \thttps://example.com/charts\tunpacked\n" +
				"reqsubchart2\t0.2.0  \thttps://example.com/charts\tunpacked\n" +
				"reqsubchart3\t>=0.1.0\thttps://example.com/charts\tok      \n\n",
		},
		{
			name:    "Requirements in chart archive",
			cmd:     "dependency list testdata/testcharts/reqtest-0.1.0.tgz",
			matches: "NAME        \tVERSION\tREPOSITORY                \tSTATUS \nreqsubchart \t0.1.0  \thttps://example.com/charts\tmissing\nreqsubchart2\t0.2.0  \thttps://example.com/charts\tmissing\n",
		},
	}
	testReleaseCmd(t, tests)
}
