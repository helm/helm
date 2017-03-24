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

package repo

import "testing"
import "io/ioutil"
import "os"

const testRepositoriesFile = "testdata/repositories.yaml"

func TestRepoFile(t *testing.T) {
	rf := NewRepoFile()
	rf.Add(
		&Entry{
			Name:  "stable",
			URL:   "https://example.com/stable/charts",
			Cache: "stable-index.yaml",
		},
		&Entry{
			Name:  "incubator",
			URL:   "https://example.com/incubator",
			Cache: "incubator-index.yaml",
		},
	)

	if len(rf.Repositories) != 2 {
		t.Fatal("Expected 2 repositories")
	}

	if rf.Has("nosuchrepo") {
		t.Error("Found nonexistent repo")
	}
	if !rf.Has("incubator") {
		t.Error("incubator repo is missing")
	}

	stable := rf.Repositories[0]
	if stable.Name != "stable" {
		t.Error("stable is not named stable")
	}
	if stable.URL != "https://example.com/stable/charts" {
		t.Error("Wrong URL for stable")
	}
	if stable.Cache != "stable-index.yaml" {
		t.Error("Wrong cache name for stable")
	}
}

func TestNewRepositoriesFile(t *testing.T) {
	expects := NewRepoFile()
	expects.Add(
		&Entry{
			Name:  "stable",
			URL:   "https://example.com/stable/charts",
			Cache: "stable-index.yaml",
		},
		&Entry{
			Name:  "incubator",
			URL:   "https://example.com/incubator",
			Cache: "incubator-index.yaml",
		},
	)

	repofile, err := LoadRepositoriesFile(testRepositoriesFile)
	if err != nil {
		t.Errorf("%q could not be loaded: %s", testRepositoriesFile, err)
	}

	if len(expects.Repositories) != len(repofile.Repositories) {
		t.Fatalf("Unexpected repo data: %#v", repofile.Repositories)
	}

	for i, expect := range expects.Repositories {
		got := repofile.Repositories[i]
		if expect.Name != got.Name {
			t.Errorf("Expected name %q, got %q", expect.Name, got.Name)
		}
		if expect.URL != got.URL {
			t.Errorf("Expected url %q, got %q", expect.URL, got.URL)
		}
		if expect.Cache != got.Cache {
			t.Errorf("Expected cache %q, got %q", expect.Cache, got.Cache)
		}
	}
}

func TestNewPreV1RepositoriesFile(t *testing.T) {
	r, err := LoadRepositoriesFile("testdata/old-repositories.yaml")
	if err != nil && err != ErrRepoOutOfDate {
		t.Fatal(err)
	}
	if len(r.Repositories) != 3 {
		t.Fatalf("Expected 3 repos: %#v", r)
	}

	// Because they are parsed as a map, we lose ordering.
	found := false
	for _, rr := range r.Repositories {
		if rr.Name == "best-charts-ever" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected the best charts ever. Got %#v", r.Repositories)
	}
}

func TestRemoveRepository(t *testing.T) {
	sampleRepository := NewRepoFile()
	sampleRepository.Add(
		&Entry{
			Name:  "stable",
			URL:   "https://example.com/stable/charts",
			Cache: "stable-index.yaml",
		},
		&Entry{
			Name:  "incubator",
			URL:   "https://example.com/incubator",
			Cache: "incubator-index.yaml",
		},
	)

	removeRepository := "stable"
	found := sampleRepository.Remove(removeRepository)
	if !found {
		t.Errorf("expected repository %s not found", removeRepository)
	}

	found = sampleRepository.Has(removeRepository)
	if found {
		t.Errorf("repository %s not deleted", removeRepository)
	}
}

func TestUpdateRepository(t *testing.T) {
	sampleRepository := NewRepoFile()
	sampleRepository.Add(
		&Entry{
			Name:  "stable",
			URL:   "https://example.com/stable/charts",
			Cache: "stable-index.yaml",
		},
		&Entry{
			Name:  "incubator",
			URL:   "https://example.com/incubator",
			Cache: "incubator-index.yaml",
		},
	)
	newRepoName := "sample"
	sampleRepository.Update(&Entry{Name: newRepoName,
		URL:   "https://example.com/sample",
		Cache: "sample-index.yaml",
	})

	if !sampleRepository.Has(newRepoName) {
		t.Errorf("expected repository %s not found", newRepoName)
	}
	repoCount := len(sampleRepository.Repositories)

	sampleRepository.Update(&Entry{Name: newRepoName,
		URL:   "https://example.com/sample",
		Cache: "sample-index.yaml",
	})

	if repoCount != len(sampleRepository.Repositories) {
		t.Errorf("invalid number of repositories found %d, expected number of repositories %d", len(sampleRepository.Repositories), repoCount)
	}
}

func TestWriteFile(t *testing.T) {
	sampleRepository := NewRepoFile()
	sampleRepository.Add(
		&Entry{
			Name:  "stable",
			URL:   "https://example.com/stable/charts",
			Cache: "stable-index.yaml",
		},
		&Entry{
			Name:  "incubator",
			URL:   "https://example.com/incubator",
			Cache: "incubator-index.yaml",
		},
	)

	repoFile, err := ioutil.TempFile("", "helm-repo")
	if err != nil {
		t.Errorf("failed to create test-file (%v)", err)
	}
	defer os.Remove(repoFile.Name())
	if err := sampleRepository.WriteFile(repoFile.Name(), 744); err != nil {
		t.Errorf("failed to write file (%v)", err)
	}

	repos, err := LoadRepositoriesFile(repoFile.Name())
	if err != nil {
		t.Errorf("failed to load file (%v)", err)
	}
	for _, repo := range sampleRepository.Repositories {
		if !repos.Has(repo.Name) {
			t.Errorf("expected repository %s not found", repo.Name)
		}
	}
}
