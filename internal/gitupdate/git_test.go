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

package gitupdate

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdatePushesExistingBranch(t *testing.T) {
	remotePath, remote := createRemoteRepository(t)

	result, err := Update(context.Background(), repositoryFixture(remotePath, EditOptions{
		Method:          MethodYAMLSet,
		Content:         []byte("v2"),
		ContentProvided: true,
		Pointer:         "/image/tag",
	}))
	require.NoError(t, err)
	assert.True(t, result.Changed)
	assert.True(t, result.Committed)
	assert.True(t, result.Pushed)
	assert.Len(t, result.CommitHash, 40)

	commit := branchCommit(t, remote, "main")
	assert.Equal(t, "update image tag", commit.Message)
	assert.Equal(t, "Automation", commit.Author.Name)
	assert.Equal(t, "automation@example.com", commit.Author.Email)
	assert.Equal(t, "image:\n  repository: example/app\n  tag: v2\n", commitFile(t, commit, "values.yaml"))
}

func TestUpdateDryRunDoesNotPush(t *testing.T) {
	remotePath, remote := createRemoteRepository(t)
	before := branchCommit(t, remote, "main").Hash

	options := repositoryFixture(remotePath, textOptions(MethodOverwrite, "changed\n"))
	options.DryRun = true
	result, err := Update(context.Background(), options)
	require.NoError(t, err)
	assert.True(t, result.Changed)
	assert.False(t, result.Committed)
	assert.False(t, result.Pushed)
	assert.Equal(t, before, branchCommit(t, remote, "main").Hash)
}

func TestUpdateReportsNoChange(t *testing.T) {
	remotePath, remote := createRemoteRepository(t)
	before := branchCommit(t, remote, "main").Hash

	result, err := Update(context.Background(), repositoryFixture(remotePath, EditOptions{
		Method:          MethodYAMLSet,
		Content:         []byte("v1"),
		ContentProvided: true,
		Pointer:         "/image/tag",
	}))
	require.NoError(t, err)
	assert.False(t, result.Changed)
	assert.False(t, result.Committed)
	assert.False(t, result.Pushed)
	assert.Equal(t, before, branchCommit(t, remote, "main").Hash)
}

func TestUpdateCreatesBranch(t *testing.T) {
	remotePath, remote := createRemoteRepository(t)
	options := repositoryFixture(remotePath, textOptions(MethodOverwrite, "new branch\n"))
	options.Branch = "automation/image-update"
	options.BaseBranch = "main"
	options.CreateBranch = true

	result, err := Update(context.Background(), options)
	require.NoError(t, err)
	assert.True(t, result.Pushed)
	assert.Equal(t, "new branch\n", commitFile(t, branchCommit(t, remote, "automation/image-update"), "values.yaml"))
	assert.Equal(t, "image:\n  repository: example/app\n  tag: v1\n", commitFile(t, branchCommit(t, remote, "main"), "values.yaml"))
}

func TestUpdateRefusesToCreateExistingBranch(t *testing.T) {
	remotePath, _ := createRemoteRepository(t)
	options := repositoryFixture(remotePath, textOptions(MethodOverwrite, "changed\n"))
	options.Branch = "main"
	options.BaseBranch = "other"
	options.CreateBranch = true

	// Add an independent base branch so the command can clone it before checking
	// whether the target already exists.
	seedPath := filepath.Join(t.TempDir(), "other-seed")
	seed, err := git.PlainClone(seedPath, false, &git.CloneOptions{
		URL:           remotePath,
		ReferenceName: plumbing.NewBranchReferenceName("main"),
		SingleBranch:  true,
	})
	require.NoError(t, err)
	worktree, err := seed.Worktree()
	require.NoError(t, err)
	require.NoError(t, worktree.Checkout(&git.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName("other"),
		Create: true,
	}))
	require.NoError(t, seed.Push(&git.PushOptions{
		RefSpecs: []config.RefSpec{"refs/heads/other:refs/heads/other"},
	}))

	_, err = Update(context.Background(), options)
	require.ErrorContains(t, err, `branch "main": it already exists on the remote`)
}

func createRemoteRepository(t *testing.T) (string, *git.Repository) {
	t.Helper()
	root := t.TempDir()
	seedPath := filepath.Join(root, "seed")
	remotePath := filepath.Join(root, "remote.git")

	seed, err := git.PlainInit(seedPath, false)
	require.NoError(t, err)
	require.NoError(t, seed.Storer.SetReference(plumbing.NewSymbolicReference(
		plumbing.HEAD,
		plumbing.NewBranchReferenceName("main"),
	)))
	require.NoError(t, os.WriteFile(
		filepath.Join(seedPath, "values.yaml"),
		[]byte("image:\n  repository: example/app\n  tag: v1\n"),
		0o644,
	))
	worktree, err := seed.Worktree()
	require.NoError(t, err)
	_, err = worktree.Add("values.yaml")
	require.NoError(t, err)
	_, err = worktree.Commit("initial", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Initial Author",
			Email: "initial@example.com",
			When:  time.Unix(1_700_000_000, 0).UTC(),
		},
	})
	require.NoError(t, err)

	remote, err := git.PlainInit(remotePath, true)
	require.NoError(t, err)
	_, err = seed.CreateRemote(&config.RemoteConfig{Name: "origin", URLs: []string{remotePath}})
	require.NoError(t, err)
	require.NoError(t, seed.Push(&git.PushOptions{
		RefSpecs: []config.RefSpec{"refs/heads/main:refs/heads/main"},
	}))
	return remotePath, remote
}

func repositoryFixture(repositoryURL string, edit EditOptions) RepositoryOptions {
	return RepositoryOptions{
		RepositoryURL: repositoryURL,
		Branch:        "main",
		TargetFile:    "values.yaml",
		Edit:          edit,
		Auth:          AuthOptions{Type: AuthNone},
		AuthorName:    "Automation",
		AuthorEmail:   "automation@example.com",
		CommitMessage: "update image tag",
		Depth:         1,
		Push:          true,
		Now: func() time.Time {
			return time.Unix(1_700_000_100, 0).UTC()
		},
	}
}

func branchCommit(t *testing.T, repository *git.Repository, branch string) *object.Commit {
	t.Helper()
	reference, err := repository.Reference(plumbing.NewBranchReferenceName(branch), true)
	require.NoError(t, err)
	commit, err := repository.CommitObject(reference.Hash())
	require.NoError(t, err)
	return commit
}

func commitFile(t *testing.T, commit *object.Commit, filename string) string {
	t.Helper()
	file, err := commit.File(filename)
	require.NoError(t, err)
	content, err := file.Contents()
	require.NoError(t, err)
	return content
}
