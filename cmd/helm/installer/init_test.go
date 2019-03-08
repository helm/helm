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

package installer // import "k8s.io/helm/cmd/helm/installer"

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	helm_env "k8s.io/helm/pkg/helm/environment"
	"k8s.io/helm/pkg/helm/helmpath"
	"k8s.io/helm/pkg/repo"
)

func TestInitialize(t *testing.T) {
	home, err := ioutil.TempDir("", "helm_home")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(home)

	b := bytes.NewBuffer(nil)
	hh := helmpath.Home(home)

	settings := helm_env.EnvSettings{
		Home: hh,
	}
	stableRepositoryURL := "https://kubernetes-charts.storage.googleapis.com"
	localRepositoryURL := "http://127.0.0.1:8879/charts"

	if err := Initialize(hh, b, false, settings, stableRepositoryURL, localRepositoryURL); err != nil {
		t.Error(err)
	}

	expectedDirs := []string{hh.String(), hh.Repository(), hh.Cache(), hh.LocalRepository()}
	for _, dir := range expectedDirs {
		if fi, err := os.Stat(dir); err != nil {
			t.Errorf("%s", err)
		} else if !fi.IsDir() {
			t.Errorf("%s is not a directory", fi)
		}
	}

	if fi, err := os.Stat(hh.RepositoryFile()); err != nil {
		t.Error(err)
	} else if fi.IsDir() {
		t.Errorf("%s should not be a directory", fi)
	}

	if fi, err := os.Stat(hh.LocalRepository(LocalRepositoryIndexFile)); err != nil {
		t.Errorf("%s", err)
	} else if fi.IsDir() {
		t.Errorf("%s should not be a directory", fi)
	}
}

func TestEnsureHome(t *testing.T) {
	home, err := ioutil.TempDir("", "helm_home")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(home)

	b := bytes.NewBuffer(nil)
	hh := helmpath.Home(home)

	settings := helm_env.EnvSettings{
		Home: hh,
	}
	stableRepositoryURL := "https://kubernetes-charts.storage.googleapis.com"
	localRepositoryURL := "http://127.0.0.1:8879/charts"

	if err := ensureDirectories(hh, b); err != nil {
		t.Error(err)
	}
	if err := ensureDefaultRepos(hh, b, false, settings, stableRepositoryURL, localRepositoryURL); err != nil {
		t.Error(err)
	}
	if err := ensureDefaultRepos(hh, b, true, settings, stableRepositoryURL, localRepositoryURL); err != nil {
		t.Error(err)
	}
	if err := ensureRepoFileFormat(hh.RepositoryFile(), b); err != nil {
		t.Error(err)
	}

	rf, err := repo.LoadRepositoriesFile(hh.RepositoryFile())
	if err != nil {
		t.Error(err)
	}

	foundStable := false
	stableCachePath := fmt.Sprintf("%s-index.yaml", stableRepository)
	foundLocal := false
	localCachePath := fmt.Sprintf("%s-index.yaml", LocalRepository)
	for _, rr := range rf.Repositories {
		if rr.Name == stableRepository {
			if rr.Cache != stableCachePath {
				t.Errorf("stable repo cache path is %s, not %s", rr.Cache, stableCachePath)
			}
			foundStable = true
		} else if rr.Name == LocalRepository {
			if rr.Cache != localCachePath {
				t.Errorf("local repo cache path is %s, not %s", rr.Cache, localCachePath)
			}
			foundLocal = true
		}
	}
	if !foundStable {
		t.Errorf("stable repo not found")
	}
	if !foundLocal {
		t.Errorf("local repo not found")
	}

	expectedDirs := []string{hh.String(), hh.Repository(), hh.Cache(), hh.LocalRepository()}
	for _, dir := range expectedDirs {
		if fi, err := os.Stat(dir); err != nil {
			t.Errorf("%s", err)
		} else if !fi.IsDir() {
			t.Errorf("%s is not a directory", fi)
		}
	}

	if fi, err := os.Stat(hh.RepositoryFile()); err != nil {
		t.Error(err)
	} else if fi.IsDir() {
		t.Errorf("%s should not be a directory", fi)
	}

	if fi, err := os.Stat(hh.LocalRepository(LocalRepositoryIndexFile)); err != nil {
		t.Errorf("%s", err)
	} else if fi.IsDir() {
		t.Errorf("%s should not be a directory", fi)
	}
}
