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
	stdtime "time"

	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/time"
)

func TestStatusCmd(t *testing.T) {
	releasesMockWithStatus := func(info *release.Info, hooks ...*release.Hook) []*release.Release {
		info.LastDeployed = time.Unix(1452902400, 0).UTC()
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

func mustParseTime(t string) time.Time {
	res, _ := time.Parse(stdtime.RFC3339, t)
	return res
}
