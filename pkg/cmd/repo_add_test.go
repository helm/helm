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
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"

	"helm.sh/helm/v4/pkg/helmpath"
	"helm.sh/helm/v4/pkg/helmpath/xdg"
	"helm.sh/helm/v4/pkg/repo/v1"
	"helm.sh/helm/v4/pkg/repo/v1/repotest"
)

func TestRepoAddCmd(t *testing.T) {
	srv := repotest.NewTempServer(
		t,
		repotest.WithChartSourceGlob("testdata/testserver/*.*"),
	)
	defer srv.Stop()

	// A second test server is setup to verify URL changing
	srv2 := repotest.NewTempServer(
		t,
		repotest.WithChartSourceGlob("testdata/testserver/*.*"),
	)
	defer srv2.Stop()

	tmpdir := filepath.Join(t.TempDir(), "path-component.yaml", "data")
	require.NoError(t, os.MkdirAll(tmpdir, 0o777))
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
	ts := repotest.NewTempServer(
		t,
		repotest.WithChartSourceGlob("testdata/testserver/*.*"),
	)
	defer ts.Stop()

	rootDir := t.TempDir()
	repoFile := filepath.Join(rootDir, "repositories.yaml")

	const testRepoName = "test-name"

	o := &repoAddOptions{
		name:        testRepoName,
		url:         ts.URL(),
		forceUpdate: false,
		repoFile:    repoFile,
	}
	t.Setenv(xdg.CacheHomeEnvVar, rootDir)

	require.NoError(t, o.run(io.Discard))

	f, err := repo.LoadFile(repoFile)
	require.NoError(t, err)

	assert.Truef(t, f.Has(testRepoName), "%s was not successfully inserted into %s", testRepoName, repoFile)

	idx := filepath.Join(helmpath.CachePath("repository"), helmpath.CacheIndexFile(testRepoName))
	_, err = os.Stat(idx)
	require.NotErrorIsf(t, err, fs.ErrNotExist, "Error cache index file was not created for repository %s", testRepoName)
	idx = filepath.Join(helmpath.CachePath("repository"), helmpath.CacheChartsFile(testRepoName))
	_, err = os.Stat(idx)
	require.NotErrorIsf(t, err, fs.ErrNotExist, "Error cache charts file was not created for repository %s", testRepoName)

	o.forceUpdate = true

	require.NoError(t, o.run(io.Discard), "Repository was not updated")
	assert.NoError(t, o.run(io.Discard), "Duplicate repository name was added")
}

func TestRepoAddCheckLegalName(t *testing.T) {
	ts := repotest.NewTempServer(
		t,
		repotest.WithChartSourceGlob("testdata/testserver/*.*"),
	)
	defer ts.Stop()
	defer resetEnv()()

	const testRepoName = "test-hub/test-name"

	rootDir := t.TempDir()
	repoFile := filepath.Join(t.TempDir(), "repositories.yaml")

	o := &repoAddOptions{
		name:        testRepoName,
		url:         ts.URL(),
		forceUpdate: false,
		repoFile:    repoFile,
	}
	t.Setenv(xdg.CacheHomeEnvVar, rootDir)

	wantErrorMsg := fmt.Sprintf("repository name (%s) contains '/', please specify a different name without '/'", testRepoName)
	require.EqualError(t, o.run(io.Discard), wantErrorMsg)
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
	t.Helper()
	ts := repotest.NewTempServer(
		t,
		repotest.WithChartSourceGlob("testdata/testserver/*.*"),
	)
	defer ts.Stop()

	var wg sync.WaitGroup
	wg.Add(3)
	for i := range 3 {
		go func(name string) {
			defer wg.Done()
			o := &repoAddOptions{
				name:        name,
				url:         ts.URL(),
				forceUpdate: false,
				repoFile:    repoFile,
			}
			assert.NoError(t, o.run(io.Discard))
		}(fmt.Sprintf("%s-%d", testName, i))
	}
	wg.Wait()

	b, err := os.ReadFile(repoFile)
	require.NoError(t, err)

	var f repo.File
	require.NoError(t, yaml.Unmarshal(b, &f))

	var name string
	for i := range 3 {
		name = fmt.Sprintf("%s-%d", testName, i)
		assert.Truef(t, f.Has(name), "%s was not successfully inserted into %s: %s", name, repoFile, f.Repositories[0])
	}
}

func TestRepoAddFileCompletion(t *testing.T) {
	checkFileCompletion(t, "repo add", false)
	checkFileCompletion(t, "repo add reponame", false)
	checkFileCompletion(t, "repo add reponame https://example.com", false)
}

func TestRepoAddWithPasswordFromStdin(t *testing.T) {
	srv := repotest.NewTempServer(
		t,
		repotest.WithChartSourceGlob("testdata/testserver/*.*"),
		repotest.WithMiddleware(repotest.BasicAuthMiddleware(t)),
	)
	defer srv.Stop()

	defer resetEnv()()

	in, err := os.Open("testdata/password")
	require.NoError(t, err)

	tmpdir := t.TempDir()
	repoFile := filepath.Join(tmpdir, "repositories.yaml")

	store := storageFixture()

	const testName = "test-name"
	const username = "username"
	cmd := fmt.Sprintf("repo add %s %s --repository-config %s --repository-cache %s --username %s --password-stdin", testName, srv.URL(), repoFile, tmpdir, username)
	var result string
	_, result, err = executeActionCommandStdinC(store, in, cmd)

	require.NoError(t, err)
	assert.Contains(t, result, fmt.Sprintf("%q has been added to your repositories", testName), "Repo was not successfully added. Output: %s", result)
}
