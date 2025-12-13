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

package action

import (
	"context"
	"errors"
	"fmt"
	"io"
	"reflect"
	"testing"
	"time"

	chart "helm.sh/helm/v4/pkg/chart/v2"
	"helm.sh/helm/v4/pkg/kube"
	"helm.sh/helm/v4/pkg/storage/driver"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	kubefake "helm.sh/helm/v4/pkg/kube/fake"
	"helm.sh/helm/v4/pkg/release/common"
	release "helm.sh/helm/v4/pkg/release/v1"
)

func upgradeAction(t *testing.T) *Upgrade {
	t.Helper()
	config := actionConfigFixture(t)
	upAction := NewUpgrade(config)
	upAction.Namespace = "spaced"

	return upAction
}

func TestUpgradeRelease_Success(t *testing.T) {
	is := assert.New(t)
	req := require.New(t)

	upAction := upgradeAction(t)
	rel := releaseStub()
	rel.Name = "previous-release"
	rel.Info.Status = common.StatusDeployed
	req.NoError(upAction.cfg.Releases.Create(rel))

	upAction.WaitStrategy = kube.StatusWatcherStrategy
	vals := map[string]interface{}{}

	ctx, done := context.WithCancel(t.Context())
	resi, err := upAction.RunWithContext(ctx, rel.Name, buildChart(), vals)
	req.NoError(err)
	res, err := releaserToV1Release(resi)
	is.NoError(err)
	is.Equal(res.Info.Status, common.StatusDeployed)
	done()

	// Detecting previous bug where context termination after successful release
	// caused release to fail.
	time.Sleep(time.Millisecond * 100)
	lastReleasei, err := upAction.cfg.Releases.Last(rel.Name)
	req.NoError(err)
	lastRelease, err := releaserToV1Release(lastReleasei)
	req.NoError(err)
	is.Equal(lastRelease.Info.Status, common.StatusDeployed)
}

func TestUpgradeRelease_Wait(t *testing.T) {
	is := assert.New(t)
	req := require.New(t)

	upAction := upgradeAction(t)
	rel := releaseStub()
	rel.Name = "come-fail-away"
	rel.Info.Status = common.StatusDeployed
	upAction.cfg.Releases.Create(rel)

	failer := upAction.cfg.KubeClient.(*kubefake.FailingKubeClient)
	failer.WaitError = fmt.Errorf("I timed out")
	upAction.cfg.KubeClient = failer
	upAction.WaitStrategy = kube.StatusWatcherStrategy
	vals := map[string]interface{}{}

	resi, err := upAction.Run(rel.Name, buildChart(), vals)
	req.Error(err)
	res, err := releaserToV1Release(resi)
	is.NoError(err)
	is.Contains(res.Info.Description, "I timed out")
	is.Equal(res.Info.Status, common.StatusFailed)
}

func TestUpgradeRelease_WaitForJobs(t *testing.T) {
	is := assert.New(t)
	req := require.New(t)

	upAction := upgradeAction(t)
	rel := releaseStub()
	rel.Name = "come-fail-away"
	rel.Info.Status = common.StatusDeployed
	upAction.cfg.Releases.Create(rel)

	failer := upAction.cfg.KubeClient.(*kubefake.FailingKubeClient)
	failer.WaitError = fmt.Errorf("I timed out")
	upAction.cfg.KubeClient = failer
	upAction.WaitStrategy = kube.StatusWatcherStrategy
	upAction.WaitForJobs = true
	vals := map[string]interface{}{}

	resi, err := upAction.Run(rel.Name, buildChart(), vals)
	req.Error(err)
	res, err := releaserToV1Release(resi)
	is.NoError(err)
	is.Contains(res.Info.Description, "I timed out")
	is.Equal(res.Info.Status, common.StatusFailed)
}

