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
	"bytes"
	"strings"
	"testing"
)

func TestSearchCmd(t *testing.T) {
	tests := []struct {
		name   string
		args   []string
		flags  []string
		expect string
		regexp bool
		fail   bool
	}{
		{
			name:   "search for 'maria', expect one match",
			args:   []string{"maria"},
			expect: "NAME           \tVERSION\tDESCRIPTION      \ntesting/mariadb\t0.3.0  \tChart for MariaDB",
		},
		{
			name:   "search for 'alpine', expect two matches",
			args:   []string{"alpine"},
			expect: "NAME          \tVERSION\tDESCRIPTION                    \ntesting/alpine\t0.2.0  \tDeploy a basic Alpine Linux pod",
		},
		{
			name:   "search for 'alpine' with versions, expect three matches",
			args:   []string{"alpine"},
			flags:  []string{"--versions"},
			expect: "NAME          \tVERSION\tDESCRIPTION                    \ntesting/alpine\t0.2.0  \tDeploy a basic Alpine Linux pod\ntesting/alpine\t0.1.0  \tDeploy a basic Alpine Linux pod",
		},
		{
			name:   "search for 'syzygy', expect no matches",
			args:   []string{"syzygy"},
			expect: "No results found",
		},
		{
			name:   "search for 'alp[a-z]+', expect two matches",
			args:   []string{"alp[a-z]+"},
			flags:  []string{"--regexp"},
			expect: "NAME          \tVERSION\tDESCRIPTION                    \ntesting/alpine\t0.2.0  \tDeploy a basic Alpine Linux pod",
			regexp: true,
		},
		{
			name:   "search for 'alp[', expect failure to compile regexp",
			args:   []string{"alp["},
			flags:  []string{"--regexp"},
			regexp: true,
			fail:   true,
		},
	}

	cleanup := resetEnv()
	defer cleanup()

	settings.Home = "testdata/helmhome"

	for _, tt := range tests {
		buf := bytes.NewBuffer(nil)
		cmd := newSearchCmd(buf)
		cmd.ParseFlags(tt.flags)
		if err := cmd.RunE(cmd, tt.args); err != nil {
			if tt.fail {
				continue
			}
			t.Fatalf("%s: unexpected error %s", tt.name, err)
		}
		got := strings.TrimSpace(buf.String())
		if got != tt.expect {
			t.Errorf("%s: expected %q, got %q", tt.name, tt.expect, got)
		}
	}
}
