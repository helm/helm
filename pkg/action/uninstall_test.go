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
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"helm.sh/helm/v3/pkg/chart"
	kubefake "helm.sh/helm/v3/pkg/kube/fake"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/time"
)

func uninstallAction(t *testing.T) *Uninstall {
	config := actionConfigFixture(t)
	unAction := NewUninstall(config)
	return unAction
}

func TestUninstallRelease_deleteRelease(t *testing.T) {
	is := assert.New(t)

	unAction := uninstallAction(t)
	unAction.DisableHooks = true
	unAction.DryRun = false
	unAction.KeepHistory = true

	rel := releaseStub()
	rel.Name = "keep-secret"
	rel.Manifest = `{
		"apiVersion": "v1",
		"kind": "Secret",
		"metadata": {
		  "name": "secret",
		  "annotations": {
			"helm.sh/resource-policy": "keep"
		  }
		},
		"type": "Opaque",
		"data": {
		  "password": "password"
		}
	}`
	unAction.cfg.Releases.Create(rel)
	res, err := unAction.Run(rel.Name)
	is.NoError(err)
	expected := `These resources were kept due to the resource policy:
[Secret] secret
`
	is.Contains(res.Info, expected)
}

func TestUninstallRelease_Wait(t *testing.T) {
	is := assert.New(t)

	unAction := uninstallAction(t)
	unAction.DisableHooks = true
	unAction.DryRun = false
	unAction.Wait = true

	rel := releaseStub()
	rel.Name = "come-fail-away"
	rel.Manifest = `{
		"apiVersion": "v1",
		"kind": "Secret",
		"metadata": {
		  "name": "secret"
		},
		"type": "Opaque",
		"data": {
		  "password": "password"
		}
	}`
	unAction.cfg.Releases.Create(rel)
	failer := unAction.cfg.KubeClient.(*kubefake.FailingKubeClient)
	failer.WaitError = fmt.Errorf("U timed out")
	unAction.cfg.KubeClient = failer
	res, err := unAction.Run(rel.Name)
	is.Error(err)
	is.Contains(err.Error(), "U timed out")
	is.Equal(res.Release.Info.Status, release.StatusUninstalled)
}

func TestUninstallRelease_HookParallelism(t *testing.T) {
	is := assert.New(t)
	t.Run("hook parallelism of 0 defaults to 1", func(t *testing.T) {
		unAction := uninstallAction(t)
		unAction.HookParallelism = 0
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
		rel := &release.Release{
			Name: "nuketown",
			Info: &release.Info{
				FirstDeployed: now,
				LastDeployed:  now,
				Status:        release.StatusDeployed,
				Description:   "Named Release Stub",
			},
			Chart:   sampleChart,
			Version: 1,
			Hooks: []*release.Hook{
				{
					Name:     "test-cm",
					Kind:     "ConfigMap",
					Path:     "test-cm",
					Manifest: manifestWithHook,
					Events: []release.HookEvent{
						release.HookPreDelete,
						release.HookPostDelete,
					},
				},
			},
		}

		err := unAction.cfg.Releases.Create(rel)
		is.NoError(err)
		res, err := unAction.Run(rel.Name)
		if err != nil {
			t.Fatalf("Failed uninstall: %s", err)
		}
		is.Equal("nuketown", res.Release.Name, "Expected release name.")
		is.Len(res.Release.Hooks, 1)
		is.Equal(manifestWithHook, res.Release.Hooks[0].Manifest)
		is.Equal(release.HookPreDelete, res.Release.Hooks[0].Events[0])
		is.Equal(release.HookPostDelete, res.Release.Hooks[0].Events[1])
		is.Equal(0, len(res.Release.Manifest))
		is.Equal("Uninstallation complete", res.Release.Info.Description)
	})

	t.Run("hook parallelism greater than number of hooks", func(t *testing.T) {
		unAction := uninstallAction(t)
		unAction.HookParallelism = 10
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
		rel := &release.Release{
			Name: "nuketown",
			Info: &release.Info{
				FirstDeployed: now,
				LastDeployed:  now,
				Status:        release.StatusDeployed,
				Description:   "Named Release Stub",
			},
			Chart:   sampleChart,
			Version: 1,
			Hooks: []*release.Hook{
				{
					Name:     "test-cm",
					Kind:     "ConfigMap",
					Path:     "test-cm",
					Manifest: manifestWithHook,
					Events: []release.HookEvent{
						release.HookPreDelete,
						release.HookPostDelete,
					},
				},
			},
		}

		err := unAction.cfg.Releases.Create(rel)
		is.NoError(err)
		res, err := unAction.Run(rel.Name)
		if err != nil {
			t.Fatalf("Failed uninstall: %s", err)
		}
		is.Equal("nuketown", res.Release.Name, "Expected release name.")
		is.Len(res.Release.Hooks, 1)
		is.Equal(manifestWithHook, res.Release.Hooks[0].Manifest)
		is.Equal(release.HookPreDelete, res.Release.Hooks[0].Events[0])
		is.Equal(release.HookPostDelete, res.Release.Hooks[0].Events[1])
		is.Equal(0, len(res.Release.Manifest))
		is.Equal("Uninstallation complete", res.Release.Info.Description)
	})

	t.Run("hook parallelism with multiple hooks", func(t *testing.T) {
		unAction := uninstallAction(t)
		unAction.HookParallelism = 2
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
			withSecondHook(manifestWithHook),
		)
		now := time.Now()
		rel := &release.Release{
			Name: "nuketown",
			Info: &release.Info{
				FirstDeployed: now,
				LastDeployed:  now,
				Status:        release.StatusDeployed,
				Description:   "Named Release Stub",
			},
			Chart:   sampleChart,
			Version: 1,
			Hooks: []*release.Hook{
				{
					Name:     "test-cm",
					Kind:     "ConfigMap",
					Path:     "test-cm",
					Manifest: manifestWithHook,
					Events: []release.HookEvent{
						release.HookPreDelete,
						release.HookPostDelete,
					},
				},
				{
					Name:     "test-cm",
					Kind:     "ConfigMap",
					Path:     "test-cm",
					Manifest: manifestWithHook,
					Events: []release.HookEvent{
						release.HookPreDelete,
						release.HookPostDelete,
					},
				},
			},
		}

		err := unAction.cfg.Releases.Create(rel)
		is.NoError(err)
		res, err := unAction.Run(rel.Name)
		if err != nil {
			t.Fatalf("Failed uninstall: %s", err)
		}
		is.Equal("nuketown", res.Release.Name, "Expected release name.")
		is.Len(rel.Hooks, 2)
		is.Equal(manifestWithHook, res.Release.Hooks[0].Manifest)
		is.Equal(release.HookPreDelete, res.Release.Hooks[0].Events[0])
		is.Equal(release.HookPostDelete, res.Release.Hooks[0].Events[1])
		is.Equal(manifestWithHook, res.Release.Hooks[1].Manifest)
		is.Equal(release.HookPreDelete, res.Release.Hooks[1].Events[0])
		is.Equal(release.HookPostDelete, res.Release.Hooks[1].Events[1])
		is.Equal(0, len(res.Release.Manifest))
		is.Equal("Uninstallation complete", res.Release.Info.Description)
	})
}
