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

package main

import (
	"testing"
	"time"

	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/release"
	helmtime "helm.sh/helm/v3/pkg/time"
)

func TestStatusCmd(t *testing.T) {
	releasesMockWithStatus := func(info *release.Info, hooks ...*release.Hook) []*release.Release {
		info.LastDeployed = helmtime.Unix(1452902400, 0).UTC()
		return []*release.Release{{
			Name:      "flummoxed-chickadee",
			Namespace: "default",
			Info:      info,
			Chart:     &chart.Chart{},
			Hooks:     hooks,
		}}
	}

	tests := []cmdTestCase{{
		name:   "get status of a deployed release",
		cmd:    "status flummoxed-chickadee",
		golden: "output/status.txt",
		rels: releasesMockWithStatus(&release.Info{
			Status: release.StatusDeployed,
		}),
	}, {
		name:   "get status of a deployed release, with desc",
		cmd:    "status --show-desc flummoxed-chickadee",
		golden: "output/status-with-desc.txt",
		rels: releasesMockWithStatus(&release.Info{
			Status:      release.StatusDeployed,
			Description: "Mock description",
		}),
	}, {
		name:   "get status of a deployed release with notes",
		cmd:    "status flummoxed-chickadee",
		golden: "output/status-with-notes.txt",
		rels: releasesMockWithStatus(&release.Info{
			Status: release.StatusDeployed,
			Notes:  "release notes",
		}),
	}, {
		name:   "get status of a deployed release with notes in json",
		cmd:    "status flummoxed-chickadee -o json",
		golden: "output/status.json",
		rels: releasesMockWithStatus(&release.Info{
			Status: release.StatusDeployed,
			Notes:  "release notes",
		}),
	}, {
		name:   "get status of a deployed release with test suite",
		cmd:    "status flummoxed-chickadee",
		golden: "output/status-with-test-suite.txt",
		rels: releasesMockWithStatus(
			&release.Info{
				Status: release.StatusDeployed,
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

func mustParseTime(t string) helmtime.Time {
	res, _ := helmtime.Parse(time.RFC3339, t)
	return res
}

func TestStatusCompletion(t *testing.T) {
	rels := []*release.Release{
		{
			Name:      "athos",
			Namespace: "default",
			Info: &release.Info{
				Status: release.StatusDeployed,
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
				Status: release.StatusFailed,
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
				Status: release.StatusUninstalled,
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
				Status: release.StatusUnknown,
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
