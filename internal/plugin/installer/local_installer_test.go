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
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"helm.sh/helm/v4/internal/test/ensure"
	"helm.sh/helm/v4/pkg/helmpath"
)

var _ Installer = new(LocalInstaller)

func TestLocalInstaller(t *testing.T) {
	ensure.HelmHome(t)
	// Make a temp dir
	tdir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tdir, "plugin.yaml"), []byte{}, 0644); err != nil {
		t.Fatal(err)
	}

	source := "../testdata/plugdir/good/echo-v1"
	i, err := NewForSource(source, "")
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	if err := Install(i); err != nil {
		t.Fatal(err)
	}

	if i.Path() != helmpath.DataPath("plugins", "echo-v1") {
		t.Fatalf("expected path '$XDG_CONFIG_HOME/helm/plugins/helm-env', got %q", i.Path())
	}
	defer os.RemoveAll(filepath.Dir(helmpath.DataPath())) // helmpath.DataPath is like /tmp/helm013130971/helm
}

func TestLocalInstallerNotAFolder(t *testing.T) {
	source := "../testdata/plugdir/good/echo-v1/plugin.yaml"
	i, err := NewForSource(source, "")
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	err = Install(i)
	if err == nil {
		t.Fatal("expected error")
	}
	if err != ErrPluginNotADirectory {
		t.Fatalf("expected error to equal: %q", err)
	}
}

func TestLocalInstallerTarball(t *testing.T) {
	ensure.HelmHome(t)

	// Create a test tarball
	tempDir := t.TempDir()
	tarballPath := filepath.Join(tempDir, "test-plugin-1.0.0.tar.gz")

	files := []struct {
		Name string
		Body string
		Mode int64
	}{
		{"test-plugin/plugin.yaml", "name: test-plugin\napiVersion: v1\ntype: cli/v1\nruntime: subprocess\nversion: 1.0.0\nconfig:\n  shortHelp: test\n  longHelp: test\nruntimeConfig:\n  platformCommand:\n  - command: echo", 0644},
		{"test-plugin/bin/test-plugin", "#!/bin/bash\necho test", 0755},
	}

	// Write tarball to file
	if err := os.WriteFile(tarballPath, createTestTarball(t, files), 0644); err != nil {
		t.Fatal(err)
	}

	// Test installation
	i, err := NewForSource(tarballPath, "")
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	// Verify it's detected as LocalInstaller
	localInstaller, ok := i.(*LocalInstaller)
	if !ok {
		t.Fatal("expected LocalInstaller")
	}

	if !localInstaller.isArchive {
		t.Fatal("expected isArchive to be true")
	}

	if err := Install(i); err != nil {
		t.Fatal(err)
	}

	expectedPath := helmpath.DataPath("plugins", "test-plugin")
	if i.Path() != expectedPath {
		t.Fatalf("expected path %q, got %q", expectedPath, i.Path())
	}

	// Verify plugin was installed
	if _, err := os.Stat(i.Path()); err != nil {
		t.Fatalf("plugin not found at %s: %v", i.Path(), err)
	}
}

func TestLocalInstaller_IgnoresGitDir(t *testing.T) {
	ensure.HelmHome(t)

	// Create a plugin directory with .git
	pluginDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(pluginDir, "plugin.yaml"), []byte("name: test-plugin\nversion: 1.0.0"), 0644); err != nil {
		t.Fatal(err)
	}
	gitDir := filepath.Join(pluginDir, ".git")
	if err := os.MkdirAll(gitDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gitDir, "config"), []byte("git config"), 0644); err != nil {
		t.Fatal(err)
	}

	i, err := NewForSource(pluginDir, "")
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	if err := Install(i); err != nil {
		t.Fatal(err)
	}

	// Verify .git was not copied
	installedGitDir := filepath.Join(i.Path(), ".git")
	_, err = os.Stat(installedGitDir)
	assert.True(t, os.IsNotExist(err), "expected .git directory to not exist, got %v", err)

	// Verify plugin.yaml was copied
	if _, err := os.Stat(filepath.Join(i.Path(), "plugin.yaml")); err != nil {
		t.Fatal("plugin.yaml should have been copied")
	}
}