func TestUpgradeRelease_CleanupOnFail(t *testing.T) {
	is := assert.New(t)
	req := require.New(t)

	upAction := upgradeAction(t)
	rel := releaseStub()
	rel.Name = "come-fail-away"
	rel.Info.Status = common.StatusDeployed
	upAction.cfg.Releases.Create(rel)

	failer := upAction.cfg.KubeClient.(*kubefake.FailingKubeClient)
	failer.WaitError = fmt.Errorf("I timed out")
	failer.DeleteError = fmt.Errorf("I tried to delete nil")
	upAction.cfg.KubeClient = failer
	upAction.WaitStrategy = kube.StatusWatcherStrategy
	upAction.CleanupOnFail = true
	vals := map[string]interface{}{}

	resi, err := upAction.Run(rel.Name, buildChart(), vals)
	req.Error(err)
	is.NotContains(err.Error(), "unable to cleanup resources")
	res, err := releaserToV1Release(resi)
	is.NoError(err)
	is.Contains(res.Info.Description, "I timed out")
	is.Equal(res.Info.Status, common.StatusFailed)
}

func TestUpgradeRelease_RollbackOnFailure(t *testing.T) {
	is := assert.New(t)
	req := require.New(t)

	t.Run("rollback-on-failure rollback succeeds", func(t *testing.T) {
		upAction := upgradeAction(t)

		rel := releaseStub()
		rel.Name = "nuketown"
		rel.Info.Status = common.StatusDeployed
		upAction.cfg.Releases.Create(rel)

		failer := upAction.cfg.KubeClient.(*kubefake.FailingKubeClient)
		// We can't make Update error because then the rollback won't work
		failer.WatchUntilReadyError = fmt.Errorf("arming key removed")
		upAction.cfg.KubeClient = failer
		upAction.RollbackOnFailure = true
		vals := map[string]interface{}{}

		resi, err := upAction.Run(rel.Name, buildChart(), vals)
		req.Error(err)
		is.Contains(err.Error(), "arming key removed")
		is.Contains(err.Error(), "rollback-on-failure")
		res, err := releaserToV1Release(resi)
		is.NoError(err)

		// Now make sure it is actually upgraded
		updatedResi, err := upAction.cfg.Releases.Get(res.Name, 3)
		is.NoError(err)
		updatedRes, err := releaserToV1Release(updatedResi)
		is.NoError(err)
		// Should have rolled back to the previous
		is.Equal(updatedRes.Info.Status, common.StatusDeployed)
	})

	t.Run("rollback-on-failure uninstall fails", func(t *testing.T) {
		upAction := upgradeAction(t)
		rel := releaseStub()
		rel.Name = "fallout"
		rel.Info.Status = common.StatusDeployed
		upAction.cfg.Releases.Create(rel)

		failer := upAction.cfg.KubeClient.(*kubefake.FailingKubeClient)
		failer.UpdateError = fmt.Errorf("update fail")
		upAction.cfg.KubeClient = failer
		upAction.RollbackOnFailure = true
		vals := map[string]interface{}{}

		_, err := upAction.Run(rel.Name, buildChart(), vals)
		req.Error(err)
		is.Contains(err.Error(), "update fail")
		is.Contains(err.Error(), "an error occurred while rolling back the release")
	})
}

