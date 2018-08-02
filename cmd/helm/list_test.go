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
	"regexp"
	"testing"

	"github.com/spf13/cobra"

	"io/ioutil"
	"os"

	"k8s.io/helm/pkg/chartutil"
	"k8s.io/helm/pkg/helm"
	"k8s.io/helm/pkg/proto/hapi/chart"
	"k8s.io/helm/pkg/proto/hapi/release"
)

func TestListCmd(t *testing.T) {
	tmpChart, _ := ioutil.TempDir("testdata", "tmp")
	defer os.RemoveAll(tmpChart)
	cfile := &chart.Metadata{
		Name:        "foo",
		Description: "A Helm chart for Kubernetes",
		Version:     "0.1.0-beta.1",
		AppVersion:  "2.X.A",
	}
	chartPath, err := chartutil.Create(cfile, tmpChart)
	if err != nil {
		t.Errorf("Error creating chart for list: %v", err)
	}
	ch, _ := chartutil.Load(chartPath)

	tests := []releaseCase{
		{
			name:     "empty",
			rels:     []*release.Release{},
			expected: "",
		},
		{
			name: "with a release",
			rels: []*release.Release{
				helm.ReleaseMock(&helm.MockReleaseOptions{Name: "thomas-guide"}),
			},
			expected: "thomas-guide",
		},
		{
			name: "list",
			rels: []*release.Release{
				helm.ReleaseMock(&helm.MockReleaseOptions{Name: "atlas"}),
			},
			expected: "NAME \tREVISION\tUPDATED                 \tSTATUS  \tCHART           \tAPP VERSION\tNAMESPACE\natlas\t1       \t(.*)\tDEPLOYED\tfoo-0.1.0-beta.1\t           \tdefault  \n",
		},
		{
			name: "list with appVersion",
			rels: []*release.Release{
				helm.ReleaseMock(&helm.MockReleaseOptions{Name: "atlas", Chart: ch}),
			},
			expected: "NAME \tREVISION\tUPDATED                 \tSTATUS  \tCHART           \tAPP VERSION\tNAMESPACE\natlas\t1       \t(.*)\tDEPLOYED\tfoo-0.1.0-beta.1\t2.X.A      \tdefault  \n",
		},
		{
			name:  "with json output",
			flags: []string{"--max", "1", "--output", "json"},
			rels: []*release.Release{
				helm.ReleaseMock(&helm.MockReleaseOptions{Name: "thomas-guide"}),
				helm.ReleaseMock(&helm.MockReleaseOptions{Name: "atlas-guide"}),
			},
			expected: regexp.QuoteMeta(`{"Next":"atlas-guide","Releases":[{"Name":"thomas-guide","Revision":1,"Updated":"`) + `([^"]*)` + regexp.QuoteMeta(`","Status":"DEPLOYED","Chart":"foo-0.1.0-beta.1","AppVersion":"","Namespace":"default"}]}
`),
		},
		{
			name:  "with yaml output",
			flags: []string{"--max", "1", "--output", "yaml"},
			rels: []*release.Release{
				helm.ReleaseMock(&helm.MockReleaseOptions{Name: "thomas-guide"}),
				helm.ReleaseMock(&helm.MockReleaseOptions{Name: "atlas-guide"}),
			},
			expected: regexp.QuoteMeta(`Next: atlas-guide
Releases:
- AppVersion: ""
  Chart: foo-0.1.0-beta.1
  Name: thomas-guide
  Namespace: default
  Revision: 1
  Status: DEPLOYED
  Updated: `) + `(.*)` + `

`,
		},
		{
			name:  "with short json output",
			flags: []string{"-q", "--output", "json"},
			rels: []*release.Release{
				helm.ReleaseMock(&helm.MockReleaseOptions{Name: "atlas"}),
			},
			expected: regexp.QuoteMeta(`["atlas"]
`),
		},
		{
			name:  "with short yaml output",
			flags: []string{"-q", "--output", "yaml"},
			rels: []*release.Release{
				helm.ReleaseMock(&helm.MockReleaseOptions{Name: "atlas"}),
			},
			expected: regexp.QuoteMeta(`- atlas

`),
		},
		{
			name:  "with json output without next",
			flags: []string{"--output", "json"},
			rels:  []*release.Release{},
			expected: regexp.QuoteMeta(`{"Next":"","Releases":[]}
`),
		},
		{
			name:  "with yaml output without next",
			flags: []string{"--output", "yaml"},
			rels:  []*release.Release{},
			expected: regexp.QuoteMeta(`Next: ""
Releases: []

`),
		},
		{
			name:     "with unknown output format",
			flags:    []string{"--output", "_unknown_"},
			rels:     []*release.Release{},
			err:      true,
			expected: regexp.QuoteMeta(``),
		},
		{
			name:  "list, one deployed, one failed",
			flags: []string{"-q"},
			rels: []*release.Release{
				helm.ReleaseMock(&helm.MockReleaseOptions{Name: "thomas-guide", StatusCode: release.Status_FAILED}),
				helm.ReleaseMock(&helm.MockReleaseOptions{Name: "atlas-guide", StatusCode: release.Status_DEPLOYED}),
			},
			expected: "thomas-guide\natlas-guide",
		},
		{
			name:  "with a release, multiple flags",
			flags: []string{"--deleted", "--deployed", "--failed", "-q"},
			rels: []*release.Release{
				helm.ReleaseMock(&helm.MockReleaseOptions{Name: "thomas-guide", StatusCode: release.Status_DELETED}),
				helm.ReleaseMock(&helm.MockReleaseOptions{Name: "atlas-guide", StatusCode: release.Status_DEPLOYED}),
			},
			// Note: We're really only testing that the flags parsed correctly. Which results are returned
			// depends on the backend. And until pkg/helm is done, we can't mock this.
			expected: "thomas-guide\natlas-guide",
		},
		{
			name:  "with a release, multiple flags",
			flags: []string{"--all", "-q"},
			rels: []*release.Release{
				helm.ReleaseMock(&helm.MockReleaseOptions{Name: "thomas-guide", StatusCode: release.Status_DELETED}),
				helm.ReleaseMock(&helm.MockReleaseOptions{Name: "atlas-guide", StatusCode: release.Status_DEPLOYED}),
			},
			// See note on previous test.
			expected: "thomas-guide\natlas-guide",
		},
		{
			name:  "with a release, multiple flags, deleting",
			flags: []string{"--all", "-q"},
			rels: []*release.Release{
				helm.ReleaseMock(&helm.MockReleaseOptions{Name: "thomas-guide", StatusCode: release.Status_DELETING}),
				helm.ReleaseMock(&helm.MockReleaseOptions{Name: "atlas-guide", StatusCode: release.Status_DEPLOYED}),
			},
			// See note on previous test.
			expected: "thomas-guide\natlas-guide",
		},
		{
			name:  "namespace defined, multiple flags",
			flags: []string{"--all", "-q", "--namespace test123"},
			rels: []*release.Release{
				helm.ReleaseMock(&helm.MockReleaseOptions{Name: "thomas-guide", Namespace: "test123"}),
				helm.ReleaseMock(&helm.MockReleaseOptions{Name: "atlas-guide", Namespace: "test321"}),
			},
			// See note on previous test.
			expected: "thomas-guide",
		},
		{
			name:  "with a pending release, multiple flags",
			flags: []string{"--all", "-q"},
			rels: []*release.Release{
				helm.ReleaseMock(&helm.MockReleaseOptions{Name: "thomas-guide", StatusCode: release.Status_PENDING_INSTALL}),
				helm.ReleaseMock(&helm.MockReleaseOptions{Name: "atlas-guide", StatusCode: release.Status_DEPLOYED}),
			},
			expected: "thomas-guide\natlas-guide",
		},
		{
			name:  "with a pending release, pending flag",
			flags: []string{"--pending", "-q"},
			rels: []*release.Release{
				helm.ReleaseMock(&helm.MockReleaseOptions{Name: "thomas-guide", StatusCode: release.Status_PENDING_INSTALL}),
				helm.ReleaseMock(&helm.MockReleaseOptions{Name: "wild-idea", StatusCode: release.Status_PENDING_UPGRADE}),
				helm.ReleaseMock(&helm.MockReleaseOptions{Name: "crazy-maps", StatusCode: release.Status_PENDING_ROLLBACK}),
				helm.ReleaseMock(&helm.MockReleaseOptions{Name: "atlas-guide", StatusCode: release.Status_DEPLOYED}),
			},
			expected: "thomas-guide\nwild-idea\ncrazy-maps",
		},
		{
			name: "with old releases",
			rels: []*release.Release{
				helm.ReleaseMock(&helm.MockReleaseOptions{Name: "thomas-guide"}),
				helm.ReleaseMock(&helm.MockReleaseOptions{Name: "thomas-guide", StatusCode: release.Status_FAILED}),
			},
			expected: "thomas-guide",
		},
	}

	runReleaseCases(t, tests, func(c *helm.FakeClient, out io.Writer) *cobra.Command {
		return newListCmd(c, out)
	})
}
