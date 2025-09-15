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

package cmd

import (
	"testing"
	"time"

	chart "helm.sh/helm/v4/pkg/chart/v2"
	"helm.sh/helm/v4/pkg/release/common"
	release "helm.sh/helm/v4/pkg/release/v1"
)

func TestListCmd(t *testing.T) {
	defaultNamespace := "default"

	sampleTimeSeconds := int64(1452902400)
	timestamp1 := time.Unix(sampleTimeSeconds+1, 0).UTC()
	timestamp2 := time.Unix(sampleTimeSeconds+2, 0).UTC()
	timestamp3 := time.Unix(sampleTimeSeconds+3, 0).UTC()
	timestamp4 := time.Unix(sampleTimeSeconds+4, 0).UTC()
	chartInfo := &chart.Chart{
		Metadata: &chart.Metadata{
			Name:       "chickadee",
			Version:    "1.0.0",
			AppVersion: "0.0.1",
		},
	}

	releaseFixture := []*release.Release{
		{
			Name:      "starlord",
			Version:   1,
			Namespace: defaultNamespace,
			Info: &release.Info{
				LastDeployed: timestamp1,
				Status:       common.StatusSuperseded,
			},
			Chart: chartInfo,
		},
		{
			Name:      "starlord",
			Version:   2,
			Namespace: defaultNamespace,
			Info: &release.Info{
				LastDeployed: timestamp1,
				Status:       common.StatusDeployed,
			},
			Chart: chartInfo,
		},
		{
			Name:      "groot",
			Version:   1,
			Namespace: defaultNamespace,
			Info: &release.Info{
				LastDeployed: timestamp1,
				Status:       common.StatusUninstalled,
			},
			Chart: chartInfo,
		},
		{
			Name:      "gamora",
			Version:   1,
			Namespace: defaultNamespace,
			Info: &release.Info{
				LastDeployed: timestamp1,
				Status:       common.StatusSuperseded,
			},
			Chart: chartInfo,
		},
		{
			Name:      "rocket",
			Version:   1,
			Namespace: defaultNamespace,
			Info: &release.Info{
				LastDeployed: timestamp2,
				Status:       common.StatusFailed,
			},
			Chart: chartInfo,
		},
		{
			Name:      "drax",
			Version:   1,
			Namespace: defaultNamespace,
			Info: &release.Info{
				LastDeployed: timestamp1,
				Status:       common.StatusUninstalling,
			},
			Chart: chartInfo,
		},
		{
			Name:      "thanos",
			Version:   1,
			Namespace: defaultNamespace,
			Info: &release.Info{
				LastDeployed: timestamp1,
				Status:       common.StatusPendingInstall,
			},
			Chart: chartInfo,
		},
		{
			Name:      "hummingbird",
			Version:   1,
			Namespace: defaultNamespace,
			Info: &release.Info{
				LastDeployed: timestamp3,
				Status:       common.StatusDeployed,
			},
			Chart: chartInfo,
		},
		{
			Name:      "iguana",
			Version:   2,
			Namespace: defaultNamespace,
			Info: &release.Info{
				LastDeployed: timestamp4,
				Status:       common.StatusDeployed,
			},
			Chart: chartInfo,
		},
		{
			Name:      "starlord",
			Version:   2,
			Namespace: "milano",
			Info: &release.Info{
				LastDeployed: timestamp1,
				Status:       common.StatusDeployed,
			},
			Chart: chartInfo,
		},
	}

	tests := []cmdTestCase{{
		name:   "list releases",
		cmd:    "list",
		golden: "output/list-all.txt",
		rels:   releaseFixture,
	}, {
		name:   "list without headers",
		cmd:    "list --no-headers",
		golden: "output/list-all-no-headers.txt",
		rels:   releaseFixture,
	}, {
		name:   "list releases sorted by release date",
		cmd:    "list --date",
		golden: "output/list-all-date.txt",
		rels:   releaseFixture,
	}, {
		name:   "list failed releases",
		cmd:    "list --failed",
		golden: "output/list-failed.txt",
		rels:   releaseFixture,
	}, {
		name:   "list filtered releases",
		cmd:    "list --filter='.*'",
		golden: "output/list-all.txt",
		rels:   releaseFixture,
	}, {
		name:   "list releases, limited to one release",
		cmd:    "list --max 1",
		golden: "output/list-all-max.txt",
		rels:   releaseFixture,
	}, {
		name:   "list releases, offset by one",
		cmd:    "list --offset 1",
		golden: "output/list-all-offset.txt",
		rels:   releaseFixture,
	}, {
		name:   "list pending releases",
		cmd:    "list --pending",
		golden: "output/list-pending.txt",
		rels:   releaseFixture,
	}, {
		name:   "list releases in reverse order",
		cmd:    "list --reverse",
		golden: "output/list-all-reverse.txt",
		rels:   releaseFixture,
	}, {
		name:   "list releases sorted by reversed release date",
		cmd:    "list --date --reverse",
		golden: "output/list-all-date-reversed.txt",
		rels:   releaseFixture,
	}, {
		name:   "list releases in short output format",
		cmd:    "list --short",
		golden: "output/list-all-short.txt",
		rels:   releaseFixture,
	}, {
		name:   "list releases in short output format",
		cmd:    "list --short --output yaml",
		golden: "output/list-all-short-yaml.txt",
		rels:   releaseFixture,
	}, {
		name:   "list releases in short output format",
		cmd:    "list --short --output json",
		golden: "output/list-all-short-json.txt",
		rels:   releaseFixture,
	}, {
		name:   "list deployed and failed releases only",
		cmd:    "list --deployed --failed",
		golden: "output/list.txt",
		rels:   releaseFixture,
	}, {
		name:   "list superseded releases",
		cmd:    "list --superseded",
		golden: "output/list-superseded.txt",
		rels:   releaseFixture,
	}, {
		name:   "list uninstalled releases",
		cmd:    "list --uninstalled",
		golden: "output/list-uninstalled.txt",
		rels:   releaseFixture,
	}, {
		name:   "list releases currently uninstalling",
		cmd:    "list --uninstalling",
		golden: "output/list-uninstalling.txt",
		rels:   releaseFixture,
	}, {
		name:   "list releases in another namespace",
		cmd:    "list -n milano",
		golden: "output/list-namespace.txt",
		rels:   releaseFixture,
	}}
	runTestCmd(t, tests)
}

func TestListOutputCompletion(t *testing.T) {
	outputFlagCompletionTest(t, "list")
}

func TestListFileCompletion(t *testing.T) {
	checkFileCompletion(t, "list", false)
}
