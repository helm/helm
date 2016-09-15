/*
Copyright 2016 The Kubernetes Authors All rights reserved.

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
	"testing"

	"k8s.io/helm/pkg/repo"
)

func TestRepoRemove(t *testing.T) {
	home := createTmpHome()
	helmHome = home
	if err := ensureHome(); err != nil {
		t.Errorf("%s", err)
	}

	if err := removeRepoLine(testName); err == nil {
		t.Errorf("Expected error removing %s, but did not get one.", testName)
	}

	if err := insertRepoLine(testName, testURL); err != nil {
		t.Errorf("%s", err)
	}

	mf, _ := os.Create(cacheIndexFile(testName))
	mf.Close()

	if err := removeRepoLine(testName); err != nil {
		t.Errorf("Error removing %s from repositories", testName)
	}

	if _, err := os.Stat(cacheIndexFile(testName)); err == nil {
		t.Errorf("Error cache file was not removed for repository %s", testName)
	}

	f, err := repo.LoadRepositoriesFile(repositoriesFile())
	if err != nil {
		t.Errorf("%s", err)
	}

	if _, ok := f.Repositories[testName]; ok {
		t.Errorf("%s was not successfully removed from repositories list", testName)
	}
}
