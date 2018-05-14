/*
Copyright 2017 The Kubernetes Authors All rights reserved.

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

	"k8s.io/helm/pkg/hapi/release"
)

func TestStatusCmd(t *testing.T) {
	releasesMockWithStatus := func(info *release.Info) []*release.Release {
		info.LastDeployed = time.Unix(1452902400, 0)
		return []*release.Release{{
			Name: "flummoxed-chickadee",
			Info: info,
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
		name:   "get status of a deployed release with resources",
		cmd:    "status flummoxed-chickadee",
		golden: "output/status-with-resource.txt",
		rels: releasesMockWithStatus(&release.Info{
			Status:    release.StatusDeployed,
			Resources: "resource A\nresource B\n",
		}),
	}, {
		name:   "get status of a deployed release with resources in YAML",
		cmd:    "status flummoxed-chickadee -o yaml",
		golden: "output/status.yaml",
		rels: releasesMockWithStatus(&release.Info{
			Status:    release.StatusDeployed,
			Resources: "resource A\nresource B\n",
		}),
	}, {
		name:   "get status of a deployed release with test suite",
		cmd:    "status flummoxed-chickadee",
		golden: "output/status-with-test-suite.txt",
		rels: releasesMockWithStatus(&release.Info{
			Status: release.StatusDeployed,
			LastTestSuiteRun: &release.TestSuite{
				Results: []*release.TestRun{{
					Name:   "test run 1",
					Status: release.TestRunSuccess,
					Info:   "extra info",
				}, {
					Name:   "test run 2",
					Status: release.TestRunFailure,
				}},
			},
		}),
	}}
	runTestCmd(t, tests)
}
