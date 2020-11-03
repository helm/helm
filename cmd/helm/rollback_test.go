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

	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/release"
)

func TestRollbackCmd(t *testing.T) {
	rels := []*release.Release{
		{
			Name:    "funny-honey",
			Info:    &release.Info{Status: release.StatusSuperseded},
			Chart:   &chart.Chart{},
			Version: 1,
		},
		{
			Name:    "funny-honey",
			Info:    &release.Info{Status: release.StatusDeployed},
			Chart:   &chart.Chart{},
			Version: 2,
		},
	}

	tests := []cmdTestCase{{
		name:   "rollback a release",
		cmd:    "rollback funny-honey 1",
		golden: "output/rollback.txt",
		rels:   rels,
	}, {
		name:   "rollback a release with timeout",
		cmd:    "rollback funny-honey 1 --timeout 120s",
		golden: "output/rollback-timeout.txt",
		rels:   rels,
	}, {
		name:   "rollback a release with wait",
		cmd:    "rollback funny-honey 1 --wait",
		golden: "output/rollback-wait.txt",
		rels:   rels,
	}, {
		name:   "rollback a release with wait-for-jobs",
		cmd:    "rollback funny-honey 1 --wait --wait-for-jobs",
		golden: "output/rollback-wait-for-jobs.txt",
		rels:   rels,
	}, {
		name:   "rollback a release without revision",
		cmd:    "rollback funny-honey",
		golden: "output/rollback-no-revision.txt",
		rels:   rels,
	}, {
		name:      "rollback a release without release name",
		cmd:       "rollback",
		golden:    "output/rollback-no-args.txt",
		rels:      rels,
		wantError: true,
	}}
	runTestCmd(t, tests)
}

func TestRollbackRevisionCompletion(t *testing.T) {
	mk := func(name string, vers int, status release.Status) *release.Release {
		return release.Mock(&release.MockReleaseOptions{
			Name:    name,
			Version: vers,
			Status:  status,
		})
	}

	releases := []*release.Release{
		mk("musketeers", 11, release.StatusDeployed),
		mk("musketeers", 10, release.StatusSuperseded),
		mk("musketeers", 9, release.StatusSuperseded),
		mk("musketeers", 8, release.StatusSuperseded),
		mk("carabins", 1, release.StatusSuperseded),
	}

	tests := []cmdTestCase{{
		name:   "completion for release parameter",
		cmd:    "__complete rollback ''",
		rels:   releases,
		golden: "output/rollback-comp.txt",
	}, {
		name:   "completion for revision parameter",
		cmd:    "__complete rollback musketeers ''",
		rels:   releases,
		golden: "output/revision-comp.txt",
	}, {
		name:   "completion for with too many args",
		cmd:    "__complete rollback musketeers 11 ''",
		rels:   releases,
		golden: "output/rollback-wrong-args-comp.txt",
	}}
	runTestCmd(t, tests)
}

func TestRollbackFileCompletion(t *testing.T) {
	checkFileCompletion(t, "rollback", false)
	checkFileCompletion(t, "rollback myrelease", false)
	checkFileCompletion(t, "rollback myrelease 1", false)
}
