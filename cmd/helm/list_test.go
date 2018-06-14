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
	tests := []cmdTestCase{{
		name: "with a release",
		cmd:  "list",
		rels: []*release.Release{
			helm.ReleaseMock(&helm.MockReleaseOptions{Name: "thomas-guide"}),
		},
		golden: "output/list-with-release.txt",
	}, {
		name: "list",
		cmd:  "list",
		rels: []*release.Release{
			helm.ReleaseMock(&helm.MockReleaseOptions{Name: "atlas"}),
		},
		golden: "output/list.txt",
	}, {
		name: "list, one deployed, one failed",
		cmd:  "list -q",
		rels: []*release.Release{
			helm.ReleaseMock(&helm.MockReleaseOptions{Name: "thomas-guide", Status: release.StatusFailed}),
			helm.ReleaseMock(&helm.MockReleaseOptions{Name: "atlas-guide", Status: release.StatusDeployed}),
		},
		golden: "output/list-with-failed.txt",
	}, {
		name: "with a release, multiple flags",
		cmd:  "list --uninstalled --deployed --failed -q",
		rels: []*release.Release{
			helm.ReleaseMock(&helm.MockReleaseOptions{Name: "thomas-guide", Status: release.StatusUninstalled}),
			helm.ReleaseMock(&helm.MockReleaseOptions{Name: "atlas-guide", Status: release.StatusDeployed}),
		},
		// Note: We're really only testing that the flags parsed correctly. Which results are returned
		// depends on the backend. And until pkg/helm is done, we can't mock this.
		golden: "output/list-with-mulitple-flags.txt",
	}, {
		name: "with a release, multiple flags",
		cmd:  "list --all -q",
		rels: []*release.Release{
			helm.ReleaseMock(&helm.MockReleaseOptions{Name: "thomas-guide", Status: release.StatusUninstalled}),
			helm.ReleaseMock(&helm.MockReleaseOptions{Name: "atlas-guide", Status: release.StatusDeployed}),
		},
		// See note on previous test.
		golden: "output/list-with-mulitple-flags2.txt",
	}, {
		name: "with a release, multiple flags, deleting",
		cmd:  "list --all -q",
		rels: []*release.Release{
			helm.ReleaseMock(&helm.MockReleaseOptions{Name: "thomas-guide", Status: release.StatusUninstalling}),
			helm.ReleaseMock(&helm.MockReleaseOptions{Name: "atlas-guide", Status: release.StatusDeployed}),
		},
		// See note on previous test.
		golden: "output/list-with-mulitple-flags-deleting.txt",
	}, {
		name: "namespace defined, multiple flags",
		cmd:  "list --all -q --namespace test123",
		rels: []*release.Release{
			helm.ReleaseMock(&helm.MockReleaseOptions{Name: "thomas-guide", Namespace: "test123"}),
			helm.ReleaseMock(&helm.MockReleaseOptions{Name: "atlas-guide", Namespace: "test321"}),
		},
		// See note on previous test.
		golden: "output/list-with-mulitple-flags-namespaced.txt",
	}, {
		name: "with a pending release, multiple flags",
		cmd:  "list --all -q",
		rels: []*release.Release{
			helm.ReleaseMock(&helm.MockReleaseOptions{Name: "thomas-guide", Status: release.StatusPendingInstall}),
			helm.ReleaseMock(&helm.MockReleaseOptions{Name: "atlas-guide", Status: release.StatusDeployed}),
		},
		golden: "output/list-with-mulitple-flags-pending.txt",
	}, {
		name: "with a pending release, pending flag",
		cmd:  "list --pending -q",
		rels: []*release.Release{
			helm.ReleaseMock(&helm.MockReleaseOptions{Name: "thomas-guide", Status: release.StatusPendingInstall}),
			helm.ReleaseMock(&helm.MockReleaseOptions{Name: "wild-idea", Status: release.StatusPendingUpgrade}),
			helm.ReleaseMock(&helm.MockReleaseOptions{Name: "crazy-maps", Status: release.StatusPendingRollback}),
			helm.ReleaseMock(&helm.MockReleaseOptions{Name: "atlas-guide", Status: release.StatusDeployed}),
		},
		golden: "output/list-with-pending.txt",
	}, {
		name: "with old releases",
		cmd:  "list",
		rels: []*release.Release{
			helm.ReleaseMock(&helm.MockReleaseOptions{Name: "thomas-guide"}),
			helm.ReleaseMock(&helm.MockReleaseOptions{Name: "thomas-guide", Status: release.StatusFailed}),
		},
		golden: "output/list-with-old-releases.txt",
	}}

	runTestCmd(t, tests)
}
