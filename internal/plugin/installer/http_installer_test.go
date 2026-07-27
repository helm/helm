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
	"strconv"
	"strings"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
	assert.Equal(t, "fake-plugin", stripPluginName("fake-plugin-0.0.1.tar.gz"), "name does not match expected value")
	assert.Equal(t, "fake-plugin", stripPluginName("fake-plugin-0.0.1.tgz"), "name does not match expected value")
	assert.Equal(t, "fake-plugin", stripPluginName("fake-plugin.tgz"), "name does not match expected value")
	assert.Equal(t, "fake-plugin", stripPluginName("fake-plugin.tar.gz"), "name does not match expected value")
}

func mockArchiveServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, ".tar.gz") {
			w.Header().Add("Content-Type", "text/html")
			fmt.Fprintln(w, "broken")
		} else {
			w.Header().Add("Content-Type", "application/gzip")
			fmt.Fprintln(w, "test")
		}
	}))
}

func TestHTTPInstaller(t *testing.T) {
	ensure.HelmHome(t)

	srv := mockArchiveServer()
	defer srv.Close()
	source := srv.URL + "/plugins/fake-plugin-0.0.1.tar.gz"

	require.NoErrorf(t, os.MkdirAll(helmpath.DataPath("plugins"), 0o755), "Could not create %s", helmpath.DataPath("plugins"))

	i, err := NewForSource(source, "0.0.1")
	require.NoError(t, err)

	// ensure a HTTPInstaller was returned
	httpInstaller, ok := i.(*HTTPInstaller)
	require.True(t, ok, "expected a HTTPInstaller")

	// inject fake http client responding with minimal plugin tarball
	mockTgz, err := base64.StdEncoding.DecodeString(fakePluginB64)
	require.NoError(t, err, "Could not decode fake tgz plugin")

	httpInstaller.getter = &TestHTTPGetter{
		MockResponse: bytes.NewBuffer(mockTgz),
	}

	// install the plugin
	require.NoError(t, Install(i))
	require.Equal(t, helmpath.DataPath("plugins", "fake-plugin"), i.Path(), "expected path '$XDG_CONFIG_HOME/helm/plugins/fake-plugin', got %q", i.Path())

	// Install again to test plugin exists error
	require.EqualErrorf(t, Install(i), "plugin already exists", "expected error for plugin exists")
}

func TestHTTPInstallerNonExistentVersion(t *testing.T) {
	ensure.HelmHome(t)
	srv := mockArchiveServer()
	defer srv.Close()
	source := srv.URL + "/plugins/fake-plugin-0.0.1.tar.gz"

	require.NoErrorf(t, os.MkdirAll(helmpath.DataPath("plugins"), 0o755), "Could not create %s", helmpath.DataPath("plugins"))

	i, err := NewForSource(source, "0.0.2")
	require.NoError(t, err)

	// ensure a HTTPInstaller was returned
	httpInstaller, ok := i.(*HTTPInstaller)
	require.True(t, ok, "expected a HTTPInstaller")

	// inject fake http client responding with error
	httpInstaller.getter = &TestHTTPGetter{
		MockError: errors.New("failed to download plugin for some reason"),
	}

	// attempt to install the plugin
	require.Error(t, Install(i), "expected error from http client")
}

