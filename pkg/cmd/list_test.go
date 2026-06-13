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
	"testing"
	"time"

	chart "helm.sh/helm/v4/pkg/chart/v2"
	"helm.sh/helm/v4/pkg/release/common"
	release "helm.sh/helm/v4/pkg/release/v1"
)

func TestListCmd(t *testing.T) {
	defaultNamespace := "default"

	sampleTimeSeconds := int64(1452902400)
	timestamp1 := time.Unix(sampleTimeSeconds+1, 0).UTC()
	timestamp2 := time.Unix(sampleTimeSeconds+2, 0).UTC()
	timestamp3 := time.Unix(sampleTimeSeconds+3, 0).UTC()
	timestamp4 := time.Unix(sampleTimeSeconds+4, 0).UTC()
	chartInfo := &chart.Chart{
		Metadata: &chart.Metadata{
			Name:       "chickadee",
			Version:    "1.0.0",
			AppVersion: "0.0.1",
		},
	}

	releaseFixture := []*release.Release{
		{
			Name:      "starlord",
			Version:   1,
			Namespace: defaultNamespace,
			Info: &release.Info{
				LastDeployed: timestamp1,
				Status:       common.StatusSuperseded,
			},
			Chart: chartInfo,
		},
		{
			Name:      "starlord",
			Version:   2,
			Namespace: defaultNamespace,
			Info: &release.Info{
				LastDeployed: timestamp1,
				Status:       common.StatusDeployed,
			},
			Chart: chartInfo,
		},
		{
			Name:      "groot",
			Version:   1,
			Namespace: defaultNamespace,
			Info: &release.Info{
				LastDeployed: timestamp1,
				Status:       common.StatusUninstalled,
			},
			Chart: chartInfo,
		},
		{
			Name:      "gamora",
			Version:   1,
			Namespace: defaultNamespace,
			Info: &release.Info{
				LastDeployed: timestamp1,
				Status:       common.StatusSuperseded,
			},
			Chart: chartInfo,
		},
		{
			Name:      "rocket",
			Version:   1,
			Namespace: defaultNamespace,
			Info: &release.Info{
				LastDeployed: timestamp2,
				Status:       common.StatusFailed,
			},
			Chart: chartInfo,
		},
		{
			Name:      "drax",
			Version:   1,
			Namespace: defaultNamespace,
			Info: &release.Info{
				LastDeployed: timestamp1,
				Status:       common.StatusUninstalling,
			},
			Chart: chartInfo,
		},
		{
			Name:      "thanos",
			Version:   1,
			Namespace: defaultNamespace,
			Info: &release.Info{
				LastDeployed: timestamp1,
				Status:       common.StatusPendingInstall,
			},
			Chart: chartInfo,
		},
		{
			Name:      "hummingbird",
			Version:   1,
			Namespace: defaultNamespace,
			Info: &release.Info{
				LastDeployed: timestamp3,
				Status:       common.StatusDeployed,
			},
			Chart: chartInfo,
		},
		{
			Name:      "iguana",
			Version:   2,
			Namespace: defaultNamespace,
			Info: &release.Info{
				LastDeployed: timestamp4,
				Status:       common.StatusDeployed,
			},
			Chart: chartInfo,
		},
		{
			Name:      "starlord",
			Version:   2,
			Namespace: "milano",
			Info: &release.Info{
				LastDeployed: timestamp1,
				Status:       common.StatusDeployed,
			},
			Chart: chartInfo,
		},
	}

	tests := []cmdTestCase{{
		name:   "list releases",
		cmd:    "list",
		golden: "output/list-all.txt",
		rels:   releaseFixture,
	}, {
		name:   "list without headers",
		cmd:    "list --no-headers",
		golden: "output/list-all-no-headers.txt",
		rels:   releaseFixture,
	}, {
		name:   "list releases sorted by release date",
		cmd:    "list --date",
		golden: "output/list-all-date.txt",
		rels:   releaseFixture,
	}, {
		name:   "list failed releases",
		cmd:    "list --failed",
		golden: "output/list-failed.txt",
		rels:   releaseFixture,
	}, {
		name:   "list filtered releases",
		cmd:    "list --filter='.*'",
		golden: "output/list-all.txt",
		rels:   releaseFixture,
	}, {
		name:   "list releases, limited to one release",
		cmd:    "list --max 1",
		golden: "output/list-all-max.txt",
		rels:   releaseFixture,
	}, {
		name:   "list releases, offset by one",
		cmd:    "list --offset 1",
		golden: "output/list-all-offset.txt",
		rels:   releaseFixture,
	}, {
		name:   "list pending releases",
		cmd:    "list --pending",
		golden: "output/list-pending.txt",
		rels:   releaseFixture,
	}, {
		name:   "list releases in reverse order",
		cmd:    "list --reverse",
		golden: "output/list-all-reverse.txt",
		rels:   releaseFixture,
	}, {
		name:   "list releases sorted by reversed release date",
		cmd:    "list --date --reverse",
		golden: "output/list-all-date-reversed.txt",
		rels:   releaseFixture,
	}, {
		name:   "list releases in short output format",
		cmd:    "list --short",
		golden: "output/list-all-short.txt",
		rels:   releaseFixture,
	}, {
		name:   "list releases in short output format",
		cmd:    "list --short --output yaml",
		golden: "output/list-all-short-yaml.txt",
		rels:   releaseFixture,
	}, {
		name:   "list releases in short output format",
		cmd:    "list --short --output json",
		golden: "output/list-all-short-json.txt",
		rels:   releaseFixture,
	}, {
		name:   "list deployed and failed releases only",
		cmd:    "list --deployed --failed",
		golden: "output/list.txt",
		rels:   releaseFixture,
	}, {
		name:   "list superseded releases",
		cmd:    "list --superseded",
		golden: "output/list-superseded.txt",
		rels:   releaseFixture,
	}, {
		name:   "list uninstalled releases",
		cmd:    "list --uninstalled",
		golden: "output/list-uninstalled.txt",
		rels:   releaseFixture,
	}, {
		name:   "list releases currently uninstalling",
		cmd:    "list --uninstalling",
		golden: "output/list-uninstalling.txt",
		rels:   releaseFixture,
	}, {
		name:   "list releases in another namespace",
		cmd:    "list -n milano",
		golden: "output/list-namespace.txt",
		rels:   releaseFixture,
	}}
	runTestCmd(t, tests)
}

