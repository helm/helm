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
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"sigs.k8s.io/yaml"

	"helm.sh/helm/v3/pkg/helmpath"
	"helm.sh/helm/v3/pkg/helmpath/xdg"
	"helm.sh/helm/v3/pkg/repo"
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

	tmpdir := filepath.Join(t.TempDir(), "path-component.yaml/data")
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

func TestRepoAdd(t *testing.T) {
	ts, err := repotest.NewTempServerWithCleanup(t, "testdata/testserver/*.*")
	if err != nil {
		t.Fatal(err)
	}
	defer ts.Stop()

	rootDir := t.TempDir()
	repoFile := filepath.Join(rootDir, "repositories.yaml")

	const testRepoName = "test-name"

	o := &repoAddOptions{
		name:               testRepoName,
		url:                ts.URL(),
		forceUpdate:        false,
		deprecatedNoUpdate: true,
		repoFile:           repoFile,
	}
	os.Setenv(xdg.CacheHomeEnvVar, rootDir)

	if err := o.run(io.Discard); err != nil {
		t.Error(err)
	}

	f, err := repo.LoadFile(repoFile)
	if err != nil {
		t.Fatal(err)
	}

	if !f.Has(testRepoName) {
		t.Errorf("%s was not successfully inserted into %s", testRepoName, repoFile)
	}

	idx := filepath.Join(helmpath.CachePath("repository"), helmpath.CacheIndexFile(testRepoName))
	if _, err := os.Stat(idx); os.IsNotExist(err) {
		t.Errorf("Error cache index file was not created for repository %s", testRepoName)
	}
	idx = filepath.Join(helmpath.CachePath("repository"), helmpath.CacheChartsFile(testRepoName))
	if _, err := os.Stat(idx); os.IsNotExist(err) {
		t.Errorf("Error cache charts file was not created for repository %s", testRepoName)
	}

	o.forceUpdate = true

	if err := o.run(io.Discard); err != nil {
		t.Errorf("Repository was not updated: %s", err)
	}

	if err := o.run(io.Discard); err != nil {
		t.Errorf("Duplicate repository name was added")
	}
}

func TestRepoAddCheckLegalName(t *testing.T) {
	ts, err := repotest.NewTempServerWithCleanup(t, "testdata/testserver/*.*")
	if err != nil {
		t.Fatal(err)
	}
	defer ts.Stop()
	defer resetEnv()()

	const testRepoName = "test-hub/test-name"

	rootDir := t.TempDir()
	repoFile := filepath.Join(t.TempDir(), "repositories.yaml")

	o := &repoAddOptions{
		name:               testRepoName,
		url:                ts.URL(),
		forceUpdate:        false,
		deprecatedNoUpdate: true,
		repoFile:           repoFile,
	}
	os.Setenv(xdg.CacheHomeEnvVar, rootDir)

	wantErrorMsg := fmt.Sprintf("repository name (%s) contains '/', please specify a different name without '/'", testRepoName)

	if err := o.run(io.Discard); err != nil {
		if wantErrorMsg != err.Error() {
			t.Fatalf("Actual error %s, not equal to expected error %s", err, wantErrorMsg)
		}
	} else {
		t.Fatalf("expect reported an error.")
	}
}

func TestRepoAddConcurrentGoRoutines(t *testing.T) {
	const testName = "test-name"
	repoFile := filepath.Join(t.TempDir(), "repositories.yaml")
	repoAddConcurrent(t, testName, repoFile)
}

func TestRepoAddConcurrentDirNotExist(t *testing.T) {
	const testName = "test-name-2"
	repoFile := filepath.Join(t.TempDir(), "foo", "repositories.yaml")
	repoAddConcurrent(t, testName, repoFile)
}

func TestRepoAddConcurrentNoFileExtension(t *testing.T) {
	const testName = "test-name-3"
	repoFile := filepath.Join(t.TempDir(), "repositories")
	repoAddConcurrent(t, testName, repoFile)
}

func TestRepoAddConcurrentHiddenFile(t *testing.T) {
	const testName = "test-name-4"
	repoFile := filepath.Join(t.TempDir(), ".repositories")
	repoAddConcurrent(t, testName, repoFile)
}

func repoAddConcurrent(t *testing.T, testName, repoFile string) {
	ts, err := repotest.NewTempServerWithCleanup(t, "testdata/testserver/*.*")
	if err != nil {
		t.Fatal(err)
	}
	defer ts.Stop()

	var wg sync.WaitGroup
	wg.Add(3)
	for i := 0; i < 3; i++ {
		go func(name string) {
			defer wg.Done()
			o := &repoAddOptions{
				name:               name,
				url:                ts.URL(),
				deprecatedNoUpdate: true,
				forceUpdate:        false,
				repoFile:           repoFile,
			}
			if err := o.run(io.Discard); err != nil {
				t.Error(err)
			}
		}(fmt.Sprintf("%s-%d", testName, i))
	}
	wg.Wait()

	b, err := os.ReadFile(repoFile)
	if err != nil {
		t.Error(err)
	}

	var f repo.File
	if err := yaml.Unmarshal(b, &f); err != nil {
		t.Error(err)
	}

	var name string
	for i := 0; i < 3; i++ {
		name = fmt.Sprintf("%s-%d", testName, i)
		if !f.Has(name) {
			t.Errorf("%s was not successfully inserted into %s: %s", name, repoFile, f.Repositories[0])
		}
	}
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

	tmpdir := t.TempDir()
	repoFile := filepath.Join(tmpdir, "repositories.yaml")

	store := storageFixture()

	const testName = "test-name"
	const username = "username"
	cmd := fmt.Sprintf("repo add %s %s --repository-config %s --repository-cache %s --username %s --password-stdin", testName, srv.URL(), repoFile, tmpdir, username)
	var result string
	_, result, err = executeActionCommandStdinC(store, in, cmd, nil, nil)
	if err != nil {
		t.Errorf("unexpected error, got '%v'", err)
	}

	if !strings.Contains(result, fmt.Sprintf("\"%s\" has been added to your repositories", testName)) {
		t.Errorf("Repo was not successfully added. Output: %s", result)
	}
}
