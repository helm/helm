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
	"bytes"
	"strings"
	"testing"
	"time"

	chart "helm.sh/helm/v4/pkg/chart/v2"
	"helm.sh/helm/v4/pkg/release/common"
	release "helm.sh/helm/v4/pkg/release/v1"
)

// Regression test for #31902: status table timestamps must be rendered in
// the local timezone (derived from TZ), not the zone they were stored with.
func TestStatusTableUsesLocalTime(t *testing.T) {
	originalLocal := time.Local
	time.Local = time.FixedZone("UTC+2", 2*60*60)
	defer func() { time.Local = originalLocal }()

	rel := &release.Release{
		Name:      "tz-release",
		Namespace: "default",
		Info: &release.Info{
			LastDeployed: time.Date(2026, 3, 4, 14, 38, 52, 0, time.UTC),
			Status:       common.StatusDeployed,
		},
	}

	var buf bytes.Buffer
	if err := (statusPrinter{release: rel}).WriteTable(&buf); err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(buf.String(), "LAST DEPLOYED: Wed Mar  4 16:38:52 2026") {
		t.Fatalf("expected LAST DEPLOYED rendered in local zone (UTC+2), got:\n%s", buf.String())
	}
}

func TestStatusCmd(t *testing.T) {
	releasesMockWithStatus := func(info *release.Info, hooks ...*release.Hook) []*release.Release {
		info.LastDeployed = time.Unix(1452902400, 0).UTC()
		return []*release.Release{{
			Name:      "flummoxed-chickadee",
			Namespace: "default",
			Info:      info,
			Chart:     &chart.Chart{Metadata: &chart.Metadata{Name: "name", Version: "1.2.3", AppVersion: "3.2.1"}},
			Hooks:     hooks,
		}}
	}

	tests := []cmdTestCase{{
		name:   "get status of a deployed release",
		cmd:    "status flummoxed-chickadee",
		golden: "output/status.txt",
		rels: releasesMockWithStatus(&release.Info{
			Status: common.StatusDeployed,
		}),
	}, {
		name:   "get status of a deployed release, with desc",
		cmd:    "status flummoxed-chickadee",
		golden: "output/status-with-desc.txt",
		rels: releasesMockWithStatus(&release.Info{
			Status:      common.StatusDeployed,
			Description: "Mock description",
		}),
	}, {
		name:   "get status of a deployed release with notes",
		cmd:    "status flummoxed-chickadee",
		golden: "output/status-with-notes.txt",
		rels: releasesMockWithStatus(&release.Info{
			Status: common.StatusDeployed,
			Notes:  "release notes",
		}),
	}, {
		name:   "get status of a deployed release with notes in json",
		cmd:    "status flummoxed-chickadee -o json",
		golden: "output/status.json",
		rels: releasesMockWithStatus(&release.Info{
			Status: common.StatusDeployed,
			Notes:  "release notes",
		}),
	}, {
		name:   "get status of a deployed release with resources",
		cmd:    "status flummoxed-chickadee",
		golden: "output/status-with-resources.txt",
		rels: releasesMockWithStatus(
			&release.Info{
				Status: common.StatusDeployed,
			},
		),
	}, {
		name:   "get status of a deployed release with resources in json",
		cmd:    "status flummoxed-chickadee -o json",
		golden: "output/status-with-resources.json",
		rels: releasesMockWithStatus(
			&release.Info{
				Status: common.StatusDeployed,
			},
		),
	}, {
		name:   "get status of a deployed release with test suite",
		cmd:    "status flummoxed-chickadee",
		golden: "output/status-with-test-suite.txt",
		rels: releasesMockWithStatus(
			&release.Info{
				Status: common.StatusDeployed,
			},
			&release.Hook{
				Name:   "never-run-test",
				Events: []release.HookEvent{release.HookTest},
			},
			&release.Hook{
				Name:   "passing-test",
				Events: []release.HookEvent{release.HookTest},
				LastRun: release.HookExecution{
					StartedAt:   mustParseTime("2006-01-02T15:04:05Z"),
					CompletedAt: mustParseTime("2006-01-02T15:04:07Z"),
					Phase:       release.HookPhaseSucceeded,
				},
			},
			&release.Hook{
				Name:   "failing-test",
				Events: []release.HookEvent{release.HookTest},
				LastRun: release.HookExecution{
					StartedAt:   mustParseTime("2006-01-02T15:10:05Z"),
					CompletedAt: mustParseTime("2006-01-02T15:10:07Z"),
					Phase:       release.HookPhaseFailed,
				},
			},
			&release.Hook{
				Name:   "passing-pre-install",
				Events: []release.HookEvent{release.HookPreInstall},
				LastRun: release.HookExecution{
					StartedAt:   mustParseTime("2006-01-02T15:00:05Z"),
					CompletedAt: mustParseTime("2006-01-02T15:00:07Z"),
					Phase:       release.HookPhaseSucceeded,
				},
			},
		),
	}}
	runTestCmd(t, tests)
}

func mustParseTime(t string) time.Time {
	res, _ := time.Parse(time.RFC3339, t)
	return res
}

func TestStatusCompletion(t *testing.T) {
	rels := []*release.Release{
		{
			Name:      "athos",
			Namespace: "default",
			Info: &release.Info{
				Status: common.StatusDeployed,
			},
			Chart: &chart.Chart{
				Metadata: &chart.Metadata{
					Name:    "Athos-chart",
					Version: "1.2.3",
				},
			},
		}, {
			Name:      "porthos",
			Namespace: "default",
			Info: &release.Info{
				Status: common.StatusFailed,
			},
			Chart: &chart.Chart{
				Metadata: &chart.Metadata{
					Name:    "Porthos-chart",
					Version: "111.222.333",
				},
			},
		}, {
			Name:      "aramis",
			Namespace: "default",
			Info: &release.Info{
				Status: common.StatusUninstalled,
			},
			Chart: &chart.Chart{
				Metadata: &chart.Metadata{
					Name:    "Aramis-chart",
					Version: "0.0.0",
				},
			},
		}, {
			Name:      "dartagnan",
			Namespace: "gascony",
			Info: &release.Info{
				Status: common.StatusUnknown,
			},
			Chart: &chart.Chart{
				Metadata: &chart.Metadata{
					Name:    "Dartagnan-chart",
					Version: "1.2.3-prerelease",
				},
			},
		}}

	tests := []cmdTestCase{{
		name:   "completion for status",
		cmd:    "__complete status a",
		golden: "output/status-comp.txt",
		rels:   rels,
	}, {
		name:   "completion for status with too many arguments",
		cmd:    "__complete status dartagnan ''",
		golden: "output/status-wrong-args-comp.txt",
		rels:   rels,
	}, {
		name:   "completion for status with global flag",
		cmd:    "__complete status --debug a",
		golden: "output/status-comp.txt",
		rels:   rels,
	}}
	runTestCmd(t, tests)
}

func TestStatusRevisionCompletion(t *testing.T) {
	revisionFlagCompletionTest(t, "status")
}

func TestStatusOutputCompletion(t *testing.T) {
	outputFlagCompletionTest(t, "status")
}

func TestStatusFileCompletion(t *testing.T) {
	checkFileCompletion(t, "status", false)
	checkFileCompletion(t, "status myrelease", false)
}
