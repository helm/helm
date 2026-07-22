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
	"encoding/binary"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"helm.sh/helm/v4/pkg/chart/common"
	chart "helm.sh/helm/v4/pkg/chart/v2"
	"helm.sh/helm/v4/pkg/chart/v2/loader"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSave(t *testing.T) {
	tmp := t.TempDir()

	for _, dest := range []string{tmp, filepath.Join(tmp, "newdir")} {
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
				Files: []*common.File{
					{Name: "scheherazade/shahryar.txt", ModTime: time.Now(), Data: []byte("1,001 Nights")},
				},
				Schema: []byte("{\n  \"title\": \"Values\"\n}"),
			}
			chartWithInvalidJSON := withSchema(*c, []byte("{"))

			where, err := Save(c, dest)
			require.NoError(t, err, "Failed to save")
			require.Truef(t, strings.HasPrefix(where, dest), "Expected %q to start with %q", where, dest)
			require.Truef(t, strings.HasSuffix(where, ".tgz"), "Expected %q to end with .tgz", where)

			c2, err := loader.LoadFile(where)
			require.NoError(t, err)
			require.Equal(t, c.Name(), c2.Name(), "Expected chart archive to have %q, got %q", c.Name(), c2.Name())
			require.Len(t, c2.Files, 1, "Files data did not match")
			require.Equal(t, "scheherazade/shahryar.txt", c2.Files[0].Name, "Files data did not match")
			require.Nil(t, c2.Lock, "Expected v1 chart archive not to contain Chart.lock file")

			if !bytes.Equal(c.Schema, c2.Schema) {
				indentation := 4
				formattedExpected := Indent(indentation, string(c.Schema))
				formattedActual := Indent(indentation, string(c2.Schema))
				t.Fatalf("Schema data did not match.\nExpected:\n%s\nActual:\n%s", formattedExpected, formattedActual)
			}
			_, err = Save(&chartWithInvalidJSON, dest)
			require.Error(t, err, "Invalid JSON was not caught while saving chart")

			c.Metadata.APIVersion = chart.APIVersionV2
			where, err = Save(c, dest)
			require.NoError(t, err, "Failed to save")
			c2, err = loader.LoadFile(where)
			require.NoError(t, err)
			require.NotNil(t, c2.Lock, "Expected v2 chart archive to contain a Chart.lock file")
			require.Equal(t, c.Lock.Digest, c2.Lock.Digest, "Chart.lock data did not match")
		})
	}

	c := &chart.Chart{
		Metadata: &chart.Metadata{
			APIVersion: chart.APIVersionV1,
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
	require.Error(t, err, "Expected error saving chart with invalid name")
}

// https://github.com/helm/helm/issues/31844
func TestSavedGzipExtraFieldIsValid(t *testing.T) {
	tmp := t.TempDir()
	c := &chart.Chart{
		Metadata: &chart.Metadata{
			APIVersion: chart.APIVersionV2,
			Name:       "ahab",
			Version:    "1.2.3",
		},
	}

	where, err := Save(c, tmp)
	require.NoError(t, err, "Failed to save")

	f, err := os.Open(where)
	require.NoError(t, err, "Failed to open saved file")
	defer f.Close()

	r, err := gzip.NewReader(f)
	require.NoError(t, err, "Failed to create gzip reader")
	defer r.Close()

	// RFC 1952 §2.3.1.1:
	// Each subfield consists of SI1, SI2 (1 byte each),
	// a 2-byte little-endian LEN, and LEN bytes of data.
	// https://www.rfc-editor.org/rfc/rfc1952.html#page-8
	extra := r.Extra

	require.NotEmpty(t, extra)
	require.GreaterOrEqual(t, len(extra), 4)

	dataLen := int(binary.LittleEndian.Uint16(extra[2:4]))
	// Assume a single subfield.
	require.Lenf(t, extra, 4+dataLen, "gzip extra field has malformed subfield: LEN=%d but %d data byte(s) follow the subfield header", dataLen, len(extra)-4)
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
			APIVersion: chart.APIVersionV1,
			Name:       "ahab",
			Version:    "1.2.3",
		},
		ModTime: initialCreateTime,
		Values: map[string]any{
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
	require.NoError(t, err, "Failed to save")

	allHeaders, err := retrieveAllHeadersFromTar(where)
	require.NoError(t, err, "Failed to parse tar")

	roundedTime := initialCreateTime.Round(time.Second)
	for _, header := range allHeaders {
		require.Truef(t, header.ModTime.Equal(roundedTime), "File timestamp not preserved: %v", header.ModTime)
	}
}

func TestSaveWithSourceDateEpoch(t *testing.T) {
	epoch := time.Unix(1609459200, 0).UTC()
	tmp := t.TempDir()
	c := &chart.Chart{
		Metadata: &chart.Metadata{
			APIVersion: chart.APIVersionV2,
			Name:       "ahab",
			Version:    "1.2.3",
		},
		Files: []*common.File{
			{Name: "scheherazade/shahryar.txt", Data: []byte("1,001 Nights")},
		},
		Schema: []byte("{\n  \"title\": \"Values\"\n}"),
	}

	c.StampModTimes(epoch)
	where, err := Save(c, tmp)
	require.NoError(t, err, "Failed to save")

	allHeaders, err := retrieveAllHeadersFromTar(where)
	require.NoError(t, err, "Failed to parse tar")

	expected := epoch.Round(time.Second)
	for _, header := range allHeaders {
		require.Truef(t, header.ModTime.Equal(expected), "Expected SOURCE_DATE_EPOCH timestamp %v, got %v for %q", expected, header.ModTime, header.Name)
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
			APIVersion: chart.APIVersionV1,
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

	require.NoErrorf(t, SaveDir(c, tmp), "Failed to save")

	c2, err := loader.LoadDir(tmp + "/ahab")
	require.NoError(t, err)

	require.Equal(t, c.Name(), c2.Name(), "Expected chart archive to have %q, got %q", c.Name(), c2.Name())

	require.Len(t, c2.Templates, 1, "Templates data did not match")
	require.Equal(t, c2.Templates[0].Name, c.Templates[0].Name, "Templates data did not match")

	require.Len(t, c2.Files, 1, "Files data did not match")
	require.Equal(t, c2.Files[0].Name, c.Files[0].Name, "Files data did not match")

	tmp2 := t.TempDir()
	c.Metadata.Name = "../ahab"
	pth := filepath.Join(tmp2, "tmpcharts")
	require.NoError(t, os.MkdirAll(filepath.Join(pth), 0o755))

	require.EqualErrorf(t, SaveDir(c, pth), "\"../ahab\" is not a valid chart name", "Did not get expected error for chart named %q", c.Name())
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
					APIVersion: chart.APIVersionV2,
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
			want: "63358874b93ea095c857cd66bcf5d0a4464840cf84a07547db744d81d6c5af59",
		},
		{
			name: "Package 2 files",
			chart: &chart.Chart{
				Metadata: &chart.Metadata{
					APIVersion: chart.APIVersionV2,
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
			want: "c2a43990053da788ad4e260d3b00d52a0b103ccc67ab9f48278a7b6dcfb2a4bd",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// create package
			dest := path.Join(tmp, "newdir")
			where, err := Save(test.chart, dest)
			require.NoError(t, err, "Failed to save")
			// get shasum for package
			result, err := sha256Sum(where)
			require.NoError(t, err, "Failed to check shasum")
			// assert that the package SHA is what we wanted.
			assert.Equal(t, test.want, result, "FormatName() result = %v, want %v", result, test.want)
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

	return hex.EncodeToString(h.Sum(nil)), nil
}
