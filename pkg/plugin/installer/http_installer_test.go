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
	"bytes"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"k8s.io/helm/pkg/helm/helmpath"
	"os"
	"testing"
)

var _ Installer = new(HTTPInstaller)

// Fake http client
type TestHTTPGetter struct {
	MockResponse *bytes.Buffer
	MockError    error
}

func (t *TestHTTPGetter) Get(href string) (*bytes.Buffer, error) { return t.MockResponse, t.MockError }

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
	source := "https://repo.localdomain/plugins/fake-plugin-0.0.1.tar.gz"
	hh, err := ioutil.TempDir("", "helm-home-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(hh)

	home := helmpath.Home(hh)
	if err := os.MkdirAll(home.Plugins(), 0755); err != nil {
		t.Fatalf("Could not create %s: %s", home.Plugins(), err)
	}

	i, err := NewForSource(source, "0.0.1", home)
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}

	// ensure a HTTPInstaller was returned
	httpInstaller, ok := i.(*HTTPInstaller)
	if !ok {
		t.Error("expected a HTTPInstaller")
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
		t.Error(err)
	}
	if i.Path() != home.Path("plugins", "fake-plugin") {
		t.Errorf("expected path '$HELM_HOME/plugins/fake-plugin', got %q", i.Path())
	}

	// Install again to test plugin exists error
	if err := Install(i); err == nil {
		t.Error("expected error for plugin exists, got none")
	} else if err.Error() != "plugin already exists" {
		t.Errorf("expected error for plugin exists, got (%v)", err)
	}

}

func TestHTTPInstallerNonExistentVersion(t *testing.T) {
	source := "https://repo.localdomain/plugins/fake-plugin-0.0.2.tar.gz"
	hh, err := ioutil.TempDir("", "helm-home-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(hh)

	home := helmpath.Home(hh)
	if err := os.MkdirAll(home.Plugins(), 0755); err != nil {
		t.Fatalf("Could not create %s: %s", home.Plugins(), err)
	}

	i, err := NewForSource(source, "0.0.2", home)
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}

	// ensure a HTTPInstaller was returned
	httpInstaller, ok := i.(*HTTPInstaller)
	if !ok {
		t.Error("expected a HTTPInstaller")
	}

	// inject fake http client responding with error
	httpInstaller.getter = &TestHTTPGetter{
		MockError: fmt.Errorf("failed to download plugin for some reason"),
	}

	// attempt to install the plugin
	if err := Install(i); err == nil {
		t.Error("expected error from http client")
	}

}

func TestHTTPInstallerUpdate(t *testing.T) {
	source := "https://repo.localdomain/plugins/fake-plugin-0.0.1.tar.gz"
	hh, err := ioutil.TempDir("", "helm-home-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(hh)

	home := helmpath.Home(hh)
	if err := os.MkdirAll(home.Plugins(), 0755); err != nil {
		t.Fatalf("Could not create %s: %s", home.Plugins(), err)
	}

	i, err := NewForSource(source, "0.0.1", home)
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}

	// ensure a HTTPInstaller was returned
	httpInstaller, ok := i.(*HTTPInstaller)
	if !ok {
		t.Error("expected a HTTPInstaller")
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
		t.Error(err)
	}
	if i.Path() != home.Path("plugins", "fake-plugin") {
		t.Errorf("expected path '$HELM_HOME/plugins/fake-plugin', got %q", i.Path())
	}

	// Update plugin, should fail because it is not implemented
	if err := Update(i); err == nil {
		t.Error("update method not implemented for http installer")
	}
}
