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

package releaseutil

import (
	"testing"

	"helm.sh/helm/v3/pkg/release"
)

func TestSortHooksToDelete(t *testing.T) {
	tests := []struct {
		name     string
		hooks    []*release.Hook
		expected []string
	}{
		{
			name: "the same weight, claim should be deleted first",
			hooks: []*release.Hook{
				{Kind: "PersistentVolume", Name: "a", Weight: 0},
				{Kind: "PersistentVolumeClaim", Name: "b", Weight: 0},
			},
			expected: []string{"b", "a"},
		},
		{
			name: "different weight, volume should be deleted first",
			hooks: []*release.Hook{
				{Kind: "PersistentVolume", Name: "a", Weight: 1},
				{Kind: "PersistentVolumeClaim", Name: "b", Weight: 0},
			},
			expected: []string{"a", "b"},
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				SortHooksToDelete(tt.hooks)
				for i, h := range tt.hooks {
					if h.Name != tt.expected[i] {
						t.Errorf("Expected %s, got %s", tt.expected[i], h.Name)
					}
				}
			},
		)
	}
}