func TestHTTPInstallerUpdate(t *testing.T) {
	srv := mockArchiveServer()
	defer srv.Close()
	source := srv.URL + "/plugins/fake-plugin-0.0.1.tar.gz"
	ensure.HelmHome(t)

	require.NoErrorf(t, os.MkdirAll(helmpath.DataPath("plugins"), 0o755), "Could not create %s", helmpath.DataPath("plugins"))

	i, err := NewForSource(source, "0.0.1")
	require.NoError(t, err)

	// ensure a HTTPInstaller was returned
	httpInstaller, ok := i.(*HTTPInstaller)
	require.True(t, ok, "expected a HTTPInstaller")

	// inject fake http client responding with minimal plugin tarball
	mockTgz, err := base64.StdEncoding.DecodeString(fakePluginB64)
	require.NoError(t, err, "Could not decode fake tgz plugin")

	httpInstaller.getter = &TestHTTPGetter{
		MockResponse: bytes.NewBuffer(mockTgz),
	}

	// install the plugin before updating
	require.NoError(t, Install(i))
	require.Equal(t, helmpath.DataPath("plugins", "fake-plugin"), i.Path(), "expected path '$XDG_CONFIG_HOME/helm/plugins/fake-plugin', got %q", i.Path())

	// Update plugin, should fail because it is not implemented
	require.Error(t, Update(i), "update method not implemented for http installer")
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
		{"plugin.yaml", "plugin metadata", 0o600},
		{"README.md", "some text", 0o777},
	}
	for _, file := range files {
		hdr := &tar.Header{
			Name:     file.Name,
			Typeflag: tar.TypeReg,
			Mode:     file.Mode,
			Size:     int64(len(file.Body)),
		}
		require.NoError(t, tw.WriteHeader(hdr))
		_, err := tw.Write([]byte(file.Body))
		require.NoError(t, err)
	}

	// Add pax global headers. This should be ignored.
	// Note the PAX header that isn't global cannot be written using WriteHeader.
	// Details are in the internal Go function for the tar packaged named
	// allowedFormats. For a TypeXHeader it will return a message stating
	// "cannot manually encode TypeXHeader, TypeGNULongName, or TypeGNULongLink headers"
	require.NoError(t, tw.WriteHeader(&tar.Header{
		Name:     "pax_global_header",
		Typeflag: tar.TypeXGlobalHeader,
	}))

	require.NoError(t, tw.Close())

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	_, err := gz.Write(tarbuf.Bytes())
	require.NoError(t, err)
	gz.Close()
	// END tarball creation

	extractor, err := NewExtractor(source)
	require.NoError(t, err)

	require.NoErrorf(t, extractor.Extract(&buf, tempDir), "Did not expect error")

	// Calculate expected permissions after umask is applied
	expectedPluginYAMLPerm := os.FileMode(0o600 &^ currentUmask)
	expectedReadmePerm := os.FileMode(0o777 &^ currentUmask)

	pluginYAMLFullPath := filepath.Join(tempDir, "plugin.yaml")
	info, err := os.Stat(pluginYAMLFullPath)
	if err != nil {
		require.NotErrorIs(t, err, fs.ErrNotExist, "Expected %s to exist but doesn't", pluginYAMLFullPath)
	}
	require.NoError(t, err)
	require.Equalf(t, expectedPluginYAMLPerm, info.Mode().Perm(), "Expected %s to have %o mode but has %o (umask: %o)",
		pluginYAMLFullPath, expectedPluginYAMLPerm, info.Mode().Perm(), currentUmask)

	readmeFullPath := filepath.Join(tempDir, "README.md")
	info, err = os.Stat(readmeFullPath)
	if err != nil {
		require.NotErrorIs(t, err, fs.ErrNotExist, "Expected %s to exist but doesn't", readmeFullPath)
	}
	require.NoError(t, err)
	require.Equalf(t, expectedReadmePerm, info.Mode().Perm(), "Expected %s to have %o mode but has %o (umask: %o)",
		readmeFullPath, expectedReadmePerm, info.Mode().Perm(), currentUmask)
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
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			out, err := cleanJoin("/tmp", fixture.path)
			if fixture.expectError {
				require.Error(t, err, "Test %d: Path was not cleaned", i)
			} else {
				require.NoError(t, err)
				assert.Equal(t, fixture.expect, out, "Test %d: Expected %q but got %q", i, fixture.expect, out)
			}
		})
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
		assert.Equal(t, shouldPass, ok, "Media type %q failed test", mt)
		if shouldPass {
			assert.NotEmpty(t, ext, "Expected an extension but got empty string for media type %q", mt)
		} else {
			assert.Empty(t, ext, "Expected extension to be empty for unrecognized media type %q", mt)
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
		{"plugin.yaml", "plugin metadata", 0o600, tar.TypeReg},
		{"bin/", "", 0o755, tar.TypeDir},
		{"bin/plugin", "#!/usr/bin/env sh\necho plugin", 0o755, tar.TypeReg},
		{"docs/", "", 0o755, tar.TypeDir},
		{"docs/README.md", "readme content", 0o644, tar.TypeReg},
		{"docs/examples/", "", 0o755, tar.TypeDir},
		{"docs/examples/example1.yaml", "example content", 0o644, tar.TypeReg},
	}

	for _, file := range files {
		hdr := &tar.Header{
			Name:     file.Name,
			Typeflag: file.TypeFlag,
			Mode:     file.Mode,
			Size:     int64(len(file.Body)),
		}
		require.NoError(t, tw.WriteHeader(hdr))
		if file.TypeFlag == tar.TypeReg {
			_, err := tw.Write([]byte(file.Body))
			require.NoError(t, err)
		}
	}

	require.NoError(t, tw.Close())

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	_, err := gz.Write(tarbuf.Bytes())
	require.NoError(t, err)
	gz.Close()

	extractor, err := NewExtractor(source)
	require.NoError(t, err)

	// First extraction
	require.NoError(t, extractor.Extract(&buf, tempDir), "First extraction failed")

	// Verify nested structure was created
	nestedFile := filepath.Join(tempDir, "docs", "examples", "example1.yaml")
	_, err = os.Stat(nestedFile)
	require.NoErrorf(t, err, "Expected nested file %s to exist", nestedFile)

	// Reset buffer for second extraction
	buf.Reset()
	gz = gzip.NewWriter(&buf)
	_, err = gz.Write(tarbuf.Bytes())
	require.NoError(t, err)
	gz.Close()

	// Second extraction to same directory (should not fail)
	require.NoErrorf(t, extractor.Extract(&buf, tempDir), "Second extraction to existing directory failed")
}

