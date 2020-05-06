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
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"io/ioutil"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/pkg/errors"

	"helm.sh/helm/v3/internal/test/ensure"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/helmpath"
)

var _ Installer = new(HTTPInstaller)

// Fake http client
type TestHTTPGetter struct {
	MockResponse *bytes.Buffer
	MockError    error
}

func (t *TestHTTPGetter) Get(href string, _ ...getter.Option) (*bytes.Buffer, error) {
	return t.MockResponse, t.MockError
}

// Fake plugin tarball data
var fakePluginB64 = "H4sIAKRj51kAA+3UX0vCUBgGcC9jn+Iwuk3Peza3GeyiUlJQkcogCOzgli7dJm4TvYk+a5+k479UqquUCJ/fLs549sLO2TnvWnJa9aXnjwujYdYLovxMhsPcfnHOLdNkOXthM/IVQQYjg2yyLLJ4kXGhLp5j0z3P41tZksqxmspL3B/O+j/XtZu1y8rdYzkOZRCxduKPk53ny6Wwz/GfIIf1As8lxzGJSmoHNLJZphKHG4YpTCE0wVk3DULfpSJ3DMMqkj3P5JfMYLdX1Vr9Ie/5E5cstcdC8K04iGLX5HaJuKpWL17F0TCIBi5pf/0pjtLhun5j3f9v6r7wfnI/H0eNp9d1/5P6Gez0vzo7wsoxfrAZbTny/o9k6J8z/VkO/LPlWdC1iVpbEEcq5nmeJ13LEtmbV0k2r2PrOs9PuuNglC5rL1Y5S/syXRQmutaNw1BGnnp8Wq3UG51WvX1da3bKtZtCN/R09DwAAAAAAAAAAAAAAAAAAADAb30AoMczDwAoAAA="

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

func TestHTTPInstaller(t *testing.T) {
	defer ensure.HelmHome(t)()
	source := "https://repo.localdomain/plugins/fake-plugin-0.0.1.tar.gz"

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
	defer ensure.HelmHome(t)()
	source := "https://repo.localdomain/plugins/fake-plugin-0.0.2.tar.gz"

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
		MockError: errors.Errorf("failed to download plugin for some reason"),
	}

	// attempt to install the plugin
	if err := Install(i); err == nil {
		t.Fatal("expected error from http client")
	}

}

func TestHTTPInstallerUpdate(t *testing.T) {
	source := "https://repo.localdomain/plugins/fake-plugin-0.0.1.tar.gz"
	defer ensure.HelmHome(t)()

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

	tempDir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	// Set the umask to default open permissions so we can actually test
	oldmask := syscall.Umask(0000)
	defer func() {
		syscall.Umask(oldmask)
	}()

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

	pluginYAMLFullPath := filepath.Join(tempDir, "plugin.yaml")
	if info, err := os.Stat(pluginYAMLFullPath); err != nil {
		if os.IsNotExist(err) {
			t.Fatalf("Expected %s to exist but doesn't", pluginYAMLFullPath)
		}
		t.Fatal(err)
	} else if info.Mode().Perm() != 0600 {
		t.Fatalf("Expected %s to have 0600 mode it but has %o", pluginYAMLFullPath, info.Mode().Perm())
	}

	readmeFullPath := filepath.Join(tempDir, "README.md")
	if info, err := os.Stat(readmeFullPath); err != nil {
		if os.IsNotExist(err) {
			t.Fatalf("Expected %s to exist but doesn't", readmeFullPath)
		}
		t.Fatal(err)
	} else if info.Mode().Perm() != 0777 {
		t.Fatalf("Expected %s to have 0777 mode it but has %o", readmeFullPath, info.Mode().Perm())
	}

}
