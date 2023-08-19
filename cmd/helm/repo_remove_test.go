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
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"helm.sh/helm/v3/internal/test/ensure"
	"helm.sh/helm/v3/pkg/helmpath"
	"helm.sh/helm/v3/pkg/repo"
	"helm.sh/helm/v3/pkg/repo/repotest"
)

func TestRepoRemove(t *testing.T) {
	ts, err := repotest.NewTempServerWithCleanup(t, "testdata/testserver/*.*")
	if err != nil {
		t.Fatal(err)
	}
	defer ts.Stop()

	rootDir := ensure.TempDir(t)
	repoFile := filepath.Join(rootDir, "repositories.yaml")

	const testRepoName = "test-name"

	b := bytes.NewBuffer(nil)

	rmOpts := repoRemoveOptions{
		names:     []string{testRepoName},
		repoFile:  repoFile,
		repoCache: rootDir,
	}

	if err := rmOpts.run(os.Stderr); err == nil {
		t.Errorf("Expected error removing %s, but did not get one.", testRepoName)
	}
	o := &repoAddOptions{
		name:     testRepoName,
		url:      ts.URL(),
		repoFile: repoFile,
	}

	if err := o.run(os.Stderr); err != nil {
		t.Error(err)
	}

	cacheIndexFile, cacheChartsFile := createCacheFiles(rootDir, testRepoName)

	// Reset the buffer before running repo remove
	b.Reset()

	if err := rmOpts.run(b); err != nil {
		t.Errorf("Error removing %s from repositories", testRepoName)
	}
	if !strings.Contains(b.String(), "has been removed") {
		t.Errorf("Unexpected output: %s", b.String())
	}

	testCacheFiles(t, cacheIndexFile, cacheChartsFile, testRepoName)

	f, err := repo.LoadFile(repoFile)
	if err != nil {
		t.Error(err)
	}

	if f.Has(testRepoName) {
		t.Errorf("%s was not successfully removed from repositories list", testRepoName)
	}

	// Test removal of multiple repos in one go
	var testRepoNames = []string{"foo", "bar", "baz"}
	cacheFiles := make(map[string][]string, len(testRepoNames))

	// Add test repos
	for _, repoName := range testRepoNames {
		o := &repoAddOptions{
			name:     repoName,
			url:      ts.URL(),
			repoFile: repoFile,
		}

		if err := o.run(os.Stderr); err != nil {
			t.Error(err)
		}

		cacheIndex, cacheChart := createCacheFiles(rootDir, repoName)
		cacheFiles[repoName] = []string{cacheIndex, cacheChart}

	}

	// Create repo remove command
	multiRmOpts := repoRemoveOptions{
		names:     testRepoNames,
		repoFile:  repoFile,
		repoCache: rootDir,
	}

	// Reset the buffer before running repo remove
	b.Reset()

	// Run repo remove command
	if err := multiRmOpts.run(b); err != nil {
		t.Errorf("Error removing list of repos from repositories: %q", testRepoNames)
	}

	// Check that stuff were removed
	if !strings.Contains(b.String(), "has been removed") {
		t.Errorf("Unexpected output: %s", b.String())
	}

	for _, repoName := range testRepoNames {
		f, err := repo.LoadFile(repoFile)
		if err != nil {
			t.Error(err)
		}
		if f.Has(repoName) {
			t.Errorf("%s was not successfully removed from repositories list", repoName)
		}
		cacheIndex := cacheFiles[repoName][0]
		cacheChart := cacheFiles[repoName][1]
		testCacheFiles(t, cacheIndex, cacheChart, repoName)
	}
}

func createCacheFiles(rootDir string, repoName string) (cacheIndexFile string, cacheChartsFile string) {
	cacheIndexFile = filepath.Join(rootDir, helmpath.CacheIndexFile(repoName))
	mf, _ := os.Create(cacheIndexFile)
	mf.Close()

	cacheChartsFile = filepath.Join(rootDir, helmpath.CacheChartsFile(repoName))
	mf, _ = os.Create(cacheChartsFile)
	mf.Close()

	return cacheIndexFile, cacheChartsFile
}

func testCacheFiles(t *testing.T, cacheIndexFile string, cacheChartsFile string, repoName string) {
	if _, err := os.Stat(cacheIndexFile); err == nil {
		t.Errorf("Error cache index file was not removed for repository %s", repoName)
	}
	if _, err := os.Stat(cacheChartsFile); err == nil {
		t.Errorf("Error cache chart file was not removed for repository %s", repoName)
	}
}

func TestRepoRemoveCompletion(t *testing.T) {
	ts, err := repotest.NewTempServerWithCleanup(t, "testdata/testserver/*.*")
	if err != nil {
		t.Fatal(err)
	}
	defer ts.Stop()

	rootDir := ensure.TempDir(t)
	repoFile := filepath.Join(rootDir, "repositories.yaml")
	repoCache := filepath.Join(rootDir, "cache/")

	var testRepoNames = []string{"foo", "bar", "baz"}

	// Add test repos
	for _, repoName := range testRepoNames {
		o := &repoAddOptions{
			name:     repoName,
			url:      ts.URL(),
			repoFile: repoFile,
		}

		if err := o.run(os.Stderr); err != nil {
			t.Error(err)
		}
	}

	repoSetup := fmt.Sprintf("--repository-config %s --repository-cache %s", repoFile, repoCache)

	// In the following tests, we turn off descriptions for completions by using __completeNoDesc.
	// We have to do this because the description will contain the port used by the webserver,
	// and that port changes each time we run the test.
	tests := []cmdTestCase{{
		name:   "completion for repo remove",
		cmd:    fmt.Sprintf("%s __completeNoDesc repo remove ''", repoSetup),
		golden: "output/repo_list_comp.txt",
	}, {
		name:   "completion for repo remove, no filter",
		cmd:    fmt.Sprintf("%s __completeNoDesc repo remove fo", repoSetup),
		golden: "output/repo_list_comp.txt",
	}, {
		name:   "completion for repo remove repetition",
		cmd:    fmt.Sprintf("%s __completeNoDesc repo remove foo ''", repoSetup),
		golden: "output/repo_repeat_comp.txt",
	}}
	for _, test := range tests {
		runTestCmd(t, []cmdTestCase{test})
	}
}

func TestRepoRemoveFileCompletion(t *testing.T) {
	checkFileCompletion(t, "repo remove", false)
	checkFileCompletion(t, "repo remove repo1", false)
}