func TestExtractWithExistingDirectory(t *testing.T) {
	source := "https://repo.localdomain/plugins/test-plugin-0.0.1.tar.gz"
	tempDir := t.TempDir()

	// Pre-create the cache directory structure
	cacheDir := filepath.Join(tempDir, "cache")
	require.NoError(t, os.MkdirAll(filepath.Join(cacheDir, "existing", "dir"), 0o755))

	// Create a file in the existing directory
	existingFile := filepath.Join(cacheDir, "existing", "file.txt")
	require.NoError(t, os.WriteFile(existingFile, []byte("existing content"), 0o644))

	// Write a tarball
	var tarbuf bytes.Buffer
	tw := tar.NewWriter(&tarbuf)
	files := []struct {
		Name     string
		Body     string
		Mode     int64
		TypeFlag byte
	}{
		{"plugin.yaml", "plugin metadata", 0o600, tar.TypeReg},
		{"existing/", "", 0o755, tar.TypeDir},
		{"existing/dir/", "", 0o755, tar.TypeDir},
		{"existing/dir/newfile.txt", "new content", 0o644, tar.TypeReg},
	}

	for _, file := range files {
		hdr := &tar.Header{
			Name:     file.Name,
			Typeflag: file.TypeFlag,
			Mode:     file.Mode,
			Size:     int64(len(file.Body)),
		}
		require.NoError(t, tw.WriteHeader(hdr))
		if file.TypeFlag == tar.TypeReg {
			_, err := tw.Write([]byte(file.Body))
			require.NoError(t, err)
		}
	}

	require.NoError(t, tw.Close())

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	_, err := gz.Write(tarbuf.Bytes())
	require.NoError(t, err)
	gz.Close()

	extractor, err := NewExtractor(source)
	require.NoError(t, err)

	// Extract to directory with existing content
	require.NoErrorf(t, extractor.Extract(&buf, cacheDir), "Extraction to directory with existing content failed")

	// Verify new file was created
	newFile := filepath.Join(cacheDir, "existing", "dir", "newfile.txt")
	_, err = os.Stat(newFile)
	require.NoErrorf(t, err, "Expected new file %s to exist but got error", newFile)

	// Verify existing file is still there
	_, err = os.Stat(existingFile)
	require.NoErrorf(t, err, "Expected existing file %s to still exist", existingFile)
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
		{"my-plugin/", "", 0o755, tar.TypeDir},
		{"my-plugin/plugin.yaml", "name: my-plugin\nversion: 1.0.0\nusage: test\ndescription: test plugin\ncommand: $HELM_PLUGIN_DIR/bin/my-plugin", 0o644, tar.TypeReg},
		{"my-plugin/bin/", "", 0o755, tar.TypeDir},
		{"my-plugin/bin/my-plugin", "#!/usr/bin/env sh\necho test", 0o755, tar.TypeReg},
	}

	for _, file := range files {
		hdr := &tar.Header{
			Name:     file.Name,
			Typeflag: file.TypeFlag,
			Mode:     file.Mode,
			Size:     int64(len(file.Body)),
		}
		require.NoError(t, tw.WriteHeader(hdr))
		if file.TypeFlag == tar.TypeReg {
			_, err := tw.Write([]byte(file.Body))
			require.NoError(t, err)
		}
	}

	require.NoError(t, tw.Close())

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	_, err := gz.Write(tarbuf.Bytes())
	require.NoError(t, err)
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
	require.NoErrorf(t, os.RemoveAll(destPath), "Failed to clean destination path")

	// Install should handle the subdirectory correctly
	require.NoErrorf(t, installer.Install(), "Failed to install plugin with subdirectory")

	// The plugin should be installed from the subdirectory
	// Check that detectPluginRoot found the correct location
	pluginRoot, err := detectPluginRoot(tempDir)
	require.NoError(t, err, "Failed to detect plugin root")

	expectedRoot := filepath.Join(tempDir, "my-plugin")
	assert.Equal(t, expectedRoot, pluginRoot, "Expected plugin root to be %s but got %s", expectedRoot, pluginRoot)
}
