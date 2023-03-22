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

package ensure

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"helm.sh/helm/v3/pkg/helmpath"
	"helm.sh/helm/v3/pkg/helmpath/xdg"
)

// HelmHome sets up a Helm Home in a temp dir.
func HelmHome(t *testing.T) {
	t.Helper()
	base := TempDir(t)
	os.Setenv(xdg.CacheHomeEnvVar, base)
	os.Setenv(xdg.ConfigHomeEnvVar, base)
	os.Setenv(xdg.DataHomeEnvVar, base)
	os.Setenv(helmpath.CacheHomeEnvVar, "")
	os.Setenv(helmpath.ConfigHomeEnvVar, "")
	os.Setenv(helmpath.DataHomeEnvVar, "")
}

// TempDir ensures a scratch test directory for unit testing purposes.
func TempDir(t *testing.T) string {
	t.Helper()
	d, err := ioutil.TempDir("", "helm")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if !t.Failed() {
			os.RemoveAll(d)
		}
	})

	return d
}

// TempFile ensures a temp file for unit testing purposes.
//
// It returns the path to the directory (to which you will still need to join the filename)
//
// The whole TempDir will be Remove automatically
func TempFile(t *testing.T, name string, data []byte) string {
	path := TempDir(t)
	filename := filepath.Join(path, name)
	if err := ioutil.WriteFile(filename, data, 0755); err != nil {
		t.Fatal(err)
	}
	return path
}
