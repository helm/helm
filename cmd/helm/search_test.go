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
	tests := []releaseCase{
		{
			name:    "search for 'maria', expect one match",
			cmd:     "search maria",
			matches: "NAME           \tCHART VERSION\tAPP VERSION\tDESCRIPTION      \ntesting/mariadb\t0.3.0        \t           \tChart for MariaDB",
		},
		{
			name:    "search for 'alpine', expect two matches",
			cmd:     "search alpine",
			matches: "NAME          \tCHART VERSION\tAPP VERSION\tDESCRIPTION                    \ntesting/alpine\t0.2.0        \t2.3.4      \tDeploy a basic Alpine Linux pod",
		},
		{
			name:    "search for 'alpine' with versions, expect three matches",
			cmd:     "search alpine --versions",
			matches: "NAME          \tCHART VERSION\tAPP VERSION\tDESCRIPTION                    \ntesting/alpine\t0.2.0        \t2.3.4      \tDeploy a basic Alpine Linux pod\ntesting/alpine\t0.1.0        \t1.2.3      \tDeploy a basic Alpine Linux pod",
		},
		{
			name:    "search for 'alpine' with version constraint, expect one match with version 0.1.0",
			cmd:     "search alpine --version '>= 0.1, < 0.2'",
			matches: "NAME          \tCHART VERSION\tAPP VERSION\tDESCRIPTION                    \ntesting/alpine\t0.1.0        \t1.2.3      \tDeploy a basic Alpine Linux pod",
		},
		{
			name:    "search for 'alpine' with version constraint, expect one match with version 0.1.0",
			cmd:     "search alpine --versions --version '>= 0.1, < 0.2'",
			matches: "NAME          \tCHART VERSION\tAPP VERSION\tDESCRIPTION                    \ntesting/alpine\t0.1.0        \t1.2.3      \tDeploy a basic Alpine Linux pod",
		},
		{
			name:    "search for 'alpine' with version constraint, expect one match with version 0.2.0",
			cmd:     "search alpine --version '>= 0.1'",
			matches: "NAME          \tCHART VERSION\tAPP VERSION\tDESCRIPTION                    \ntesting/alpine\t0.2.0        \t2.3.4      \tDeploy a basic Alpine Linux pod",
		},
		{
			name:    "search for 'alpine' with version constraint and --versions, expect two matches",
			cmd:     "search alpine --versions --version '>= 0.1'",
			matches: "NAME          \tCHART VERSION\tAPP VERSION\tDESCRIPTION                    \ntesting/alpine\t0.2.0        \t2.3.4      \tDeploy a basic Alpine Linux pod\ntesting/alpine\t0.1.0        \t1.2.3      \tDeploy a basic Alpine Linux pod",
		},
		{
			name:    "search for 'syzygy', expect no matches",
			cmd:     "search syzygy",
			matches: "No results found",
		},
		{
			name:    "search for 'alp[a-z]+', expect two matches",
			cmd:     "search alp[a-z]+ --regexp",
			matches: "NAME          \tCHART VERSION\tAPP VERSION\tDESCRIPTION                    \ntesting/alpine\t0.2.0        \t2.3.4      \tDeploy a basic Alpine Linux pod",
		},
		{
			name:      "search for 'alp[', expect failure to compile regexp",
			cmd:       "search alp[ --regexp",
			wantError: true,
		},
	}

	cleanup := resetEnv()
	defer cleanup()

	settings.Home = "testdata/helmhome"
	testReleaseCmd(t, tests)
}
