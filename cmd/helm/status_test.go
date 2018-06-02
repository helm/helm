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
	"io"
	"testing"

	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/spf13/cobra"

	"k8s.io/helm/pkg/helm"
	"k8s.io/helm/pkg/proto/hapi/release"
	"k8s.io/helm/pkg/timeconv"
)

var (
	date       = timestamp.Timestamp{Seconds: 242085845, Nanos: 0}
	dateString = timeconv.String(&date)
)

func TestStatusCmd(t *testing.T) {
	tests := []releaseCase{
		{
			name:     "get status of a deployed release",
			args:     []string{"flummoxed-chickadee"},
			expected: outputWithStatus("DEPLOYED\n\n"),
			rels: []*release.Release{
				releaseMockWithStatus(&release.Status{
					Code: release.Status_DEPLOYED,
				}),
			},
		},
		{
			name:     "get status of a deployed release with notes",
			args:     []string{"flummoxed-chickadee"},
			expected: outputWithStatus("DEPLOYED\n\nNOTES:\nrelease notes\n"),
			rels: []*release.Release{
				releaseMockWithStatus(&release.Status{
					Code:  release.Status_DEPLOYED,
					Notes: "release notes",
				}),
			},
		},
		{
			name:     "get status of a deployed release with notes in json",
			args:     []string{"flummoxed-chickadee"},
			flags:    []string{"-o", "json"},
			expected: `{"name":"flummoxed-chickadee","info":{"status":{"code":1,"notes":"release notes"},"first_deployed":{"seconds":242085845},"last_deployed":{"seconds":242085845}}}`,
			rels: []*release.Release{
				releaseMockWithStatus(&release.Status{
					Code:  release.Status_DEPLOYED,
					Notes: "release notes",
				}),
			},
		},
		{
			name:     "get status of a deployed release with resources",
			args:     []string{"flummoxed-chickadee"},
			expected: outputWithStatus("DEPLOYED\n\nRESOURCES:\nresource A\nresource B\n\n"),
			rels: []*release.Release{
				releaseMockWithStatus(&release.Status{
					Code:      release.Status_DEPLOYED,
					Resources: "resource A\nresource B\n",
				}),
			},
		},
		{
			name:     "get status of a deployed release with resources in YAML",
			args:     []string{"flummoxed-chickadee"},
			flags:    []string{"-o", "yaml"},
			expected: "info:\n (.*)first_deployed:\n (.*)seconds: 242085845\n (.*)last_deployed:\n (.*)seconds: 242085845\n (.*)status:\n code: 1\n (.*)resources: |\n (.*)resource A\n (.*)resource B\nname: flummoxed-chickadee\n",
			rels: []*release.Release{
				releaseMockWithStatus(&release.Status{
					Code:      release.Status_DEPLOYED,
					Resources: "resource A\nresource B\n",
				}),
			},
		},
		{
			name: "get status of a deployed release with test suite",
			args: []string{"flummoxed-chickadee"},
			expected: outputWithStatus(
				fmt.Sprintf("DEPLOYED\n\nTEST SUITE:\nLast Started: %s\nLast Completed: %s\n\n", dateString, dateString) +
					"TEST      \tSTATUS (.*)\tINFO (.*)\tSTARTED (.*)\tCOMPLETED (.*)\n" +
					fmt.Sprintf("test run 1\tSUCCESS (.*)\textra info\t%s\t%s\n", dateString, dateString) +
					fmt.Sprintf("test run 2\tFAILURE (.*)\t (.*)\t%s\t%s\n", dateString, dateString)),
			rels: []*release.Release{
				releaseMockWithStatus(&release.Status{
					Code: release.Status_DEPLOYED,
					LastTestSuiteRun: &release.TestSuite{
						StartedAt:   &date,
						CompletedAt: &date,
						Results: []*release.TestRun{
							{
								Name:        "test run 1",
								Status:      release.TestRun_SUCCESS,
								Info:        "extra info",
								StartedAt:   &date,
								CompletedAt: &date,
							},
							{
								Name:        "test run 2",
								Status:      release.TestRun_FAILURE,
								StartedAt:   &date,
								CompletedAt: &date,
							},
						},
					},
				}),
			},
		},
	}

	runReleaseCases(t, tests, func(c *helm.FakeClient, out io.Writer) *cobra.Command {
		return newStatusCmd(c, out)
	})

}

func outputWithStatus(status string) string {
	return fmt.Sprintf("LAST DEPLOYED: %s\nNAMESPACE: \nSTATUS: %s",
		dateString,
		status)
}

func releaseMockWithStatus(status *release.Status) *release.Release {
	return &release.Release{
		Name: "flummoxed-chickadee",
		Info: &release.Info{
			FirstDeployed: &date,
			LastDeployed:  &date,
			Status:        status,
		},
	}
}