func TestListOutputCompletion(t *testing.T) {
	outputFlagCompletionTest(t, "list")
}

func TestListFileCompletion(t *testing.T) {
	checkFileCompletion(t, "list", false)
}

func TestListOutputFormats(t *testing.T) {
	defaultNamespace := "default"
	timestamp := time.Unix(1452902400, 0).UTC()
	chartInfo := &chart.Chart{
		Metadata: &chart.Metadata{
			Name:       "test-chart",
			Version:    "1.0.0",
			AppVersion: "0.0.1",
		},
	}

	releaseFixture := []*release.Release{
		{
			Name:      "test-release",
			Version:   1,
			Namespace: defaultNamespace,
			Info: &release.Info{
				LastDeployed: timestamp,
				Status:       common.StatusDeployed,
			},
			Chart: chartInfo,
		},
	}

	tests := []cmdTestCase{{
		name:   "list releases in json format",
		cmd:    "list --output json",
		golden: "output/list-json.txt",
		rels:   releaseFixture,
	}, {
		name:   "list releases in yaml format",
		cmd:    "list --output yaml",
		golden: "output/list-yaml.txt",
		rels:   releaseFixture,
	}}
	runTestCmd(t, tests)
}

func TestReleaseListWriter(t *testing.T) {
	timestamp := time.Unix(1452902400, 0).UTC()
	chartInfo := &chart.Chart{
		Metadata: &chart.Metadata{
			Name:       "test-chart",
			Version:    "1.0.0",
			AppVersion: "0.0.1",
		},
	}

	releases := []*release.Release{
		{
			Name:      "test-release",
			Version:   1,
			Namespace: "default",
			Info: &release.Info{
				LastDeployed: timestamp,
				Status:       common.StatusDeployed,
			},
			Chart: chartInfo,
		},
	}

	tests := []struct {
		name       string
		releases   []*release.Release
		timeFormat string
		noHeaders  bool
		noColor    bool
	}{
		{
			name:       "empty releases list",
			releases:   []*release.Release{},
			timeFormat: "",
			noHeaders:  false,
			noColor:    false,
		},
		{
			name:       "custom time format",
			releases:   releases,
			timeFormat: "2006-01-02",
			noHeaders:  false,
			noColor:    false,
		},
		{
			name:       "no headers",
			releases:   releases,
			timeFormat: "",
			noHeaders:  true,
			noColor:    false,
		},
		{
			name:       "no color",
			releases:   releases,
			timeFormat: "",
			noHeaders:  false,
			noColor:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			writer := newReleaseListWriter(tt.releases, tt.timeFormat, tt.noHeaders, tt.noColor)

			if writer == nil {
				t.Error("Expected writer to be non-nil")
			} else {
				if len(writer.releases) != len(tt.releases) {
					t.Errorf("Expected %d releases, got %d", len(tt.releases), len(writer.releases))
				}
			}
		})
	}
}

