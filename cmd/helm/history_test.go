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

	rpb "k8s.io/helm/pkg/hapi/release"
	"k8s.io/helm/pkg/helm"
)

func TestHistoryCmd(t *testing.T) {
	mk := func(name string, vers int, status rpb.ReleaseStatus) *rpb.Release {
		return helm.ReleaseMock(&helm.MockReleaseOptions{
			Name:    name,
			Version: vers,
			Status:  status,
		})
	}

	tests := []cmdTestCase{{
		name: "get history for release",
		cmd:  "history angry-bird",
		rels: []*rpb.Release{
			mk("angry-bird", 4, rpb.StatusDeployed),
			mk("angry-bird", 3, rpb.StatusSuperseded),
			mk("angry-bird", 2, rpb.StatusSuperseded),
			mk("angry-bird", 1, rpb.StatusSuperseded),
		},
		golden: "output/history.txt",
	}, {
		name: "get history with max limit set",
		cmd:  "history angry-bird --max 2",
		rels: []*rpb.Release{
			mk("angry-bird", 4, rpb.StatusDeployed),
			mk("angry-bird", 3, rpb.StatusSuperseded),
		},
		golden: "output/history-limit.txt",
	}, {
		name: "get history with yaml output format",
		cmd:  "history angry-bird --output yaml",
		rels: []*rpb.Release{
			mk("angry-bird", 4, rpb.StatusDeployed),
			mk("angry-bird", 3, rpb.StatusSuperseded),
		},
		golden: "output/history.yaml",
	}, {
		name: "get history with json output format",
		cmd:  "history angry-bird --output json",
		rels: []*rpb.Release{
			mk("angry-bird", 4, rpb.StatusDeployed),
			mk("angry-bird", 3, rpb.StatusSuperseded),
		},
		golden: "output/history.json",
	}}
	runTestCmd(t, tests)
}
