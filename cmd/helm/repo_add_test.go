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
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"helm.sh/helm/v3/internal/test/ensure"
	"helm.sh/helm/v3/pkg/repo/repotest"
)

func TestRepoAddCmd(t *testing.T) {
	srv, err := repotest.NewTempServerWithCleanup(t, "testdata/testserver/*.*")
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Stop()

	// A second test server is setup to verify URL changing
	srv2, err := repotest.NewTempServerWithCleanup(t, "testdata/testserver/*.*")
	if err != nil {
		t.Fatal(err)
	}
	defer srv2.Stop()

	tmpdir := filepath.Join(ensure.TempDir(t), "path-component.yaml/data")
	err = os.MkdirAll(tmpdir, 0777)
	if err != nil {
		t.Fatal(err)
	}
	repoFile := filepath.Join(tmpdir, "repositories.yaml")

	tests := []cmdTestCase{
		{
			name:   "add a repository",
			cmd:    fmt.Sprintf("repo add test-name %s --repository-config %s --repository-cache %s", srv.URL(), repoFile, tmpdir),
			golden: "output/repo-add.txt",
		},
		{
			name:   "add repository second time",
			cmd:    fmt.Sprintf("repo add test-name %s --repository-config %s --repository-cache %s", srv.URL(), repoFile, tmpdir),
			golden: "output/repo-add2.txt",
		},
		{
			name:      "add repository different url",
			cmd:       fmt.Sprintf("repo add test-name %s --repository-config %s --repository-cache %s", srv2.URL(), repoFile, tmpdir),
			wantError: true,
		},
		{
			name:   "add repository second time",
			cmd:    fmt.Sprintf("repo add test-name %s --repository-config %s --repository-cache %s --force-update", srv2.URL(), repoFile, tmpdir),
			golden: "output/repo-add.txt",
		},
	}

	runTestCmd(t, tests)
}

func TestRepoAddFileCompletion(t *testing.T) {
	checkFileCompletion(t, "repo add", false)
	checkFileCompletion(t, "repo add reponame", false)
	checkFileCompletion(t, "repo add reponame https://example.com", false)
}

func TestRepoAddWithPasswordFromStdin(t *testing.T) {
	srv := repotest.NewTempServerWithCleanupAndBasicAuth(t, "testdata/testserver/*.*")
	defer srv.Stop()

	defer resetEnv()()

	in, err := os.Open("testdata/password")
	if err != nil {
		t.Errorf("unexpected error, got '%v'", err)
	}

	tmpdir := ensure.TempDir(t)
	repoFile := filepath.Join(tmpdir, "repositories.yaml")

	store := storageFixture()

	const testName = "test-name"
	const username = "username"
	cmd := fmt.Sprintf("repo add %s %s --repository-config %s --repository-cache %s --username %s --password-stdin", testName, srv.URL(), repoFile, tmpdir, username)
	var result string
	_, result, err = executeActionCommandStdinC(store, in, cmd)
	if err != nil {
		t.Errorf("unexpected error, got '%v'", err)
	}

	if !strings.Contains(result, fmt.Sprintf("\"%s\" has been added to your repositories", testName)) {
		t.Errorf("Repo was not successfully added. Output: %s", result)
	}
}