func TestReleaseListWriterMethods(t *testing.T) {
	timestamp := time.Unix(1452902400, 0).UTC()
	zeroTimestamp := time.Time{}
	chartInfo := &chart.Chart{
		Metadata: &chart.Metadata{
			Name:       "test-chart",
			Version:    "1.0.0",
			AppVersion: "0.0.1",
		},
	}

	releases := []*release.Release{
		{
			Name:      "test-release",
			Version:   1,
			Namespace: "default",
			Info: &release.Info{
				LastDeployed: timestamp,
				Status:       common.StatusDeployed,
			},
			Chart: chartInfo,
		},
		{
			Name:      "zero-time-release",
			Version:   1,
			Namespace: "default",
			Info: &release.Info{
				LastDeployed: zeroTimestamp,
				Status:       common.StatusFailed,
			},
			Chart: chartInfo,
		},
	}

	tests := []struct {
		name   string
		status common.Status
	}{
		{"deployed", common.StatusDeployed},
		{"failed", common.StatusFailed},
		{"pending-install", common.StatusPendingInstall},
		{"pending-upgrade", common.StatusPendingUpgrade},
		{"pending-rollback", common.StatusPendingRollback},
		{"uninstalling", common.StatusUninstalling},
		{"uninstalled", common.StatusUninstalled},
		{"superseded", common.StatusSuperseded},
		{"unknown", common.StatusUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testReleases := []*release.Release{
				{
					Name:      "test-release",
					Version:   1,
					Namespace: "default",
					Info: &release.Info{
						LastDeployed: timestamp,
						Status:       tt.status,
					},
					Chart: chartInfo,
				},
			}

			writer := newReleaseListWriter(testReleases, "", false, false)

			var buf []byte
			out := &bytesWriter{buf: &buf}

			err := writer.WriteJSON(out)
			if err != nil {
				t.Errorf("WriteJSON failed: %v", err)
			}

			err = writer.WriteYAML(out)
			if err != nil {
				t.Errorf("WriteYAML failed: %v", err)
			}

			err = writer.WriteTable(out)
			if err != nil {
				t.Errorf("WriteTable failed: %v", err)
			}
		})
	}

	writer := newReleaseListWriter(releases, "", false, false)

	var buf []byte
	out := &bytesWriter{buf: &buf}

	err := writer.WriteJSON(out)
	if err != nil {
		t.Errorf("WriteJSON failed: %v", err)
	}

	err = writer.WriteYAML(out)
	if err != nil {
		t.Errorf("WriteYAML failed: %v", err)
	}

	err = writer.WriteTable(out)
	if err != nil {
		t.Errorf("WriteTable failed: %v", err)
	}
}

