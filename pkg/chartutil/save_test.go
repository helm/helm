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

package chartutil

import (
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"helm.sh/helm/pkg/chart"
	"helm.sh/helm/pkg/chart/loader"
)

func TestSave(t *testing.T) {
	tmp, err := ioutil.TempDir("", "helm-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmp)

	c := &chart.Chart{
		Metadata: &chart.Metadata{
			APIVersion: chart.APIVersionV1,
			Name:       "ahab",
			Version:    "1.2.3",
		},
		Files: []*chart.File{
			{Name: "scheherazade/shahryar.txt", Data: []byte("1,001 Nights")},
		},
	}

	where, err := Save(c, tmp)
	if err != nil {
		t.Fatalf("Failed to save: %s", err)
	}
	if !strings.HasPrefix(where, tmp) {
		t.Fatalf("Expected %q to start with %q", where, tmp)
	}
	if !strings.HasSuffix(where, ".tgz") {
		t.Fatalf("Expected %q to end with .tgz", where)
	}

	c2, err := loader.LoadFile(where)
	if err != nil {
		t.Fatal(err)
	}
	if c2.Name() != c.Name() {
		t.Fatalf("Expected chart archive to have %q, got %q", c.Name(), c2.Name())
	}
	if len(c2.Files) != 1 || c2.Files[0].Name != "scheherazade/shahryar.txt" {
		t.Fatal("Files data did not match")
	}
}

func TestSaveDir(t *testing.T) {
	tmp, err := ioutil.TempDir("", "helm-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmp)

	c := &chart.Chart{
		Metadata: &chart.Metadata{
			APIVersion: chart.APIVersionV1,
			Name:       "ahab",
			Version:    "1.2.3",
		},
		Files: []*chart.File{
			{Name: "scheherazade/shahryar.txt", Data: []byte("1,001 Nights")},
		},
	}

	if err := SaveDir(c, tmp); err != nil {
		t.Fatalf("Failed to save: %s", err)
	}

	c2, err := loader.LoadDir(tmp + "/ahab")
	if err != nil {
		t.Fatal(err)
	}

	if c2.Name() != c.Name() {
		t.Fatalf("Expected chart archive to have %q, got %q", c.Name(), c2.Name())
	}
	if len(c2.Files) != 1 || c2.Files[0].Name != "scheherazade/shahryar.txt" {
		t.Fatal("Files data did not match")
	}
}
