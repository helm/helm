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
	"compress/gzip"
	"os"
	"path"
	"testing"
	"time"

	"helm.sh/helm/v4/pkg/chart/common"
	chart "helm.sh/helm/v4/pkg/chart/v2"
)

func TestParseSourceDateEpoch(t *testing.T) {
	tests := []struct {
		name    string
		envVal  string
		envSet  bool
		want    time.Time
		wantErr bool
	}{
		{
			name:   "not set",
			envSet: false,
			want:   time.Time{},
		},
		{
			name:   "empty string",
			envVal: "",
			envSet: true,
			want:   time.Time{},
		},
		{
			name:   "valid epoch",
			envVal: "1609459200",
			envSet: true,
			want:   time.Unix(1609459200, 0),
		},
		{
			name:   "zero",
			envVal: "0",
			envSet: true,
			want:   time.Unix(0, 0),
		},
		{
			name:    "negative value",
			envVal:  "-1",
			envSet:  true,
			wantErr: true,
		},
		{
			name:    "not a number",
			envVal:  "not-a-number",
			envSet:  true,
			wantErr: true,
		},
		{
			name:    "floating point",
			envVal:  "1609459200.5",
			envSet:  true,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envSet {
				t.Setenv("SOURCE_DATE_EPOCH", tt.envVal)
			} else {
				os.Unsetenv("SOURCE_DATE_EPOCH")
			}
			got, err := ParseSourceDateEpoch()
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseSourceDateEpoch() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !got.Equal(tt.want) {
				t.Errorf("ParseSourceDateEpoch() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestApplySourceDateEpoch(t *testing.T) {
	epoch := time.Unix(1609459200, 0)
	existing := time.Unix(1000000000, 0)

	c := &chart.Chart{
		Metadata: &chart.Metadata{
			APIVersion: chart.APIVersionV2,
			Name:       "test",
			Version:    "1.0.0",
		},
		Lock:   &chart.Lock{Digest: "d"},
		Schema: []byte("{}"),
		Files: []*common.File{
			{Name: "README.md", Data: []byte("# readme")},
			{Name: "existing.txt", Data: []byte("data"), ModTime: existing},
		},
		Templates: []*common.File{
			{Name: "templates/test.yaml", Data: []byte("test")},
		},
	}

	ApplySourceDateEpoch(c, epoch)

	if !c.ModTime.Equal(epoch) {
		t.Errorf("Chart.ModTime = %v, want %v", c.ModTime, epoch)
	}
	if !c.Lock.Generated.Equal(epoch) {
		t.Errorf("Lock.Generated = %v, want %v", c.Lock.Generated, epoch)
	}
	if !c.SchemaModTime.Equal(epoch) {
		t.Errorf("SchemaModTime = %v, want %v", c.SchemaModTime, epoch)
	}
	// File with zero ModTime should be updated
	if !c.Files[0].ModTime.Equal(epoch) {
		t.Errorf("Files[0].ModTime = %v, want %v", c.Files[0].ModTime, epoch)
	}
	// File with existing ModTime should be preserved
	if !c.Files[1].ModTime.Equal(existing) {
		t.Errorf("Files[1].ModTime = %v, want %v (preserved)", c.Files[1].ModTime, existing)
	}
	if !c.Templates[0].ModTime.Equal(epoch) {
		t.Errorf("Templates[0].ModTime = %v, want %v", c.Templates[0].ModTime, epoch)
	}
}

func TestApplySourceDateEpochZeroTimeNoop(t *testing.T) {
	c := &chart.Chart{
		Metadata: &chart.Metadata{
			APIVersion: chart.APIVersionV2,
			Name:       "test",
			Version:    "1.0.0",
		},
	}

	ApplySourceDateEpoch(c, time.Time{})

	if !c.ModTime.IsZero() {
		t.Errorf("expected ModTime to remain zero, got %v", c.ModTime)
	}
}

func TestRepeatableSaveWithSourceDateEpoch(t *testing.T) {
	const sde = "1609459200" // 2021-01-01T00:00:00Z
	expectedTime := time.Unix(1609459200, 0)
	t.Setenv("SOURCE_DATE_EPOCH", sde)

	tmp := t.TempDir()

	c := &chart.Chart{
		Metadata: &chart.Metadata{
			APIVersion: chart.APIVersionV2,
			Name:       "ahab",
			Version:    "1.2.3",
		},
		Lock: &chart.Lock{Digest: "testdigest"},
		Files: []*common.File{
			{Name: "scheherazade/shahryar.txt", Data: []byte("1,001 Nights")},
		},
		Schema: []byte("{\n  \"title\": \"Values\"\n}"),
	}

	// Parse and apply SOURCE_DATE_EPOCH before saving, as the caller would.
	epochTime, err := ParseSourceDateEpoch()
	if err != nil {
		t.Fatalf("ParseSourceDateEpoch() error: %v", err)
	}
	ApplySourceDateEpoch(c, epochTime)

	dest1 := path.Join(tmp, "out1")
	where1, err := Save(c, dest1)
	if err != nil {
		t.Fatalf("Failed to save: %s", err)
	}
	h1, err := sha256Sum(where1)
	if err != nil {
		t.Fatalf("Failed to check shasum: %s", err)
	}

	dest2 := path.Join(tmp, "out2")
	where2, err := Save(c, dest2)
	if err != nil {
		t.Fatalf("Failed to save: %s", err)
	}
	h2, err := sha256Sum(where2)
	if err != nil {
		t.Fatalf("Failed to check shasum: %s", err)
	}

	if h1 != h2 {
		t.Fatalf("Expected deterministic output with SOURCE_DATE_EPOCH set, got %s and %s", h1, h2)
	}

	// Verify that tar headers actually have the expected ModTime, not time.Now().
	f, err := os.Open(where1)
	if err != nil {
		t.Fatalf("Failed to open archive: %s", err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatalf("Failed to create gzip reader: %s", err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err != nil {
			break
		}
		if !hdr.ModTime.Equal(expectedTime) {
			t.Errorf("Entry %s: expected ModTime %v, got %v", hdr.Name, expectedTime, hdr.ModTime)
		}
	}
}
