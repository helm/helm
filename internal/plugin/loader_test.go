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
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPeekAPIVersion(t *testing.T) {
	testCases := map[string]struct {
		data     []byte
		expected string
	}{
		"v1": {
			data: []byte(`---
apiVersion: v1
name: "test-plugin"
`),
			expected: "v1",
		},
		"legacy": { // No apiVersion field
			data: []byte(`---
name: "test-plugin"
`),
			expected: "",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			version, err := peekAPIVersion(bytes.NewReader(tc.data))
			require.NoError(t, err)
			assert.Equal(t, tc.expected, version)
		})
	}

	// invalid yaml
	{
		data := []byte(`bad yaml`)
		_, err := peekAPIVersion(bytes.NewReader(data))
		assert.Error(t, err)
	}
}