func TestUpgradeRelease_ReuseValues(t *testing.T) {
	is := assert.New(t)

	t.Run("reuse values should work with values", func(t *testing.T) {
		upAction := upgradeAction(t)

		existingValues := map[string]interface{}{
			"name":        "value",
			"maxHeapSize": "128m",
			"replicas":    2,
		}
		newValues := map[string]interface{}{
			"name":        "newValue",
			"maxHeapSize": "512m",
			"cpu":         "12m",
		}
		expectedValues := map[string]interface{}{
			"name":        "newValue",
			"maxHeapSize": "512m",
			"cpu":         "12m",
			"replicas":    2,
		}

		rel := releaseStub()
		rel.Name = "nuketown"
		rel.Info.Status = common.StatusDeployed
		rel.Config = existingValues

		err := upAction.cfg.Releases.Create(rel)
		is.NoError(err)

		upAction.ReuseValues = true
		// setting newValues and upgrading
		resi, err := upAction.Run(rel.Name, buildChart(), newValues)
		is.NoError(err)
		res, err := releaserToV1Release(resi)
		is.NoError(err)

		// Now make sure it is actually upgraded
		updatedResi, err := upAction.cfg.Releases.Get(res.Name, 2)
		is.NoError(err)

		if updatedResi == nil {
			is.Fail("Updated Release is nil")
			return
		}
		updatedRes, err := releaserToV1Release(updatedResi)
		is.NoError(err)

		is.Equal(common.StatusDeployed, updatedRes.Info.Status)
		is.Equal(expectedValues, updatedRes.Config)
	})

	t.Run("reuse values should not install disabled charts", func(t *testing.T) {
		upAction := upgradeAction(t)
		chartDefaultValues := map[string]interface{}{
			"subchart": map[string]interface{}{
				"enabled": true,
			},
		}
		dependency := chart.Dependency{
			Name:       "subchart",
			Version:    "0.1.0",
			Repository: "http://some-repo.com",
			Condition:  "subchart.enabled",
		}
		sampleChart := buildChart(
			withName("sample"),
			withValues(chartDefaultValues),
			withMetadataDependency(dependency),
		)
		now := time.Now()
		existingValues := map[string]interface{}{
			"subchart": map[string]interface{}{
				"enabled": false,
			},
		}
		rel := &release.Release{
			Name: "nuketown",
			Info: &release.Info{
				FirstDeployed: now,
				LastDeployed:  now,
				Status:        common.StatusDeployed,
				Description:   "Named Release Stub",
			},
			Chart:   sampleChart,
			Config:  existingValues,
			Version: 1,
		}
		err := upAction.cfg.Releases.Create(rel)
		is.NoError(err)

		upAction.ReuseValues = true
		sampleChartWithSubChart := buildChart(
			withName(sampleChart.Name()),
			withValues(sampleChart.Values),
			withDependency(withName("subchart")),
			withMetadataDependency(dependency),
		)
		// reusing values and upgrading
		resi, err := upAction.Run(rel.Name, sampleChartWithSubChart, map[string]interface{}{})
		is.NoError(err)
		res, err := releaserToV1Release(resi)
		is.NoError(err)

		// Now get the upgraded release
		updatedResi, err := upAction.cfg.Releases.Get(res.Name, 2)
		is.NoError(err)

		if updatedResi == nil {
			is.Fail("Updated Release is nil")
			return
		}
		updatedRes, err := releaserToV1Release(updatedResi)
		is.NoError(err)

		is.Equal(common.StatusDeployed, updatedRes.Info.Status)
		is.Equal(0, len(updatedRes.Chart.Dependencies()), "expected 0 dependencies")

		expectedValues := map[string]interface{}{
			"subchart": map[string]interface{}{
				"enabled": false,
			},
		}
		is.Equal(expectedValues, updatedRes.Config)
	})
}

func TestUpgradeRelease_ResetThenReuseValues(t *testing.T) {
	is := assert.New(t)

	t.Run("reset then reuse values should work with values", func(t *testing.T) {
		upAction := upgradeAction(t)

		existingValues := map[string]interface{}{
			"name":        "value",
			"maxHeapSize": "128m",
			"replicas":    2,
		}
		newValues := map[string]interface{}{
			"name":        "newValue",
			"maxHeapSize": "512m",
			"cpu":         "12m",
		}
		newChartValues := map[string]interface{}{
			"memory": "256m",
		}
		expectedValues := map[string]interface{}{
			"name":        "newValue",
			"maxHeapSize": "512m",
			"cpu":         "12m",
			"replicas":    2,
		}

		rel := releaseStub()
		rel.Name = "nuketown"
		rel.Info.Status = common.StatusDeployed
		rel.Config = existingValues

		err := upAction.cfg.Releases.Create(rel)
		is.NoError(err)

		upAction.ResetThenReuseValues = true
		// setting newValues and upgrading
		resi, err := upAction.Run(rel.Name, buildChart(withValues(newChartValues)), newValues)
		is.NoError(err)
		res, err := releaserToV1Release(resi)
		is.NoError(err)

		// Now make sure it is actually upgraded
		updatedResi, err := upAction.cfg.Releases.Get(res.Name, 2)
		is.NoError(err)

		if updatedResi == nil {
			is.Fail("Updated Release is nil")
			return
		}
		updatedRes, err := releaserToV1Release(updatedResi)
		is.NoError(err)

		is.Equal(common.StatusDeployed, updatedRes.Info.Status)
		is.Equal(expectedValues, updatedRes.Config)
		is.Equal(newChartValues, updatedRes.Chart.Values)
	})
}

