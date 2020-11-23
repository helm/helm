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
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"helm.sh/helm/v3/pkg/helmpath"
)

var _ Installer = new(LocalInstaller)

func TestLocalInstaller(t *testing.T) {
	// Make a temp dir
	tdir, err := ioutil.TempDir("", "helm-installer-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tdir)
	if err := ioutil.WriteFile(filepath.Join(tdir, "plugin.yaml"), []byte{}, 0644); err != nil {
		t.Fatal(err)
	}

	source := "../testdata/plugdir/good/echo"
	i, err := NewForSource(source, "")
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	if err := Install(i); err != nil {
		t.Fatal(err)
	}

	if i.Path() != helmpath.DataPath("plugins", "echo") {
		t.Fatalf("expected path '$XDG_CONFIG_HOME/helm/plugins/helm-env', got %q", i.Path())
	}
	defer os.RemoveAll(filepath.Dir(helmpath.DataPath())) // helmpath.DataPath is like /tmp/helm013130971/helm
}
