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
	"fmt"
	"testing"
	"time"

	"k8s.io/helm/pkg/hapi/release"
)

func TestStatusCmd(t *testing.T) {
	tests := []releaseCase{
		{
			name:    "get status of a deployed release",
			cmd:     "status flummoxed-chickadee",
			matches: outputWithStatus("DEPLOYED"),
			rels: []*release.Release{
				releaseMockWithStatus(&release.Status{
					Code: release.Status_DEPLOYED,
				}),
			},
		},
		{
			name:    "get status of a deployed release with notes",
			cmd:     "status flummoxed-chickadee",
			matches: outputWithStatus("DEPLOYED\n\nNOTES:\nrelease notes\n"),
			rels: []*release.Release{
				releaseMockWithStatus(&release.Status{
					Code:  release.Status_DEPLOYED,
					Notes: "release notes",
				}),
			},
		},
		{
			name:    "get status of a deployed release with notes in json",
			cmd:     "status flummoxed-chickadee -o json",
			matches: `{"name":"flummoxed-chickadee","info":{"status":{"code":1,"notes":"release notes"},"first_deployed":(.*),"last_deployed":(.*)}}`,
			rels: []*release.Release{
				releaseMockWithStatus(&release.Status{
					Code:  release.Status_DEPLOYED,
					Notes: "release notes",
				}),
			},
		},
		{
			name:    "get status of a deployed release with resources",
			cmd:     "status flummoxed-chickadee",
			matches: outputWithStatus("DEPLOYED\n\nRESOURCES:\nresource A\nresource B\n\n"),
			rels: []*release.Release{
				releaseMockWithStatus(&release.Status{
					Code:      release.Status_DEPLOYED,
					Resources: "resource A\nresource B\n",
				}),
			},
		},
		{
			name:    "get status of a deployed release with resources in YAML",
			cmd:     "status flummoxed-chickadee -o yaml",
			matches: "info:\n (.*)first_deployed:\n (.*)seconds: 242085845\n (.*)last_deployed:\n (.*)seconds: 242085845\n (.*)status:\n code: 1\n (.*)resources: |\n (.*)resource A\n (.*)resource B\nname: flummoxed-chickadee\n",
			rels: []*release.Release{
				releaseMockWithStatus(&release.Status{
					Code:      release.Status_DEPLOYED,
					Resources: "resource A\nresource B\n",
				}),
			},
		},
		{
			name: "get status of a deployed release with test suite",
			cmd:  "status flummoxed-chickadee",
			matches: outputWithStatus(
				"DEPLOYED\n\nTEST SUITE:\nLast Started: (.*)\nLast Completed: (.*)\n\n" +
					"TEST      \tSTATUS (.*)\tINFO (.*)\tSTARTED (.*)\tCOMPLETED (.*)\n" +
					"test run 1\tSUCCESS (.*)\textra info\t(.*)\t(.*)\n" +
					"test run 2\tFAILURE (.*)\t (.*)\t(.*)\t(.*)\n"),
			rels: []*release.Release{
				releaseMockWithStatus(&release.Status{
					Code: release.Status_DEPLOYED,
					LastTestSuiteRun: &release.TestSuite{
						StartedAt:   time.Now(),
						CompletedAt: time.Now(),
						Results: []*release.TestRun{
							{
								Name:        "test run 1",
								Status:      release.TestRun_SUCCESS,
								Info:        "extra info",
								StartedAt:   time.Now(),
								CompletedAt: time.Now(),
							},
							{
								Name:        "test run 2",
								Status:      release.TestRun_FAILURE,
								StartedAt:   time.Now(),
								CompletedAt: time.Now(),
							},
						},
					},
				}),
			},
		},
	}
	testReleaseCmd(t, tests)
}

func outputWithStatus(status string) string {
	return fmt.Sprintf(`LAST DEPLOYED:(.*)\nNAMESPACE: \nSTATUS: %s`, status)
}

func releaseMockWithStatus(status *release.Status) *release.Release {
	return &release.Release{
		Name: "flummoxed-chickadee",
		Info: &release.Info{
			FirstDeployed: time.Now(),
			LastDeployed:  time.Now(),
			Status:        status,
		},
	}
}
