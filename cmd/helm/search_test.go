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

func TestSearchCmd(t *testing.T) {
	tests := []releaseCase{{
		name:   "search for 'maria', expect one match",
		cmd:    "search maria",
		golden: "output/search-single.txt",
	}, {
		name:   "search for 'alpine', expect two matches",
		cmd:    "search alpine",
		golden: "output/search-multiple.txt",
	}, {
		name:   "search for 'alpine' with versions, expect three matches",
		cmd:    "search alpine --versions",
		golden: "output/search-multiple-versions.txt",
	}, {
		name:   "search for 'alpine' with version constraint, expect one match with version 0.1.0",
		cmd:    "search alpine --version '>= 0.1, < 0.2'",
		golden: "output/search-constraint.txt",
	}, {
		name:   "search for 'alpine' with version constraint, expect one match with version 0.1.0",
		cmd:    "search alpine --versions --version '>= 0.1, < 0.2'",
		golden: "output/search-versions-constraint.txt",
	}, {
		name:   "search for 'alpine' with version constraint, expect one match with version 0.2.0",
		cmd:    "search alpine --version '>= 0.1'",
		golden: "output/search-constraint-single.txt",
	}, {
		name:   "search for 'alpine' with version constraint and --versions, expect two matches",
		cmd:    "search alpine --versions --version '>= 0.1'",
		golden: "output/search-multiple-versions-constraints.txt",
	}, {
		name:   "search for 'syzygy', expect no matches",
		cmd:    "search syzygy",
		golden: "output/search-not-found.txt",
	}, {
		name:   "search for 'alp[a-z]+', expect two matches",
		cmd:    "search alp[a-z]+ --regexp",
		golden: "output/search-regex.txt",
	}, {
		name:      "search for 'alp[', expect failure to compile regexp",
		cmd:       "search alp[ --regexp",
		wantError: true,
	}}

	cleanup := resetEnv()
	defer cleanup()

	settings.Home = "testdata/helmhome"
	testReleaseCmd(t, tests)
}
