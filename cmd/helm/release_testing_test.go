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

	"k8s.io/helm/pkg/release"
)

func TestReleaseTesting(t *testing.T) {
	timestamp := time.Unix(1452902400, 0).UTC()

	tests := []cmdTestCase{{
		name: "successful test",
		cmd:  "status test-success",
		rels: []*release.Release{release.Mock(&release.MockReleaseOptions{
			Name: "test-success",
			TestSuiteResults: []*release.TestRun{
				{
					Name:        "test-success",
					Status:      release.TestRunSuccess,
					StartedAt:   timestamp,
					CompletedAt: timestamp,
				},
			},
		})},
		golden: "output/test-success.txt",
	}, {
		name: "test failure",
		cmd:  "status test-failure",
		rels: []*release.Release{release.Mock(&release.MockReleaseOptions{
			Name: "test-failure",
			TestSuiteResults: []*release.TestRun{
				{
					Name:        "test-failure",
					Status:      release.TestRunFailure,
					StartedAt:   timestamp,
					CompletedAt: timestamp,
				},
			},
		})},
		golden: "output/test-failure.txt",
	}, {
		name: "test unknown",
		cmd:  "status test-unknown",
		rels: []*release.Release{release.Mock(&release.MockReleaseOptions{
			Name: "test-unknown",
			TestSuiteResults: []*release.TestRun{
				{
					Name:        "test-unknown",
					Status:      release.TestRunUnknown,
					StartedAt:   timestamp,
					CompletedAt: timestamp,
				},
			},
		})},
		golden: "output/test-unknown.txt",
	}, {
		name: "test running",
		cmd:  "status test-running",
		rels: []*release.Release{release.Mock(&release.MockReleaseOptions{
			Name: "test-running",
			TestSuiteResults: []*release.TestRun{
				{
					Name:        "test-running",
					Status:      release.TestRunRunning,
					StartedAt:   timestamp,
					CompletedAt: timestamp,
				},
			},
		})},
		golden: "output/test-running.txt",
	}, {
		name:   "test with no tests",
		cmd:    "test no-tests",
		rels:   []*release.Release{release.Mock(&release.MockReleaseOptions{Name: "no-tests"})},
		golden: "output/test-no-tests.txt",
	}}
	runTestCmd(t, tests)
}
