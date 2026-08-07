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
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"helm.sh/helm/v4/pkg/chart/common"
	chart "helm.sh/helm/v4/pkg/chart/v2"
	"helm.sh/helm/v4/pkg/kube"
	ri "helm.sh/helm/v4/pkg/release"
	rcommon "helm.sh/helm/v4/pkg/release/common"
	release "helm.sh/helm/v4/pkg/release/v1"
	releaseutil "helm.sh/helm/v4/pkg/release/v1/util"
)

func newSequencedUpgradeAction(t *testing.T, kubeClient kube.Interface) *Upgrade {
	t.Helper()
	cfg := actionConfigFixture(t)
	cfg.KubeClient = kubeClient

	upgrade := NewUpgrade(cfg)
	upgrade.Namespace = "spaced"
	upgrade.Timeout = 5 * time.Minute
	upgrade.ReadinessTimeout = time.Minute
	upgrade.WaitStrategy = kube.OrderedWaitStrategy
	return upgrade
}

func seedDeployedRelease(t *testing.T, upgrade *Upgrade, name string, ch *chart.Chart, manifest string, sequencingInfo *release.SequencingInfo) {
	t.Helper()

	now := time.Now()
	rel := &release.Release{
		Name:      name,
		Namespace: "spaced",
		Chart:     ch,
		Config:    map[string]any{},
		Manifest:  manifest,
		Info: &release.Info{
			FirstDeployed: now,
			LastDeployed:  now,
			Status:        rcommon.StatusDeployed,
			Description:   "Seeded release",
		},
		Version:        1,
		SequencingInfo: sequencingInfo,
	}

	require.NoError(t, upgrade.cfg.Releases.Create(rel))
}

func joinManifestDocs(docs ...string) string {
	return strings.Join(docs, "\n---\n")
}

func updateTargets(calls []updateCall) [][]string {
	targets := make([][]string, 0, len(calls))
	for _, call := range calls {
		targets = append(targets, call.target)
	}
	return targets
}

func TestUpgrade_Sequenced_Basic(t *testing.T) {
	client := newRecordingKubeClient()
	upgrade := newSequencedUpgradeAction(t, client)

	currentChart := buildChartWithTemplates([]*common.File{
		makeConfigMapTemplate("templates/database.yaml", "database", map[string]string{
			releaseutil.AnnotationResourceGroup: "database",
		}),
		makeConfigMapTemplate("templates/app.yaml", "app", map[string]string{
			releaseutil.AnnotationResourceGroup:           "app",
			releaseutil.AnnotationDependsOnResourceGroups: `["database"]`,
		}),
	}, withName("sequenced-upgrade"))
	seedDeployedRelease(t, upgrade, "sequenced-upgrade", currentChart, joinManifestDocs(
		configMapManifest("database", map[string]string{releaseutil.AnnotationResourceGroup: "database"}),
		configMapManifest("app", map[string]string{
			releaseutil.AnnotationResourceGroup:           "app",
			releaseutil.AnnotationDependsOnResourceGroups: `["database"]`,
		}),
	), nil)

	upgradedChart := buildChartWithTemplates([]*common.File{
		makeConfigMapTemplate("templates/database.yaml", "database", map[string]string{
			releaseutil.AnnotationResourceGroup: "database",
		}),
		makeConfigMapTemplate("templates/app.yaml", "app", map[string]string{
			releaseutil.AnnotationResourceGroup:           "app",
			releaseutil.AnnotationDependsOnResourceGroups: `["database"]`,
		}),
	}, withName("sequenced-upgrade"))

	rel := mustRelease(t, mustRunUpgrade(t, upgrade, "sequenced-upgrade", upgradedChart))

	assert.Equal(t, [][]string{{"ConfigMap/database"}, {"ConfigMap/app"}}, updateTargets(client.updateCalls))
	assert.Equal(t, [][]string{{"ConfigMap/database"}, {"ConfigMap/app"}}, client.waitCalls)
	require.Len(t, client.updateCalls, 2)
	assert.Equal(t, []string{"ConfigMap/database"}, client.updateCalls[0].current)
	assert.Equal(t, []string{"ConfigMap/app"}, client.updateCalls[1].current)
	assert.Equal(t, rcommon.StatusDeployed, rel.Info.Status)
	assert.True(t, rel.Sequenced)
	assert.True(t, rel.IsSequenced())
	assert.Nil(t, rel.SequencingInfo)
}

