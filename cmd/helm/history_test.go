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
	"io"
	"testing"

	"github.com/spf13/cobra"

	"k8s.io/helm/pkg/helm"
	"k8s.io/helm/pkg/proto/hapi/chart"
	rpb "k8s.io/helm/pkg/proto/hapi/release"
)

func TestHistoryCmd(t *testing.T) {
	mk := func(name string, vers int32, code rpb.Status_Code, appVersion string) *rpb.Release {
		ch := &chart.Chart{
			Metadata: &chart.Metadata{
				Name:       "foo",
				Version:    "0.1.0-beta.1",
				AppVersion: appVersion,
			},
		}

		return helm.ReleaseMock(&helm.MockReleaseOptions{
			Name:       name,
			Chart:      ch,
			Version:    vers,
			StatusCode: code,
		})
	}

	releases := []*rpb.Release{
		mk("angry-bird", 4, rpb.Status_DEPLOYED, "1.4"),
		mk("angry-bird", 3, rpb.Status_SUPERSEDED, "1.3"),
		mk("angry-bird", 2, rpb.Status_SUPERSEDED, "1.2"),
		mk("angry-bird", 1, rpb.Status_SUPERSEDED, "1.1"),
	}

	tests := []releaseCase{
		{
			name:     "get history for release",
			args:     []string{"angry-bird"},
			rels:     releases,
			expected: "REVISION\tUPDATED                 \tSTATUS    \tCHART           \tAPP VERSION\tDESCRIPTION \n1       \t(.*)\tSUPERSEDED\tfoo-0.1.0-beta.1\t1.1        \tRelease mock\n2       \t(.*)\tSUPERSEDED\tfoo-0.1.0-beta.1\t1.2        \tRelease mock\n3       \t(.*)\tSUPERSEDED\tfoo-0.1.0-beta.1\t1.3        \tRelease mock\n4       \t(.*)\tDEPLOYED  \tfoo-0.1.0-beta.1\t1.4        \tRelease mock\n",
		},
		{
			name:     "get history with max limit set",
			args:     []string{"angry-bird"},
			flags:    []string{"--max", "2"},
			rels:     releases,
			expected: "REVISION\tUPDATED                 \tSTATUS    \tCHART           \tAPP VERSION\tDESCRIPTION \n3       \t(.*)\tSUPERSEDED\tfoo-0.1.0-beta.1\t1.3        \tRelease mock\n4       \t(.*)\tDEPLOYED  \tfoo-0.1.0-beta.1\t1.4        \tRelease mock\n",
		},
		{
			name:     "get history with yaml output format",
			args:     []string{"angry-bird"},
			flags:    []string{"--output", "yaml"},
			rels:     releases[:2],
			expected: "- appVersion: \"1.3\"\n  chart: foo-0.1.0-beta.1\n  description: Release mock\n  revision: 3\n  status: SUPERSEDED\n  updated: (.*)\n- appVersion: \"1.4\"\n  chart: foo-0.1.0-beta.1\n  description: Release mock\n  revision: 4\n  status: DEPLOYED\n  updated: (.*)\n\n",
		},
		{
			name:     "get history with json output format",
			args:     []string{"angry-bird"},
			flags:    []string{"--output", "json"},
			rels:     releases[:2],
			expected: `[{"revision":3,"updated":".*","status":"SUPERSEDED","chart":"foo\-0.1.0-beta.1","appVersion":"1.3","description":"Release mock"},{"revision":4,"updated":".*","status":"DEPLOYED","chart":"foo\-0.1.0-beta.1","appVersion":"1.4","description":"Release mock"}]\n`,
		},
	}

	runReleaseCases(t, tests, func(c *helm.FakeClient, out io.Writer) *cobra.Command {
		return newHistoryCmd(c, out)
	})
}