func TestUpgradeRelease_Pending(t *testing.T) {
	req := require.New(t)

	upAction := upgradeAction(t)
	rel := releaseStub()
	rel.Name = "come-fail-away"
	rel.Info.Status = common.StatusDeployed
	upAction.cfg.Releases.Create(rel)
	rel2 := releaseStub()
	rel2.Name = "come-fail-away"
	rel2.Info.Status = common.StatusPendingUpgrade
	rel2.Version = 2
	upAction.cfg.Releases.Create(rel2)

	vals := map[string]interface{}{}

	_, err := upAction.Run(rel.Name, buildChart(), vals)
	req.Contains(err.Error(), "progress", err)
}

func TestUpgradeRelease_Interrupted_Wait(t *testing.T) {
	is := assert.New(t)
	req := require.New(t)

	upAction := upgradeAction(t)
	rel := releaseStub()
	rel.Name = "interrupted-release"
	rel.Info.Status = common.StatusDeployed
	upAction.cfg.Releases.Create(rel)

	failer := upAction.cfg.KubeClient.(*kubefake.FailingKubeClient)
	failer.WaitDuration = 10 * time.Second
	upAction.cfg.KubeClient = failer
	upAction.WaitStrategy = kube.StatusWatcherStrategy
	vals := map[string]interface{}{}

	ctx, cancel := context.WithCancel(t.Context())
	time.AfterFunc(time.Second, cancel)

	resi, err := upAction.RunWithContext(ctx, rel.Name, buildChart(), vals)

	req.Error(err)
	res, err := releaserToV1Release(resi)
	is.NoError(err)
	is.Contains(res.Info.Description, "Upgrade \"interrupted-release\" failed: context canceled")
	is.Equal(res.Info.Status, common.StatusFailed)
}

func TestUpgradeRelease_Interrupted_RollbackOnFailure(t *testing.T) {

	is := assert.New(t)
	req := require.New(t)

	upAction := upgradeAction(t)
	rel := releaseStub()
	rel.Name = "interrupted-release"
	rel.Info.Status = common.StatusDeployed
	upAction.cfg.Releases.Create(rel)

	failer := upAction.cfg.KubeClient.(*kubefake.FailingKubeClient)
	failer.WaitDuration = 5 * time.Second
	upAction.cfg.KubeClient = failer
	upAction.RollbackOnFailure = true
	vals := map[string]interface{}{}

	ctx, cancel := context.WithCancel(t.Context())
	time.AfterFunc(time.Second, cancel)

	resi, err := upAction.RunWithContext(ctx, rel.Name, buildChart(), vals)

	req.Error(err)
	is.Contains(err.Error(), "release interrupted-release failed, and has been rolled back due to rollback-on-failure being set: context canceled")
	res, err := releaserToV1Release(resi)
	is.NoError(err)
	// Now make sure it is actually upgraded
	updatedResi, err := upAction.cfg.Releases.Get(res.Name, 3)
	is.NoError(err)
	updatedRes, err := releaserToV1Release(updatedResi)
	is.NoError(err)
	// Should have rolled back to the previous
	is.Equal(updatedRes.Info.Status, common.StatusDeployed)
}

