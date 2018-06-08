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
	"archive/tar"
	"compress/gzip"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/golang/protobuf/ptypes/any"
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
		Files: []*any.Any{
			{TypeUrl: "scheherazade/shahryar.txt", Value: []byte("1,001 Nights")},
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
	if len(c2.Files) != 1 || c2.Files[0].TypeUrl != "scheherazade/shahryar.txt" {
		t.Fatal("Files data did not match")
	}
}

func TestSavePreservesTimestamps(t *testing.T) {
	// Test executes so quickly that if we don't subtract a second, the
	// check will fail because `initialCreateTime` will be identical to the
	// written timestamp for the files.
	initialCreateTime := time.Now().Add(-1 * time.Second)

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
		Files: []*any.Any{
			{TypeUrl: "scheherazade/shahryar.txt", Value: []byte("1,001 Nights")},
		},
	}

	where, err := Save(c, tmp)
	if err != nil {
		t.Fatalf("Failed to save: %s", err)
	}

	allHeaders, err := retrieveAllHeadersFromTar(where)
	if err != nil {
		t.Fatalf("Failed to parse tar: %v", err)
	}

	for _, header := range allHeaders {
		if header.ModTime.Before(initialCreateTime) {
			t.Fatalf("File timestamp not preserved: %v", header.ModTime)
		}
	}
}

// We could refactor `load.go` to use this `retrieveAllHeadersFromTar` function
// as well, so we are not duplicating components of the code which iterate
// through the tar.
func retrieveAllHeadersFromTar(path string) ([]*tar.Header, error) {
	raw, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer raw.Close()

	unzipped, err := gzip.NewReader(raw)
	if err != nil {
		return nil, err
	}
	defer unzipped.Close()

	tr := tar.NewReader(unzipped)
	headers := []*tar.Header{}
	for {
		hd, err := tr.Next()
		if err == io.EOF {
			break
		}

		if err != nil {
			return nil, err
		}

		headers = append(headers, hd)
	}

	return headers, nil
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
		Files: []*any.Any{
			{TypeUrl: "scheherazade/shahryar.txt", Value: []byte("1,001 Nights")},
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
	if len(c2.Files) != 1 || c2.Files[0].TypeUrl != "scheherazade/shahryar.txt" {
		t.Fatal("Files data did not match")
	}
}