func TestUpgrade_Sequenced_NewResourceGroup(t *testing.T) {
	client := newRecordingKubeClient()
	upgrade := newSequencedUpgradeAction(t, client)

	currentChart := buildChartWithTemplates([]*common.File{
		makeConfigMapTemplate("templates/database.yaml", "database", map[string]string{
			releaseutil.AnnotationResourceGroup: "database",
		}),
		makeConfigMapTemplate("templates/app.yaml", "app", map[string]string{
			releaseutil.AnnotationResourceGroup:           "app",
			releaseutil.AnnotationDependsOnResourceGroups: `["database"]`,
		}),
	}, withName("new-group"))
	seedDeployedRelease(t, upgrade, "new-group", currentChart, joinManifestDocs(
		configMapManifest("database", map[string]string{releaseutil.AnnotationResourceGroup: "database"}),
		configMapManifest("app", map[string]string{
			releaseutil.AnnotationResourceGroup:           "app",
			releaseutil.AnnotationDependsOnResourceGroups: `["database"]`,
		}),
	), nil)

	upgradedChart := buildChartWithTemplates([]*common.File{
		makeConfigMapTemplate("templates/database.yaml", "database", map[string]string{
			releaseutil.AnnotationResourceGroup: "database",
		}),
		makeConfigMapTemplate("templates/queue.yaml", "queue", map[string]string{
			releaseutil.AnnotationResourceGroup:           "queue",
			releaseutil.AnnotationDependsOnResourceGroups: `["database"]`,
		}),
		makeConfigMapTemplate("templates/app.yaml", "app", map[string]string{
			releaseutil.AnnotationResourceGroup:           "app",
			releaseutil.AnnotationDependsOnResourceGroups: `["queue"]`,
		}),
	}, withName("new-group"))

	mustRunUpgrade(t, upgrade, "new-group", upgradedChart)

	require.Len(t, client.updateCalls, 3)
	assert.Equal(t, [][]string{{"ConfigMap/database"}, {"ConfigMap/queue"}, {"ConfigMap/app"}}, updateTargets(client.updateCalls))
	assert.Empty(t, client.updateCalls[1].current)
	assert.Equal(t, []string{"ConfigMap/queue"}, client.updateCalls[1].created)
	assert.Equal(t, [][]string{{"ConfigMap/database"}, {"ConfigMap/queue"}, {"ConfigMap/app"}}, client.waitCalls)
}

func TestUpgrade_Sequenced_RemovedResourceGroup(t *testing.T) {
	client := newRecordingKubeClient()
	upgrade := newSequencedUpgradeAction(t, client)

	currentChart := buildChartWithTemplates([]*common.File{
		makeConfigMapTemplate("templates/database.yaml", "database", map[string]string{
			releaseutil.AnnotationResourceGroup: "database",
		}),
		makeConfigMapTemplate("templates/queue.yaml", "queue", map[string]string{
			releaseutil.AnnotationResourceGroup:           "queue",
			releaseutil.AnnotationDependsOnResourceGroups: `["database"]`,
		}),
		makeConfigMapTemplate("templates/app.yaml", "app", map[string]string{
			releaseutil.AnnotationResourceGroup:           "app",
			releaseutil.AnnotationDependsOnResourceGroups: `["queue"]`,
		}),
	}, withName("removed-group"))
	seedDeployedRelease(t, upgrade, "removed-group", currentChart, joinManifestDocs(
		configMapManifest("database", map[string]string{releaseutil.AnnotationResourceGroup: "database"}),
		configMapManifest("queue", map[string]string{
			releaseutil.AnnotationResourceGroup:           "queue",
			releaseutil.AnnotationDependsOnResourceGroups: `["database"]`,
		}),
		configMapManifest("app", map[string]string{
			releaseutil.AnnotationResourceGroup:           "app",
			releaseutil.AnnotationDependsOnResourceGroups: `["queue"]`,
		}),
	), nil)

	upgradedChart := buildChartWithTemplates([]*common.File{
		makeConfigMapTemplate("templates/database.yaml", "database", map[string]string{
			releaseutil.AnnotationResourceGroup: "database",
		}),
		makeConfigMapTemplate("templates/app.yaml", "app", map[string]string{
			releaseutil.AnnotationResourceGroup:           "app",
			releaseutil.AnnotationDependsOnResourceGroups: `["database"]`,
		}),
	}, withName("removed-group"))

	mustRunUpgrade(t, upgrade, "removed-group", upgradedChart)

	assert.Equal(t, [][]string{{"ConfigMap/database"}, {"ConfigMap/app"}}, updateTargets(client.updateCalls))
	assert.Equal(t, [][]string{{"ConfigMap/queue"}}, client.deleteCalls)
	assert.Equal(t, []string{
		"update:ConfigMap/database",
		"wait:ConfigMap/database",
		"update:ConfigMap/app",
		"wait:ConfigMap/app",
		"delete:ConfigMap/queue",
		"wait-delete:ConfigMap/queue",
	}, client.operations)
}