func TestMergeCustomLabels(t *testing.T) {
	tests := [][3]map[string]string{
		{nil, nil, map[string]string{}},
		{map[string]string{}, map[string]string{}, map[string]string{}},
		{map[string]string{"k1": "v1", "k2": "v2"}, nil, map[string]string{"k1": "v1", "k2": "v2"}},
		{nil, map[string]string{"k1": "v1", "k2": "v2"}, map[string]string{"k1": "v1", "k2": "v2"}},
		{map[string]string{"k1": "v1", "k2": "v2"}, map[string]string{"k1": "null", "k2": "v3"}, map[string]string{"k2": "v3"}},
	}
	for _, test := range tests {
		if output := mergeCustomLabels(test[0], test[1]); !reflect.DeepEqual(test[2], output) {
			t.Errorf("Expected {%v}, got {%v}", test[2], output)
		}
	}
}

func TestUpgradeRelease_Labels(t *testing.T) {
	is := assert.New(t)
	upAction := upgradeAction(t)

	rel := releaseStub()
	rel.Name = "labels"
	// It's needed to check that suppressed release would keep original labels
	rel.Labels = map[string]string{
		"key1": "val1",
		"key2": "val2.1",
	}
	rel.Info.Status = common.StatusDeployed

	err := upAction.cfg.Releases.Create(rel)
	is.NoError(err)

	upAction.Labels = map[string]string{
		"key1": "null",
		"key2": "val2.2",
		"key3": "val3",
	}
	// setting newValues and upgrading
	resi, err := upAction.Run(rel.Name, buildChart(), nil)
	is.NoError(err)
	res, err := releaserToV1Release(resi)
	is.NoError(err)

	// Now make sure it is actually upgraded and labels were merged
	updatedResi, err := upAction.cfg.Releases.Get(res.Name, 2)
	is.NoError(err)

	if updatedResi == nil {
		is.Fail("Updated Release is nil")
		return
	}
	updatedRes, err := releaserToV1Release(updatedResi)
	is.NoError(err)
	is.Equal(common.StatusDeployed, updatedRes.Info.Status)
	is.Equal(mergeCustomLabels(rel.Labels, upAction.Labels), updatedRes.Labels)

	// Now make sure it is suppressed release still contains original labels
	initialResi, err := upAction.cfg.Releases.Get(res.Name, 1)
	is.NoError(err)

	if initialResi == nil {
		is.Fail("Updated Release is nil")
		return
	}
	initialRes, err := releaserToV1Release(initialResi)
	is.NoError(err)
	is.Equal(initialRes.Info.Status, common.StatusSuperseded)
	is.Equal(initialRes.Labels, rel.Labels)
}

func TestUpgradeRelease_SystemLabels(t *testing.T) {
	is := assert.New(t)
	upAction := upgradeAction(t)

	rel := releaseStub()
	rel.Name = "labels"
	// It's needed to check that suppressed release would keep original labels
	rel.Labels = map[string]string{
		"key1": "val1",
		"key2": "val2.1",
	}
	rel.Info.Status = common.StatusDeployed

	err := upAction.cfg.Releases.Create(rel)
	is.NoError(err)

	upAction.Labels = map[string]string{
		"key1":  "null",
		"key2":  "val2.2",
		"owner": "val3",
	}
	// setting newValues and upgrading
	_, err = upAction.Run(rel.Name, buildChart(), nil)
	if err == nil {
		t.Fatal("expected an error")
	}

	is.Equal(fmt.Errorf("user supplied labels contains system reserved label name. System labels: %+v", driver.GetSystemLabels()), err)
}

