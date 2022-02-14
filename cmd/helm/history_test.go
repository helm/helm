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
	"fmt"
	"testing"

	"helm.sh/helm/v3/pkg/release"
)

func TestHistoryCmd(t *testing.T) {
	mk := func(name string, vers int, status release.Status) *release.Release {
		return release.Mock(&release.MockReleaseOptions{
			Name:    name,
			Version: vers,
			Status:  status,
		})
	}

	tests := []cmdTestCase{{
		name: "get history for release",
		cmd:  "history angry-bird",
		rels: []*release.Release{
			mk("angry-bird", 4, release.StatusDeployed),
			mk("angry-bird", 3, release.StatusSuperseded),
			mk("angry-bird", 2, release.StatusSuperseded),
			mk("angry-bird", 1, release.StatusSuperseded),
		},
		golden: "output/history.txt",
	}, {
		name: "get history with max limit set",
		cmd:  "history angry-bird --max 2",
		rels: []*release.Release{
			mk("angry-bird", 4, release.StatusDeployed),
			mk("angry-bird", 3, release.StatusSuperseded),
		},
		golden: "output/history-limit.txt",
	}, {
		name: "get history with yaml output format",
		cmd:  "history angry-bird --output yaml",
		rels: []*release.Release{
			mk("angry-bird", 4, release.StatusDeployed),
			mk("angry-bird", 3, release.StatusSuperseded),
		},
		golden: "output/history.yaml",
	}, {
		name: "get history with json output format",
		cmd:  "history angry-bird --output json",
		rels: []*release.Release{
			mk("angry-bird", 4, release.StatusDeployed),
			mk("angry-bird", 3, release.StatusSuperseded),
		},
		golden: "output/history.json",
	}}
	runTestCmd(t, tests)
}

func TestHistoryOutputCompletion(t *testing.T) {
	outputFlagCompletionTest(t, "history")
}

func revisionFlagCompletionTest(t *testing.T, cmdName string) {
	mk := func(name string, vers int, status release.Status) *release.Release {
		return release.Mock(&release.MockReleaseOptions{
			Name:    name,
			Version: vers,
			Status:  status,
		})
	}

	releases := []*release.Release{
		mk("musketeers", 11, release.StatusDeployed),
		mk("musketeers", 10, release.StatusSuperseded),
		mk("musketeers", 9, release.StatusSuperseded),
		mk("musketeers", 8, release.StatusSuperseded),
	}

	tests := []cmdTestCase{{
		name:   "completion for revision flag",
		cmd:    fmt.Sprintf("__complete %s musketeers --revision ''", cmdName),
		rels:   releases,
		golden: "output/revision-comp.txt",
	}, {
		name:   "completion for revision flag, no filter",
		cmd:    fmt.Sprintf("__complete %s musketeers --revision 1", cmdName),
		rels:   releases,
		golden: "output/revision-comp.txt",
	}, {
		name:   "completion for revision flag with too few args",
		cmd:    fmt.Sprintf("__complete %s --revision ''", cmdName),
		rels:   releases,
		golden: "output/revision-wrong-args-comp.txt",
	}, {
		name:   "completion for revision flag with too many args",
		cmd:    fmt.Sprintf("__complete %s three musketeers --revision ''", cmdName),
		rels:   releases,
		golden: "output/revision-wrong-args-comp.txt",
	}}
	runTestCmd(t, tests)
}

func TestHistoryCompletion(t *testing.T) {
	checkReleaseCompletion(t, "history", false)
}

func TestHistoryFileCompletion(t *testing.T) {
	checkFileCompletion(t, "history", false)
	checkFileCompletion(t, "history myrelease", false)
}