// TestUpgrade_Sequenced_NewResourceConflictAndAdoption locks bead 9ui
// (upgrade): a sequenced upgrade must run the same existing-resource
// conflict / adoption step over newly added resources as the default path
// (performUpgrade) does.
func TestUpgrade_Sequenced_NewResourceConflictAndAdoption(t *testing.T) {
	currentChartFiles := []*common.File{
		makeConfigMapTemplate("templates/database.yaml", "database", map[string]string{
			releaseutil.AnnotationResourceGroup: "database",
		}),
	}
	currentManifest := configMapManifest("database", map[string]string{
		releaseutil.AnnotationResourceGroup: "database",
	})
	upgradedChartFiles := []*common.File{
		makeConfigMapTemplate("templates/database.yaml", "database", map[string]string{
			releaseutil.AnnotationResourceGroup: "database",
		}),
		makeConfigMapTemplate("templates/queue.yaml", "queue", map[string]string{
			releaseutil.AnnotationResourceGroup:           "queue",
			releaseutil.AnnotationDependsOnResourceGroups: `["database"]`,
		}),
	}

	t.Run("unowned existing resource conflicts before anything is applied", func(t *testing.T) {
		client := newRecordingKubeClient()
		// queue already exists in the cluster, owned by another release.
		client.buildRESTClient = statefulOwnedRESTClient(client, "some-other-release", "spaced")
		client.createdObjects["ConfigMap/queue"] = true

		upgrade := newSequencedUpgradeAction(t, client)
		seedDeployedRelease(t, upgrade, "adopt-upgrade", buildChartWithTemplates(currentChartFiles, withName("adopt-upgrade")), currentManifest, nil)

		_, err := upgrade.Run("adopt-upgrade", buildChartWithTemplates(upgradedChartFiles, withName("adopt-upgrade")), map[string]any{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unable to continue with update")
		assert.Empty(t, client.updateCalls, "no batch may be applied after a conflict")
	})

	t.Run("take-ownership adopts the existing resource in its batch", func(t *testing.T) {
		client := newRecordingKubeClient()
		client.buildRESTClient = statefulOwnedRESTClient(client, "some-other-release", "spaced")
		client.createdObjects["ConfigMap/queue"] = true

		upgrade := newSequencedUpgradeAction(t, client)
		upgrade.TakeOwnership = true
		seedDeployedRelease(t, upgrade, "adopt-upgrade-to", buildChartWithTemplates(currentChartFiles, withName("adopt-upgrade-to")), currentManifest, nil)

		mustRunUpgrade(t, upgrade, "adopt-upgrade-to", buildChartWithTemplates(upgradedChartFiles, withName("adopt-upgrade-to")))

		require.Len(t, client.updateCalls, 2)
		assert.Equal(t, []string{"ConfigMap/queue"}, client.updateCalls[1].current,
			"pre-existing queue must be adopted (updated against a current object), not treated as new")
		assert.Empty(t, client.updateCalls[1].created)
	})
}

// TestUpgrade_Sequenced_RemovedGroupsDeletedInReverseOrder locks bead 7yi
// (upgrade): resources removed by an upgrade are deleted in exact reverse
// deployment order of the OLD release's plan, with a delete-wait gating each
// batch, not as one unordered bulk delete.
func TestUpgrade_Sequenced_RemovedGroupsDeletedInReverseOrder(t *testing.T) {
	client := newRecordingKubeClient()
	upgrade := newSequencedUpgradeAction(t, client)

	group := func(name string, deps string) map[string]string {
		ann := map[string]string{releaseutil.AnnotationResourceGroup: name}
		if deps != "" {
			ann[releaseutil.AnnotationDependsOnResourceGroups] = deps
		}
		return ann
	}

	currentChart := buildChartWithTemplates([]*common.File{
		makeConfigMapTemplate("templates/database.yaml", "database", group("database", "")),
		makeConfigMapTemplate("templates/queue.yaml", "queue", group("queue", `["database"]`)),
		makeConfigMapTemplate("templates/worker.yaml", "worker", group("worker", `["queue"]`)),
		makeConfigMapTemplate("templates/app.yaml", "app", group("app", `["worker"]`)),
	}, withName("reverse-removed"))
	seedDeployedRelease(t, upgrade, "reverse-removed", currentChart, joinManifestDocs(
		configMapManifest("database", group("database", "")),
		configMapManifest("queue", group("queue", `["database"]`)),
		configMapManifest("worker", group("worker", `["queue"]`)),
		configMapManifest("app", group("app", `["worker"]`)),
	), nil)

	upgradedChart := buildChartWithTemplates([]*common.File{
		makeConfigMapTemplate("templates/database.yaml", "database", group("database", "")),
		makeConfigMapTemplate("templates/app.yaml", "app", group("app", `["database"]`)),
	}, withName("reverse-removed"))

	mustRunUpgrade(t, upgrade, "reverse-removed", upgradedChart)

	// Old plan order: database, queue, worker, app. Reverse: app, worker,
	// queue, database. Removed set: {queue, worker} -> worker deleted (and
	// delete-waited) strictly before queue.
	assert.Equal(t, [][]string{{"ConfigMap/worker"}, {"ConfigMap/queue"}}, client.deleteCalls)
	assert.Equal(t, [][]string{{"ConfigMap/worker"}, {"ConfigMap/queue"}}, client.deleteWaitCalls)
}

func TestUpgrade_NonSequencedToSequenced(t *testing.T) {
	client := newRecordingKubeClient()
	upgrade := newSequencedUpgradeAction(t, client)

	currentChart := buildChartWithTemplates([]*common.File{
		makeConfigMapTemplate("templates/database.yaml", "database", map[string]string{
			releaseutil.AnnotationResourceGroup: "database",
		}),
		makeConfigMapTemplate("templates/app.yaml", "app", map[string]string{
			releaseutil.AnnotationResourceGroup:           "app",
			releaseutil.AnnotationDependsOnResourceGroups: `["database"]`,
		}),
	}, withName("transition-to-sequenced"))
	seedDeployedRelease(t, upgrade, "transition-to-sequenced", currentChart, joinManifestDocs(
		configMapManifest("database", map[string]string{releaseutil.AnnotationResourceGroup: "database"}),
		configMapManifest("app", map[string]string{
			releaseutil.AnnotationResourceGroup:           "app",
			releaseutil.AnnotationDependsOnResourceGroups: `["database"]`,
		}),
	), nil)

	rel := mustRelease(t, mustRunUpgrade(t, upgrade, "transition-to-sequenced", currentChart))

	require.Len(t, client.updateCalls, 2)
	assert.Equal(t, [][]string{{"ConfigMap/database"}, {"ConfigMap/app"}}, updateTargets(client.updateCalls))
	assert.True(t, rel.Sequenced)
	assert.True(t, rel.IsSequenced())
}

func TestUpgrade_SequencedToNonSequenced(t *testing.T) {
	client := newRecordingKubeClient()
	upgrade := newSequencedUpgradeAction(t, client)
	upgrade.WaitStrategy = kube.StatusWatcherStrategy

	currentChart := buildChartWithTemplates([]*common.File{
		makeConfigMapTemplate("templates/database.yaml", "database", map[string]string{
			releaseutil.AnnotationResourceGroup: "database",
		}),
		makeConfigMapTemplate("templates/app.yaml", "app", map[string]string{
			releaseutil.AnnotationResourceGroup:           "app",
			releaseutil.AnnotationDependsOnResourceGroups: `["database"]`,
		}),
	}, withName("transition-to-standard"))
	seedDeployedRelease(t, upgrade, "transition-to-standard", currentChart, joinManifestDocs(
		configMapManifest("database", map[string]string{releaseutil.AnnotationResourceGroup: "database"}),
		configMapManifest("app", map[string]string{
			releaseutil.AnnotationResourceGroup:           "app",
			releaseutil.AnnotationDependsOnResourceGroups: `["database"]`,
		}),
	), &release.SequencingInfo{Enabled: true, Strategy: string(kube.OrderedWaitStrategy)})

	rel := mustRelease(t, mustRunUpgrade(t, upgrade, "transition-to-standard", currentChart))

	require.Len(t, client.updateCalls, 1)
	assert.ElementsMatch(t, []string{"ConfigMap/database", "ConfigMap/app"}, client.updateCalls[0].target)
	require.Len(t, client.waitCalls, 1)
	assert.ElementsMatch(t, []string{"ConfigMap/database", "ConfigMap/app"}, client.waitCalls[0])
	assert.Nil(t, rel.SequencingInfo)
}

func TestUpgrade_Sequenced_FailureRollback(t *testing.T) {
	client := newRecordingKubeClient()
	client.waitErrorOnCall = 2
	client.waitError = assert.AnError

	upgrade := newSequencedUpgradeAction(t, client)
	upgrade.RollbackOnFailure = true

	currentChart := buildChartWithTemplates([]*common.File{
		makeConfigMapTemplate("templates/database.yaml", "database", map[string]string{
			releaseutil.AnnotationResourceGroup: "database",
		}),
		makeConfigMapTemplate("templates/app.yaml", "app", map[string]string{
			releaseutil.AnnotationResourceGroup:           "app",
			releaseutil.AnnotationDependsOnResourceGroups: `["database"]`,
		}),
	}, withName("failure-rollback"))
	seedDeployedRelease(t, upgrade, "failure-rollback", currentChart, joinManifestDocs(
		configMapManifest("database", map[string]string{releaseutil.AnnotationResourceGroup: "database"}),
		configMapManifest("app", map[string]string{
			releaseutil.AnnotationResourceGroup:           "app",
			releaseutil.AnnotationDependsOnResourceGroups: `["database"]`,
		}),
	), nil)

	upgradedChart := buildChartWithTemplates([]*common.File{
		makeConfigMapTemplate("templates/database.yaml", "database", map[string]string{
			releaseutil.AnnotationResourceGroup: "database",
		}),
		makeConfigMapTemplate("templates/queue.yaml", "queue", map[string]string{
			releaseutil.AnnotationResourceGroup:           "queue",
			releaseutil.AnnotationDependsOnResourceGroups: `["database"]`,
		}),
		makeConfigMapTemplate("templates/app.yaml", "app", map[string]string{
			releaseutil.AnnotationResourceGroup:           "app",
			releaseutil.AnnotationDependsOnResourceGroups: `["queue"]`,
		}),
	}, withName("failure-rollback"))

	rel, err := upgrade.Run("failure-rollback", upgradedChart, map[string]any{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rolled back due to rollback-on-failure")

	failed := mustRelease(t, rel)
	assert.Equal(t, rcommon.StatusFailed, failed.Info.Status)

	last, getErr := upgrade.cfg.Releases.Last("failure-rollback")
	require.NoError(t, getErr)
	assert.Equal(t, rcommon.StatusDeployed, mustRelease(t, last).Info.Status)
}

func TestUpgrade_Sequenced_SubchartAdded(t *testing.T) {
	client := newRecordingKubeClient()
	upgrade := newSequencedUpgradeAction(t, client)

	currentParent := buildChartWithTemplates([]*common.File{
		makeConfigMapTemplate("templates/parent.yaml", "parent", nil),
	}, withName("parent"))
	currentDatabase := buildChartWithTemplates([]*common.File{
		makeConfigMapTemplate("templates/database.yaml", "database", nil),
	}, withName("database"))
	currentParent.AddDependency(currentDatabase)
	currentParent.Metadata.Dependencies = []*chart.Dependency{
		{Name: "database"},
	}
	seedDeployedRelease(t, upgrade, "subchart-added", currentParent, joinManifestDocs(
		configMapManifest("database", nil),
		configMapManifest("parent", nil),
	), nil)

	upgradedParent := buildChartWithTemplates([]*common.File{
		makeConfigMapTemplate("templates/parent.yaml", "parent", nil),
	}, withName("parent"))
	upgradedDatabase := buildChartWithTemplates([]*common.File{
		makeConfigMapTemplate("templates/database.yaml", "database", nil),
	}, withName("database"))
	upgradedAPI := buildChartWithTemplates([]*common.File{
		makeConfigMapTemplate("templates/api.yaml", "api", nil),
	}, withName("api"))
	upgradedParent.AddDependency(upgradedDatabase, upgradedAPI)
	upgradedParent.Metadata.Dependencies = []*chart.Dependency{
		{Name: "database"},
		{Name: "api", DependsOn: []string{"database"}},
	}

	mustRunUpgrade(t, upgrade, "subchart-added", upgradedParent)

	require.Len(t, client.updateCalls, 3)
	assert.Equal(t, [][]string{{"ConfigMap/database"}, {"ConfigMap/api"}, {"ConfigMap/parent"}}, updateTargets(client.updateCalls))
	assert.Empty(t, client.updateCalls[1].current)
	assert.Equal(t, []string{"ConfigMap/api"}, client.updateCalls[1].created)
}

func TestUpgrade_Sequenced_DryRun(t *testing.T) {
	client := newRecordingKubeClient()
	upgrade := newSequencedUpgradeAction(t, client)
	upgrade.DryRunStrategy = DryRunClient

	currentChart := buildChartWithTemplates([]*common.File{
		makeConfigMapTemplate("templates/database.yaml", "database", map[string]string{
			releaseutil.AnnotationResourceGroup: "database",
		}),
		makeConfigMapTemplate("templates/app.yaml", "app", map[string]string{
			releaseutil.AnnotationResourceGroup:           "app",
			releaseutil.AnnotationDependsOnResourceGroups: `["database"]`,
		}),
	}, withName("dry-run"))
	seedDeployedRelease(t, upgrade, "dry-run", currentChart, joinManifestDocs(
		configMapManifest("database", map[string]string{releaseutil.AnnotationResourceGroup: "database"}),
		configMapManifest("app", map[string]string{
			releaseutil.AnnotationResourceGroup:           "app",
			releaseutil.AnnotationDependsOnResourceGroups: `["database"]`,
		}),
	), nil)

	rel := mustRelease(t, mustRunUpgrade(t, upgrade, "dry-run", currentChart))

	assert.Empty(t, client.updateCalls)
	assert.Empty(t, client.waitCalls)
	assert.Empty(t, client.deleteCalls)
	assert.Equal(t, "Dry run complete", rel.Info.Description)
}

func TestUpgrade_Sequenced_CleanupOnFail(t *testing.T) {
	client := newRecordingKubeClient()
	client.waitErrorOnCall = 2
	client.waitError = errors.New("timed out waiting for batch")

	upgrade := newSequencedUpgradeAction(t, client)
	upgrade.CleanupOnFail = true

	currentChart := buildChartWithTemplates([]*common.File{
		makeConfigMapTemplate("templates/database.yaml", "database", map[string]string{
			releaseutil.AnnotationResourceGroup: "database",
		}),
		makeConfigMapTemplate("templates/app.yaml", "app", map[string]string{
			releaseutil.AnnotationResourceGroup:           "app",
			releaseutil.AnnotationDependsOnResourceGroups: `["database"]`,
		}),
	}, withName("cleanup-on-fail"))
	seedDeployedRelease(t, upgrade, "cleanup-on-fail", currentChart, joinManifestDocs(
		configMapManifest("database", map[string]string{releaseutil.AnnotationResourceGroup: "database"}),
		configMapManifest("app", map[string]string{
			releaseutil.AnnotationResourceGroup:           "app",
			releaseutil.AnnotationDependsOnResourceGroups: `["database"]`,
		}),
	), nil)

	upgradedChart := buildChartWithTemplates([]*common.File{
		makeConfigMapTemplate("templates/database.yaml", "database", map[string]string{
			releaseutil.AnnotationResourceGroup: "database",
		}),
		makeConfigMapTemplate("templates/queue.yaml", "queue", map[string]string{
			releaseutil.AnnotationResourceGroup:           "queue",
			releaseutil.AnnotationDependsOnResourceGroups: `["database"]`,
		}),
		makeConfigMapTemplate("templates/app.yaml", "app", map[string]string{
			releaseutil.AnnotationResourceGroup:           "app",
			releaseutil.AnnotationDependsOnResourceGroups: `["queue"]`,
		}),
	}, withName("cleanup-on-fail"))

	rel, err := upgrade.Run("cleanup-on-fail", upgradedChart, map[string]any{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "timed out waiting for batch")
	assert.Equal(t, [][]string{{"ConfigMap/queue"}}, client.deleteCalls)
	assert.Equal(t, rcommon.StatusFailed, mustRelease(t, rel).Info.Status)
}

func mustRunUpgrade(t *testing.T, upgrade *Upgrade, name string, ch *chart.Chart) ri.Releaser {
	t.Helper()
	rel, err := upgrade.Run(name, ch, map[string]any{})
	require.NoError(t, err)
	return rel
}
