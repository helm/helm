/*
Copyright 2016 The Kubernetes Authors All rights reserved.

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
	"io"
	"testing"

	"github.com/spf13/cobra"

	"k8s.io/helm/pkg/helm"
	rpb "k8s.io/helm/pkg/proto/hapi/release"
)

func TestHistoryCmd(t *testing.T) {
	mk := func(name string, vers int32, code rpb.Status_Code) *rpb.Release {
		return helm.ReleaseMock(&helm.MockReleaseOptions{
			Name:       name,
			Version:    vers,
			StatusCode: code,
		})
	}

	tests := []releaseCase{
		{
			name: "get history for release",
			args: []string{"angry-bird"},
			rels: []*rpb.Release{
				mk("angry-bird", 4, rpb.Status_DEPLOYED),
				mk("angry-bird", 3, rpb.Status_SUPERSEDED),
				mk("angry-bird", 2, rpb.Status_SUPERSEDED),
				mk("angry-bird", 1, rpb.Status_SUPERSEDED),
			},
			expected: "REVISION\tUPDATED                 \tSTATUS    \tCHART           \tDESCRIPTION \n1       \t(.*)\tSUPERSEDED\tfoo-0.1.0-beta.1\tRelease mock\n2       \t(.*)\tSUPERSEDED\tfoo-0.1.0-beta.1\tRelease mock\n3       \t(.*)\tSUPERSEDED\tfoo-0.1.0-beta.1\tRelease mock\n4       \t(.*)\tDEPLOYED  \tfoo-0.1.0-beta.1\tRelease mock\n",
		},
		{
			name:  "get history with max limit set",
			args:  []string{"angry-bird"},
			flags: []string{"--max", "2"},
			rels: []*rpb.Release{
				mk("angry-bird", 4, rpb.Status_DEPLOYED),
				mk("angry-bird", 3, rpb.Status_SUPERSEDED),
			},
			expected: "REVISION\tUPDATED                 \tSTATUS    \tCHART           \tDESCRIPTION \n3       \t(.*)\tSUPERSEDED\tfoo-0.1.0-beta.1\tRelease mock\n4       \t(.*)\tDEPLOYED  \tfoo-0.1.0-beta.1\tRelease mock\n",
		},
		{
			name:  "get history with yaml output format",
			args:  []string{"angry-bird"},
			flags: []string{"--output", "yaml"},
			rels: []*rpb.Release{
				mk("angry-bird", 4, rpb.Status_DEPLOYED),
				mk("angry-bird", 3, rpb.Status_SUPERSEDED),
			},
			expected: "- chart: foo-0.1.0-beta.1\n  description: Release mock\n  revision: 3\n  status: SUPERSEDED\n  updated: (.*)\n- chart: foo-0.1.0-beta.1\n  description: Release mock\n  revision: 4\n  status: DEPLOYED\n  updated: (.*)\n\n",
		},
		{
			name:  "get history with json output format",
			args:  []string{"angry-bird"},
			flags: []string{"--output", "json"},
			rels: []*rpb.Release{
				mk("angry-bird", 4, rpb.Status_DEPLOYED),
				mk("angry-bird", 3, rpb.Status_SUPERSEDED),
			},
			expected: `[{"revision":3,"updated":".*","status":"SUPERSEDED","chart":"foo\-0.1.0-beta.1","description":"Release mock"},{"revision":4,"updated":".*","status":"DEPLOYED","chart":"foo\-0.1.0-beta.1","description":"Release mock"}]\n`,
		},
	}

	runReleaseCases(t, tests, func(c *helm.FakeClient, out io.Writer) *cobra.Command {
		return newHistoryCmd(c, out)
	})
}
