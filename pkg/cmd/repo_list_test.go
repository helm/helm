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
	"path/filepath"
	"testing"
)

func TestRepoListOutputCompletion(t *testing.T) {
	outputFlagCompletionTest(t, "repo list")
}

func TestRepoListFileCompletion(t *testing.T) {
	checkFileCompletion(t, "repo list", false)
}

func TestRepoList(t *testing.T) {
	rootDir := t.TempDir()
	repoFile := filepath.Join(rootDir, "repositories.yaml")
	repoFile2 := "testdata/repositories.yaml"

	tests := []cmdTestCase{
		{
			name:      "list with no repos",
			cmd:       fmt.Sprintf("repo list --repository-config %s --repository-cache %s", repoFile, rootDir),
			golden:    "output/repo-list-empty.txt",
			wantError: false,
		},
		{
			name:      "list with repos",
			cmd:       fmt.Sprintf("repo list --repository-config %s --repository-cache %s", repoFile2, rootDir),
			golden:    "output/repo-list.txt",
			wantError: false,
		},
		{
			name:      "list without headers",
			cmd:       fmt.Sprintf("repo list --repository-config %s --repository-cache %s --no-headers", repoFile2, rootDir),
			golden:    "output/repo-list-no-headers.txt",
			wantError: false,
		},
	}

	runTestCmd(t, tests)
}
