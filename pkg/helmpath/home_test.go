// Copyright The Helm Authors.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package helmpath

import (
	"testing"
)

func TestCacheIndexFile(t *testing.T) {
	tests := []struct {
		name     string
		repoName string
		expected string
	}{
		{
			name:     "empty repository name",
			repoName: "",
			expected: "index.yaml",
		},
		{
			name:     "standard repository name",
			repoName: "stable",
			expected: "stable-index.yaml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := CacheIndexFile(tt.repoName)
			if actual != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, actual)
			}
		})
	}
}

func TestCacheChartsFile(t *testing.T) {
	tests := []struct {
		name     string
		repoName string
		expected string
	}{
		{
			name:     "empty repository name",
			repoName: "",
			expected: "charts.txt",
		},
		{
			name:     "standard repository name",
			repoName: "stable",
			expected: "stable-charts.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := CacheChartsFile(tt.repoName)
			if actual != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, actual)
			}
		})
	}
}
