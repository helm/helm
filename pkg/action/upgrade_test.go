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
	"fmt"
	"testing"
	"time"

	"helm.sh/helm/v3/pkg/chart"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	kubefake "helm.sh/helm/v3/pkg/kube/fake"
	"helm.sh/helm/v3/pkg/release"
	helmtime "helm.sh/helm/v3/pkg/time"
)

func upgradeAction(t *testing.T) *Upgrade {
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
	rel.Info.Status = release.StatusDeployed
	req.NoError(upAction.cfg.Releases.Create(rel))

	upAction.Wait = true
	vals := map[string]interface{}{}

	ctx, done := context.WithCancel(context.Background())
	res, err := upAction.RunWithContext(ctx, rel.Name, buildChart(), vals)
	done()
	req.NoError(err)
	is.Equal(res.Info.Status, release.StatusDeployed)

	// Detecting previous bug where context termination after successful release
	// caused release to fail.
	time.Sleep(time.Millisecond * 100)
	lastRelease, err := upAction.cfg.Releases.Last(rel.Name)
	req.NoError(err)
	is.Equal(lastRelease.Info.Status, release.StatusDeployed)
}

func TestUpgradeRelease_Wait(t *testing.T) {
	is := assert.New(t)
	req := require.New(t)

	upAction := upgradeAction(t)
	rel := releaseStub()
	rel.Name = "come-fail-away"
	rel.Info.Status = release.StatusDeployed
	upAction.cfg.Releases.Create(rel)

	failer := upAction.cfg.KubeClient.(*kubefake.FailingKubeClient)
	failer.WaitError = fmt.Errorf("I timed out")
	upAction.cfg.KubeClient = failer
	upAction.Wait = true
	vals := map[string]interface{}{}

	res, err := upAction.Run(rel.Name, buildChart(), vals)
	req.Error(err)
	is.Contains(res.Info.Description, "I timed out")
	is.Equal(res.Info.Status, release.StatusFailed)
}

func TestUpgradeRelease_WaitForJobs(t *testing.T) {
	is := assert.New(t)
	req := require.New(t)

	upAction := upgradeAction(t)
	rel := releaseStub()
	rel.Name = "come-fail-away"
	rel.Info.Status = release.StatusDeployed
	upAction.cfg.Releases.Create(rel)

	failer := upAction.cfg.KubeClient.(*kubefake.FailingKubeClient)
	failer.WaitError = fmt.Errorf("I timed out")
	upAction.cfg.KubeClient = failer
	upAction.Wait = true
	upAction.WaitForJobs = true
	vals := map[string]interface{}{}

	res, err := upAction.Run(rel.Name, buildChart(), vals)
	req.Error(err)
	is.Contains(res.Info.Description, "I timed out")
	is.Equal(res.Info.Status, release.StatusFailed)
}

func TestUpgradeRelease_CleanupOnFail(t *testing.T) {
	is := assert.New(t)
	req := require.New(t)

	upAction := upgradeAction(t)
	rel := releaseStub()
	rel.Name = "come-fail-away"
	rel.Info.Status = release.StatusDeployed
	upAction.cfg.Releases.Create(rel)

	failer := upAction.cfg.KubeClient.(*kubefake.FailingKubeClient)
	failer.WaitError = fmt.Errorf("I timed out")
	failer.DeleteError = fmt.Errorf("I tried to delete nil")
	upAction.cfg.KubeClient = failer
	upAction.Wait = true
	upAction.CleanupOnFail = true
	vals := map[string]interface{}{}

	res, err := upAction.Run(rel.Name, buildChart(), vals)
	req.Error(err)
	is.NotContains(err.Error(), "unable to cleanup resources")
	is.Contains(res.Info.Description, "I timed out")
	is.Equal(res.Info.Status, release.StatusFailed)
}

