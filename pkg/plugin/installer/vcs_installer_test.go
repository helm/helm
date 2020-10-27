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

package installer // import "helm.sh/helm/v3/pkg/plugin/installer"

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/Masterminds/vcs"

	"helm.sh/helm/v3/internal/test/ensure"
	"helm.sh/helm/v3/pkg/helmpath"
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
	defer ensure.HelmHome(t)()

	if err := os.MkdirAll(helmpath.DataPath("plugins"), 0755); err != nil {
		t.Fatalf("Could not create %s: %s", helmpath.DataPath("plugins"), err)
	}

	source := "https://github.com/adamreese/helm-env"
	testRepoPath, _ := filepath.Abs("../testdata/plugdir/good/echo")
	repo := &testRepo{
		local: testRepoPath,
		tags:  []string{"0.1.0", "0.1.1"},
	}

	i, err := NewForSource(source, "~0.1.0")
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	// ensure a VCSInstaller was returned
	vcsInstaller, ok := i.(*VCSInstaller)
	if !ok {
		t.Fatal("expected a VCSInstaller")
	}

	// set the testRepo in the VCSInstaller
	vcsInstaller.Repo = repo

	if err := Install(i); err != nil {
		t.Fatal(err)
	}
	if repo.current != "0.1.1" {
		t.Fatalf("expected version '0.1.1', got %q", repo.current)
	}
	if i.Path() != helmpath.DataPath("plugins", "helm-env") {
		t.Fatalf("expected path '$XDG_CONFIG_HOME/helm/plugins/helm-env', got %q", i.Path())
	}

	// Install again to test plugin exists error
	if err := Install(i); err == nil {
		t.Fatalf("expected error for plugin exists, got none")
	} else if err.Error() != "plugin already exists" {
		t.Fatalf("expected error for plugin exists, got (%v)", err)
	}

	// Testing FindSource method, expect error because plugin code is not a cloned repository
	if _, err := FindSource(i.Path()); err == nil {
		t.Fatalf("expected error for inability to find plugin source, got none")
	} else if err.Error() != "cannot get information about plugin source" {
		t.Fatalf("expected error for inability to find plugin source, got (%v)", err)
	}
}

func TestVCSInstallerNonExistentVersion(t *testing.T) {
	defer ensure.HelmHome(t)()

	source := "https://github.com/adamreese/helm-env"
	version := "0.2.0"

	i, err := NewForSource(source, version)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	// ensure a VCSInstaller was returned
	if _, ok := i.(*VCSInstaller); !ok {
		t.Fatal("expected a VCSInstaller")
	}

	if err := Install(i); err == nil {
		t.Fatalf("expected error for version does not exists, got none")
	} else if err.Error() != fmt.Sprintf("requested version %q does not exist for plugin %q", version, source) {
		t.Fatalf("expected error for version does not exists, got (%v)", err)
	}
}
func TestVCSInstallerUpdate(t *testing.T) {
	defer ensure.HelmHome(t)()

	source := "https://github.com/adamreese/helm-env"

	i, err := NewForSource(source, "")
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	// ensure a VCSInstaller was returned
	if _, ok := i.(*VCSInstaller); !ok {
		t.Fatal("expected a VCSInstaller")
	}

	if err := Update(i); err == nil {
		t.Fatal("expected error for plugin does not exist, got none")
	} else if err.Error() != "plugin does not exist" {
		t.Fatalf("expected error for plugin does not exist, got (%v)", err)
	}

	// Install plugin before update
	if err := Install(i); err != nil {
		t.Fatal(err)
	}

	// Test FindSource method for positive result
	pluginInfo, err := FindSource(i.Path())
	if err != nil {
		t.Fatal(err)
	}

	vcsInstaller := pluginInfo.(*VCSInstaller)

	repoRemote := vcsInstaller.Repo.Remote()
	if repoRemote != source {
		t.Fatalf("invalid source found, expected %q got %q", source, repoRemote)
	}

	// Update plugin
	if err := Update(i); err != nil {
		t.Fatal(err)
	}

	// Test update failure
	if err := os.Remove(filepath.Join(vcsInstaller.Repo.LocalPath(), "plugin.yaml")); err != nil {
		t.Fatal(err)
	}
	// Testing update for error
	if err := Update(vcsInstaller); err == nil {
		t.Fatalf("expected error for plugin modified, got none")
	} else if err.Error() != "plugin repo was modified" {
		t.Fatalf("expected error for plugin modified, got (%v)", err)
	}

}
