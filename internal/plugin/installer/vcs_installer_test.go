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

package installer

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Masterminds/vcs"
	"github.com/stretchr/testify/require"

	"helm.sh/helm/v4/internal/test/ensure"
	"helm.sh/helm/v4/pkg/helmpath"
)

var _ Installer = new(VCSInstaller)

type testRepo struct {
	local, remote, current string
	tags, branches         []string
	err                    error
	vcs.Repo
}

func (r *testRepo) LocalPath() string           { return r.local }
func (r *testRepo) Remote() string              { return r.remote }
func (r *testRepo) Update() error               { return r.err }
func (r *testRepo) Get() error                  { return r.err }
func (r *testRepo) IsReference(string) bool     { return false }
func (r *testRepo) Tags() ([]string, error)     { return r.tags, r.err }
func (r *testRepo) Branches() ([]string, error) { return r.branches, r.err }
func (r *testRepo) UpdateVersion(version string) error {
	r.current = version
	return r.err
}

func TestVCSInstaller(t *testing.T) {
	ensure.HelmHome(t)

	require.NoErrorf(t, os.MkdirAll(helmpath.DataPath("plugins"), 0o755), "Could not create %s", helmpath.DataPath("plugins"))

	source := "https://github.com/adamreese/helm-env"
	testRepoPath, _ := filepath.Abs("../testdata/plugdir/good/echo-v1")
	repo := &testRepo{
		local: testRepoPath,
		tags:  []string{"0.1.0", "0.1.1"},
	}

	i, err := NewForSource(source, "~0.1.0")
	require.NoError(t, err)

	// ensure a VCSInstaller was returned
	vcsInstaller, ok := i.(*VCSInstaller)
	require.True(t, ok, "expected a VCSInstaller")

	// set the testRepo in the VCSInstaller
	vcsInstaller.Repo = repo

	require.NoError(t, Install(i))
	require.Equal(t, "0.1.1", repo.current, "expected version '0.1.1', got %q", repo.current)
	expectedPath := helmpath.DataPath("plugins", "helm-env")
	require.Equal(t, expectedPath, i.Path(), "expected path %q, got %q", expectedPath, i.Path())

	// Install again to test plugin exists error
	require.EqualErrorf(t, Install(i), "plugin already exists", "expected error for plugin exists")

	// Testing FindSource method, expect error because plugin code is not a cloned repository
	_, err = FindSource(i.Path())
	require.Error(t, err, "expected error for inability to find plugin source, got none")
	require.EqualErrorf(t, err, "cannot get information about plugin source", "expected error for inability to find plugin source")
}

func TestVCSInstallerNonExistentVersion(t *testing.T) {
	ensure.HelmHome(t)

	source := "https://github.com/adamreese/helm-env"
	version := "0.2.0"

	i, err := NewForSource(source, version)
	require.NoError(t, err)

	// ensure a VCSInstaller was returned
	require.IsType(t, &VCSInstaller{}, i, "expected a VCSInstaller")

	err = Install(i)
	require.Error(t, err, "expected error for version does not exists, got none")
	if strings.Contains(err.Error(), "Could not resolve host: github.com") {
		t.Skip("Unable to run test without Internet access")
	}
	require.EqualErrorf(t, err, fmt.Sprintf("requested version %q does not exist for plugin %q", version, source), "expected error for version does not exists")
}
func TestVCSInstallerUpdate(t *testing.T) {
	ensure.HelmHome(t)

	source := "https://github.com/adamreese/helm-env"

	i, err := NewForSource(source, "")
	require.NoError(t, err)

	// ensure a VCSInstaller was returned
	require.IsType(t, &VCSInstaller{}, i, "expected a VCSInstaller")

	require.EqualErrorf(t, Update(i), "plugin does not exist", "expected error for plugin does not exist")

	// Install plugin before update
	err = Install(i)
	if err != nil && strings.Contains(err.Error(), "Could not resolve host: github.com") {
		t.Skip("Unable to run test without Internet access")
	}
	require.NoError(t, err)

	// Test FindSource method for positive result
	pluginInfo, err := FindSource(i.Path())
	require.NoError(t, err)

	vcsInstaller := pluginInfo.(*VCSInstaller)

	repoRemote := vcsInstaller.Repo.Remote()
	require.Equal(t, source, repoRemote, "invalid source found, expected %q got %q", source, repoRemote)

	// Update plugin
	require.NoError(t, Update(i))

	// Test update failure
	require.NoError(t, os.Remove(filepath.Join(vcsInstaller.Repo.LocalPath(), "plugin.yaml")))
	// Testing update for error
	require.EqualErrorf(t, Update(vcsInstaller), "plugin repo was modified", "expected error for plugin modified")
}
