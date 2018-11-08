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
	"os"
	"testing"

	"k8s.io/helm/pkg/helm/helmpath"
	"k8s.io/helm/pkg/repo"
)

func TestEnsureHome(t *testing.T) {
	hh := helmpath.Home(testTempDir(t))

	b := bytes.NewBuffer(nil)
	settings.Home = hh
	if err := ensureDirectories(hh, b); err != nil {
		t.Error(err)
	}
	if err := ensureDefaultRepos(hh, b, false, defaultStableRepositoryURL); err != nil {
		t.Error(err)
	}
	if err := ensureDefaultRepos(hh, b, true, defaultStableRepositoryURL); err != nil {
		t.Error(err)
	}
	if err := ensureRepoFileFormat(hh.RepositoryFile(), b); err != nil {
		t.Error(err)
	}

	rr, err := repo.LoadRepositoriesFile(hh.RepositoryFile())
	if err != nil {
		t.Error(err)
	}

	foundStable := false
	for _, rr := range rr.Repositories {
		if rr.Name == stableRepository {
			foundStable = true
			if err != nil {
				t.Error(err)
			}
			if filepath.IsAbs(rr.Cache) {
				t.Errorf("%s stable repo cache path is an absolute path", rr.Cache)
			}
			absCache, err := filepath.Abs(filepath.Join(hh.Cache(), rr.Cache))
			if err != nil {
				t.Error(err)
			}
			if absCache != hh.CacheIndex(stableRepository) {
				t.Errorf("%s stable repo cache path doesn't resolve to absolute cache index path", rr.Cache)
			}
			break
		}
	}
	if !foundStable {
		t.Errorf("stable repo not found")
	}

	expectedDirs := []string{hh.String(), hh.Repository(), hh.Cache()}
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
}
