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

package installer // import "k8s.io/helm/pkg/plugin/installer"

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"k8s.io/helm/pkg/helm/helmpath"

	"github.com/Masterminds/vcs"
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
	hh, err := ioutil.TempDir("", "helm-home-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(hh)

	home := helmpath.Home(hh)
	if err := os.MkdirAll(home.Plugins(), 0755); err != nil {
		t.Fatalf("Could not create %s: %s", home.Plugins(), err)
	}

	source := "https://github.com/adamreese/helm-env"
	testRepoPath, _ := filepath.Abs("../testdata/plugdir/echo")
	repo := &testRepo{
		local: testRepoPath,
		tags:  []string{"0.1.0", "0.1.1"},
	}

	i, err := NewForSource(source, "~0.1.0", home)
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
		t.Errorf("expected version '0.1.1', got %q", repo.current)
	}
	if i.Path() != home.Path("plugins", "helm-env") {
		t.Errorf("expected path '$HELM_HOME/plugins/helm-env', got %q", i.Path())
	}

	// Install again to test plugin exists error
	if err := Install(i); err == nil {
		t.Error("expected error for plugin exists, got none")
	} else if err.Error() != "plugin already exists" {
		t.Errorf("expected error for plugin exists, got (%v)", err)
	}

	//Testing FindSource method, expect error because plugin code is not a cloned repository
	if _, err := FindSource(i.Path(), home); err == nil {
		t.Error("expected error for inability to find plugin source, got none")
	} else if err.Error() != "cannot get information about plugin source" {
		t.Errorf("expected error for inability to find plugin source, got (%v)", err)
	}
}

func TestVCSInstallerNonExistentVersion(t *testing.T) {
	hh, err := ioutil.TempDir("", "helm-home-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(hh)

	home := helmpath.Home(hh)
	if err := os.MkdirAll(home.Plugins(), 0755); err != nil {
		t.Fatalf("Could not create %s: %s", home.Plugins(), err)
	}

	source := "https://github.com/adamreese/helm-env"
	version := "0.2.0"

	i, err := NewForSource(source, version, home)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	// ensure a VCSInstaller was returned
	_, ok := i.(*VCSInstaller)
	if !ok {
		t.Fatal("expected a VCSInstaller")
	}

	if err := Install(i); err == nil {
		t.Error("expected error for version does not exists, got none")
	} else if err.Error() != fmt.Sprintf("requested version %q does not exist for plugin %q", version, source) {
		t.Errorf("expected error for version does not exists, got (%v)", err)
	}
}
func TestVCSInstallerUpdate(t *testing.T) {

	hh, err := ioutil.TempDir("", "helm-home-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(hh)

	home := helmpath.Home(hh)
	if err := os.MkdirAll(home.Plugins(), 0755); err != nil {
		t.Fatalf("Could not create %s: %s", home.Plugins(), err)
	}

	source := "https://github.com/adamreese/helm-env"

	i, err := NewForSource(source, "", home)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	// ensure a VCSInstaller was returned
	_, ok := i.(*VCSInstaller)
	if !ok {
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
	pluginInfo, err := FindSource(i.Path(), home)
	if err != nil {
		t.Fatal(err)
	}

	repoRemote := pluginInfo.(*VCSInstaller).Repo.Remote()
	if repoRemote != source {
		t.Fatalf("invalid source found, expected %q got %q", source, repoRemote)
	}

	// Update plugin
	if err := Update(i); err != nil {
		t.Fatal(err)
	}

	// Test update failure
	os.Remove(filepath.Join(i.Path(), "plugin.yaml"))
	// Testing update for error
	if err := Update(i); err == nil {
		t.Error("expected error for plugin modified, got none")
	} else if err.Error() != "plugin repo was modified" {
		t.Errorf("expected error for plugin modified, got (%v)", err)
	}

}
