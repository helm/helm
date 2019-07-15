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

	"helm.sh/helm/pkg/chart"
	"helm.sh/helm/pkg/release"
)

func TestRollbackCmd(t *testing.T) {

	relMock := func(n string, v int, ch *chart.Chart) *release.Release {
		return release.Mock(&release.MockReleaseOptions{Name: n, Version: v, Chart: ch})
	}

	ch := &chart.Chart{
		Metadata: &chart.Metadata{},
	}

	tests := []cmdTestCase{
		{
			name:   "rollback a release",
			cmd:    "rollback funny-honey 1",
			golden: "output/rollback.txt",
			rels: []*release.Release{
				relMock("funny-honey", 1, ch),
				relMock("funny-honey", 2, ch),
			},
		}, {
			name:   "rollback a release with timeout",
			cmd:    "rollback funny-honey 1 --timeout 120s",
			golden: "output/rollback-timeout.txt",
			rels: []*release.Release{
				relMock("funny-honey", 1, ch),
				relMock("funny-honey", 2, ch),
			},
		}, {
			name:   "rollback a release with wait",
			cmd:    "rollback funny-honey 1 --wait",
			golden: "output/rollback-wait.txt",
			rels: []*release.Release{
				relMock("funny-honey", 1, ch),
				relMock("funny-honey", 2, ch),
			},
		}, {
			name:   "rollback a release without revision",
			cmd:    "rollback funny-honey",
			golden: "output/rollback-no-args.txt",
			rels: []*release.Release{
				relMock("funny-honey", 1, ch),
				relMock("funny-honey", 2, ch),
			},
			wantError: true,
		},
	}
	runTestCmd(t, tests)
}
