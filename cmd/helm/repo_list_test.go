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
	"os"
	"path/filepath"
	"testing"

	"helm.sh/helm/v3/pkg/cli/output"
	"helm.sh/helm/v3/pkg/repo/repotest"
)

func TestRepoListOutputCompletion(t *testing.T) {
	outputFlagCompletionTest(t, "repo list")
}

func TestRepoListFileCompletion(t *testing.T) {
	checkFileCompletion(t, "repo list", false)
}

func TestRepoListAllowEmpty(t *testing.T) {
	ts, err := repotest.NewTempServerWithCleanup(t, "testdata/testserver/*.*")
	if err != nil {
		t.Fatal(err)
	}
	defer ts.Stop()

	rootDir := ts.Root()
	repoFile := filepath.Join(rootDir, "repositories.yaml")

	// Remove existing repo
	const testRepoName = "test"

	rmOpts := repoRemoveOptions{
		names:     []string{testRepoName},
		repoFile:  repoFile,
		repoCache: rootDir,
	}
	if err := rmOpts.run(os.Stderr); err != nil {
		t.Error(err)
	}

	listOpts := repoListOptions{
		allowEmpty:   true,
		repoFile:     repoFile,
		outputFormat: output.Table,
	}
	if err := listOpts.run(os.Stderr); err != nil {
		t.Error("Error not expected when listing repositories with allow-empty flag")
	}
}
