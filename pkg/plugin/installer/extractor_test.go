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

package installer // import "k8s.io/helm/pkg/plugin/installer"

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"k8s.io/helm/pkg/helm/helmpath"
)

// Fake plugin tarball data
var fakePluginB64 = "H4sIAKRj51kAA+3UX0vCUBgGcC9jn+Iwuk3Peza3GeyiUlJQkcogCOzgli7dJm4TvYk+a5+k479UqquUCJ/fLs549sLO2TnvWnJa9aXnjwujYdYLovxMhsPcfnHOLdNkOXthM/IVQQYjg2yyLLJ4kXGhLp5j0z3P41tZksqxmspL3B/O+j/XtZu1y8rdYzkOZRCxduKPk53ny6Wwz/GfIIf1As8lxzGJSmoHNLJZphKHG4YpTCE0wVk3DULfpSJ3DMMqkj3P5JfMYLdX1Vr9Ie/5E5cstcdC8K04iGLX5HaJuKpWL17F0TCIBi5pf/0pjtLhun5j3f9v6r7wfnI/H0eNp9d1/5P6Gez0vzo7wsoxfrAZbTny/o9k6J8z/VkO/LPlWdC1iVpbEEcq5nmeJ13LEtmbV0k2r2PrOs9PuuNglC5rL1Y5S/syXRQmutaNw1BGnnp8Wq3UG51WvX1da3bKtZtCN/R09DwAAAAAAAAAAAAAAAAAAADAb30AoMczDwAoAAA="

var fakePluginWithDirB64 = "H4sICCOnslwAA2Zha2UtcGx1Z2luLTAuMC4xLnRhcgDtks1qwzAQhHP2UwjRa52VrR/IrZDSBtJSCj0HYUupSWQbyw7k7bux3eKLe0oIIf4uA6tB2t2R1TvzWO6bbZbPZxcCAJQQpFUFrULEO+0hLOaCiyiOeEyAcQ7xjIhLNTSk8bWusBWbVcYnRWlGfGiz9p97+jn+9Eawg/w7CY/a7c/6Bu5Dcj6eP1MK82eKSckkCMxfCoX5w1m7GOHO88+1MwtCB9+ABgdT+azIsQwhhIwGjdfbk0uTk4/8+lJcWJWVded96uuk/tZ1a/Q0SArndJ7i8cPr8/pt87H+elm9b5arz3niUhpce/yJiYmJu+UHTs7l6wAKAAA="

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

func TestExtract(t *testing.T) {
	//create a temp home
	hh, err := ioutil.TempDir("", "helm-home-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(hh)

	home := helmpath.Home(hh)
	if err := os.MkdirAll(home.Plugins(), 0755); err != nil {
		t.Fatalf("Could not create %s: %s", home.Plugins(), err)
	}

	cacheDir := filepath.Join(home.Cache(), "plugins", "plugin-key")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		t.Fatalf("Could not create %s: %s", cacheDir, err)
	}

	//{"plugin.yaml", "plugin metadata up in here"},
	//{"README.md", "so you know what's upp"},
	//{"script.sh", "echo script"},

	var tarbuf bytes.Buffer
	tw := tar.NewWriter(&tarbuf)
	var files = []struct {
		Name, Body string
	}{
		{"../../plugin.yaml", "sneaky plugin metadata"},
		{"README.md", "some text"},
	}
	for _, file := range files {
		hdr := &tar.Header{
			Name:     file.Name,
			Typeflag: tar.TypeReg,
			Mode:     0600,
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

	source := "https://repo.localdomain/plugins/fake-plugin-0.0.1.tgz"
	extr, err := NewExtractor(source)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := extr.Extract(&buf, cacheDir); err != nil {
		t.Errorf("Did not expect error but got error: %v", err)
	}

	pluginYAMLFullPath := filepath.Join(cacheDir, "plugin.yaml")
	if _, err := os.Stat(pluginYAMLFullPath); err != nil {
		if os.IsNotExist(err) {
			t.Errorf("Expected %s to exist but doesn't", pluginYAMLFullPath)
		} else {
			t.Error(err)
		}
	}

	readmeFullPath := filepath.Join(cacheDir, "README.md")
	if _, err := os.Stat(readmeFullPath); err != nil {
		if os.IsNotExist(err) {
			t.Errorf("Expected %s to exist but doesn't", readmeFullPath)
		} else {
			t.Error(err)
		}
	}

}

func TestExtractWithDir(t *testing.T) {
	//create a temp home
	hh, err := ioutil.TempDir("", "helm-home-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(hh)

	home := helmpath.Home(hh)
	if err := os.MkdirAll(home.Plugins(), 0755); err != nil {
		t.Fatalf("Could not create %s: %s", home.Plugins(), err)
	}

	cacheDir := filepath.Join(home.Cache(), "plugins", "fake-plugin")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		t.Fatalf("Could not create %s: %s", cacheDir, err)
	}

	source := "https://repo.localdomain/plugins/fake-plugin-0.0.1.tgz"
	extr, err := NewExtractor(source)
	if err != nil {
		t.Fatal(err)
	}

	// inject fake http client responding with minimal plugin tarball
	buf, err := base64.StdEncoding.DecodeString(fakePluginWithDirB64)
	if err != nil {
		t.Fatalf("Could not decode fake tgz plugin: %s", err)
	}

	pluginDir, err := extr.Extract(bytes.NewBuffer(buf), cacheDir)
	if err != nil {
		t.Errorf("Did not expect error but got error: %v", err)
	}

	if filepath.Clean(pluginDir) != filepath.Join(cacheDir, "fake-plugin") {
		t.Errorf("Did not detect plugin.yaml in sub-directory")
	}

	pluginYAMLFullPath := filepath.Join(pluginDir, "plugin.yaml")
	if _, err := os.Stat(pluginYAMLFullPath); err != nil {
		if os.IsNotExist(err) {
			t.Errorf("Expected %s to exist but doesn't", pluginYAMLFullPath)
		} else {
			t.Error(err)
		}
	}
}
