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
	"io"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"k8s.io/helm/pkg/helm"
)

func TestSearchCmd(t *testing.T) {
	tests := []releaseCase{
		{
			name:     "search for 'maria', expect one match",
			args:     []string{"maria"},
			expected: "NAME           \tCHART VERSION\tAPP VERSION\tDESCRIPTION      \ntesting/mariadb\t0.3.0        \t           \tChart for MariaDB",
		},
		{
			name:     "search for 'alpine', expect two matches",
			args:     []string{"alpine"},
			expected: "NAME          \tCHART VERSION\tAPP VERSION\tDESCRIPTION                    \ntesting/alpine\t0.2.0        \t2.3.4      \tDeploy a basic Alpine Linux pod",
		},
		{
			name:     "search for 'alpine' with versions, expect three matches",
			args:     []string{"alpine"},
			flags:    []string{"--versions"},
			expected: "NAME          \tCHART VERSION\tAPP VERSION\tDESCRIPTION                    \ntesting/alpine\t0.2.0        \t2.3.4      \tDeploy a basic Alpine Linux pod\ntesting/alpine\t0.1.0        \t1.2.3      \tDeploy a basic Alpine Linux pod",
		},
		{
			name:     "search for 'alpine' with version constraint, expect one match with version 0.1.0",
			args:     []string{"alpine"},
			flags:    []string{"--version", ">= 0.1, < 0.2"},
			expected: "NAME          \tCHART VERSION\tAPP VERSION\tDESCRIPTION                    \ntesting/alpine\t0.1.0        \t1.2.3      \tDeploy a basic Alpine Linux pod",
		},
		{
			name:     "search for 'alpine' with version constraint, expect one match with version 0.1.0",
			args:     []string{"alpine"},
			flags:    []string{"--versions", "--version", ">= 0.1, < 0.2"},
			expected: "NAME          \tCHART VERSION\tAPP VERSION\tDESCRIPTION                    \ntesting/alpine\t0.1.0        \t1.2.3      \tDeploy a basic Alpine Linux pod",
		},
		{
			name:     "search for 'alpine' with version constraint, expect one match with version 0.2.0",
			args:     []string{"alpine"},
			flags:    []string{"--version", ">= 0.1"},
			expected: "NAME          \tCHART VERSION\tAPP VERSION\tDESCRIPTION                    \ntesting/alpine\t0.2.0        \t2.3.4      \tDeploy a basic Alpine Linux pod",
		},
		{
			name:     "search for 'alpine' with version constraint and --versions, expect two matches",
			args:     []string{"alpine"},
			flags:    []string{"--versions", "--version", ">= 0.1"},
			expected: "NAME          \tCHART VERSION\tAPP VERSION\tDESCRIPTION                    \ntesting/alpine\t0.2.0        \t2.3.4      \tDeploy a basic Alpine Linux pod\ntesting/alpine\t0.1.0        \t1.2.3      \tDeploy a basic Alpine Linux pod",
		},
		{
			name:     "search for 'syzygy', expect no matches",
			args:     []string{"syzygy"},
			expected: "No results found",
		},
		{
			name:     "search for 'alp[a-z]+', expect two matches",
			args:     []string{"alp[a-z]+"},
			flags:    []string{"--regexp"},
			expected: "NAME          \tCHART VERSION\tAPP VERSION\tDESCRIPTION                    \ntesting/alpine\t0.2.0        \t2.3.4      \tDeploy a basic Alpine Linux pod",
		},
		{
			name:  "search for 'alp[', expect failure to compile regexp",
			args:  []string{"alp["},
			flags: []string{"--regexp"},
			err:   true,
		},
		{
			name:     "search for 'maria', expect one match output json",
			args:     []string{"maria"},
			flags:    strings.Split("--output json", " "),
			expected: `[{"Name":"testing/mariadb","Version":"0.3.0","Appversion":"","Description":"Chart for MariaDB"}]`,
		},
		{
			name:     "search for 'alpine', expect two matches output json",
			args:     []string{"alpine"},
			flags:    strings.Split("--output json", " "),
			expected: `[{"Name":"testing/alpine","Version":"0.2.0","Appversion":"2.3.4","Description":"Deploy a basic Alpine Linux pod"}]`,
		},
		{
			name:     "search for 'maria', expect one match output yaml",
			args:     []string{"maria"},
			flags:    strings.Split("--output yaml", " "),
			expected: "- AppVersion: \"\"\n  Description: Chart for MariaDB\n  Name: testing/mariadb\n  Version: 0.3.0\n\n",
		},
		{
			name:     "search for 'alpine', expect two matches output yaml",
			args:     []string{"alpine"},
			flags:    strings.Split("--output yaml", " "),
			expected: "- AppVersion: 2.3.4\n  Description: Deploy a basic Alpine Linux pod\n  Name: testing/alpine\n  Version: 0.2.0\n\n",
		},
	}

	cleanup := resetEnv()
	defer cleanup()

	settings.Home = "testdata/helmhome"

	runReleaseCases(t, tests, func(c *helm.FakeClient, out io.Writer) *cobra.Command {
		return newSearchCmd(out)
	})
}
