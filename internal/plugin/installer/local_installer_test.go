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
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"

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

	// Create tarball content
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	files := []struct {
		Name string
		Body string
		Mode int64
	}{
		{"test-plugin/plugin.yaml", "name: test-plugin\napiVersion: v1\ntype: cli/v1\nruntime: subprocess\nversion: 1.0.0\nconfig:\n  shortHelp: test\n  longHelp: test\nruntimeConfig:\n  platformCommand:\n  - command: echo", 0644},
		{"test-plugin/bin/test-plugin", "#!/usr/bin/env sh\necho test", 0755},
	}

	for _, file := range files {
		hdr := &tar.Header{
			Name: file.Name,
			Mode: file.Mode,
			Size: int64(len(file.Body)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(file.Body)); err != nil {
			t.Fatal(err)
		}
	}

	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gw.Close(); err != nil {
		t.Fatal(err)
	}

	// Write tarball to file
	if err := os.WriteFile(tarballPath, buf.Bytes(), 0644); err != nil {
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
