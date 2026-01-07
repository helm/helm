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
	"encoding/base64"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"helm.sh/helm/v4/internal/test/ensure"
	"helm.sh/helm/v4/pkg/getter"
	"helm.sh/helm/v4/pkg/helmpath"
)

var _ Installer = new(HTTPInstaller)

// Fake http client
type TestHTTPGetter struct {
	MockResponse *bytes.Buffer
	MockError    error
}

func (t *TestHTTPGetter) Get(_ string, _ ...getter.Option) (*bytes.Buffer, error) {
	return t.MockResponse, t.MockError
}

// Fake plugin tarball data
var fakePluginB64 = "H4sIAAAAAAAAA+3SQUvDMBgG4Jz7K0LwapdvSxrwJig6mCKC5xHabBaXdDSt4L+3cQ56mV42ZPg+lw+SF5LwZmXf3OV206/rMGEnIgdG6zTJaDmee4y01FOlZpqGHJGZSsb1qS401sfOtpyz0FTup9xv+2dqNep/N/IP6zdHPSMVXCh1sH8yhtGMDBUFFTL1r4iIcXnUWxzwz/sP1rsrLkbfQGTvro11E4ZlmcucRNZHu04py1OO73OVi2Vbb7td9vp7nXevtvsKRpGVjfc2VMP2xf3t4mH5tHi5mz8ub+bPk9JXIvvr5wMAAAAAAAAAAAAAAAAAAAAAnLVPqwHcXQAoAAA="

func TestStripName(t *testing.T) {
	if stripPluginName("fake-plugin-0.0.1.tar.gz") != "fake-plugin" {
		t.Errorf("name does not match expected value")
	}
	if stripPluginName("fake-plugin-0.0.1.tgz") != "fake-plugin" {
		t.Errorf("name does not match expected value")
	}
	if stripPluginName("fake-plugin.tgz") != "fake-plugin" {
		t.Errorf("name does not match expected value")
	}
	if stripPluginName("fake-plugin.tar.gz") != "fake-plugin" {
		t.Errorf("name does not match expected value")
	}
}

func mockArchiveServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, ".tar.gz") {
			w.Header().Add("Content-Type", "text/html")
			fmt.Fprintln(w, "broken")
			return
		}
		w.Header().Add("Content-Type", "application/gzip")
		fmt.Fprintln(w, "test")
	}))
}

func TestHTTPInstaller(t *testing.T) {
	ensure.HelmHome(t)

	srv := mockArchiveServer()
	defer srv.Close()
	source := srv.URL + "/plugins/fake-plugin-0.0.1.tar.gz"

	if err := os.MkdirAll(helmpath.DataPath("plugins"), 0755); err != nil {
		t.Fatalf("Could not create %s: %s", helmpath.DataPath("plugins"), err)
	}

	i, err := NewForSource(source, "0.0.1")
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	// ensure a HTTPInstaller was returned
	httpInstaller, ok := i.(*HTTPInstaller)
	if !ok {
		t.Fatal("expected a HTTPInstaller")
	}

	// inject fake http client responding with minimal plugin tarball
	mockTgz, err := base64.StdEncoding.DecodeString(fakePluginB64)
	if err != nil {
		t.Fatalf("Could not decode fake tgz plugin: %s", err)
	}

	httpInstaller.getter = &TestHTTPGetter{
		MockResponse: bytes.NewBuffer(mockTgz),
	}

	// install the plugin
	if err := Install(i); err != nil {
		t.Fatal(err)
	}
	if i.Path() != helmpath.DataPath("plugins", "fake-plugin") {
		t.Fatalf("expected path '$XDG_CONFIG_HOME/helm/plugins/fake-plugin', got %q", i.Path())
	}

	// Install again to test plugin exists error
	if err := Install(i); err == nil {
		t.Fatal("expected error for plugin exists, got none")
	} else if err.Error() != "plugin already exists" {
		t.Fatalf("expected error for plugin exists, got (%v)", err)
	}

}

