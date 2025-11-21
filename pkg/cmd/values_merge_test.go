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
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"

	"helm.sh/helm/v4/pkg/action"
	"helm.sh/helm/v4/pkg/chart"
	"helm.sh/helm/v4/pkg/release"
	"helm.sh/helm/v4/pkg/storage"
	"helm.sh/helm/v4/pkg/storage/driver"
)

func TestValuesMergeCmd(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		flags       map[string]string
		expectedErr bool
	}{
		{
			name:        "requires release name",
			args:        []string{},
			expectedErr: true,
		},
		{
			name: "valid merge command",
			args: []string{"test-release"},
			flags: map[string]string{
				"revisions": "1,2",
			},
			expectedErr: false,
		},
		{
			name: "invalid revisions",
			args: []string{"test-release"},
			flags: map[string]string{
				"revisions": "invalid",
			},
			expectedErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test action configuration
			store := storage.Init(driver.NewMemory())
			actionConfig := action.NewConfiguration()
			actionConfig.Releases = store

			// Create test releases
			release1 := createTestReleaseForCmd("test-release", 1)
			release2 := createTestReleaseForCmd("test-release", 2)

			// Add releases to storage
			err := actionConfig.Releases.Create(release1)
			assert.NoError(t, err)
			err = actionConfig.Releases.Create(release2)
			assert.NoError(t, err)

			// Create command
			buf := new(bytes.Buffer)
			cmd := newValuesMergeCmd(actionConfig, buf)

			// Set flags
			for flag, value := range tt.flags {
				cmd.Flags().Set(flag, value)
			}

			// Run command
			err = cmd.RunE(cmd, tt.args)

			if tt.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValuesMergeHelp(t *testing.T) {
	store := storage.Init(driver.NewMemory())
	actionConfig := action.NewConfiguration()
	actionConfig.Releases = store

	buf := new(bytes.Buffer)
	cmd := newValuesMergeCmd(actionConfig, buf)

	assert.Equal(t, "merge", cmd.Name())
	assert.Equal(t, "intelligently merge values from multiple release revisions", cmd.Short)
	assert.Contains(t, cmd.Long, "version hell")
	assert.Contains(t, cmd.Long, "merge strategies")
}

func createTestReleaseForCmd(name string, version int) *release.Release {
	chrt := &chart.Chart{
		Metadata: &chart.Metadata{
			Name:    name,
			Version: "1.0.0",
		},
	}

	return &release.Release{
		Name:    name,
		Version: version,
		Config: map[string]interface{}{
			"replicas": version,
			"image":    "nginx:1.0",
		},
		Chart:    chrt,
		Manifest: "# Manifest",
		Info: &release.Info{
			Status:        release.StatusDeployed,
			LastDeployed:  "2024-01-01T00:00:00Z",
			Description:   "Release description",
		},
	}
}