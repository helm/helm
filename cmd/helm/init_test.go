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

package main

import (
	"bytes"
	"os"
	"testing"

	"helm.sh/helm/internal/test/ensure"
	"helm.sh/helm/pkg/helmpath"
)

const testPluginsFile = "testdata/plugins.yaml"

func TestEnsureHome(t *testing.T) {
	defer ensure.HelmHome(t)()

	b := bytes.NewBuffer(nil)
	if err := ensureDirectories(b); err != nil {
		t.Error(err)
	}
	if err := ensurePluginsInstalled(testPluginsFile, b); err != nil {
		t.Error(err)
	}

	expectedDirs := []string{helmpath.CachePath(), helmpath.ConfigPath(), helmpath.DataPath()}
	for _, dir := range expectedDirs {
		if fi, err := os.Stat(dir); err != nil {
			t.Errorf("%s", err)
		} else if !fi.IsDir() {
			t.Errorf("%s is not a directory", fi)
		}
	}

	if plugins, err := findPlugins(helmpath.DataPath("plugins")); err != nil {
		t.Error(err)
	} else if len(plugins) != 1 {
		t.Errorf("Expected 1 plugin, got %d", len(plugins))
	} else if plugins[0].Metadata.Name != "testplugin" {
		t.Errorf("Expected %s to be installed", "testplugin")
	}
}
