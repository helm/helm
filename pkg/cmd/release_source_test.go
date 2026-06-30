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

package cmd

import "testing"

func TestSanitizeChartSource(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{
			name:   "strips userinfo from https URL",
			source: "https://user:pass@charts.example.com/mychart",
			want:   "https://charts.example.com/mychart",
		},
		{
			name:   "strips username-only userinfo",
			source: "https://user@charts.example.com/mychart",
			want:   "https://charts.example.com/mychart",
		},
		{
			name:   "strips query string and fragment",
			source: "https://charts.example.com/index.yaml?token=secret#section",
			want:   "https://charts.example.com/index.yaml",
		},
		{
			name:   "strips userinfo, query and fragment together",
			source: "https://user:pass@charts.example.com/index.yaml?token=secret#frag",
			want:   "https://charts.example.com/index.yaml",
		},
		{
			name:   "strips userinfo from oci URL",
			source: "oci://user:pass@registry.example.com/charts/mychart",
			want:   "oci://registry.example.com/charts/mychart",
		},
		{
			name:   "leaves credential-free https URL unchanged",
			source: "https://charts.example.com/mychart",
			want:   "https://charts.example.com/mychart",
		},
		{
			name:   "leaves chart reference unchanged",
			source: "stable/mychart",
			want:   "stable/mychart",
		},
		{
			name:   "leaves relative local path unchanged",
			source: "./charts/mychart",
			want:   "./charts/mychart",
		},
		{
			name:   "leaves absolute unix path unchanged",
			source: "/var/charts/mychart",
			want:   "/var/charts/mychart",
		},
		{
			name:   "leaves windows absolute path unchanged",
			source: `C:\charts\mychart`,
			want:   `C:\charts\mychart`,
		},
		{
			name:   "leaves windows forward-slash path unchanged",
			source: "C:/charts/mychart",
			want:   "C:/charts/mychart",
		},
		{
			name:   "leaves bare chart name unchanged",
			source: "mychart",
			want:   "mychart",
		},
		{
			name:   "leaves empty source unchanged",
			source: "",
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sanitizeChartSource(tt.source); got != tt.want {
				t.Errorf("sanitizeChartSource(%q) = %q, want %q", tt.source, got, tt.want)
			}
		})
	}
}
