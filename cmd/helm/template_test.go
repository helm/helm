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
	"path/filepath"
	"testing"
)

var chartPath = "./../../pkg/chartutil/testdata/subpop/charts/subchart1"

func TestTemplateCmd(t *testing.T) {
	absChartPath, err := filepath.Abs(chartPath)
	if err != nil {
		t.Fatal(err)
	}
	tests := []cmdTestCase{
		{
			name:   "check name",
			cmd:    fmt.Sprintf("template '%s'", chartPath),
			golden: "output/template.txt",
		},
		{
			name:   "check set name",
			cmd:    fmt.Sprintf("template '%s' -x '%s' --set service.name=apache", chartPath, filepath.Join("templates", "service.yaml")),
			golden: "output/template-set.txt",
		},
		{
			name:   "check execute absolute",
			cmd:    fmt.Sprintf("template '%s' -x '%s' --set service.name=apache", chartPath, filepath.Join(absChartPath, "templates", "service.yaml")),
			golden: "output/template-absolute.txt",
		},
		{
			name:   "check release name",
			cmd:    fmt.Sprintf("template '%s' --name test", chartPath),
			golden: "output/template-name.txt",
		},
		{
			name:   "check notes",
			cmd:    fmt.Sprintf("template '%s' --notes", chartPath),
			golden: "output/template-notes.txt",
		},
		{
			name:   "check values files",
			cmd:    fmt.Sprintf("template '%s' --values '%s'", chartPath, filepath.Join(chartPath, "/charts/subchartA/values.yaml")),
			golden: "output/template-values-files.txt",
		},
		{
			name:   "check name template",
			cmd:    fmt.Sprintf(`template '%s' --name-template='foobar-{{ b64enc "abc" }}-baz'`, chartPath),
			golden: "output/template-name-template.txt",
		},
		{
			name:   "check kube version",
			cmd:    fmt.Sprintf("template '%s' --kube-version 1.6", chartPath),
			golden: "output/template-kube-version.txt",
		},
		{
			name:      "check no args",
			cmd:       "template",
			wantError: true,
			golden:    "output/template-no-args.txt",
		},
		{
			name:      "check library chart",
			cmd:       fmt.Sprintf("template '%s'", "testdata/testcharts/lib-chart"),
			wantError: true,
			golden:    "output/template-lib-chart.txt",
		},
		{
			name:      "check chart bad type",
			cmd:       fmt.Sprintf("template '%s'", "testdata/testcharts/chart-bad-type"),
			wantError: true,
			golden:    "output/template-chart-bad-type.txt",
		},
	}
	runTestCmd(t, tests)
}
