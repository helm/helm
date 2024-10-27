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
	"strings"
	"testing"

	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/release"
)

// Check if file completion should be performed according to parameter 'shouldBePerformed'
func checkFileCompletion(t *testing.T, cmdName string, shouldBePerformed bool) {
	storage := storageFixture()
	storage.Create(&release.Release{
		Name: "myrelease",
		Info: &release.Info{Status: release.StatusDeployed},
		Chart: &chart.Chart{
			Metadata: &chart.Metadata{
				Name:    "Myrelease-Chart",
				Version: "1.2.3",
			},
		},
		Version: 1,
	})

	testcmd := fmt.Sprintf("__complete %s ''", cmdName)
	_, out, err := executeActionCommandC(storage, testcmd, nil, nil)
	if err != nil {
		t.Errorf("unexpected error, %s", err)
	}
	if !strings.Contains(out, "ShellCompDirectiveNoFileComp") != shouldBePerformed {
		if shouldBePerformed {
			t.Errorf("Unexpected directive ShellCompDirectiveNoFileComp when completing '%s'", cmdName)
		} else {

			t.Errorf("Did not receive directive ShellCompDirectiveNoFileComp when completing '%s'", cmdName)
		}
		t.Log(out)
	}
}

func TestCompletionFileCompletion(t *testing.T) {
	checkFileCompletion(t, "completion", false)
	checkFileCompletion(t, "completion bash", false)
	checkFileCompletion(t, "completion zsh", false)
	checkFileCompletion(t, "completion fish", false)
}

func checkReleaseCompletion(t *testing.T, cmdName string, multiReleasesAllowed bool) {
	multiReleaseTestGolden := "output/empty_nofile_comp.txt"
	if multiReleasesAllowed {
		multiReleaseTestGolden = "output/release_list_repeat_comp.txt"
	}
	tests := []cmdTestCase{{
		name:   "completion for uninstall",
		cmd:    fmt.Sprintf("__complete %s ''", cmdName),
		golden: "output/release_list_comp.txt",
		rels: []*release.Release{
			release.Mock(&release.MockReleaseOptions{Name: "athos"}),
			release.Mock(&release.MockReleaseOptions{Name: "porthos"}),
			release.Mock(&release.MockReleaseOptions{Name: "aramis"}),
		},
	}, {
		name:   "completion for uninstall repetition",
		cmd:    fmt.Sprintf("__complete %s porthos ''", cmdName),
		golden: multiReleaseTestGolden,
		rels: []*release.Release{
			release.Mock(&release.MockReleaseOptions{Name: "athos"}),
			release.Mock(&release.MockReleaseOptions{Name: "porthos"}),
			release.Mock(&release.MockReleaseOptions{Name: "aramis"}),
		},
	}}
	for _, test := range tests {
		runTestCmd(t, []cmdTestCase{test})
	}
}
