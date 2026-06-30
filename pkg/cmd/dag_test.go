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
	"fmt"
	"strings"
	"testing"
)

func TestDagCmd(t *testing.T) {
	chartPath := "testdata/testcharts/sequenced-chart"

	tests := []cmdTestCase{
		{
			name:   "sequenced chart prints subchart and resource-group batches",
			cmd:    fmt.Sprintf("dag '%s'", chartPath),
			golden: "output/dag-sequenced-chart.txt",
		},
	}
	runTestCmd(t, tests)
}

func TestDagCmd_RequiresChartArg(t *testing.T) {
	_, _, err := executeActionCommandC(storageFixture(), "dag")
	if err == nil {
		t.Fatal("expected error when chart argument is missing")
	}
}

func TestDagCmd_NonexistentChart(t *testing.T) {
	_, _, err := executeActionCommandC(storageFixture(), "dag testdata/testcharts/does-not-exist")
	if err == nil {
		t.Fatal("expected error for nonexistent chart path")
	}
	if !strings.Contains(err.Error(), "does-not-exist") &&
		!strings.Contains(strings.ToLower(err.Error()), "no such file") &&
		!strings.Contains(strings.ToLower(err.Error()), "not found") {
		t.Fatalf("expected error to mention the missing chart, got: %v", err)
	}
}
