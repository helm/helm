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

package util

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	chart "helm.sh/helm/v4/internal/chart/v3"
	"helm.sh/helm/v4/internal/chart/v3/loader"
	"helm.sh/helm/v4/pkg/chart/common"
)

func TestSave(t *testing.T) {
	tmp := t.TempDir()

	for _, dest := range []string{tmp, filepath.Join(tmp, "newdir")} {
		t.Run("outDir="+dest, func(t *testing.T) {
			c := &chart.Chart{
				Metadata: &chart.Metadata{
					APIVersion: chart.APIVersionV3,
					Name:       "ahab",
					Version:    "1.2.3",
				},
				Lock: &chart.Lock{
					Digest: "testdigest",
				},
				Files: []*common.File{
					{Name: "scheherazade/shahryar.txt", ModTime: time.Now(), Data: []byte("1,001 Nights")},
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

			c.Metadata.APIVersion = chart.APIVersionV3
			where, err = Save(c, dest)
			if err != nil {
				t.Fatalf("Failed to save: %s", err)
			}
			c2, err = loader.LoadFile(where)
			if err != nil {
				t.Fatal(err)
			}
			if c2.Lock == nil {
				t.Fatal("Expected v3 chart archive to contain a Chart.lock file")
			}
			if c2.Lock.Digest != c.Lock.Digest {
				t.Fatal("Chart.lock data did not match")
			}
		})
	}

	c := &chart.Chart{
		Metadata: &chart.Metadata{
			APIVersion: chart.APIVersionV3,
			Name:       "../ahab",
			Version:    "1.2.3",
		},
		Lock: &chart.Lock{
			Digest: "testdigest",
		},
		Files: []*common.File{
			{Name: "scheherazade/shahryar.txt", ModTime: time.Now(), Data: []byte("1,001 Nights")},
		},
	}
	_, err := Save(c, tmp)
	if err == nil {
		t.Fatal("Expected error saving chart with invalid name")
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
	tmp := t.TempDir()

	c := &chart.Chart{
		Metadata: &chart.Metadata{
			APIVersion: chart.APIVersionV3,
			Name:       "ahab",
			Version:    "1.2.3",
		},
		ModTime: initialCreateTime,
		Values: map[string]interface{}{
			"imageName": "testimage",
			"imageId":   42,
		},
		Files: []*common.File{
			{Name: "scheherazade/shahryar.txt", ModTime: initialCreateTime, Data: []byte("1,001 Nights")},
		},
		Schema:        []byte("{\n  \"title\": \"Values\"\n}"),
		SchemaModTime: initialCreateTime,
	}

	where, err := Save(c, tmp)
	if err != nil {
		t.Fatalf("Failed to save: %s", err)
	}

	allHeaders, err := retrieveAllHeadersFromTar(where)
	if err != nil {
		t.Fatalf("Failed to parse tar: %v", err)
	}

	roundedTime := initialCreateTime.Round(time.Second)
	for _, header := range allHeaders {
		if !header.ModTime.Equal(roundedTime) {
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
		if errors.Is(err, io.EOF) {
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
	tmp := t.TempDir()
	modTime := time.Now()

	c := &chart.Chart{
		Metadata: &chart.Metadata{
			APIVersion: chart.APIVersionV3,
			Name:       "ahab",
			Version:    "1.2.3",
		},
		Files: []*common.File{
			{Name: "scheherazade/shahryar.txt", ModTime: modTime, Data: []byte("1,001 Nights")},
		},
		Templates: []*common.File{
			{Name: path.Join(TemplatesDir, "nested", "dir", "thing.yaml"), ModTime: modTime, Data: []byte("abc: {{ .Values.abc }}")},
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

	if len(c2.Templates) != 1 || c2.Templates[0].Name != c.Templates[0].Name {
		t.Fatal("Templates data did not match")
	}

	if len(c2.Files) != 1 || c2.Files[0].Name != c.Files[0].Name {
		t.Fatal("Files data did not match")
	}

	tmp2 := t.TempDir()
	c.Metadata.Name = "../ahab"
	pth := filepath.Join(tmp2, "tmpcharts")
	if err := os.MkdirAll(filepath.Join(pth), 0755); err != nil {
		t.Fatal(err)
	}

	if err := SaveDir(c, pth); err.Error() != "\"../ahab\" is not a valid chart name" {
		t.Fatalf("Did not get expected error for chart named %q", c.Name())
	}
}

func TestRepeatableSave(t *testing.T) {
	tmp := t.TempDir()
	defer os.RemoveAll(tmp)
	modTime := time.Date(2021, 9, 1, 20, 34, 58, 651387237, time.UTC)
	tests := []struct {
		name  string
		chart *chart.Chart
		want  string
	}{
		{
			name: "Package 1 file",
			chart: &chart.Chart{
				Metadata: &chart.Metadata{
					APIVersion: chart.APIVersionV3,
					Name:       "ahab",
					Version:    "1.2.3",
				},
				ModTime: modTime,
				Lock: &chart.Lock{
					Digest:    "testdigest",
					Generated: modTime,
				},
				Files: []*common.File{
					{Name: "scheherazade/shahryar.txt", ModTime: modTime, Data: []byte("1,001 Nights")},
				},
				Schema:        []byte("{\n  \"title\": \"Values\"\n}"),
				SchemaModTime: modTime,
			},
			want: "5bfea18cc3c8cbc265744bc32bffa9489a4dbe87d6b51b90f4255e4839d35e03",
		},
		{
			name: "Package 2 files",
			chart: &chart.Chart{
				Metadata: &chart.Metadata{
					APIVersion: chart.APIVersionV3,
					Name:       "ahab",
					Version:    "1.2.3",
				},
				ModTime: modTime,
				Lock: &chart.Lock{
					Digest:    "testdigest",
					Generated: modTime,
				},
				Files: []*common.File{
					{Name: "scheherazade/shahryar.txt", ModTime: modTime, Data: []byte("1,001 Nights")},
					{Name: "scheherazade/dunyazad.txt", ModTime: modTime, Data: []byte("1,001 Nights again")},
				},
				Schema:        []byte("{\n  \"title\": \"Values\"\n}"),
				SchemaModTime: modTime,
			},
			want: "a240365c21e0a2f4a57873132a9b686566a612d08bcb3f20c9446bfff005ccce",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// create package
			dest := path.Join(tmp, "newdir")
			where, err := Save(test.chart, dest)
			if err != nil {
				t.Fatalf("Failed to save: %s", err)
			}
			// get shasum for package
			result, err := sha256Sum(where)
			if err != nil {
				t.Fatalf("Failed to check shasum: %s", err)
			}
			// assert that the package SHA is what we wanted.
			if result != test.want {
				t.Errorf("FormatName() result = %v, want %v", result, test.want)
			}
		})
	}
}

func sha256Sum(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}
