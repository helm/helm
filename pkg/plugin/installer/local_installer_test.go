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
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"k8s.io/helm/pkg/helm/helmpath"
)

var _ Installer = new(LocalInstaller)

func TestLocalInstaller(t *testing.T) {
	hh, err := ioutil.TempDir("", "helm-home-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(hh)

	home := helmpath.Home(hh)
	if err := os.MkdirAll(home.Plugins(), 0755); err != nil {
		t.Fatalf("Could not create %s: %s", home.Plugins(), err)
	}

	// Make a temp dir
	tdir, err := ioutil.TempDir("", "helm-installer-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tdir)
	if err := ioutil.WriteFile(filepath.Join(tdir, "plugin.yaml"), []byte{}, 0644); err != nil {
		t.Fatal(err)
	}

	source := "../testdata/plugdir/echo"
	i, err := NewForSource(source, "", home)
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}

	if err := Install(i); err != nil {
		t.Error(err)
	}

	if i.Path() != home.Path("plugins", "echo") {
		t.Errorf("expected path '$HELM_HOME/plugins/helm-env', got %q", i.Path())
	}
}
