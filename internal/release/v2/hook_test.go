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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHookExecutionMarshalJSON(t *testing.T) {
	started := time.Date(2025, 10, 8, 12, 0, 0, 0, time.UTC)
	completed := time.Date(2025, 10, 8, 12, 5, 0, 0, time.UTC)

	tests := []struct {
		name     string
		exec     HookExecution
		expected string
	}{
		{
			name: "all fields populated",
			exec: HookExecution{
				StartedAt:   started,
				CompletedAt: completed,
				Phase:       HookPhaseSucceeded,
			},
			expected: `{"started_at":"2025-10-08T12:00:00Z","completed_at":"2025-10-08T12:05:00Z","phase":"Succeeded"}`,
		},
		{
			name: "only phase",
			exec: HookExecution{
				Phase: HookPhaseRunning,
			},
			expected: `{"phase":"Running"}`,
		},
		{
			name: "with started time only",
			exec: HookExecution{
				StartedAt: started,
				Phase:     HookPhaseRunning,
			},
			expected: `{"started_at":"2025-10-08T12:00:00Z","phase":"Running"}`,
		},
		{
			name: "failed phase",
			exec: HookExecution{
				StartedAt:   started,
				CompletedAt: completed,
				Phase:       HookPhaseFailed,
			},
			expected: `{"started_at":"2025-10-08T12:00:00Z","completed_at":"2025-10-08T12:05:00Z","phase":"Failed"}`,
		},
		{
			name: "unknown phase",
			exec: HookExecution{
				Phase: HookPhaseUnknown,
			},
			expected: `{"phase":"Unknown"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(&tt.exec)
			require.NoError(t, err)
			assert.JSONEq(t, tt.expected, string(data))
		})
	}
}

func TestHookExecutionUnmarshalJSON(t *testing.T) {
	started := time.Date(2025, 10, 8, 12, 0, 0, 0, time.UTC)
	completed := time.Date(2025, 10, 8, 12, 5, 0, 0, time.UTC)

	tests := []struct {
		name     string
		input    string
		expected HookExecution
		wantErr  bool
	}{
		{
			name:  "all fields populated",
			input: `{"started_at":"2025-10-08T12:00:00Z","completed_at":"2025-10-08T12:05:00Z","phase":"Succeeded"}`,
			expected: HookExecution{
				StartedAt:   started,
				CompletedAt: completed,
				Phase:       HookPhaseSucceeded,
			},
		},
		{
			name:  "only phase",
			input: `{"phase":"Running"}`,
			expected: HookExecution{
				Phase: HookPhaseRunning,
			},
		},
		{
			name:  "empty string time fields",
			input: `{"started_at":"","completed_at":"","phase":"Succeeded"}`,
			expected: HookExecution{
				Phase: HookPhaseSucceeded,
			},
		},
		{
			name:  "missing time fields",
			input: `{"phase":"Failed"}`,
			expected: HookExecution{
				Phase: HookPhaseFailed,
			},
		},
		{
			name:  "null time fields",
			input: `{"started_at":null,"completed_at":null,"phase":"Unknown"}`,
			expected: HookExecution{
				Phase: HookPhaseUnknown,
			},
		},
		{
			name:  "mixed empty and valid time fields",
			input: `{"started_at":"2025-10-08T12:00:00Z","completed_at":"","phase":"Running"}`,
			expected: HookExecution{
				StartedAt: started,
				Phase:     HookPhaseRunning,
			},
		},
		{
			name:  "with started time only",
			input: `{"started_at":"2025-10-08T12:00:00Z","phase":"Running"}`,
			expected: HookExecution{
				StartedAt: started,
				Phase:     HookPhaseRunning,
			},
		},
		{
			name:  "failed phase with times",
			input: `{"started_at":"2025-10-08T12:00:00Z","completed_at":"2025-10-08T12:05:00Z","phase":"Failed"}`,
			expected: HookExecution{
				StartedAt:   started,
				CompletedAt: completed,
				Phase:       HookPhaseFailed,
			},
		},
		{
			name:    "invalid time format",
			input:   `{"started_at":"invalid-time","phase":"Running"}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var exec HookExecution
			err := json.Unmarshal([]byte(tt.input), &exec)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expected.StartedAt.Unix(), exec.StartedAt.Unix())
			assert.Equal(t, tt.expected.CompletedAt.Unix(), exec.CompletedAt.Unix())
			assert.Equal(t, tt.expected.Phase, exec.Phase)
		})
	}
}

func TestHookExecutionRoundTrip(t *testing.T) {
	started := time.Date(2025, 10, 8, 12, 0, 0, 0, time.UTC)
	completed := time.Date(2025, 10, 8, 12, 5, 0, 0, time.UTC)

	original := HookExecution{
		StartedAt:   started,
		CompletedAt: completed,
		Phase:       HookPhaseSucceeded,
	}

	data, err := json.Marshal(&original)
	require.NoError(t, err)

	var decoded HookExecution
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, original.StartedAt.Unix(), decoded.StartedAt.Unix())
	assert.Equal(t, original.CompletedAt.Unix(), decoded.CompletedAt.Unix())
	assert.Equal(t, original.Phase, decoded.Phase)
}

func TestHookExecutionEmptyStringRoundTrip(t *testing.T) {
	// This test specifically verifies that empty string time fields
	// are handled correctly during parsing
	input := `{"started_at":"","completed_at":"","phase":"Succeeded"}`

	var exec HookExecution
	err := json.Unmarshal([]byte(input), &exec)
	require.NoError(t, err)

	// Verify time fields are zero values
	assert.True(t, exec.StartedAt.IsZero())
	assert.True(t, exec.CompletedAt.IsZero())
	assert.Equal(t, HookPhaseSucceeded, exec.Phase)

	// Marshal back and verify empty time fields are omitted
	data, err := json.Marshal(&exec)
	require.NoError(t, err)

	var result map[string]interface{}
	err = json.Unmarshal(data, &result)
	require.NoError(t, err)

	// Zero time values should be omitted
	assert.NotContains(t, result, "started_at")
	assert.NotContains(t, result, "completed_at")
	assert.Equal(t, "Succeeded", result["phase"])
}