func TestHTTPInstallerNonExistentVersion(t *testing.T) {
	ensure.HelmHome(t)
	srv := mockArchiveServer()
	defer srv.Close()
	source := srv.URL + "/plugins/fake-plugin-0.0.1.tar.gz"

	if err := os.MkdirAll(helmpath.DataPath("plugins"), 0755); err != nil {
		t.Fatalf("Could not create %s: %s", helmpath.DataPath("plugins"), err)
	}

	i, err := NewForSource(source, "0.0.2")
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	// ensure a HTTPInstaller was returned
	httpInstaller, ok := i.(*HTTPInstaller)
	if !ok {
		t.Fatal("expected a HTTPInstaller")
	}

	// inject fake http client responding with error
	httpInstaller.getter = &TestHTTPGetter{
		MockError: fmt.Errorf("failed to download plugin for some reason"),
	}

	// attempt to install the plugin
	if err := Install(i); err == nil {
		t.Fatal("expected error from http client")
	}

}

func TestHTTPInstallerUpdate(t *testing.T) {
	srv := mockArchiveServer()
	defer srv.Close()
	source := srv.URL + "/plugins/fake-plugin-0.0.1.tar.gz"
	ensure.HelmHome(t)

	if err := os.MkdirAll(helmpath.DataPath("plugins"), 0755); err != nil {
		t.Fatalf("Could not create %s: %s", helmpath.DataPath("plugins"), err)
	}

	i, err := NewForSource(source, "0.0.1")
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	// ensure a HTTPInstaller was returned
	httpInstaller, ok := i.(*HTTPInstaller)
	if !ok {
		t.Fatal("expected a HTTPInstaller")
	}

	// inject fake http client responding with minimal plugin tarball
	mockTgz, err := base64.StdEncoding.DecodeString(fakePluginB64)
	if err != nil {
		t.Fatalf("Could not decode fake tgz plugin: %s", err)
	}

	httpInstaller.getter = &TestHTTPGetter{
		MockResponse: bytes.NewBuffer(mockTgz),
	}

	// install the plugin before updating
	if err := Install(i); err != nil {
		t.Fatal(err)
	}
	if i.Path() != helmpath.DataPath("plugins", "fake-plugin") {
		t.Fatalf("expected path '$XDG_CONFIG_HOME/helm/plugins/fake-plugin', got %q", i.Path())
	}

	// Update plugin, should fail because it is not implemented
	if err := Update(i); err == nil {
		t.Fatal("update method not implemented for http installer")
	}
}

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

func TestCleanJoin(t *testing.T) {
	for i, fixture := range []struct {
		path        string
		expect      string
		expectError bool
	}{
		{"foo/bar.txt", "/tmp/foo/bar.txt", false},
		{"/foo/bar.txt", "", true},
		{"./foo/bar.txt", "/tmp/foo/bar.txt", false},
		{"./././././foo/bar.txt", "/tmp/foo/bar.txt", false},
		{"../../../../foo/bar.txt", "", true},
		{"foo/../../../../bar.txt", "", true},
		{"c:/foo/bar.txt", "/tmp/c:/foo/bar.txt", true},
		{"foo\\bar.txt", "/tmp/foo/bar.txt", false},
		{"c:\\foo\\bar.txt", "", true},
	} {
		out, err := cleanJoin("/tmp", fixture.path)
		if err != nil {
			if !fixture.expectError {
				t.Errorf("Test %d: Path was not cleaned: %s", i, err)
			}
			continue
		}
		if fixture.expect != out {
			t.Errorf("Test %d: Expected %q but got %q", i, fixture.expect, out)
		}
	}

}

