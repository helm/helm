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
	"os/exec"
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

// runGit runs a git command in dir, failing the test on error.
func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	args = append([]string{"-C", dir, "-c", "user.name=Helm Test", "-c", "user.email=helm@example.com", "-c", "commit.gpgsign=false"}, args...)
	out, err := exec.CommandContext(t.Context(), "git", args...).CombinedOutput()
	require.NoErrorf(t, err, "git %v: %s", args, out)
}

// TestVCSInstallerUpdateHookModifiedPlugin ensures a plugin can still be
// updated after its install hook has modified files inside the installed
// plugin directory, as helm-unittest's install-binary.sh does. Update must
// run against Helm's cached repository, not the installed copy, whose
// checkout is legitimately dirtied by hooks. See
// https://github.com/helm/helm/issues/31664.
func TestVCSInstallerUpdateHookModifiedPlugin(t *testing.T) {
	ensure.HelmHome(t)

	require.NoErrorf(t, os.MkdirAll(helmpath.DataPath("plugins"), 0o755), "Could not create %s", helmpath.DataPath("plugins"))

	// Create a local git repository containing the plugin.
	upstream := filepath.Join(t.TempDir(), "hookmod")
	require.NoError(t, os.MkdirAll(upstream, 0o755))
	pluginYAML := "name: \"hookmod\"\nversion: \"0.1.0\"\ntype: cli/v1\napiVersion: v1\nruntime: subprocess\nconfig:\n  shortHelp: \"hook modified plugin\"\n  longHelp: \"hook modified plugin\"\nruntimeConfig:\n  command: \"echo Hello\"\n"
	require.NoError(t, os.WriteFile(filepath.Join(upstream, "plugin.yaml"), []byte(pluginYAML), 0o644))
	runGit(t, upstream, "init")
	runGit(t, upstream, "add", "plugin.yaml")
	runGit(t, upstream, "commit", "-m", "initial commit")

	source := "file://" + upstream
	i, err := NewForSource(source, "")
	require.NoError(t, err)
	require.IsType(t, &VCSInstaller{}, i, "expected a VCSInstaller")
	require.NoError(t, Install(i))

	// Simulate an install hook rewriting a tracked file inside the
	// installed plugin directory.
	require.NoError(t, os.WriteFile(filepath.Join(i.Path(), "plugin.yaml"), []byte(pluginYAML+"# rewritten by install hook\n"), 0o644))

	// Publish a new version upstream.
	require.NoError(t, os.WriteFile(filepath.Join(upstream, "plugin.yaml"), []byte(strings.ReplaceAll(pluginYAML, "0.1.0", "0.2.0")), 0o644))
	runGit(t, upstream, "commit", "-am", "release 0.2.0")

	upd, err := FindSource(i.Path())
	require.NoError(t, err)
	require.NoError(t, Update(upd), "update must not treat hook-produced changes as user modifications")

	// The installed copy is refreshed from the clean checkout.
	data, err := os.ReadFile(filepath.Join(i.Path(), "plugin.yaml"))
	require.NoError(t, err)
	require.Contains(t, string(data), `version: "0.2.0"`)
	require.NotContains(t, string(data), "rewritten by install hook")

	// A second update must keep working.
	upd, err = FindSource(i.Path())
	require.NoError(t, err)
	require.NoError(t, Update(upd))
}
