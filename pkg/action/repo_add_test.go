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

package action

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"sigs.k8s.io/yaml"

	"helm.sh/helm/v3/internal/test/ensure"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/helmpath"
	"helm.sh/helm/v3/pkg/helmpath/xdg"
	"helm.sh/helm/v3/pkg/repo"
	"helm.sh/helm/v3/pkg/repo/repotest"
)

var settings = cli.New()

func TestRepoAdd(t *testing.T) {
	ts, err := repotest.NewTempServerWithCleanup(t, "testdata/testserver/*.*")
	if err != nil {
		t.Fatal(err)
	}
	defer ts.Stop()

	rootDir := ensure.TempDir(t)
	repoFile := filepath.Join(rootDir, "repositories.yaml")

	const testRepoName = "test-name"

	o := &RepoAddOptions{
		Name:               testRepoName,
		URL:                ts.URL(),
		ForceUpdate:        false,
		DeprecatedNoUpdate: true,
		RepoFile:           repoFile,
	}
	os.Setenv(xdg.CacheHomeEnvVar, rootDir)

	if err := o.Run(settings, ioutil.Discard); err != nil {
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

	o.ForceUpdate = true

	if err := o.Run(settings, ioutil.Discard); err != nil {
		t.Errorf("Repository was not updated: %s", err)
	}

	if err := o.Run(settings, ioutil.Discard); err != nil {
		t.Errorf("Duplicate repository name was added")
	}
}

func TestRepoAddCheckLegalName(t *testing.T) {
	ts, err := repotest.NewTempServerWithCleanup(t, "testdata/testserver/*.*")
	if err != nil {
		t.Fatal(err)
	}
	defer ts.Stop()

	const testRepoName = "test-hub/test-name"

	rootDir := ensure.TempDir(t)
	repoFile := filepath.Join(ensure.TempDir(t), "repositories.yaml")

	o := &RepoAddOptions{
		Name:               testRepoName,
		URL:                ts.URL(),
		ForceUpdate:        false,
		DeprecatedNoUpdate: true,
		RepoFile:           repoFile,
	}
	os.Setenv(xdg.CacheHomeEnvVar, rootDir)

	wantErrorMsg := fmt.Sprintf("repository name (%s) contains '/', please specify a different name without '/'", testRepoName)

	if err := o.Run(settings, ioutil.Discard); err != nil {
		if wantErrorMsg != err.Error() {
			t.Fatalf("Actual error %s, not equal to expected error %s", err, wantErrorMsg)
		}
	} else {
		t.Fatalf("expect reported an error.")
	}
}

func TestRepoAddConcurrentGoRoutines(t *testing.T) {
	const testName = "test-name"
	repoFile := filepath.Join(ensure.TempDir(t), "repositories.yaml")
	repoAddConcurrent(t, testName, repoFile)
}

func TestRepoAddConcurrentDirNotExist(t *testing.T) {
	const testName = "test-name-2"
	repoFile := filepath.Join(ensure.TempDir(t), "foo", "repositories.yaml")
	repoAddConcurrent(t, testName, repoFile)
}

func TestRepoAddConcurrentNoFileExtension(t *testing.T) {
	const testName = "test-name-3"
	repoFile := filepath.Join(ensure.TempDir(t), "repositories")
	repoAddConcurrent(t, testName, repoFile)
}

func TestRepoAddConcurrentHiddenFile(t *testing.T) {
	const testName = "test-name-4"
	repoFile := filepath.Join(ensure.TempDir(t), ".repositories")
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
			o := &RepoAddOptions{
				Name:               name,
				URL:                ts.URL(),
				DeprecatedNoUpdate: true,
				ForceUpdate:        false,
				RepoFile:           repoFile,
			}
			if err := o.Run(settings, ioutil.Discard); err != nil {
				t.Error(err)
			}
		}(fmt.Sprintf("%s-%d", testName, i))
	}
	wg.Wait()

	b, err := ioutil.ReadFile(repoFile)
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
