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
	"testing"

	"helm.sh/helm/v3/pkg/release"
)

func TestUninstall(t *testing.T) {
	tests := []cmdTestCase{
		{
			name:   "basic uninstall",
			cmd:    "uninstall aeneas",
			golden: "output/uninstall.txt",
			rels:   []*release.Release{release.Mock(&release.MockReleaseOptions{Name: "aeneas"})},
		},
		{
			name:   "multiple uninstall",
			cmd:    "uninstall aeneas aeneas2",
			golden: "output/uninstall-multiple.txt",
			rels: []*release.Release{
				release.Mock(&release.MockReleaseOptions{Name: "aeneas"}),
				release.Mock(&release.MockReleaseOptions{Name: "aeneas2"}),
			},
		},
		{
			name:   "uninstall with timeout",
			cmd:    "uninstall aeneas --timeout 120s",
			golden: "output/uninstall-timeout.txt",
			rels:   []*release.Release{release.Mock(&release.MockReleaseOptions{Name: "aeneas"})},
		},
		{
			name:   "uninstall without hooks",
			cmd:    "uninstall aeneas --no-hooks",
			golden: "output/uninstall-no-hooks.txt",
			rels:   []*release.Release{release.Mock(&release.MockReleaseOptions{Name: "aeneas"})},
		},
		{
			name:   "keep history",
			cmd:    "uninstall aeneas --keep-history",
			golden: "output/uninstall-keep-history.txt",
			rels:   []*release.Release{release.Mock(&release.MockReleaseOptions{Name: "aeneas"})},
		},
		{
			name:   "wait",
			cmd:    "uninstall aeneas --wait",
			golden: "output/uninstall-wait.txt",
			rels:   []*release.Release{release.Mock(&release.MockReleaseOptions{Name: "aeneas"})},
		},
		{
			name:      "uninstall without release",
			cmd:       "uninstall",
			golden:    "output/uninstall-no-args.txt",
			wantError: true,
		},
	}
	runTestCmd(t, tests)
}

func TestUninstallCompletion(t *testing.T) {
	checkReleaseCompletion(t, "uninstall", true)
}

func TestUninstallFileCompletion(t *testing.T) {
	checkFileCompletion(t, "uninstall", false)
	checkFileCompletion(t, "uninstall myrelease", false)
}
