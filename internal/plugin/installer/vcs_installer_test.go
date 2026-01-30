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

package installer // import "helm.sh/helm/v4/internal/plugin/installer"

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Masterminds/vcs"

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

	if err := os.MkdirAll(helmpath.DataPath("plugins"), 0755); err != nil {
		t.Fatalf("Could not create %s: %s", helmpath.DataPath("plugins"), err)
	}

	source := "https://github.com/adamreese/helm-env"
	testRepoPath, _ := filepath.Abs("../testdata/plugdir/good/echo-v1")
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
	expectedPath := helmpath.DataPath("plugins", "helm-env")
	if i.Path() != expectedPath {
		t.Fatalf("expected path %q, got %q", expectedPath, i.Path())
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
	ensure.HelmHome(t)

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
	} else if strings.Contains(err.Error(), "Could not resolve host: github.com") {
		t.Skip("Unable to run test without Internet access")
	} else if err.Error() != fmt.Sprintf("requested version %q does not exist for plugin %q", version, source) {
		t.Fatalf("expected error for version does not exists, got (%v)", err)
	}
}
func TestVCSInstallerUpdate(t *testing.T) {
	ensure.HelmHome(t)

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
		if strings.Contains(err.Error(), "Could not resolve host: github.com") {
			t.Skip("Unable to run test without Internet access")
		} else {
			t.Fatal(err)
		}
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

	// Test that local modifications are automatically reset during update
	// Remove plugin.yaml to simulate modifications
	pluginYamlPath := filepath.Join(vcsInstaller.Repo.LocalPath(), "plugin.yaml")
	if err := os.Remove(pluginYamlPath); err != nil {
		t.Fatal(err)
	}

	// Verify the repo is dirty
	if !vcsInstaller.Repo.IsDirty() {
		t.Fatal("expected repo to be dirty after removing plugin.yaml")
	}

	// Update should succeed because local modifications are automatically reset
	if err := Update(vcsInstaller); err != nil {
		t.Fatalf("update should succeed after automatic reset, got error: %v", err)
	}

	// Verify plugin.yaml was restored after reset
	if _, err := os.Stat(pluginYamlPath); err != nil {
		t.Fatalf("plugin.yaml should be restored after update, got error: %v", err)
	}

}

func TestResetPluginYaml(t *testing.T) {
	// Use a real git repository by cloning a test repo
	ensure.HelmHome(t)

	source := "https://github.com/adamreese/helm-env"

	i, err := NewForSource(source, "")
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	vcsInstaller, ok := i.(*VCSInstaller)
	if !ok {
		t.Fatal("expected a VCSInstaller")
	}

	// Clone the repository
	if err := vcsInstaller.sync(vcsInstaller.Repo); err != nil {
		if strings.Contains(err.Error(), "Could not resolve host: github.com") {
			t.Skip("Unable to run test without Internet access")
		}
		t.Fatalf("Failed to sync repo: %v", err)
	}

	// Modify plugin.yaml to make the repo dirty
	pluginYaml := filepath.Join(vcsInstaller.Repo.LocalPath(), "plugin.yaml")
	originalContent, err := os.ReadFile(pluginYaml)
	if err != nil {
		t.Fatalf("Failed to read plugin.yaml: %v", err)
	}

	modifiedContent := append(originalContent, []byte("\n# Test modification\n")...)
	if err := os.WriteFile(pluginYaml, modifiedContent, 0644); err != nil {
		t.Fatalf("Failed to modify plugin.yaml: %v", err)
	}

	// Verify the repo is dirty
	if !vcsInstaller.Repo.IsDirty() {
		t.Fatal("Expected repo to be dirty after modifying plugin.yaml")
	}

	// Reset only plugin.yaml
	if err := resetPluginYaml(vcsInstaller.Repo); err != nil {
		t.Fatalf("resetPluginYaml failed: %v", err)
	}

	// Verify the repo is clean
	if vcsInstaller.Repo.IsDirty() {
		t.Fatal("Expected repo to be clean after resetting plugin.yaml")
	}

	// Verify the plugin.yaml was restored to original content
	restoredContent, err := os.ReadFile(pluginYaml)
	if err != nil {
		t.Fatalf("Failed to read plugin.yaml after reset: %v", err)
	}

	if string(restoredContent) != string(originalContent) {
		t.Fatal("Expected plugin.yaml to be restored to original content after reset")
	}
}
