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
	"bytes"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
)

func TestSave(t *testing.T) {
	tmp, err := ioutil.TempDir("", "helm-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmp)

	for _, dest := range []string{tmp, path.Join(tmp, "newdir")} {
		t.Run("outDir="+dest, func(t *testing.T) {
			c := &chart.Chart{
				Metadata: &chart.Metadata{
					APIVersion: chart.APIVersionV1,
					Name:       "ahab",
					Version:    "1.2.3",
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

			if !bytes.Equal(c.Schema, c2.Schema) {
				indentation := 4
				formattedExpected := Indent(indentation, string(c.Schema))
				formattedActual := Indent(indentation, string(c2.Schema))
				t.Fatalf("Schema data did not match.\nExpected:\n%s\nActual:\n%s", formattedExpected, formattedActual)
			}
			if _, err := Save(&chartWithInvalidJSON, dest); err == nil {
				t.Fatalf("Invalid JSON was not caught while saving chart")
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
		ValuesTemplates: []*chart.File{
			{Name: filepath.Join(ValuesTemplatesDir, "other", "nested", "stuff.yaml"), Data: []byte("def: {{ .Values.def }}")},
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

	if len(c2.ValuesTemplates) != 1 || c2.ValuesTemplates[0].Name != filepath.Join(ValuesTemplatesDir, "other", "nested", "stuff.yaml") {
		t.Fatal("ValuesTemplates data did not match")
	}

	if len(c2.Files) != 1 || c2.Files[0].Name != "scheherazade/shahryar.txt" {
		t.Fatal("Files data did not match")
	}
}
