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
	"testing"
)

func TestSearchRepositoriesCmd(t *testing.T) {
	repoFile := "testdata/helmhome/helm/repositories.yaml"
	repoCache := "testdata/helmhome/helm/repository"

	tests := []cmdTestCase{{
		name:   "search for 'alpine', expect two matches",
		cmd:    "search repo alpine",
		golden: "output/search-multiple.txt",
	}, {
		name:   "search for 'alpine', expect two matches",
		cmd:    "search repo alpine",
		golden: "output/search-multiple.txt",
	}, {
		name:   "search for 'alpine' with versions, expect three matches",
		cmd:    "search repo alpine --versions",
		golden: "output/search-multiple-versions.txt",
	}, {
		name:   "search for 'alpine' with version constraint, expect one match with version 0.1.0",
		cmd:    "search repo alpine --version '>= 0.1, < 0.2'",
		golden: "output/search-constraint.txt",
	}, {
		name:   "search for 'alpine' with version constraint, expect one match with version 0.1.0",
		cmd:    "search repo alpine --versions --version '>= 0.1, < 0.2'",
		golden: "output/search-versions-constraint.txt",
	}, {
		name:   "search for 'alpine' with version constraint, expect one match with version 0.2.0",
		cmd:    "search repo alpine --version '>= 0.1'",
		golden: "output/search-constraint-single.txt",
	}, {
		name:   "search for 'alpine' with version constraint and --versions, expect two matches",
		cmd:    "search repo alpine --versions --version '>= 0.1'",
		golden: "output/search-multiple-versions-constraints.txt",
	}, {
		name:   "search for 'syzygy', expect no matches",
		cmd:    "search repo syzygy",
		golden: "output/search-not-found.txt",
	}, {
		name:   "search for 'alp[a-z]+', expect two matches",
		cmd:    "search repo alp[a-z]+ --regexp",
		golden: "output/search-regex.txt",
	}, {
		name:      "search for 'alp[', expect failure to compile regexp",
		cmd:       "search repo alp[ --regexp",
		wantError: true,
	}}

	settings.Debug = true
	defer func() { settings.Debug = false }()

	for i := range tests {
		tests[i].cmd += " --repository-config " + repoFile
		tests[i].cmd += " --repository-cache " + repoCache
	}
	runTestCmd(t, tests)
}
