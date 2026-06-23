//go:build !windows

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
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestExtract(t *testing.T) {
	source := "https://repo.localdomain/plugins/fake-plugin-0.0.1.tar.gz"

	tempDir := t.TempDir()

	// Get current umask to predict expected permissions
	currentUmask := syscall.Umask(0)
	syscall.Umask(currentUmask)

	// Write a tarball to a buffer for us to extract
	var tarbuf bytes.Buffer
	tw := tar.NewWriter(&tarbuf)
	var files = []struct {
		Name, Body string
		Mode       int64
	}{
		{"plugin.yaml", "plugin metadata", 0600},
		{"README.md", "some text", 0777},
	}
	for _, file := range files {
		hdr := &tar.Header{
			Name:     file.Name,
			Typeflag: tar.TypeReg,
			Mode:     file.Mode,
			Size:     int64(len(file.Body)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(file.Body)); err != nil {
			t.Fatal(err)
		}
	}

	// Add pax global headers. This should be ignored.
	// Note the PAX header that isn't global cannot be written using WriteHeader.
	// Details are in the internal Go function for the tar packaged named
	// allowedFormats. For a TypeXHeader it will return a message stating
	// "cannot manually encode TypeXHeader, TypeGNULongName, or TypeGNULongLink headers"
	if err := tw.WriteHeader(&tar.Header{
		Name:     "pax_global_header",
		Typeflag: tar.TypeXGlobalHeader,
	}); err != nil {
		t.Fatal(err)
	}

	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if _, err := gz.Write(tarbuf.Bytes()); err != nil {
		t.Fatal(err)
	}
	gz.Close()
	// END tarball creation

	extractor, err := NewExtractor(source)
	if err != nil {
		t.Fatal(err)
	}

	if err = extractor.Extract(&buf, tempDir); err != nil {
		t.Fatalf("Did not expect error but got error: %v", err)
	}

	// Calculate expected permissions after umask is applied
	expectedPluginYAMLPerm := os.FileMode(0600 &^ currentUmask)
	expectedReadmePerm := os.FileMode(0777 &^ currentUmask)

	pluginYAMLFullPath := filepath.Join(tempDir, "plugin.yaml")
	if info, err := os.Stat(pluginYAMLFullPath); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("Expected %s to exist but doesn't", pluginYAMLFullPath)
		}
		t.Fatal(err)
	} else if info.Mode().Perm() != expectedPluginYAMLPerm {
		t.Fatalf("Expected %s to have %o mode but has %o (umask: %o)",
			pluginYAMLFullPath, expectedPluginYAMLPerm, info.Mode().Perm(), currentUmask)
	}

	readmeFullPath := filepath.Join(tempDir, "README.md")
	if info, err := os.Stat(readmeFullPath); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("Expected %s to exist but doesn't", readmeFullPath)
		}
		t.Fatal(err)
	} else if info.Mode().Perm() != expectedReadmePerm {
		t.Fatalf("Expected %s to have %o mode but has %o (umask: %o)",
			readmeFullPath, expectedReadmePerm, info.Mode().Perm(), currentUmask)
	}

}
