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
	"bytes"
	"compress/gzip"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"helm.sh/helm/v3/internal/test/ensure"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
)

func TestSave(t *testing.T) {
	tmp := ensure.TempDir(t)
	defer os.RemoveAll(tmp)

	for _, dest := range []string{tmp, path.Join(tmp, "newdir")} {
		t.Run("outDir="+dest, func(t *testing.T) {
			c := &chart.Chart{
				Metadata: &chart.Metadata{
					APIVersion: chart.APIVersionV1,
					Name:       "ahab",
					Version:    "1.2.3",
				},
				Lock: &chart.Lock{
					Digest: "testdigest",
				},
				Files: []*chart.File{
					{Name: "scheherazade/shahryar.txt", Data: []byte("1,001 Nights")},
				},
				Schema: []byte("{\n  \"title\": \"Values\"\n}"),
			}
			chartWithInvalidJSON := withSchema(*c, []byte("{"))

			where, err := Save(c, dest)
			if err != nil {
				t.Fatalf("Failed to save: %s", err)
			}
			if !strings.HasPrefix(where, dest) {
				t.Fatalf("Expected %q to start with %q", where, dest)
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
			if c2.Lock != nil {
				t.Fatal("Expected v1 chart archive not to contain Chart.lock file")
			}

			if !bytes.Equal(c.Schema, c2.Schema) {
				indentation := 4
				formattedExpected := Indent(indentation, string(c.Schema))
				formattedActual := Indent(indentation, string(c2.Schema))
				t.Fatalf("Schema data did not match.\nExpected:\n%s\nActual:\n%s", formattedExpected, formattedActual)
			}
			if _, err := Save(&chartWithInvalidJSON, dest); err == nil {
				t.Fatalf("Invalid JSON was not caught while saving chart")
			}

			c.Metadata.APIVersion = chart.APIVersionV2
			where, err = Save(c, dest)
			if err != nil {
				t.Fatalf("Failed to save: %s", err)
			}
			c2, err = loader.LoadFile(where)
			if err != nil {
				t.Fatal(err)
			}
			if c2.Lock == nil {
				t.Fatal("Expected v2 chart archive to contain a Chart.lock file")
			}
			if c2.Lock.Digest != c.Lock.Digest {
				t.Fatal("Chart.lock data did not match")
			}
		})
	}
}

// Creates a copy with a different schema; does not modify anything.
func withSchema(chart chart.Chart, schema []byte) chart.Chart {
	chart.Schema = schema
	return chart
}

func Indent(n int, text string) string {
	startOfLine := regexp.MustCompile(`(?m)^`)
	indentation := strings.Repeat(" ", n)
	return startOfLine.ReplaceAllLiteralString(text, indentation)
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
			APIVersion: chart.APIVersionV1,
			Name:       "ahab",
			Version:    "1.2.3",
		},
		Values: map[string]interface{}{
			"imageName": "testimage",
			"imageId":   42,
		},
		Files: []*chart.File{
			{Name: "scheherazade/shahryar.txt", Data: []byte("1,001 Nights")},
		},
		Schema: []byte("{\n  \"title\": \"Values\"\n}"),
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
			APIVersion: chart.APIVersionV1,
			Name:       "ahab",
			Version:    "1.2.3",
		},
		Files: []*chart.File{
			{Name: "scheherazade/shahryar.txt", Data: []byte("1,001 Nights")},
		},
		Templates: []*chart.File{
			{Name: filepath.Join(TemplatesDir, "nested", "dir", "thing.yaml"), Data: []byte("abc: {{ .Values.abc }}")},
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

	if len(c2.Templates) != 1 || c2.Templates[0].Name != filepath.Join(TemplatesDir, "nested", "dir", "thing.yaml") {
		t.Fatal("Templates data did not match")
	}

	if len(c2.Files) != 1 || c2.Files[0].Name != "scheherazade/shahryar.txt" {
		t.Fatal("Files data did not match")
	}
}
