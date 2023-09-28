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

	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/release"
	helmtime "helm.sh/helm/v3/pkg/time"
)

func outputFlagCompletionTest(t *testing.T, cmdName string) {
	releasesMockWithStatus := func(info *release.Info, hooks ...*release.Hook) []*release.Release {
		info.LastDeployed = helmtime.Unix(1452902400, 0).UTC()
		return []*release.Release{{
			Name:      "athos",
			Namespace: "default",
			Info:      info,
			Chart:     &chart.Chart{},
			Hooks:     hooks,
		}, {
			Name:      "porthos",
			Namespace: "default",
			Info:      info,
			Chart:     &chart.Chart{},
			Hooks:     hooks,
		}, {
			Name:      "aramis",
			Namespace: "default",
			Info:      info,
			Chart:     &chart.Chart{},
			Hooks:     hooks,
		}, {
			Name:      "dartagnan",
			Namespace: "gascony",
			Info:      info,
			Chart:     &chart.Chart{},
			Hooks:     hooks,
		}}
	}

	tests := []cmdTestCase{{
		name:   "completion for output flag long and before arg",
		cmd:    fmt.Sprintf("__complete %s --output ''", cmdName),
		golden: "output/output-comp.txt",
		rels: releasesMockWithStatus(&release.Info{
			Status: release.StatusDeployed,
		}),
	}, {
		name:   "completion for output flag long and after arg",
		cmd:    fmt.Sprintf("__complete %s aramis --output ''", cmdName),
		golden: "output/output-comp.txt",
		rels: releasesMockWithStatus(&release.Info{
			Status: release.StatusDeployed,
		}),
	}, {
		name:   "completion for output flag short and before arg",
		cmd:    fmt.Sprintf("__complete %s -o ''", cmdName),
		golden: "output/output-comp.txt",
		rels: releasesMockWithStatus(&release.Info{
			Status: release.StatusDeployed,
		}),
	}, {
		name:   "completion for output flag short and after arg",
		cmd:    fmt.Sprintf("__complete %s aramis -o ''", cmdName),
		golden: "output/output-comp.txt",
		rels: releasesMockWithStatus(&release.Info{
			Status: release.StatusDeployed,
		}),
	}, {
		name:   "completion for output flag, no filter",
		cmd:    fmt.Sprintf("__complete %s --output jso", cmdName),
		golden: "output/output-comp.txt",
		rels: releasesMockWithStatus(&release.Info{
			Status: release.StatusDeployed,
		}),
	}}
	runTestCmd(t, tests)
}
