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
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

	require.Len(t, rf.Repositories, 2, "Expected 2 repositories")

	assert.False(t, rf.Has("nosuchrepo"), "Found nonexistent repo")
	assert.True(t, rf.Has("incubator"), "incubator repo is missing")

	stable := rf.Repositories[0]
	assert.Equal(t, "stable", stable.Name, "stable is not named stable")
	assert.Equal(t, "https://example.com/stable/charts", stable.URL, "Wrong URL for stable")
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
	require.NoErrorf(t, err, "%q could not be loaded", testRepositoriesFile)

	require.Lenf(t, file.Repositories, len(expects.Repositories), "Unexpected repo data: %#v", file.Repositories)

	for i, expect := range expects.Repositories {
		got := file.Repositories[i]
		assert.Equalf(t, expect.Name, got.Name, "Expected name %q, got %q", expect.Name, got.Name)
		assert.Equalf(t, expect.URL, got.URL, "Expected url %q, got %q", expect.URL, got.URL)
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
	require.NotNilf(t, entry, "Expected repo entry %q to be found", name)

	assert.Equalf(t, "https://example.com/second", entry.URL, "Expected repo URL to be %q but got %q", "https://example.com/second", entry.URL)

	entry = repo.Get("nonexistent")
	assert.Nilf(t, entry, "Got unexpected entry %+v", entry)
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
	assert.Truef(t, sampleRepository.Remove(removeRepository), "expected repository %s not found", removeRepository)
	assert.Falsef(t, sampleRepository.Has(removeRepository), "repository %s not deleted", removeRepository)
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

	assert.Truef(t, sampleRepository.Has(newRepoName), "expected repository %s not found", newRepoName)
	repoCount := len(sampleRepository.Repositories)

	sampleRepository.Update(&Entry{Name: newRepoName,
		URL: "https://example.com/sample",
	})

	assert.Lenf(t, sampleRepository.Repositories, repoCount, "invalid number of repositories found %d, expected number of repositories %d", len(sampleRepository.Repositories), repoCount)
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

	file, err := os.CreateTemp(t.TempDir(), "helm-repo")
	require.NoErrorf(t, err, "failed to create test-file")
	defer os.Remove(file.Name())
	require.NoErrorf(t, sampleRepository.WriteFile(file.Name(), 0o600), "failed to write file")

	repos, err := LoadFile(file.Name())
	require.NoErrorf(t, err, "failed to load file")
	for _, repo := range sampleRepository.Repositories {
		assert.Truef(t, repos.Has(repo.Name), "expected repository %s not found", repo.Name)
	}
}

func TestRepoNotExists(t *testing.T) {
	_, err := LoadFile("/this/path/does/not/exist.yaml")
	require.Error(t, err, "expected err to be non-nil when path does not exist")
	assert.ErrorContains(t, err, "couldn't load repositories file", "expected prompt `couldn't load repositories file`")
}

func TestRemoveRepositoryInvalidEntries(t *testing.T) {
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
		&Entry{},
		nil,
		&Entry{
			Name: "test",
			URL:  "https://example.com/test",
		},
	)

	removeRepository := "stable"
	assert.Truef(t, sampleRepository.Remove(removeRepository), "expected repository %s not found", removeRepository)
	assert.Falsef(t, sampleRepository.Has(removeRepository), "repository %s not deleted", removeRepository)
}