func TestMediaTypeToExtension(t *testing.T) {

	for mt, shouldPass := range map[string]bool{
		"":                   false,
		"application/gzip":   true,
		"application/x-gzip": true,
		"application/x-tgz":  true,
		"application/x-gtar": true,
		"application/json":   false,
	} {
		ext, ok := mediaTypeToExtension(mt)
		if ok != shouldPass {
			t.Errorf("Media type %q failed test", mt)
		}
		if shouldPass && ext == "" {
			t.Errorf("Expected an extension but got empty string")
		}
		if !shouldPass && len(ext) != 0 {
			t.Error("Expected extension to be empty for unrecognized type")
		}
	}
}

func TestExtractWithNestedDirectories(t *testing.T) {
	source := "https://repo.localdomain/plugins/nested-plugin-0.0.1.tar.gz"
	tempDir := t.TempDir()

	// Write a tarball with nested directory structure
	var tarbuf bytes.Buffer
	tw := tar.NewWriter(&tarbuf)
	var files = []struct {
		Name     string
		Body     string
		Mode     int64
		TypeFlag byte
	}{
		{"plugin.yaml", "plugin metadata", 0600, tar.TypeReg},
		{"bin/", "", 0755, tar.TypeDir},
		{"bin/plugin", "#!/usr/bin/env sh\necho plugin", 0755, tar.TypeReg},
		{"docs/", "", 0755, tar.TypeDir},
		{"docs/README.md", "readme content", 0644, tar.TypeReg},
		{"docs/examples/", "", 0755, tar.TypeDir},
		{"docs/examples/example1.yaml", "example content", 0644, tar.TypeReg},
	}

	for _, file := range files {
		hdr := &tar.Header{
			Name:     file.Name,
			Typeflag: file.TypeFlag,
			Mode:     file.Mode,
			Size:     int64(len(file.Body)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if file.TypeFlag == tar.TypeReg {
			if _, err := tw.Write([]byte(file.Body)); err != nil {
				t.Fatal(err)
			}
		}
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

	extractor, err := NewExtractor(source)
	if err != nil {
		t.Fatal(err)
	}

	// First extraction
	if err = extractor.Extract(&buf, tempDir); err != nil {
		t.Fatalf("First extraction failed: %v", err)
	}

	// Verify nested structure was created
	nestedFile := filepath.Join(tempDir, "docs", "examples", "example1.yaml")
	if _, err := os.Stat(nestedFile); err != nil {
		t.Fatalf("Expected nested file %s to exist but got error: %v", nestedFile, err)
	}

	// Reset buffer for second extraction
	buf.Reset()
	gz = gzip.NewWriter(&buf)
	if _, err := gz.Write(tarbuf.Bytes()); err != nil {
		t.Fatal(err)
	}
	gz.Close()

	// Second extraction to same directory (should not fail)
	if err = extractor.Extract(&buf, tempDir); err != nil {
		t.Fatalf("Second extraction to existing directory failed: %v", err)
	}
}

func TestExtractWithExistingDirectory(t *testing.T) {
	source := "https://repo.localdomain/plugins/test-plugin-0.0.1.tar.gz"
	tempDir := t.TempDir()

	// Pre-create the cache directory structure
	cacheDir := filepath.Join(tempDir, "cache")
	if err := os.MkdirAll(filepath.Join(cacheDir, "existing", "dir"), 0755); err != nil {
		t.Fatal(err)
	}

	// Create a file in the existing directory
	existingFile := filepath.Join(cacheDir, "existing", "file.txt")
	if err := os.WriteFile(existingFile, []byte("existing content"), 0644); err != nil {
		t.Fatal(err)
	}

	// Write a tarball
	var tarbuf bytes.Buffer
	tw := tar.NewWriter(&tarbuf)
	files := []struct {
		Name     string
		Body     string
		Mode     int64
		TypeFlag byte
	}{
		{"plugin.yaml", "plugin metadata", 0600, tar.TypeReg},
		{"existing/", "", 0755, tar.TypeDir},
		{"existing/dir/", "", 0755, tar.TypeDir},
		{"existing/dir/newfile.txt", "new content", 0644, tar.TypeReg},
	}

	for _, file := range files {
		hdr := &tar.Header{
			Name:     file.Name,
			Typeflag: file.TypeFlag,
			Mode:     file.Mode,
			Size:     int64(len(file.Body)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if file.TypeFlag == tar.TypeReg {
			if _, err := tw.Write([]byte(file.Body)); err != nil {
				t.Fatal(err)
			}
		}
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

	extractor, err := NewExtractor(source)
	if err != nil {
		t.Fatal(err)
	}

	// Extract to directory with existing content
	if err = extractor.Extract(&buf, cacheDir); err != nil {
		t.Fatalf("Extraction to directory with existing content failed: %v", err)
	}

	// Verify new file was created
	newFile := filepath.Join(cacheDir, "existing", "dir", "newfile.txt")
	if _, err := os.Stat(newFile); err != nil {
		t.Fatalf("Expected new file %s to exist but got error: %v", newFile, err)
	}

	// Verify existing file is still there
	if _, err := os.Stat(existingFile); err != nil {
		t.Fatalf("Expected existing file %s to still exist but got error: %v", existingFile, err)
	}
}

func TestExtractPluginInSubdirectory(t *testing.T) {
	ensure.HelmHome(t)
	source := "https://repo.localdomain/plugins/subdir-plugin-1.0.0.tar.gz"
	tempDir := t.TempDir()

	// Create a tarball where plugin files are in a subdirectory
	var tarbuf bytes.Buffer
	tw := tar.NewWriter(&tarbuf)
	files := []struct {
		Name     string
		Body     string
		Mode     int64
		TypeFlag byte
	}{
		{"my-plugin/", "", 0755, tar.TypeDir},
		{"my-plugin/plugin.yaml", "name: my-plugin\nversion: 1.0.0\nusage: test\ndescription: test plugin\ncommand: $HELM_PLUGIN_DIR/bin/my-plugin", 0644, tar.TypeReg},
		{"my-plugin/bin/", "", 0755, tar.TypeDir},
		{"my-plugin/bin/my-plugin", "#!/usr/bin/env sh\necho test", 0755, tar.TypeReg},
	}

	for _, file := range files {
		hdr := &tar.Header{
			Name:     file.Name,
			Typeflag: file.TypeFlag,
			Mode:     file.Mode,
			Size:     int64(len(file.Body)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if file.TypeFlag == tar.TypeReg {
			if _, err := tw.Write([]byte(file.Body)); err != nil {
				t.Fatal(err)
			}
		}
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

	// Test the installer
	installer := &HTTPInstaller{
		CacheDir:   tempDir,
		PluginName: "subdir-plugin",
		base:       newBase(source),
		extractor:  &TarGzExtractor{},
	}

	// Create a mock getter
	installer.getter = &TestHTTPGetter{
		MockResponse: &buf,
	}

	// Ensure the destination directory doesn't exist
	// (In a real scenario, this is handled by installer.Install() wrapper)
	destPath := installer.Path()
	if err := os.RemoveAll(destPath); err != nil {
		t.Fatalf("Failed to clean destination path: %v", err)
	}

	// Install should handle the subdirectory correctly
	if err := installer.Install(); err != nil {
		t.Fatalf("Failed to install plugin with subdirectory: %v", err)
	}

	// The plugin should be installed from the subdirectory
	// Check that detectPluginRoot found the correct location
	pluginRoot, err := detectPluginRoot(tempDir)
	if err != nil {
		t.Fatalf("Failed to detect plugin root: %v", err)
	}

	expectedRoot := filepath.Join(tempDir, "my-plugin")
	if pluginRoot != expectedRoot {
		t.Errorf("Expected plugin root to be %s but got %s", expectedRoot, pluginRoot)
	}
}
