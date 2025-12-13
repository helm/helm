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

package action

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseRevisions(t *testing.T) {
	tests := []struct {
		input    string
		expected []int
		expectErr bool
	}{
		{
			input:    "",
			expected: nil,
		},
		{
			input:    "1",
			expected: []int{1},
		},
		{
			input:    "1,3,5",
			expected: []int{1, 3, 5},
		},
		{
			input:    "1..5",
			expected: []int{1, 2, 3, 4, 5},
		},
		{
			input:    "1,3..5,7",
			expected: []int{1, 3, 4, 5, 7},
		},
		{
			input:    "5..1",
			expected: nil,
			expectErr: true,
		},
		{
			input:    "invalid",
			expected: nil,
			expectErr: true,
		},
		{
			input:    "1..invalid",
			expected: nil,
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := ParseRevisions(tt.input)

			if tt.expectErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}