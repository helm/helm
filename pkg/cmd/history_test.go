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

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"helm.sh/helm/v4/pkg/release/common"
	release "helm.sh/helm/v4/pkg/release/v1"
)

func TestHistoryCmd(t *testing.T) {
	mk := func(name string, vers int, status common.Status) *release.Release {
		return release.Mock(&release.MockReleaseOptions{
			Name:    name,
			Version: vers,
			Status:  status,
		})
	}

	tests := []cmdTestCase{{
		name: "get history for release",
		cmd:  "history angry-bird",
		rels: []*release.Release{
			mk("angry-bird", 4, common.StatusDeployed),
			mk("angry-bird", 3, common.StatusSuperseded),
			mk("angry-bird", 2, common.StatusSuperseded),
			mk("angry-bird", 1, common.StatusSuperseded),
		},
		golden: "output/history.txt",
	}, {
		name: "get history with max limit set",
		cmd:  "history angry-bird --max 2",
		rels: []*release.Release{
			mk("angry-bird", 4, common.StatusDeployed),
			mk("angry-bird", 3, common.StatusSuperseded),
		},
		golden: "output/history-limit.txt",
	}, {
		name: "get history with yaml output format",
		cmd:  "history angry-bird --output yaml",
		rels: []*release.Release{
			mk("angry-bird", 4, common.StatusDeployed),
			mk("angry-bird", 3, common.StatusSuperseded),
		},
		golden: "output/history.yaml",
	}, {
		name: "get history with json output format",
		cmd:  "history angry-bird --output json",
		rels: []*release.Release{
			mk("angry-bird", 4, common.StatusDeployed),
			mk("angry-bird", 3, common.StatusSuperseded),
		},
		golden: "output/history.json",
	}}
	runTestCmd(t, tests)
}

func TestHistoryOutputCompletion(t *testing.T) {
	outputFlagCompletionTest(t, "history")
}

func revisionFlagCompletionTest(t *testing.T, cmdName string) {
	t.Helper()
	mk := func(name string, vers int, status common.Status) *release.Release {
		return release.Mock(&release.MockReleaseOptions{
			Name:    name,
			Version: vers,
			Status:  status,
		})
	}

	releases := []*release.Release{
		mk("musketeers", 11, common.StatusDeployed),
		mk("musketeers", 10, common.StatusSuperseded),
		mk("musketeers", 9, common.StatusSuperseded),
		mk("musketeers", 8, common.StatusSuperseded),
	}

	tests := []cmdTestCase{{
		name:   "completion for revision flag",
		cmd:    fmt.Sprintf("__complete %s musketeers --revision ''", cmdName),
		rels:   releases,
		golden: "output/revision-comp.txt",
	}, {
		name:   "completion for revision flag, no filter",
		cmd:    fmt.Sprintf("__complete %s musketeers --revision 1", cmdName),
		rels:   releases,
		golden: "output/revision-comp.txt",
	}, {
		name:   "completion for revision flag with too few args",
		cmd:    fmt.Sprintf("__complete %s --revision ''", cmdName),
		rels:   releases,
		golden: "output/revision-wrong-args-comp.txt",
	}, {
		name:   "completion for revision flag with too many args",
		cmd:    fmt.Sprintf("__complete %s three musketeers --revision ''", cmdName),
		rels:   releases,
		golden: "output/revision-wrong-args-comp.txt",
	}}
	runTestCmd(t, tests)
}

func TestHistoryCompletion(t *testing.T) {
	checkReleaseCompletion(t, "history", false)
}

func TestHistoryFileCompletion(t *testing.T) {
	checkFileCompletion(t, "history", false)
	checkFileCompletion(t, "history myrelease", false)
}

