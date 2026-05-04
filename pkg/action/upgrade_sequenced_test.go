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
	require.NotNil(t, rel.SequencingInfo)
	assert.True(t, rel.SequencingInfo.Enabled)
	assert.Equal(t, string(kube.OrderedWaitStrategy), rel.SequencingInfo.Strategy)
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
	}, client.operations)
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
	require.NotNil(t, rel.SequencingInfo)
	assert.True(t, rel.SequencingInfo.Enabled)
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
