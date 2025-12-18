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
	"bytes"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"

	"helm.sh/helm/v4/pkg/action"
	"helm.sh/helm/v4/pkg/chart/common"
	kubefake "helm.sh/helm/v4/pkg/kube/fake"
	releasecommon "helm.sh/helm/v4/pkg/release/common"
	release "helm.sh/helm/v4/pkg/release/v1"
	"helm.sh/helm/v4/pkg/storage"
	"helm.sh/helm/v4/pkg/storage/driver"
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
			name:   "keep history with earlier deployed release",
			cmd:    "uninstall aeneas --keep-history",
			golden: "output/uninstall-keep-history-earlier-deployed.txt",
			rels: []*release.Release{
				release.Mock(&release.MockReleaseOptions{Name: "aeneas", Version: 1, Status: releasecommon.StatusDeployed}),
				release.Mock(&release.MockReleaseOptions{Name: "aeneas", Version: 2, Status: releasecommon.StatusFailed}),
			},
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

// TestUninstallCascadeDefaultWithWait tests that when --wait is used,
// the default cascade behavior changes to foreground to ensure
// dependent resources are cleaned up before returning.
func TestUninstallCascadeDefaultWithWait(t *testing.T) {
	tests := []struct {
		name                        string
		cmd                         string
		expectedDeletionPropagation string
	}{
		{
			name:                        "without --wait uses background cascade",
			cmd:                         "uninstall test-release",
			expectedDeletionPropagation: "Background",
		},
		{
			name:                        "--wait without --cascade uses foreground cascade",
			cmd:                         "uninstall test-release --wait",
			expectedDeletionPropagation: "Foreground",
		},
		{
			name:                        "--wait=watcher without --cascade uses foreground cascade",
			cmd:                         "uninstall test-release --wait=watcher",
			expectedDeletionPropagation: "Foreground",
		},
		{
			name:                        "--wait=legacy without --cascade uses foreground cascade",
			cmd:                         "uninstall test-release --wait=legacy",
			expectedDeletionPropagation: "Foreground",
		},
		{
			name:                        "--wait=hookOnly uses background cascade (default)",
			cmd:                         "uninstall test-release --wait=hookOnly",
			expectedDeletionPropagation: "Background",
		},
		{
			name:                        "--wait with explicit --cascade=background respects user setting",
			cmd:                         "uninstall test-release --wait --cascade=background",
			expectedDeletionPropagation: "Background",
		},
		{
			name:                        "--wait with explicit --cascade=orphan respects user setting",
			cmd:                         "uninstall test-release --wait --cascade=orphan",
			expectedDeletionPropagation: "Orphan",
		},
		{
			name:                        "--wait with explicit --cascade=foreground respects user setting",
			cmd:                         "uninstall test-release --wait --cascade=foreground",
			expectedDeletionPropagation: "Foreground",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a capturing KubeClient to verify the deletion propagation
			capturedDeletionPropagation := ""
			store := storage.Init(driver.NewMemory())
			rel := release.Mock(&release.MockReleaseOptions{Name: "test-release"})
			if err := store.Create(rel); err != nil {
				t.Fatal(err)
			}

			capturingClient := &kubefake.CapturingKubeClient{
				FailingKubeClient: kubefake.FailingKubeClient{
					PrintingKubeClient: kubefake.PrintingKubeClient{Out: io.Discard},
					BuildDummy:         true,
				},
				CapturedDeletionPropagation: &capturedDeletionPropagation,
			}

			actionConfig := &action.Configuration{
				Releases:     store,
				KubeClient:   capturingClient,
				Capabilities: common.DefaultCapabilities,
			}

			buf := new(bytes.Buffer)
			args := splitCmd(tt.cmd)

			root, err := newRootCmdWithConfig(actionConfig, buf, args, SetupLogging)
			if err != nil {
				t.Fatal(err)
			}

			root.SetArgs(args)
			root.SetOut(buf)
			root.SetErr(buf)

			_ = root.Execute()

			// Verify the captured deletion propagation
			assert.Equal(t, tt.expectedDeletionPropagation, capturedDeletionPropagation,
				"Expected DeletionPropagation to be %q but got %q", tt.expectedDeletionPropagation, capturedDeletionPropagation)
		})
	}
}

func splitCmd(cmd string) []string {
	// Simple split by space - doesn't handle quoted strings
	result := []string{}
	for _, part := range bytes.Fields([]byte(cmd)) {
		result = append(result, string(part))
	}
	return result
}

func TestUninstallCompletion(t *testing.T) {
	checkReleaseCompletion(t, "uninstall", true)
}

func TestUninstallFileCompletion(t *testing.T) {
	checkFileCompletion(t, "uninstall", false)
	checkFileCompletion(t, "uninstall myrelease", false)
}