func TestFilterReleases(t *testing.T) {
	releases := []*release.Release{
		{Name: "release1"},
		{Name: "release2"},
		{Name: "release3"},
	}

	tests := []struct {
		name                string
		releases            []*release.Release
		ignoredReleaseNames []string
		expectedCount       int
	}{
		{
			name:                "nil ignored list",
			releases:            releases,
			ignoredReleaseNames: nil,
			expectedCount:       3,
		},
		{
			name:                "empty ignored list",
			releases:            releases,
			ignoredReleaseNames: []string{},
			expectedCount:       3,
		},
		{
			name:                "filter one release",
			releases:            releases,
			ignoredReleaseNames: []string{"release1"},
			expectedCount:       2,
		},
		{
			name:                "filter multiple releases",
			releases:            releases,
			ignoredReleaseNames: []string{"release1", "release3"},
			expectedCount:       1,
		},
		{
			name:                "filter non-existent release",
			releases:            releases,
			ignoredReleaseNames: []string{"non-existent"},
			expectedCount:       3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filterReleases(tt.releases, tt.ignoredReleaseNames)
			if len(result) != tt.expectedCount {
				t.Errorf("Expected %d releases, got %d", tt.expectedCount, len(result))
			}
		})
	}
}

type bytesWriter struct {
	buf *[]byte
}

func (b *bytesWriter) Write(p []byte) (n int, err error) {
	*b.buf = append(*b.buf, p...)
	return len(p), nil
}

func TestListCustomTimeFormat(t *testing.T) {
	defaultNamespace := "default"
	timestamp := time.Unix(1452902400, 0).UTC()
	chartInfo := &chart.Chart{
		Metadata: &chart.Metadata{
			Name:       "test-chart",
			Version:    "1.0.0",
			AppVersion: "0.0.1",
		},
	}

	releaseFixture := []*release.Release{
		{
			Name:      "test-release",
			Version:   1,
			Namespace: defaultNamespace,
			Info: &release.Info{
				LastDeployed: timestamp,
				Status:       common.StatusDeployed,
			},
			Chart: chartInfo,
		},
	}

	tests := []cmdTestCase{{
		name:   "list releases with custom time format",
		cmd:    "list --time-format '2006-01-02 15:04:05'",
		golden: "output/list-time-format.txt",
		rels:   releaseFixture,
	}}
	runTestCmd(t, tests)
}

func TestListStatusMapping(t *testing.T) {
	defaultNamespace := "default"
	timestamp := time.Unix(1452902400, 0).UTC()
	chartInfo := &chart.Chart{
		Metadata: &chart.Metadata{
			Name:       "test-chart",
			Version:    "1.0.0",
			AppVersion: "0.0.1",
		},
	}

	testCases := []struct {
		name   string
		status common.Status
	}{
		{"deployed", common.StatusDeployed},
		{"failed", common.StatusFailed},
		{"pending-install", common.StatusPendingInstall},
		{"pending-upgrade", common.StatusPendingUpgrade},
		{"pending-rollback", common.StatusPendingRollback},
		{"uninstalling", common.StatusUninstalling},
		{"uninstalled", common.StatusUninstalled},
		{"superseded", common.StatusSuperseded},
		{"unknown", common.StatusUnknown},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			releaseFixture := []*release.Release{
				{
					Name:      "test-release",
					Version:   1,
					Namespace: defaultNamespace,
					Info: &release.Info{
						LastDeployed: timestamp,
						Status:       tc.status,
					},
					Chart: chartInfo,
				},
			}

			writer := newReleaseListWriter(releaseFixture, "", false, false)
			if len(writer.releases) != 1 {
				t.Errorf("Expected 1 release, got %d", len(writer.releases))
			}

			if writer.releases[0].Status != tc.status.String() {
				t.Errorf("Expected status %s, got %s", tc.status.String(), writer.releases[0].Status)
			}
		})
	}
}
