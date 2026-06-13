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

package plugin

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseEnv(t *testing.T) {
	type testCase struct {
		env      []string
		expected map[string]string
	}

	testCases := map[string]testCase{
		"empty": {
			env:      []string{},
			expected: map[string]string{},
		},
		"single": {
			env:      []string{"KEY=value"},
			expected: map[string]string{"KEY": "value"},
		},
		"multiple": {
			env:      []string{"KEY1=value1", "KEY2=value2"},
			expected: map[string]string{"KEY1": "value1", "KEY2": "value2"},
		},
		"no_value": {
			env:      []string{"KEY1=value1", "KEY2="},
			expected: map[string]string{"KEY1": "value1", "KEY2": ""},
		},
		"duplicate_keys": {
			env:      []string{"KEY=value1", "KEY=value2"},
			expected: map[string]string{"KEY": "value2"}, // last value should overwrite
		},
		"empty_strings": {
			env:      []string{"", "KEY=value", ""},
			expected: map[string]string{"KEY": "value"},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			result := ParseEnv(tc.env)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestFormatEnv(t *testing.T) {
	type testCase struct {
		env      map[string]string
		expected []string
	}

	testCases := map[string]testCase{
		"empty": {
			env:      map[string]string{},
			expected: []string{},
		},
		"single": {
			env:      map[string]string{"KEY": "value"},
			expected: []string{"KEY=value"},
		},
		"multiple": {
			env:      map[string]string{"KEY1": "value1", "KEY2": "value2"},
			expected: []string{"KEY1=value1", "KEY2=value2"},
		},
		"empty_key": {
			env:      map[string]string{"": "value1", "KEY2": "value2"},
			expected: []string{"=value1", "KEY2=value2"},
		},
		"empty_value": {
			env:      map[string]string{"KEY1": "value1", "KEY2": "", "KEY3": "value3"},
			expected: []string{"KEY1=value1", "KEY2=", "KEY3=value3"},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			result := FormatEnv(tc.env)
			assert.ElementsMatch(t, tc.expected, result)
		})
	}
}