func TestUpgradeRelease_Atomic(t *testing.T) {
	is := assert.New(t)
	req := require.New(t)

	t.Run("atomic rollback succeeds", func(t *testing.T) {
		upAction := upgradeAction(t)

		rel := releaseStub()
		rel.Name = "nuketown"
		rel.Info.Status = release.StatusDeployed
		upAction.cfg.Releases.Create(rel)

		failer := upAction.cfg.KubeClient.(*kubefake.FailingKubeClient)
		// We can't make Update error because then the rollback won't work
		failer.WatchUntilReadyError = fmt.Errorf("arming key removed")
		upAction.cfg.KubeClient = failer
		upAction.Atomic = true
		vals := map[string]interface{}{}

		res, err := upAction.Run(rel.Name, buildChart(), vals)
		req.Error(err)
		is.Contains(err.Error(), "arming key removed")
		is.Contains(err.Error(), "atomic")

		// Now make sure it is actually upgraded
		updatedRes, err := upAction.cfg.Releases.Get(res.Name, 3)
		is.NoError(err)
		// Should have rolled back to the previous
		is.Equal(updatedRes.Info.Status, release.StatusDeployed)
	})

	t.Run("atomic uninstall fails", func(t *testing.T) {
		upAction := upgradeAction(t)
		rel := releaseStub()
		rel.Name = "fallout"
		rel.Info.Status = release.StatusDeployed
		upAction.cfg.Releases.Create(rel)

		failer := upAction.cfg.KubeClient.(*kubefake.FailingKubeClient)
		failer.UpdateError = fmt.Errorf("update fail")
		upAction.cfg.KubeClient = failer
		upAction.Atomic = true
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
		rel.Info.Status = release.StatusDeployed
		rel.Config = existingValues

		err := upAction.cfg.Releases.Create(rel)
		is.NoError(err)

		upAction.ReuseValues = true
		// setting newValues and upgrading
		res, err := upAction.Run(rel.Name, buildChart(), newValues)
		is.NoError(err)

		// Now make sure it is actually upgraded
		updatedRes, err := upAction.cfg.Releases.Get(res.Name, 2)
		is.NoError(err)

		if updatedRes == nil {
			is.Fail("Updated Release is nil")
			return
		}
		is.Equal(release.StatusDeployed, updatedRes.Info.Status)
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
		now := helmtime.Now()
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
				Status:        release.StatusDeployed,
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
		res, err := upAction.Run(rel.Name, sampleChartWithSubChart, map[string]interface{}{})
		is.NoError(err)

		// Now get the upgraded release
		updatedRes, err := upAction.cfg.Releases.Get(res.Name, 2)
		is.NoError(err)

		if updatedRes == nil {
			is.Fail("Updated Release is nil")
			return
		}
		is.Equal(release.StatusDeployed, updatedRes.Info.Status)
		is.Equal(0, len(updatedRes.Chart.Dependencies()), "expected 0 dependencies")

		expectedValues := map[string]interface{}{
			"subchart": map[string]interface{}{
				"enabled": false,
			},
		}
		is.Equal(expectedValues, updatedRes.Config)
	})
}

func TestUpgradeRelease_Pending(t *testing.T) {
	req := require.New(t)

	upAction := upgradeAction(t)
	rel := releaseStub()
	rel.Name = "come-fail-away"
	rel.Info.Status = release.StatusDeployed
	upAction.cfg.Releases.Create(rel)
	rel2 := releaseStub()
	rel2.Name = "come-fail-away"
	rel2.Info.Status = release.StatusPendingUpgrade
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
	rel.Info.Status = release.StatusDeployed
	upAction.cfg.Releases.Create(rel)

	failer := upAction.cfg.KubeClient.(*kubefake.FailingKubeClient)
	failer.WaitDuration = 10 * time.Second
	upAction.cfg.KubeClient = failer
	upAction.Wait = true
	vals := map[string]interface{}{}

	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	time.AfterFunc(time.Second, cancel)

	res, err := upAction.RunWithContext(ctx, rel.Name, buildChart(), vals)

	req.Error(err)
	is.Contains(res.Info.Description, "Upgrade \"interrupted-release\" failed: context canceled")
	is.Equal(res.Info.Status, release.StatusFailed)

}

func TestUpgradeRelease_Interrupted_Atomic(t *testing.T) {

	is := assert.New(t)
	req := require.New(t)

	upAction := upgradeAction(t)
	rel := releaseStub()
	rel.Name = "interrupted-release"
	rel.Info.Status = release.StatusDeployed
	upAction.cfg.Releases.Create(rel)

	failer := upAction.cfg.KubeClient.(*kubefake.FailingKubeClient)
	failer.WaitDuration = 5 * time.Second
	upAction.cfg.KubeClient = failer
	upAction.Atomic = true
	vals := map[string]interface{}{}

	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	time.AfterFunc(time.Second, cancel)

	res, err := upAction.RunWithContext(ctx, rel.Name, buildChart(), vals)

	req.Error(err)
	is.Contains(err.Error(), "release interrupted-release failed, and has been rolled back due to atomic being set: context canceled")

	// Now make sure it is actually upgraded
	updatedRes, err := upAction.cfg.Releases.Get(res.Name, 3)
	is.NoError(err)
	// Should have rolled back to the previous
	is.Equal(updatedRes.Info.Status, release.StatusDeployed)

}
