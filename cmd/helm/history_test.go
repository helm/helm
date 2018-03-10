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
	"bytes"
	"regexp"
	"testing"

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

	tests := []struct {
		cmds string
		desc string
		args []string
		resp []*rpb.Release
		xout string
	}{
		{
			cmds: "helm history RELEASE_NAME",
			desc: "get history for release",
			args: []string{"angry-bird"},
			resp: []*rpb.Release{
				mk("angry-bird", 4, rpb.Status_DEPLOYED),
				mk("angry-bird", 3, rpb.Status_SUPERSEDED),
				mk("angry-bird", 2, rpb.Status_SUPERSEDED),
				mk("angry-bird", 1, rpb.Status_SUPERSEDED),
			},
			xout: "REVISION\tUPDATED                 \tSTATUS    \tCHART           \tDESCRIPTION \n1       \t(.*)\tSUPERSEDED\tfoo-0.1.0-beta.1\tRelease mock\n2       \t(.*)\tSUPERSEDED\tfoo-0.1.0-beta.1\tRelease mock\n3       \t(.*)\tSUPERSEDED\tfoo-0.1.0-beta.1\tRelease mock\n4       \t(.*)\tDEPLOYED  \tfoo-0.1.0-beta.1\tRelease mock\n",
		},
		{
			cmds: "helm history --max=MAX RELEASE_NAME",
			desc: "get history with max limit set",
			args: []string{"--max=2", "angry-bird"},
			resp: []*rpb.Release{
				mk("angry-bird", 4, rpb.Status_DEPLOYED),
				mk("angry-bird", 3, rpb.Status_SUPERSEDED),
			},
			xout: "REVISION\tUPDATED                 \tSTATUS    \tCHART           \tDESCRIPTION \n3       \t(.*)\tSUPERSEDED\tfoo-0.1.0-beta.1\tRelease mock\n4       \t(.*)\tDEPLOYED  \tfoo-0.1.0-beta.1\tRelease mock\n",
		},
		{
			cmds: "helm history --max=MAX RELEASE_NAME -o yaml",
			desc: "get history with yaml output format",
			args: []string{"--max=2", "-o=yaml", "angry-bird"},
			resp: []*rpb.Release{
				mk("angry-bird", 4, rpb.Status_DEPLOYED),
				mk("angry-bird", 3, rpb.Status_SUPERSEDED),
			},
			xout: "- chart: foo-0.1.0-beta.1\n  description: Release mock\n  revision: 3\n  status: SUPERSEDED\n  updated: (.*)\n- chart: foo-0.1.0-beta.1\n  description: Release mock\n  revision: 4\n  status: DEPLOYED\n  updated: (.*)\n\n",
		},
		{
			cmds: "helm history --max=MAX RELEASE_NAME -o json",
			desc: "get history with json output format",
			args: []string{"--max=2", "-o=json", "angry-bird"},
			resp: []*rpb.Release{
				mk("angry-bird", 4, rpb.Status_DEPLOYED),
				mk("angry-bird", 3, rpb.Status_SUPERSEDED),
			},
			xout: `[{"revision":3,"updated":".*","status":"SUPERSEDED","chart":"foo\-0.1.0-beta.1","description":"Release mock"},{"revision":4,"updated":".*","status":"DEPLOYED","chart":"foo\-0.1.0-beta.1","description":"Release mock"}]\n`,
		},
	}

	var buf bytes.Buffer
	for _, tt := range tests {
		frc := &helm.FakeClient{Rels: tt.resp}
		cmd := newHistoryCmd(frc, &buf)
		cmd.ParseFlags(tt.args)

		if err := cmd.RunE(cmd, tt.args); err != nil {
			t.Fatalf("%q\n\t%s: unexpected error: %v", tt.cmds, tt.desc, err)
		}
		re := regexp.MustCompile(tt.xout)
		if !re.Match(buf.Bytes()) {
			t.Fatalf("%q\n\t%s:\nexpected\n\t%q\nactual\n\t%q", tt.cmds, tt.desc, tt.xout, buf.String())
		}
		buf.Reset()
	}
}
