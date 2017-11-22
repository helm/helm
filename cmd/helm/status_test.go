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
	"bytes"
	"fmt"
	"io"
	"strings"
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

// statusCase describes a test case dealing with the status of a release
type statusCase struct {
	name     string
	args     []string
	flags    []string
	expected string
	err      bool
	rel      *release.Release
}

func TestStatusCmd(t *testing.T) {
	tests := []statusCase{
		{
			name:     "get status of a deployed release",
			args:     []string{"flummoxed-chickadee"},
			expected: outputWithStatus("DEPLOYED\n\n"),
			rel: releaseMockWithStatus(&release.Status{
				Code: release.Status_DEPLOYED,
			}),
		},
		{
			name:     "get status of a deployed release with notes",
			args:     []string{"flummoxed-chickadee"},
			expected: outputWithStatus("DEPLOYED\n\nNOTES:\nrelease notes\n"),
			rel: releaseMockWithStatus(&release.Status{
				Code:  release.Status_DEPLOYED,
				Notes: "release notes",
			}),
		},
		{
			name:     "get status of a deployed release with notes in json",
			args:     []string{"flummoxed-chickadee"},
			flags:    []string{"-o", "json"},
			expected: `{"name":"flummoxed-chickadee","info":{"status":{"code":1,"notes":"release notes"},"first_deployed":{"seconds":242085845},"last_deployed":{"seconds":242085845}}}`,
			rel: releaseMockWithStatus(&release.Status{
				Code:  release.Status_DEPLOYED,
				Notes: "release notes",
			}),
		},
		{
			name:     "get status of a deployed release with resources",
			args:     []string{"flummoxed-chickadee"},
			expected: outputWithStatus("DEPLOYED\n\nRESOURCES:\nresource A\nresource B\n\n"),
			rel: releaseMockWithStatus(&release.Status{
				Code:      release.Status_DEPLOYED,
				Resources: "resource A\nresource B\n",
			}),
		},
		{
			name:     "get status of a deployed release with resources in YAML",
			args:     []string{"flummoxed-chickadee"},
			flags:    []string{"-o", "yaml"},
			expected: "info:\nfirst_deployed:\nseconds:242085845\nlast_deployed:\nseconds:242085845\nstatus:\ncode:1\nresources:|\nresourceA\nresourceB\nname:flummoxed-chickadee\n",
			rel: releaseMockWithStatus(&release.Status{
				Code:      release.Status_DEPLOYED,
				Resources: "resource A\nresource B\n",
			}),
		},
		{
			name: "get status of a deployed release with test suite",
			args: []string{"flummoxed-chickadee"},
			expected: outputWithStatus(
				fmt.Sprintf("DEPLOYED\n\nTEST SUITE:\nLast Started: %s\nLast Completed: %s\n\n", dateString, dateString) +
					"TEST \tSTATUS \tINFO \tSTARTED \tCOMPLETED \n" +
					fmt.Sprintf("test run 1\tSUCCESS \textra info\t%s\t%s\n", dateString, dateString) +
					fmt.Sprintf("test run 2\tFAILURE \t \t%s\t%s\n", dateString, dateString)),
			rel: releaseMockWithStatus(&release.Status{
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
	}

	scmd := func(c *helm.FakeClient, out io.Writer) *cobra.Command {
		return newStatusCmd(c, out)
	}

	var buf bytes.Buffer
	for _, tt := range tests {
		c := &helm.FakeClient{
			Rels: []*release.Release{tt.rel},
		}
		cmd := scmd(c, &buf)
		cmd.ParseFlags(tt.flags)
		err := cmd.RunE(cmd, tt.args)
		if (err != nil) != tt.err {
			t.Errorf("%q. expected error, got '%v'", tt.name, err)
		}

		expected := strings.Replace(tt.expected, " ", "", -1)
		got := strings.Replace(buf.String(), " ", "", -1)
		if expected != got {
			t.Errorf("%q. expected\n%q\ngot\n%q", tt.name, expected, got)
		}
		buf.Reset()
	}
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