func TestUpgradeRelease_DryRun(t *testing.T) {
	is := assert.New(t)
	req := require.New(t)

	upAction := upgradeAction(t)
	rel := releaseStub()
	rel.Name = "previous-release"
	rel.Info.Status = common.StatusDeployed
	req.NoError(upAction.cfg.Releases.Create(rel))

	upAction.DryRunStrategy = DryRunClient
	vals := map[string]interface{}{}

	ctx, done := context.WithCancel(t.Context())
	resi, err := upAction.RunWithContext(ctx, rel.Name, buildChart(withSampleSecret()), vals)
	done()
	req.NoError(err)
	res, err := releaserToV1Release(resi)
	is.NoError(err)
	is.Equal(common.StatusPendingUpgrade, res.Info.Status)
	is.Contains(res.Manifest, "kind: Secret")

	lastReleasei, err := upAction.cfg.Releases.Last(rel.Name)
	req.NoError(err)
	lastRelease, err := releaserToV1Release(lastReleasei)
	req.NoError(err)
	is.Equal(lastRelease.Info.Status, common.StatusDeployed)
	is.Equal(1, lastRelease.Version)

	// Test the case for hiding the secret to ensure it is not displayed
	upAction.HideSecret = true
	vals = map[string]interface{}{}

	ctx, done = context.WithCancel(t.Context())
	resi, err = upAction.RunWithContext(ctx, rel.Name, buildChart(withSampleSecret()), vals)
	done()
	req.NoError(err)
	res, err = releaserToV1Release(resi)
	is.NoError(err)
	is.Equal(common.StatusPendingUpgrade, res.Info.Status)
	is.NotContains(res.Manifest, "kind: Secret")

	lastReleasei, err = upAction.cfg.Releases.Last(rel.Name)
	req.NoError(err)
	lastRelease, err = releaserToV1Release(lastReleasei)
	req.NoError(err)
	is.Equal(lastRelease.Info.Status, common.StatusDeployed)
	is.Equal(1, lastRelease.Version)

	// Ensure in a dry run mode when using HideSecret
	upAction.DryRunStrategy = DryRunNone
	vals = map[string]interface{}{}

	ctx, done = context.WithCancel(t.Context())
	_, err = upAction.RunWithContext(ctx, rel.Name, buildChart(withSampleSecret()), vals)
	done()
	req.Error(err)
}

func TestUpgradeRelease_DryRunServerValidation(t *testing.T) {
	// Test that server-side dry-run actually calls the Kubernetes API for validation
	is := assert.New(t)
	req := require.New(t)

	// Use a fixture that returns dummy resources so our code path is exercised
	config := actionConfigFixtureWithDummyResources(t, createDummyResourceList(true))

	upAction := NewUpgrade(config)
	upAction.Namespace = "spaced"

	// Create a previous release
	rel := releaseStub()
	rel.Name = "test-server-dry-run"
	rel.Info.Status = common.StatusDeployed
	req.NoError(upAction.cfg.Releases.Create(rel))

	// Set up the fake client to return an error on Update
	expectedErr := errors.New("validation error: unknown field in spec")
	config.KubeClient.(*kubefake.FailingKubeClient).UpdateError = expectedErr
	upAction.DryRunStrategy = DryRunServer

	vals := map[string]interface{}{}
	ctx, done := context.WithCancel(t.Context())
	_, err := upAction.RunWithContext(ctx, rel.Name, buildChart(), vals)
	done()

	// The error from the API should be returned
	is.Error(err)
	is.Contains(err.Error(), "validation error")

	// Reset and test that client-side dry-run does NOT call the API
	config2 := actionConfigFixtureWithDummyResources(t, createDummyResourceList(true))
	config2.KubeClient.(*kubefake.FailingKubeClient).UpdateError = expectedErr

	upAction2 := NewUpgrade(config2)
	upAction2.Namespace = "spaced"

	// Create a previous release
	rel2 := releaseStub()
	rel2.Name = "test-client-dry-run"
	rel2.Info.Status = common.StatusDeployed
	req.NoError(upAction2.cfg.Releases.Create(rel2))

	upAction2.DryRunStrategy = DryRunClient

	ctx, done = context.WithCancel(t.Context())
	resi, err := upAction2.RunWithContext(ctx, rel2.Name, buildChart(), vals)
	done()

	// Client-side dry-run should succeed since it doesn't call the API
	is.NoError(err)
	res, err := releaserToV1Release(resi)
	is.NoError(err)
	is.Equal(res.Info.Description, "Dry run complete")
}

