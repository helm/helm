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
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"helm.sh/helm/v4/pkg/action"
	"helm.sh/helm/v4/pkg/chart/common"
	chart "helm.sh/helm/v4/pkg/chart/v2"
	"helm.sh/helm/v4/pkg/kube"
	kubefake "helm.sh/helm/v4/pkg/kube/fake"
	rcommon "helm.sh/helm/v4/pkg/release/common"
	release "helm.sh/helm/v4/pkg/release/v1"
)

func TestReleaseTestingCompletion(t *testing.T) {
	checkReleaseCompletion(t, "test", false)
}

func TestReleaseTestingFileCompletion(t *testing.T) {
	checkFileCompletion(t, "test", false)
	checkFileCompletion(t, "test myrelease", false)
}

func TestReleaseTestNotesHandling(t *testing.T) {
	// Test that ensures notes behavior is correct for test command
	// This is a simpler test that focuses on the core functionality

	rel := &release.Release{
		Name:      "test-release",
		Namespace: "default",
		Info: &release.Info{
			Status: rcommon.StatusDeployed,
			Notes:  "Some important notes that should be hidden by default",
		},
		Chart: &chart.Chart{Metadata: &chart.Metadata{Name: "test", Version: "1.0.0"}},
	}

	// Set up storage
	store := storageFixture()
	store.Create(rel)

	// Set up action configuration properly
	actionConfig := &action.Configuration{
		Releases:     store,
		KubeClient:   &kubefake.FailingKubeClient{Out: io.Discard},
		Capabilities: common.DefaultCapabilities,
	}

	// Test the newReleaseTestCmd function directly
	var buf1 bytes.Buffer

	// Test 1: Default behavior (should hide notes)
	cmd1 := newReleaseTestCmd(actionConfig, &buf1)
	cmd1.SetArgs([]string{"test-release"})
	err1 := cmd1.Execute()
	require.NoError(t, err1, "Unexpected error for default test")
	output1 := buf1.String()
	assert.NotContains(t, output1, "NOTES:", "Expected notes to be hidden by default, but found NOTES section in output: %s", output1)
}

type releaseTestCleanupKubeClient struct {
	kubefake.FailingKubeClient
	createCalls int
	createErr   error
	watchErr    error
}

func (c *releaseTestCleanupKubeClient) Create(resources kube.ResourceList, options ...kube.ClientCreateOption) (*kube.Result, error) {
	c.createCalls++
	if c.createCalls == 2 {
		c.CreateError = c.createErr
		c.WatchUntilReadyError = c.watchErr
	}
	return c.FailingKubeClient.Create(resources, options...)
}

func TestReleaseTestPreservesCleanupErrors(t *testing.T) {
	createErr := errors.New("test hook creation failed")
	watchErr := errors.New("test hook execution failed")
	cleanupErr := errors.New("test hook cleanup failed")
	tests := []struct {
		name       string
		createErr  error
		watchErr   error
		cleanupErr error
	}{
		{name: "creation and cleanup fail", createErr: createErr, cleanupErr: cleanupErr},
		{name: "execution and cleanup fail", watchErr: watchErr, cleanupErr: cleanupErr},
		{name: "only creation fails", createErr: createErr},
		{name: "only execution fails", watchErr: watchErr},
		{name: "only cleanup fails", cleanupErr: cleanupErr},
		{name: "success"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rel := &release.Release{
				Name:      "test-release",
				Namespace: "default",
				Info:      &release.Info{Status: rcommon.StatusDeployed},
				Chart:     &chart.Chart{Metadata: &chart.Metadata{Name: "test", Version: "1.0.0"}},
				Hooks: []*release.Hook{
					{
						Name:           "successful-hook",
						Kind:           "ConfigMap",
						Weight:         -1,
						Events:         []release.HookEvent{release.HookTest},
						DeletePolicies: []release.HookDeletePolicy{release.HookSucceeded},
					},
					{
						Name:           "next-hook",
						Kind:           "Job",
						Events:         []release.HookEvent{release.HookTest},
						DeletePolicies: []release.HookDeletePolicy{release.HookSucceeded},
					},
				},
			}
			store := storageFixture()
			require.NoError(t, store.Create(rel))
			client := &releaseTestCleanupKubeClient{
				Out:         io.Discard,
				DeleteError: tt.cleanupErr,
				createErr:   tt.createErr,
				watchErr:    tt.watchErr,
			}
			cfg := &action.Configuration{
				Releases:     store,
				KubeClient:   client,
				Capabilities: common.DefaultCapabilities,
			}
			cmd := newReleaseTestCmd(cfg, io.Discard)
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
			cmd.SetArgs([]string{rel.Name})
			err := cmd.Execute()
			require.Equal(t, 2, client.createCalls)
			if tt.createErr == nil && tt.watchErr == nil && tt.cleanupErr == nil {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			for _, cause := range []error{tt.createErr, tt.watchErr, tt.cleanupErr} {
				if cause != nil {
					require.ErrorIs(t, err, cause)
					assert.Equal(t, 1, strings.Count(err.Error(), cause.Error()), "failure should be reported once")
				}
			}
		})
	}
}
