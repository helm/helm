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

package chartutil

import (
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"k8s.io/helm/pkg/proto/hapi/chart"
)

func TestSave(t *testing.T) {
	tmp, err := ioutil.TempDir("", "helm-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmp)

	c := &chart.Chart{
		Metadata: &chart.Metadata{
			Name:    "ahab",
			Version: "1.2.3.4",
		},
		Values: &chart.Config{
			Raw: "ship: Pequod",
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

	c2, err := LoadFile(where)
	if err != nil {
		t.Fatal(err)
	}

	if c2.Metadata.Name != c.Metadata.Name {
		t.Fatalf("Expected chart archive to have %q, got %q", c.Metadata.Name, c2.Metadata.Name)
	}
	if c2.Values.Raw != c.Values.Raw {
		t.Fatal("Values data did not match")
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
			Name:    "ahab",
			Version: "1.2.3.4",
		},
		Values: &chart.Config{
			Raw: "ship: Pequod",
		},
	}

	if err := SaveDir(c, tmp); err != nil {
		t.Fatalf("Failed to save: %s", err)
	}

	c2, err := LoadDir(tmp + "/ahab")
	if err != nil {
		t.Fatal(err)
	}

	if c2.Metadata.Name != c.Metadata.Name {
		t.Fatalf("Expected chart archive to have %q, got %q", c.Metadata.Name, c2.Metadata.Name)
	}
	if c2.Values.Raw != c.Values.Raw {
		t.Fatal("Values data did not match")
	}
}
