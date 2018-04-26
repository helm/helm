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
	"testing"

	"k8s.io/helm/pkg/hapi/release"
	"k8s.io/helm/pkg/helm"
)

func TestListCmd(t *testing.T) {
	tests := []releaseCase{
		{
			name: "with a release",
			cmd:  "list",
			rels: []*release.Release{
				helm.ReleaseMock(&helm.MockReleaseOptions{Name: "thomas-guide"}),
			},
			matches: "thomas-guide",
		},
		{
			name: "list",
			cmd:  "list",
			rels: []*release.Release{
				helm.ReleaseMock(&helm.MockReleaseOptions{Name: "atlas"}),
			},
			matches: `NAME\s+REVISION\s+UPDATED\s+STATUS\s+CHART\s+NAMESPACE\natlas\s+1\s+(.*)\s+deployed\s+foo-0.1.0-beta.1\s+default`,
		},
		{
			name: "list, one deployed, one failed",
			cmd:  "list -q",
			rels: []*release.Release{
				helm.ReleaseMock(&helm.MockReleaseOptions{Name: "thomas-guide", Status: release.StatusFailed}),
				helm.ReleaseMock(&helm.MockReleaseOptions{Name: "atlas-guide", Status: release.StatusDeployed}),
			},
			matches: "thomas-guide\natlas-guide",
		},
		{
			name: "with a release, multiple flags",
			cmd:  "list --deleted --deployed --failed -q",
			rels: []*release.Release{
				helm.ReleaseMock(&helm.MockReleaseOptions{Name: "thomas-guide", Status: release.StatusDeleted}),
				helm.ReleaseMock(&helm.MockReleaseOptions{Name: "atlas-guide", Status: release.StatusDeployed}),
			},
			// Note: We're really only testing that the flags parsed correctly. Which results are returned
			// depends on the backend. And until pkg/helm is done, we can't mock this.
			matches: "thomas-guide\natlas-guide",
		},
		{
			name: "with a release, multiple flags",
			cmd:  "list --all -q",
			rels: []*release.Release{
				helm.ReleaseMock(&helm.MockReleaseOptions{Name: "thomas-guide", Status: release.StatusDeleted}),
				helm.ReleaseMock(&helm.MockReleaseOptions{Name: "atlas-guide", Status: release.StatusDeployed}),
			},
			// See note on previous test.
			matches: "thomas-guide\natlas-guide",
		},
		{
			name: "with a release, multiple flags, deleting",
			cmd:  "list --all -q",
			rels: []*release.Release{
				helm.ReleaseMock(&helm.MockReleaseOptions{Name: "thomas-guide", Status: release.StatusDeleting}),
				helm.ReleaseMock(&helm.MockReleaseOptions{Name: "atlas-guide", Status: release.StatusDeployed}),
			},
			// See note on previous test.
			matches: "thomas-guide\natlas-guide",
		},
		{
			name: "namespace defined, multiple flags",
			cmd:  "list --all -q --namespace test123",
			rels: []*release.Release{
				helm.ReleaseMock(&helm.MockReleaseOptions{Name: "thomas-guide", Namespace: "test123"}),
				helm.ReleaseMock(&helm.MockReleaseOptions{Name: "atlas-guide", Namespace: "test321"}),
			},
			// See note on previous test.
			matches: "thomas-guide",
		},
		{
			name: "with a pending release, multiple flags",
			cmd:  "list --all -q",
			rels: []*release.Release{
				helm.ReleaseMock(&helm.MockReleaseOptions{Name: "thomas-guide", Status: release.StatusPendingInstall}),
				helm.ReleaseMock(&helm.MockReleaseOptions{Name: "atlas-guide", Status: release.StatusDeployed}),
			},
			matches: "thomas-guide\natlas-guide",
		},
		{
			name: "with a pending release, pending flag",
			cmd:  "list --pending -q",
			rels: []*release.Release{
				helm.ReleaseMock(&helm.MockReleaseOptions{Name: "thomas-guide", Status: release.StatusPendingInstall}),
				helm.ReleaseMock(&helm.MockReleaseOptions{Name: "wild-idea", Status: release.StatusPendingUpgrade}),
				helm.ReleaseMock(&helm.MockReleaseOptions{Name: "crazy-maps", Status: release.StatusPendingRollback}),
				helm.ReleaseMock(&helm.MockReleaseOptions{Name: "atlas-guide", Status: release.StatusDeployed}),
			},
			matches: "thomas-guide\nwild-idea\ncrazy-maps",
		},
		{
			name: "with old releases",
			cmd:  "list",
			rels: []*release.Release{
				helm.ReleaseMock(&helm.MockReleaseOptions{Name: "thomas-guide"}),
				helm.ReleaseMock(&helm.MockReleaseOptions{Name: "thomas-guide", Status: release.StatusFailed}),
			},
			matches: "thomas-guide",
		},
	}

	testReleaseCmd(t, tests)
}
