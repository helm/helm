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