func TestReleaseInfoMarshalJSON(t *testing.T) {
	updated := time.Date(2025, 10, 8, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		info     releaseInfo
		expected string
	}{
		{
			name: "all fields populated",
			info: releaseInfo{
				Revision:    1,
				Updated:     updated,
				Status:      "deployed",
				Chart:       "mychart-1.0.0",
				AppVersion:  "1.0.0",
				Description: "Initial install",
			},
			expected: `{"revision":1,"updated":"2025-10-08T12:00:00Z","status":"deployed","chart":"mychart-1.0.0","app_version":"1.0.0","description":"Initial install"}`,
		},
		{
			name: "without updated time",
			info: releaseInfo{
				Revision:    2,
				Status:      "superseded",
				Chart:       "mychart-1.0.1",
				AppVersion:  "1.0.1",
				Description: "Upgraded",
			},
			expected: `{"revision":2,"status":"superseded","chart":"mychart-1.0.1","app_version":"1.0.1","description":"Upgraded"}`,
		},
		{
			name: "with zero revision",
			info: releaseInfo{
				Revision:    0,
				Updated:     updated,
				Status:      "failed",
				Chart:       "mychart-1.0.0",
				AppVersion:  "1.0.0",
				Description: "Install failed",
			},
			expected: `{"revision":0,"updated":"2025-10-08T12:00:00Z","status":"failed","chart":"mychart-1.0.0","app_version":"1.0.0","description":"Install failed"}`,
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

func TestReleaseInfoUnmarshalJSON(t *testing.T) {
	updated := time.Date(2025, 10, 8, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		input    string
		expected releaseInfo
		wantErr  bool
	}{
		{
			name:  "all fields populated",
			input: `{"revision":1,"updated":"2025-10-08T12:00:00Z","status":"deployed","chart":"mychart-1.0.0","app_version":"1.0.0","description":"Initial install"}`,
			expected: releaseInfo{
				Revision:    1,
				Updated:     updated,
				Status:      "deployed",
				Chart:       "mychart-1.0.0",
				AppVersion:  "1.0.0",
				Description: "Initial install",
			},
		},
		{
			name:  "empty string updated field",
			input: `{"revision":2,"updated":"","status":"superseded","chart":"mychart-1.0.1","app_version":"1.0.1","description":"Upgraded"}`,
			expected: releaseInfo{
				Revision:    2,
				Status:      "superseded",
				Chart:       "mychart-1.0.1",
				AppVersion:  "1.0.1",
				Description: "Upgraded",
			},
		},
		{
			name:  "missing updated field",
			input: `{"revision":3,"status":"deployed","chart":"mychart-1.0.2","app_version":"1.0.2","description":"Upgraded"}`,
			expected: releaseInfo{
				Revision:    3,
				Status:      "deployed",
				Chart:       "mychart-1.0.2",
				AppVersion:  "1.0.2",
				Description: "Upgraded",
			},
		},
		{
			name:  "null updated field",
			input: `{"revision":4,"updated":null,"status":"failed","chart":"mychart-1.0.3","app_version":"1.0.3","description":"Failed"}`,
			expected: releaseInfo{
				Revision:    4,
				Status:      "failed",
				Chart:       "mychart-1.0.3",
				AppVersion:  "1.0.3",
				Description: "Failed",
			},
		},
		{
			name:    "invalid time format",
			input:   `{"revision":5,"updated":"invalid-time","status":"deployed","chart":"mychart-1.0.4","app_version":"1.0.4","description":"Test"}`,
			wantErr: true,
		},
		{
			name:  "zero revision",
			input: `{"revision":0,"updated":"2025-10-08T12:00:00Z","status":"pending-install","chart":"mychart-1.0.0","app_version":"1.0.0","description":"Installing"}`,
			expected: releaseInfo{
				Revision:    0,
				Updated:     updated,
				Status:      "pending-install",
				Chart:       "mychart-1.0.0",
				AppVersion:  "1.0.0",
				Description: "Installing",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var info releaseInfo
			err := json.Unmarshal([]byte(tt.input), &info)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expected.Revision, info.Revision)
			assert.Equal(t, tt.expected.Updated.Unix(), info.Updated.Unix())
			assert.Equal(t, tt.expected.Status, info.Status)
			assert.Equal(t, tt.expected.Chart, info.Chart)
			assert.Equal(t, tt.expected.AppVersion, info.AppVersion)
			assert.Equal(t, tt.expected.Description, info.Description)
		})
	}
}

func TestReleaseInfoRoundTrip(t *testing.T) {
	updated := time.Date(2025, 10, 8, 12, 0, 0, 0, time.UTC)

	original := releaseInfo{
		Revision:    1,
		Updated:     updated,
		Status:      "deployed",
		Chart:       "mychart-1.0.0",
		AppVersion:  "1.0.0",
		Description: "Initial install",
	}

	data, err := json.Marshal(&original)
	require.NoError(t, err)

	var decoded releaseInfo
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, original.Revision, decoded.Revision)
	assert.Equal(t, original.Updated.Unix(), decoded.Updated.Unix())
	assert.Equal(t, original.Status, decoded.Status)
	assert.Equal(t, original.Chart, decoded.Chart)
	assert.Equal(t, original.AppVersion, decoded.AppVersion)
	assert.Equal(t, original.Description, decoded.Description)
}

func TestReleaseInfoEmptyStringRoundTrip(t *testing.T) {
	// This test specifically verifies that empty string time fields
	// are handled correctly during parsing
	input := `{"revision":1,"updated":"","status":"deployed","chart":"mychart-1.0.0","app_version":"1.0.0","description":"Test"}`

	var info releaseInfo
	err := json.Unmarshal([]byte(input), &info)
	require.NoError(t, err)

	// Verify time field is zero value
	assert.True(t, info.Updated.IsZero())
	assert.Equal(t, 1, info.Revision)
	assert.Equal(t, "deployed", info.Status)

	// Marshal back and verify empty time field is omitted
	data, err := json.Marshal(&info)
	require.NoError(t, err)

	var result map[string]interface{}
	err = json.Unmarshal(data, &result)
	require.NoError(t, err)

	// Zero time value should be omitted
	assert.NotContains(t, result, "updated")
	assert.Equal(t, float64(1), result["revision"])
	assert.Equal(t, "deployed", result["status"])
	assert.Equal(t, "mychart-1.0.0", result["chart"])
}
