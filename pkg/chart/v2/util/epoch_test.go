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
	"os"
	"testing"
	"time"

	"helm.sh/helm/v4/pkg/chart/common"
	chart "helm.sh/helm/v4/pkg/chart/v2"
)

func TestParseSourceDateEpoch(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		set     bool
		want    time.Time
		wantErr bool
	}{
		{
			name: "not set",
			set:  false,
			want: time.Time{},
		},
		{
			name:  "valid epoch",
			value: "1700000000",
			set:   true,
			want:  time.Unix(1700000000, 0),
		},
		{
			name:    "invalid string",
			value:   "not-a-number",
			set:     true,
			wantErr: true,
		},
		{
			name:    "negative value",
			value:   "-1",
			set:     true,
			wantErr: true,
		},
		{
			name:  "zero",
			value: "0",
			set:   true,
			want:  time.Unix(0, 0),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.set {
				t.Setenv("SOURCE_DATE_EPOCH", tt.value)
			} else {
				prevVal, wasSet := os.LookupEnv("SOURCE_DATE_EPOCH")
				os.Unsetenv("SOURCE_DATE_EPOCH")
				t.Cleanup(func() {
					if wasSet {
						os.Setenv("SOURCE_DATE_EPOCH", prevVal)
					}
				})
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
	epoch := time.Unix(1700000000, 0)

	c := &chart.Chart{
		Metadata: &chart.Metadata{
			Name:    "test",
			Version: "0.1.0",
		},
		Templates: []*common.File{
			{Name: "templates/test.yaml"},
		},
		Files: []*common.File{
			{Name: "README.md"},
		},
	}

	ApplySourceDateEpoch(c, epoch)

	if !c.ModTime.Equal(epoch) {
		t.Errorf("Chart.ModTime = %v, want %v", c.ModTime, epoch)
	}
	for _, f := range c.Templates {
		if !f.ModTime.Equal(epoch) {
			t.Errorf("Template %s ModTime = %v, want %v", f.Name, f.ModTime, epoch)
		}
	}
	for _, f := range c.Files {
		if !f.ModTime.Equal(epoch) {
			t.Errorf("File %s ModTime = %v, want %v", f.Name, f.ModTime, epoch)
		}
	}
}

func TestApplySourceDateEpochPreservesExisting(t *testing.T) {
	epoch := time.Unix(1700000000, 0)
	existing := time.Unix(1600000000, 0)

	c := &chart.Chart{
		Metadata: &chart.Metadata{
			Name:    "test",
			Version: "0.1.0",
		},
		ModTime: existing,
	}

	ApplySourceDateEpoch(c, epoch)

	if !c.ModTime.Equal(existing) {
		t.Errorf("Chart.ModTime = %v, want existing %v", c.ModTime, existing)
	}
}

func TestApplySourceDateEpochZeroNoop(t *testing.T) {
	c := &chart.Chart{
		Metadata: &chart.Metadata{
			Name:    "test",
			Version: "0.1.0",
		},
	}

	ApplySourceDateEpoch(c, time.Time{})

	if !c.ModTime.IsZero() {
		t.Errorf("Chart.ModTime = %v, want zero", c.ModTime)
	}
}

func TestApplySourceDateEpochDependencies(t *testing.T) {
	epoch := time.Unix(1700000000, 0)
	existing := time.Unix(1600000000, 0)

	dep := &chart.Chart{
		Metadata: &chart.Metadata{
			APIVersion: chart.APIVersionV2,
			Name:       "dep",
			Version:    "0.1.0",
		},
		Templates: []*common.File{
			{Name: "templates/dep.yaml"},
		},
	}

	c := &chart.Chart{
		Metadata: &chart.Metadata{
			APIVersion: chart.APIVersionV2,
			Name:       "parent",
			Version:    "1.0.0",
		},
		ModTime: existing,
		Templates: []*common.File{
			{Name: "templates/main.yaml"},
		},
	}
	c.AddDependency(dep)

	ApplySourceDateEpoch(c, epoch)

	// Parent chart already had a ModTime, so it should be preserved.
	if !c.ModTime.Equal(existing) {
		t.Errorf("parent Chart.ModTime = %v, want existing %v", c.ModTime, existing)
	}
	// Dependency had a zero ModTime, so it should be stamped.
	if !dep.ModTime.Equal(epoch) {
		t.Errorf("dep Chart.ModTime = %v, want %v", dep.ModTime, epoch)
	}
	for _, f := range dep.Templates {
		if !f.ModTime.Equal(epoch) {
			t.Errorf("dep Template %s ModTime = %v, want %v", f.Name, f.ModTime, epoch)
		}
	}
}

func TestSaveWithSourceDateEpoch(t *testing.T) {
	// End-to-end: parse SOURCE_DATE_EPOCH, apply to a chart with zero
	// ModTimes, save as a tar archive, and verify every tar entry carries
	// exactly the expected timestamp.
	const epochStr = "1700000000"
	want := time.Unix(1700000000, 0)

	t.Setenv("SOURCE_DATE_EPOCH", epochStr)

	epoch, err := ParseSourceDateEpoch()
	if err != nil {
		t.Fatalf("ParseSourceDateEpoch() error: %v", err)
	}

	c := &chart.Chart{
		Metadata: &chart.Metadata{
			APIVersion: chart.APIVersionV2,
			Name:       "epoch-test",
			Version:    "0.1.0",
		},
		Values:   map[string]any{"key": "value"},
		Schema:   []byte(`{"title": "Values"}`),
		Files:    []*common.File{{Name: "README.md", Data: []byte("# test")}},
		Templates: []*common.File{{Name: "templates/test.yaml", Data: []byte("apiVersion: v1")}},
	}

	ApplySourceDateEpoch(c, epoch)

	tmp := t.TempDir()
	where, err := Save(c, tmp)
	if err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	headers, err := retrieveAllHeadersFromTar(where)
	if err != nil {
		t.Fatalf("failed to read tar: %v", err)
	}

	if len(headers) == 0 {
		t.Fatal("archive contains no entries")
	}

	for _, h := range headers {
		if !h.ModTime.Equal(want) {
			t.Errorf("tar entry %q ModTime = %v, want %v", h.Name, h.ModTime, want)
		}
	}
}