func TestGetUpgradeServerSideValue(t *testing.T) {
	tests := []struct {
		name                    string
		actionServerSideOption  string
		releaseApplyMethod      string
		expectedServerSideApply bool
	}{
		{
			name:                    "action ssa auto / release csa",
			actionServerSideOption:  "auto",
			releaseApplyMethod:      "csa",
			expectedServerSideApply: false,
		},
		{
			name:                    "action ssa auto / release ssa",
			actionServerSideOption:  "auto",
			releaseApplyMethod:      "ssa",
			expectedServerSideApply: true,
		},
		{
			name:                    "action ssa auto / release empty",
			actionServerSideOption:  "auto",
			releaseApplyMethod:      "",
			expectedServerSideApply: false,
		},
		{
			name:                    "action ssa true / release csa",
			actionServerSideOption:  "true",
			releaseApplyMethod:      "csa",
			expectedServerSideApply: true,
		},
		{
			name:                    "action ssa true / release ssa",
			actionServerSideOption:  "true",
			releaseApplyMethod:      "ssa",
			expectedServerSideApply: true,
		},
		{
			name:                    "action ssa true / release 'unknown'",
			actionServerSideOption:  "true",
			releaseApplyMethod:      "foo",
			expectedServerSideApply: true,
		},
		{
			name:                    "action ssa true / release empty",
			actionServerSideOption:  "true",
			releaseApplyMethod:      "",
			expectedServerSideApply: true,
		},
		{
			name:                    "action ssa false / release csa",
			actionServerSideOption:  "false",
			releaseApplyMethod:      "ssa",
			expectedServerSideApply: false,
		},
		{
			name:                    "action ssa false / release ssa",
			actionServerSideOption:  "false",
			releaseApplyMethod:      "ssa",
			expectedServerSideApply: false,
		},
		{
			name:                    "action ssa false / release 'unknown'",
			actionServerSideOption:  "false",
			releaseApplyMethod:      "foo",
			expectedServerSideApply: false,
		},
		{
			name:                    "action ssa false / release empty",
			actionServerSideOption:  "false",
			releaseApplyMethod:      "ssa",
			expectedServerSideApply: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			serverSideApply, err := getUpgradeServerSideValue(tt.actionServerSideOption, tt.releaseApplyMethod)
			assert.Nil(t, err)
			assert.Equal(t, tt.expectedServerSideApply, serverSideApply)
		})
	}

	testsError := []struct {
		name                   string
		actionServerSideOption string
		releaseApplyMethod     string
		expectedErrorMsg       string
	}{
		{
			name:                   "action invalid option",
			actionServerSideOption: "invalid",
			releaseApplyMethod:     "ssa",
			expectedErrorMsg:       "invalid/unknown release server-side apply method: invalid",
		},
	}

	for _, tt := range testsError {
		t.Run(tt.name, func(t *testing.T) {
			_, err := getUpgradeServerSideValue(tt.actionServerSideOption, tt.releaseApplyMethod)
			assert.ErrorContains(t, err, tt.expectedErrorMsg)
		})
	}

}

func TestUpgradeRun_UnreachableKubeClient(t *testing.T) {
	t.Helper()
	config := actionConfigFixture(t)
	failingKubeClient := kubefake.FailingKubeClient{PrintingKubeClient: kubefake.PrintingKubeClient{Out: io.Discard}, DummyResources: nil}
	failingKubeClient.ConnectionError = errors.New("connection refused")
	config.KubeClient = &failingKubeClient

	client := NewUpgrade(config)
	vals := map[string]interface{}{}
	result, err := client.Run("", buildChart(), vals)

	assert.Nil(t, result)
	assert.ErrorContains(t, err, "connection refused")
}
