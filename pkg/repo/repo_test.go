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

package repo

import (
	"io/ioutil"
	"os"
	"strings"
	"testing"
)

const testRepositoriesFile = "testdata/repositories.yaml"

func TestFile(t *testing.T) {
	rf := NewFile()
	rf.Add(
		&Entry{
			Name: "stable",
			URL:  "https://example.com/stable/charts",
		},
		&Entry{
			Name: "incubator",
			URL:  "https://example.com/incubator",
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
}

func TestNewFile(t *testing.T) {
	expects := NewFile()
	expects.Add(
		&Entry{
			Name: "stable",
			URL:  "https://example.com/stable/charts",
		},
		&Entry{
			Name: "incubator",
			URL:  "https://example.com/incubator",
		},
	)

	file, err := LoadFile(testRepositoriesFile)
	if err != nil {
		t.Errorf("%q could not be loaded: %s", testRepositoriesFile, err)
	}

	if len(expects.Repositories) != len(file.Repositories) {
		t.Fatalf("Unexpected repo data: %#v", file.Repositories)
	}

	for i, expect := range expects.Repositories {
		got := file.Repositories[i]
		if expect.Name != got.Name {
			t.Errorf("Expected name %q, got %q", expect.Name, got.Name)
		}
		if expect.URL != got.URL {
			t.Errorf("Expected url %q, got %q", expect.URL, got.URL)
		}
	}
}

func TestRepoFile_Get(t *testing.T) {
	repo := NewFile()
	repo.Add(
		&Entry{
			Name: "first",
			URL:  "https://example.com/first",
		},
		&Entry{
			Name: "second",
			URL:  "https://example.com/second",
		},
		&Entry{
			Name: "third",
			URL:  "https://example.com/third",
		},
		&Entry{
			Name: "fourth",
			URL:  "https://example.com/fourth",
		},
	)

	name := "second"

	entry := repo.Get(name)
	if entry == nil {
		t.Fatalf("Expected repo entry %q to be found", name)
	}

	if entry.URL != "https://example.com/second" {
		t.Errorf("Expected repo URL to be %q but got %q", "https://example.com/second", entry.URL)
	}

	entry = repo.Get("nonexistent")
	if entry != nil {
		t.Errorf("Got unexpected entry %+v", entry)
	}
}

func TestRemoveRepository(t *testing.T) {
	sampleRepository := NewFile()
	sampleRepository.Add(
		&Entry{
			Name: "stable",
			URL:  "https://example.com/stable/charts",
		},
		&Entry{
			Name: "incubator",
			URL:  "https://example.com/incubator",
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
	sampleRepository := NewFile()
	sampleRepository.Add(
		&Entry{
			Name: "stable",
			URL:  "https://example.com/stable/charts",
		},
		&Entry{
			Name: "incubator",
			URL:  "https://example.com/incubator",
		},
	)
	newRepoName := "sample"
	sampleRepository.Update(&Entry{Name: newRepoName,
		URL: "https://example.com/sample",
	})

	if !sampleRepository.Has(newRepoName) {
		t.Errorf("expected repository %s not found", newRepoName)
	}
	repoCount := len(sampleRepository.Repositories)

	sampleRepository.Update(&Entry{Name: newRepoName,
		URL: "https://example.com/sample",
	})

	if repoCount != len(sampleRepository.Repositories) {
		t.Errorf("invalid number of repositories found %d, expected number of repositories %d", len(sampleRepository.Repositories), repoCount)
	}
}

func TestWriteFile(t *testing.T) {
	sampleRepository := NewFile()
	sampleRepository.Add(
		&Entry{
			Name: "stable",
			URL:  "https://example.com/stable/charts",
		},
		&Entry{
			Name: "incubator",
			URL:  "https://example.com/incubator",
		},
	)

	file, err := ioutil.TempFile("", "helm-repo")
	if err != nil {
		t.Errorf("failed to create test-file (%v)", err)
	}
	defer os.Remove(file.Name())
	if err := sampleRepository.WriteFile(file.Name(), 0644); err != nil {
		t.Errorf("failed to write file (%v)", err)
	}

	repos, err := LoadFile(file.Name())
	if err != nil {
		t.Errorf("failed to load file (%v)", err)
	}
	for _, repo := range sampleRepository.Repositories {
		if !repos.Has(repo.Name) {
			t.Errorf("expected repository %s not found", repo.Name)
		}
	}
}

func TestRepoNotExists(t *testing.T) {
	if _, err := LoadFile("/this/path/does/not/exist.yaml"); err == nil {
		t.Errorf("expected err to be non-nil when path does not exist")
	} else if !strings.Contains(err.Error(), "couldn't load repositories file") {
		t.Errorf("expected prompt `couldn't load repositories file`")
	}
}
