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
	"testing"

	"github.com/stretchr/testify/assert"

	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/release"
	helmtime "helm.sh/helm/v3/pkg/time"
)

func rollbackAction(t *testing.T) *Rollback {
	config := actionConfigFixture(t)
	rollAction := NewRollback(config)
	return rollAction
}

func TestRollbackRelease_HookParallelism(t *testing.T) {
	is := assert.New(t)
	t.Run("hook parallelism of 0 defaults to 1", func(t *testing.T) {
		rollAction := rollbackAction(t)
		rollAction.HookParallelism = 0
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
		rel1 := &release.Release{
			Name: "nuketown",
			Info: &release.Info{
				FirstDeployed: now,
				LastDeployed:  now,
				Status:        release.StatusDeployed,
				Description:   "Release 1",
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
						release.HookPreRollback,
						release.HookPostRollback,
					},
				},
			},
		}
		rel2 := &release.Release{
			Name: "nuketown",
			Info: &release.Info{
				FirstDeployed: now,
				LastDeployed:  now,
				Status:        release.StatusDeployed,
				Description:   "Release 2",
			},
			Chart:   sampleChart,
			Version: 2,
		}
		err := rollAction.cfg.Releases.Create(rel1)
		is.NoError(err)
		err = rollAction.cfg.Releases.Create(rel2)
		is.NoError(err)
		err = rollAction.Run(rel2.Name)
		if err != nil {
			t.Fatalf("Failed rollback: %s", err)
		}

		rel, err := rollAction.cfg.Releases.Get(rel1.Name, 3)
		is.NoError(err)

		is.Equal("nuketown", rel.Name, "Expected release name.")
		is.Len(rel.Hooks, 1)
		is.Equal(manifestWithHook, rel.Hooks[0].Manifest)
		is.Equal(release.HookPreRollback, rel.Hooks[0].Events[0])
		is.Equal(release.HookPostRollback, rel.Hooks[0].Events[1])
		is.Equal(0, len(rel.Manifest))
		is.Equal("Rollback to 1", rel.Info.Description)
	})

	t.Run("hook parallelism greater than number of hooks", func(t *testing.T) {
		rollAction := rollbackAction(t)
		rollAction.HookParallelism = 10
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
		rel1 := &release.Release{
			Name: "nuketown",
			Info: &release.Info{
				FirstDeployed: now,
				LastDeployed:  now,
				Status:        release.StatusDeployed,
				Description:   "Release 1",
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
						release.HookPreRollback,
						release.HookPostRollback,
					},
				},
			},
		}
		rel2 := &release.Release{
			Name: "nuketown",
			Info: &release.Info{
				FirstDeployed: now,
				LastDeployed:  now,
				Status:        release.StatusDeployed,
				Description:   "Release 2",
			},
			Chart:   sampleChart,
			Version: 2,
		}
		err := rollAction.cfg.Releases.Create(rel1)
		is.NoError(err)
		err = rollAction.cfg.Releases.Create(rel2)
		is.NoError(err)
		err = rollAction.Run(rel2.Name)
		if err != nil {
			t.Fatalf("Failed rollback: %s", err)
		}

		rel, err := rollAction.cfg.Releases.Get(rel1.Name, 3)
		is.NoError(err)

		is.Equal("nuketown", rel.Name, "Expected release name.")
		is.Len(rel.Hooks, 1)
		is.Equal(manifestWithHook, rel.Hooks[0].Manifest)
		is.Equal(release.HookPreRollback, rel.Hooks[0].Events[0])
		is.Equal(release.HookPostRollback, rel.Hooks[0].Events[1])
		is.Equal(0, len(rel.Manifest))
		is.Equal("Rollback to 1", rel.Info.Description)
	})

	t.Run("hook parallelism with multiple hooks", func(t *testing.T) {
		rollAction := rollbackAction(t)
		rollAction.HookParallelism = 10
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
		now := helmtime.Now()
		rel1 := &release.Release{
			Name: "nuketown",
			Info: &release.Info{
				FirstDeployed: now,
				LastDeployed:  now,
				Status:        release.StatusDeployed,
				Description:   "Release 1",
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
						release.HookPreRollback,
						release.HookPostRollback,
					},
				},
				{
					Name:     "test-cm",
					Kind:     "ConfigMap",
					Path:     "test-cm",
					Manifest: manifestWithHook,
					Events: []release.HookEvent{
						release.HookPreRollback,
						release.HookPostRollback,
					},
				},
			},
		}
		rel2 := &release.Release{
			Name: "nuketown",
			Info: &release.Info{
				FirstDeployed: now,
				LastDeployed:  now,
				Status:        release.StatusDeployed,
				Description:   "Release 2",
			},
			Chart:   sampleChart,
			Version: 2,
		}
		err := rollAction.cfg.Releases.Create(rel1)
		is.NoError(err)
		err = rollAction.cfg.Releases.Create(rel2)
		is.NoError(err)
		err = rollAction.Run(rel2.Name)
		if err != nil {
			t.Fatalf("Failed rollback: %s", err)
		}

		rel, err := rollAction.cfg.Releases.Get(rel1.Name, 3)
		is.NoError(err)

		is.Equal("nuketown", rel.Name, "Expected release name.")
		is.Len(rel.Hooks, 2)
		is.Equal(manifestWithHook, rel.Hooks[0].Manifest)
		is.Equal(release.HookPreRollback, rel.Hooks[0].Events[0])
		is.Equal(release.HookPostRollback, rel.Hooks[0].Events[1])
		is.Equal(manifestWithHook, rel.Hooks[1].Manifest)
		is.Equal(release.HookPreRollback, rel.Hooks[1].Events[0])
		is.Equal(release.HookPostRollback, rel.Hooks[1].Events[1])
		is.Equal(0, len(rel.Manifest))
		is.Equal("Rollback to 1", rel.Info.Description)
	})
}
