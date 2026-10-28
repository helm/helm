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
	"path/filepath"
	"runtime"
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
	sampleRepository.Update(&Entry{
		Name: newRepoName,
		URL:  "https://example.com/sample",
	})

	assert.Truef(t, sampleRepository.Has(newRepoName), "expected repository %s not found", newRepoName)
	repoCount := len(sampleRepository.Repositories)

	sampleRepository.Update(&Entry{
		Name: newRepoName,
		URL:  "https://example.com/sample",
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

func TestWriteFileKeepsPreviousContentsOnFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("a read-only directory does not prevent replacing a file on Windows")
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "repositories.yaml")

	old := NewFile()
	old.Add(&Entry{Name: "stable", URL: "https://example.com/stable/charts"})
	require.NoErrorf(t, old.WriteFile(path, 0o600), "failed to write the initial file")

	require.NoErrorf(t, os.Chmod(dir, 0o500), "failed to make %q read-only", dir)
	t.Cleanup(func() {
		require.NoErrorf(t, os.Chmod(dir, 0o700), "failed to restore permissions on %q", dir)
	})

	fresh := NewFile()
	fresh.Add(&Entry{Name: "incubator", URL: "https://example.com/incubator"})
	err := fresh.WriteFile(path, 0o600)
	require.Error(t, err, "expected the write to fail in a directory that rejects new files")

	repos, err := LoadFile(path)
	require.NoError(t, err, "the previous file must still load")
	assert.Truef(t, repos.Has("stable"), "expected the previous entry to survive, got %v", repos.Repositories)
	assert.Falsef(t, repos.Has("incubator"), "unexpected entry from the failed write: %v", repos.Repositories)
}

func TestWriteFileRelativeBareNameUsesDestinationDir(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	f := NewFile()
	f.Add(&Entry{Name: "stable", URL: "https://example.com/stable/charts"})
	require.NoError(t, f.WriteFile("repositories.yaml", 0o600))

	dest := filepath.Join(dir, "repositories.yaml")
	repos, err := LoadFile(dest)
	require.NoError(t, err)
	assert.True(t, repos.Has("stable"))

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	assert.Equal(t, []string{"repositories.yaml"}, names)
}

func TestWriteFileUpdatesSymlinkTarget(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "shared-repositories.yaml")
	link := filepath.Join(dir, "repositories.yaml")

	old := NewFile()
	old.Add(&Entry{Name: "stable", URL: "https://example.com/stable/charts"})
	require.NoError(t, old.WriteFile(target, 0o640))
	require.NoError(t, os.Symlink(target, link))

	fresh := NewFile()
	fresh.Add(&Entry{Name: "stable", URL: "https://example.com/stable/charts"})
	fresh.Add(&Entry{Name: "incubator", URL: "https://example.com/incubator"})
	require.NoError(t, fresh.WriteFile(link, 0o600))

	fi, err := os.Lstat(link)
	require.NoError(t, err)
	assert.NotEqual(t, os.FileMode(0), fi.Mode()&os.ModeSymlink, "destination symlink must be preserved")

	repos, err := LoadFile(target)
	require.NoError(t, err)
	assert.True(t, repos.Has("stable"))
	assert.True(t, repos.Has("incubator"))

	info, err := os.Stat(target)
	require.NoError(t, err)
	if runtime.GOOS != "windows" {
		assert.Equalf(t, os.FileMode(0o640), info.Mode().Perm(), "existing target mode must be preserved")
	}
}

func TestWriteFilePreservesExistingPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not implement POSIX permission bits")
	}

	path := filepath.Join(t.TempDir(), "repositories.yaml")

	old := NewFile()
	old.Add(&Entry{Name: "stable", URL: "https://example.com/stable/charts"})
	require.NoError(t, old.WriteFile(path, 0o640))

	info, err := os.Stat(path)
	require.NoError(t, err)
	require.Equalf(t, os.FileMode(0o640), info.Mode().Perm(), "expected the first write to honor 0o640")

	fresh := NewFile()
	fresh.Add(&Entry{Name: "stable", URL: "https://example.com/stable/charts"})
	fresh.Add(&Entry{Name: "incubator", URL: "https://example.com/incubator"})
	require.NoError(t, fresh.WriteFile(path, 0o600))

	info, err = os.Stat(path)
	require.NoError(t, err)
	assert.Equalf(t, os.FileMode(0o640), info.Mode().Perm(), "existing file mode must be preserved")
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
