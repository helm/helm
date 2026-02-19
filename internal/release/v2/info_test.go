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

package v2

import (
	"encoding/json"
	"testing"
	"time"

	"helm.sh/helm/v4/pkg/release/common"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInfoMarshalJSON(t *testing.T) {
	now := time.Date(2025, 10, 8, 12, 0, 0, 0, time.UTC)
	later := time.Date(2025, 10, 8, 13, 0, 0, 0, time.UTC)
	deleted := time.Date(2025, 10, 8, 14, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		info     Info
		expected string
	}{
		{
			name: "all fields populated",
			info: Info{
				FirstDeployed: now,
				LastDeployed:  later,
				Deleted:       deleted,
				Description:   "Test release",
				Status:        common.StatusDeployed,
				Notes:         "Test notes",
			},
			expected: `{"first_deployed":"2025-10-08T12:00:00Z","last_deployed":"2025-10-08T13:00:00Z","deleted":"2025-10-08T14:00:00Z","description":"Test release","status":"deployed","notes":"Test notes"}`,
		},
		{
			name: "only required fields",
			info: Info{
				FirstDeployed: now,
				LastDeployed:  later,
				Status:        common.StatusDeployed,
			},
			expected: `{"first_deployed":"2025-10-08T12:00:00Z","last_deployed":"2025-10-08T13:00:00Z","status":"deployed"}`,
		},
		{
			name: "zero time values omitted",
			info: Info{
				Description: "Test release",
				Status:      common.StatusDeployed,
			},
			expected: `{"description":"Test release","status":"deployed"}`,
		},
		{
			name: "with pending status",
			info: Info{
				FirstDeployed: now,
				LastDeployed:  later,
				Status:        common.StatusPendingInstall,
				Description:   "Installing release",
			},
			expected: `{"first_deployed":"2025-10-08T12:00:00Z","last_deployed":"2025-10-08T13:00:00Z","description":"Installing release","status":"pending-install"}`,
		},
		{
			name: "uninstalled with deleted time",
			info: Info{
				FirstDeployed: now,
				LastDeployed:  later,
				Deleted:       deleted,
				Status:        common.StatusUninstalled,
				Description:   "Uninstalled release",
			},
			expected: `{"first_deployed":"2025-10-08T12:00:00Z","last_deployed":"2025-10-08T13:00:00Z","deleted":"2025-10-08T14:00:00Z","description":"Uninstalled release","status":"uninstalled"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(&tt.info)
			require.NoError(t, err)
			assert.JSONEq(t, tt.expected, string(data))
		})
	}
}

func TestInfoUnmarshalJSON(t *testing.T) {
	now := time.Date(2025, 10, 8, 12, 0, 0, 0, time.UTC)
	later := time.Date(2025, 10, 8, 13, 0, 0, 0, time.UTC)
	deleted := time.Date(2025, 10, 8, 14, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		input    string
		expected Info
		wantErr  bool
	}{
		{
			name:  "all fields populated",
			input: `{"first_deployed":"2025-10-08T12:00:00Z","last_deployed":"2025-10-08T13:00:00Z","deleted":"2025-10-08T14:00:00Z","description":"Test release","status":"deployed","notes":"Test notes"}`,
			expected: Info{
				FirstDeployed: now,
				LastDeployed:  later,
				Deleted:       deleted,
				Description:   "Test release",
				Status:        common.StatusDeployed,
				Notes:         "Test notes",
			},
		},
		{
			name:  "only required fields",
			input: `{"first_deployed":"2025-10-08T12:00:00Z","last_deployed":"2025-10-08T13:00:00Z","status":"deployed"}`,
			expected: Info{
				FirstDeployed: now,
				LastDeployed:  later,
				Status:        common.StatusDeployed,
			},
		},
		{
			name:  "empty string time fields",
			input: `{"first_deployed":"","last_deployed":"","deleted":"","description":"Test release","status":"deployed"}`,
			expected: Info{
				Description: "Test release",
				Status:      common.StatusDeployed,
			},
		},
		{
			name:  "missing time fields",
			input: `{"description":"Test release","status":"deployed"}`,
			expected: Info{
				Description: "Test release",
				Status:      common.StatusDeployed,
			},
		},
		{
			name:  "null time fields",
			input: `{"first_deployed":null,"last_deployed":null,"deleted":null,"description":"Test release","status":"deployed"}`,
			expected: Info{
				Description: "Test release",
				Status:      common.StatusDeployed,
			},
		},
		{
			name:  "mixed empty and valid time fields",
			input: `{"first_deployed":"2025-10-08T12:00:00Z","last_deployed":"","deleted":"","status":"deployed"}`,
			expected: Info{
				FirstDeployed: now,
				Status:        common.StatusDeployed,
			},
		},
		{
			name:  "pending install status",
			input: `{"first_deployed":"2025-10-08T12:00:00Z","status":"pending-install","description":"Installing"}`,
			expected: Info{
				FirstDeployed: now,
				Status:        common.StatusPendingInstall,
				Description:   "Installing",
			},
		},
		{
			name:  "uninstalled with deleted time",
			input: `{"first_deployed":"2025-10-08T12:00:00Z","last_deployed":"2025-10-08T13:00:00Z","deleted":"2025-10-08T14:00:00Z","status":"uninstalled"}`,
			expected: Info{
				FirstDeployed: now,
				LastDeployed:  later,
				Deleted:       deleted,
				Status:        common.StatusUninstalled,
			},
		},
		{
			name:  "failed status",
			input: `{"first_deployed":"2025-10-08T12:00:00Z","last_deployed":"2025-10-08T13:00:00Z","status":"failed","description":"Deployment failed"}`,
			expected: Info{
				FirstDeployed: now,
				LastDeployed:  later,
				Status:        common.StatusFailed,
				Description:   "Deployment failed",
			},
		},
		{
			name:    "invalid time format",
			input:   `{"first_deployed":"invalid-time","status":"deployed"}`,
			wantErr: true,
		},
		{
			name:  "empty object",
			input: `{}`,
			expected: Info{
				Status: "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var info Info
			err := json.Unmarshal([]byte(tt.input), &info)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expected.FirstDeployed.Unix(), info.FirstDeployed.Unix())
			assert.Equal(t, tt.expected.LastDeployed.Unix(), info.LastDeployed.Unix())
			assert.Equal(t, tt.expected.Deleted.Unix(), info.Deleted.Unix())
			assert.Equal(t, tt.expected.Description, info.Description)
			assert.Equal(t, tt.expected.Status, info.Status)
			assert.Equal(t, tt.expected.Notes, info.Notes)
			assert.Equal(t, tt.expected.Resources, info.Resources)
		})
	}
}

func TestInfoRoundTrip(t *testing.T) {
	now := time.Date(2025, 10, 8, 12, 0, 0, 0, time.UTC)
	later := time.Date(2025, 10, 8, 13, 0, 0, 0, time.UTC)

	original := Info{
		FirstDeployed: now,
		LastDeployed:  later,
		Description:   "Test release",
		Status:        common.StatusDeployed,
		Notes:         "Release notes",
	}

	data, err := json.Marshal(&original)
	require.NoError(t, err)

	var decoded Info
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, original.FirstDeployed.Unix(), decoded.FirstDeployed.Unix())
	assert.Equal(t, original.LastDeployed.Unix(), decoded.LastDeployed.Unix())
	assert.Equal(t, original.Deleted.Unix(), decoded.Deleted.Unix())
	assert.Equal(t, original.Description, decoded.Description)
	assert.Equal(t, original.Status, decoded.Status)
	assert.Equal(t, original.Notes, decoded.Notes)
}

func TestInfoEmptyStringRoundTrip(t *testing.T) {
	// This test specifically verifies that empty string time fields
	// are handled correctly during parsing
	input := `{"first_deployed":"","last_deployed":"","deleted":"","status":"deployed","description":"test"}`

	var info Info
	err := json.Unmarshal([]byte(input), &info)
	require.NoError(t, err)

	// Verify time fields are zero values
	assert.True(t, info.FirstDeployed.IsZero())
	assert.True(t, info.LastDeployed.IsZero())
	assert.True(t, info.Deleted.IsZero())
	assert.Equal(t, common.StatusDeployed, info.Status)
	assert.Equal(t, "test", info.Description)

	// Marshal back and verify empty time fields are omitted
	data, err := json.Marshal(&info)
	require.NoError(t, err)

	var result map[string]interface{}
	err = json.Unmarshal(data, &result)
	require.NoError(t, err)

	// Zero time values should be omitted due to omitzero tag
	assert.NotContains(t, result, "first_deployed")
	assert.NotContains(t, result, "last_deployed")
	assert.NotContains(t, result, "deleted")
	assert.Equal(t, "deployed", result["status"])
	assert.Equal(t, "test", result["description"])
}
