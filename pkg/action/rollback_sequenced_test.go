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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"helm.sh/helm/v4/pkg/chart/common"
	chart "helm.sh/helm/v4/pkg/chart/v2"
	"helm.sh/helm/v4/pkg/kube"
	rcommon "helm.sh/helm/v4/pkg/release/common"
	release "helm.sh/helm/v4/pkg/release/v1"
	releaseutil "helm.sh/helm/v4/pkg/release/v1/util"
)

func newRollbackAction(t *testing.T, kubeClient kube.Interface) *Rollback {
	t.Helper()

	cfg := actionConfigFixture(t)
	cfg.KubeClient = kubeClient

	rollback := NewRollback(cfg)
	rollback.Timeout = 5 * time.Minute
	rollback.WaitStrategy = kube.OrderedWaitStrategy
	rollback.ServerSideApply = "auto"

	return rollback
}

func seedRollbackRelease(t *testing.T, rollback *Rollback, name string, version int, status rcommon.Status, ch *chart.Chart, manifest string, sequencingInfo *release.SequencingInfo) {
	t.Helper()

	now := time.Now()
	rel := &release.Release{
		Name:      name,
		Namespace: "spaced",
		Chart:     ch,
		Config:    map[string]interface{}{},
		Manifest:  manifest,
		Info: &release.Info{
			FirstDeployed: now,
			LastDeployed:  now,
			Status:        status,
			Description:   "Seeded release",
		},
		Version:        version,
		ApplyMethod:    "csa",
		SequencingInfo: sequencingInfo,
	}

	require.NoError(t, rollback.cfg.Releases.Create(rel))
}

func storedRollbackRelease(t *testing.T, rollback *Rollback, name string, version int) *release.Release {
	t.Helper()

	reli, err := rollback.cfg.Releases.Get(name, version)
	require.NoError(t, err)

	rel, err := releaserToV1Release(reli)
	require.NoError(t, err)

	return rel
}

func latestRollbackRelease(t *testing.T, rollback *Rollback, name string) *release.Release {
	t.Helper()

	reli, err := rollback.cfg.Releases.Last(name)
	require.NoError(t, err)

	rel, err := releaserToV1Release(reli)
	require.NoError(t, err)

	return rel
}

func TestRollback_Sequenced_Basic(t *testing.T) {
	client := newRecordingKubeClient()
	rollback := newRollbackAction(t, client)

	targetChart := buildChartWithTemplates([]*common.File{
		makeConfigMapTemplate("templates/database.yaml", "database", map[string]string{
			releaseutil.AnnotationResourceGroup: "database",
		}),
		makeConfigMapTemplate("templates/app.yaml", "app", map[string]string{
			releaseutil.AnnotationResourceGroup:           "app",
			releaseutil.AnnotationDependsOnResourceGroups: `["database"]`,
		}),
	}, withName("rollback-basic"))

	sequencingInfo := &release.SequencingInfo{Enabled: true, Strategy: string(kube.OrderedWaitStrategy)}
	seedRollbackRelease(t, rollback, "rollback-basic", 1, rcommon.StatusSuperseded, targetChart, joinManifestDocs(
		configMapManifest("database", map[string]string{releaseutil.AnnotationResourceGroup: "database"}),
		configMapManifest("app", map[string]string{
			releaseutil.AnnotationResourceGroup:           "app",
			releaseutil.AnnotationDependsOnResourceGroups: `["database"]`,
		}),
	), sequencingInfo)

	currentChart := buildChartWithTemplates([]*common.File{
		makeConfigMapTemplate("templates/database.yaml", "database", nil),
		makeConfigMapTemplate("templates/app.yaml", "app", nil),
	}, withName("rollback-basic"))
	seedRollbackRelease(t, rollback, "rollback-basic", 2, rcommon.StatusDeployed, currentChart, joinManifestDocs(
		configMapManifest("database", nil),
		configMapManifest("app", nil),
	), nil)

	rollback.Version = 1
	require.NoError(t, rollback.Run("rollback-basic"))

	require.Len(t, client.updateCalls, 2)
	assert.Equal(t, [][]string{{"ConfigMap/database"}, {"ConfigMap/app"}}, updateTargets(client.updateCalls))
	assert.Equal(t, [][]string{{"ConfigMap/database"}, {"ConfigMap/app"}}, client.waitCalls)
	assert.Equal(t, []string{"ConfigMap/database"}, client.updateCalls[0].current)
	assert.Equal(t, []string{"ConfigMap/app"}, client.updateCalls[1].current)

	rel := latestRollbackRelease(t, rollback, "rollback-basic")
	assert.Equal(t, 3, rel.Version)
	assert.Equal(t, rcommon.StatusDeployed, rel.Info.Status)
	require.NotNil(t, rel.SequencingInfo)
	assert.Equal(t, *sequencingInfo, *rel.SequencingInfo)
}

func TestRollback_ToNonSequenced(t *testing.T) {
	client := newRecordingKubeClient()
	rollback := newRollbackAction(t, client)

	targetChart := buildChartWithTemplates([]*common.File{
		makeConfigMapTemplate("templates/database.yaml", "database", map[string]string{
			releaseutil.AnnotationResourceGroup: "database",
		}),
		makeConfigMapTemplate("templates/app.yaml", "app", map[string]string{
			releaseutil.AnnotationResourceGroup:           "app",
			releaseutil.AnnotationDependsOnResourceGroups: `["database"]`,
		}),
	}, withName("rollback-to-standard"))
	seedRollbackRelease(t, rollback, "rollback-to-standard", 1, rcommon.StatusSuperseded, targetChart, joinManifestDocs(
		configMapManifest("database", map[string]string{releaseutil.AnnotationResourceGroup: "database"}),
		configMapManifest("app", map[string]string{
			releaseutil.AnnotationResourceGroup:           "app",
			releaseutil.AnnotationDependsOnResourceGroups: `["database"]`,
		}),
	), nil)

	currentChart := buildChartWithTemplates([]*common.File{
		makeConfigMapTemplate("templates/database.yaml", "database", nil),
		makeConfigMapTemplate("templates/app.yaml", "app", nil),
	}, withName("rollback-to-standard"))
	seedRollbackRelease(t, rollback, "rollback-to-standard", 2, rcommon.StatusDeployed, currentChart, joinManifestDocs(
		configMapManifest("database", nil),
		configMapManifest("app", nil),
	), nil)

	rollback.Version = 1
	require.NoError(t, rollback.Run("rollback-to-standard"))

	require.Len(t, client.updateCalls, 1)
	assert.ElementsMatch(t, []string{"ConfigMap/database", "ConfigMap/app"}, client.updateCalls[0].target)
	require.Len(t, client.waitCalls, 1)
	assert.ElementsMatch(t, []string{"ConfigMap/database", "ConfigMap/app"}, client.waitCalls[0])
	assert.Nil(t, latestRollbackRelease(t, rollback, "rollback-to-standard").SequencingInfo)
}

func TestRollback_FromSequencedToNonSequenced(t *testing.T) {
	client := newRecordingKubeClient()
	rollback := newRollbackAction(t, client)

	targetChart := buildChartWithTemplates([]*common.File{
		makeConfigMapTemplate("templates/database.yaml", "database", nil),
		makeConfigMapTemplate("templates/app.yaml", "app", nil),
	}, withName("rollback-transition"))
	seedRollbackRelease(t, rollback, "rollback-transition", 1, rcommon.StatusSuperseded, targetChart, joinManifestDocs(
		configMapManifest("database", nil),
		configMapManifest("app", nil),
	), nil)

	currentChart := buildChartWithTemplates([]*common.File{
		makeConfigMapTemplate("templates/database.yaml", "database", map[string]string{
			releaseutil.AnnotationResourceGroup: "database",
		}),
		makeConfigMapTemplate("templates/app.yaml", "app", map[string]string{
			releaseutil.AnnotationResourceGroup:           "app",
			releaseutil.AnnotationDependsOnResourceGroups: `["database"]`,
		}),
	}, withName("rollback-transition"))
	seedRollbackRelease(t, rollback, "rollback-transition", 2, rcommon.StatusDeployed, currentChart, joinManifestDocs(
		configMapManifest("database", map[string]string{releaseutil.AnnotationResourceGroup: "database"}),
		configMapManifest("app", map[string]string{
			releaseutil.AnnotationResourceGroup:           "app",
			releaseutil.AnnotationDependsOnResourceGroups: `["database"]`,
		}),
	), &release.SequencingInfo{Enabled: true, Strategy: string(kube.OrderedWaitStrategy)})

	rollback.Version = 1
	require.NoError(t, rollback.Run("rollback-transition"))

	require.Len(t, client.updateCalls, 1)
	assert.ElementsMatch(t, []string{"ConfigMap/database", "ConfigMap/app"}, client.updateCalls[0].target)
	assert.Nil(t, latestRollbackRelease(t, rollback, "rollback-transition").SequencingInfo)
}

func TestRollback_SequencingInfoPreserved(t *testing.T) {
	client := newRecordingKubeClient()
	rollback := newRollbackAction(t, client)

	targetChart := buildChartWithTemplates([]*common.File{
		makeConfigMapTemplate("templates/database.yaml", "database", map[string]string{
			releaseutil.AnnotationResourceGroup: "database",
		}),
	}, withName("rollback-sequencing-info"))
	sequencingInfo := &release.SequencingInfo{Enabled: true, Strategy: string(kube.OrderedWaitStrategy)}
	seedRollbackRelease(t, rollback, "rollback-sequencing-info", 1, rcommon.StatusSuperseded, targetChart, configMapManifest("database", map[string]string{
		releaseutil.AnnotationResourceGroup: "database",
	}), sequencingInfo)

	currentChart := buildChartWithTemplates([]*common.File{
		makeConfigMapTemplate("templates/database.yaml", "database", nil),
	}, withName("rollback-sequencing-info"))
	seedRollbackRelease(t, rollback, "rollback-sequencing-info", 2, rcommon.StatusDeployed, currentChart, configMapManifest("database", nil), nil)

	rollback.Version = 1
	require.NoError(t, rollback.Run("rollback-sequencing-info"))

	rel := latestRollbackRelease(t, rollback, "rollback-sequencing-info")
	require.NotNil(t, rel.SequencingInfo)
	assert.Equal(t, *sequencingInfo, *rel.SequencingInfo)
}

func TestRollback_Sequenced_Failure(t *testing.T) {
	client := newRecordingKubeClient()
	client.waitErrorOnCall = 2
	client.waitError = errors.New("timed out waiting for batch")

	rollback := newRollbackAction(t, client)

	targetChart := buildChartWithTemplates([]*common.File{
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
	}, withName("rollback-failure"))
	seedRollbackRelease(t, rollback, "rollback-failure", 1, rcommon.StatusSuperseded, targetChart, joinManifestDocs(
		configMapManifest("database", map[string]string{releaseutil.AnnotationResourceGroup: "database"}),
		configMapManifest("queue", map[string]string{
			releaseutil.AnnotationResourceGroup:           "queue",
			releaseutil.AnnotationDependsOnResourceGroups: `["database"]`,
		}),
		configMapManifest("app", map[string]string{
			releaseutil.AnnotationResourceGroup:           "app",
			releaseutil.AnnotationDependsOnResourceGroups: `["queue"]`,
		}),
	), &release.SequencingInfo{Enabled: true, Strategy: string(kube.OrderedWaitStrategy)})

	currentChart := buildChartWithTemplates([]*common.File{
		makeConfigMapTemplate("templates/database.yaml", "database", nil),
		makeConfigMapTemplate("templates/app.yaml", "app", nil),
	}, withName("rollback-failure"))
	seedRollbackRelease(t, rollback, "rollback-failure", 2, rcommon.StatusDeployed, currentChart, joinManifestDocs(
		configMapManifest("database", nil),
		configMapManifest("app", nil),
	), nil)

	rollback.Version = 1
	err := rollback.Run("rollback-failure")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "timed out waiting for batch")

	require.Len(t, client.updateCalls, 2)
	assert.Equal(t, [][]string{{"ConfigMap/database"}, {"ConfigMap/queue"}}, updateTargets(client.updateCalls))
	assert.Empty(t, client.deleteCalls)

	assert.Equal(t, rcommon.StatusFailed, latestRollbackRelease(t, rollback, "rollback-failure").Info.Status)
	assert.Equal(t, rcommon.StatusSuperseded, storedRollbackRelease(t, rollback, "rollback-failure", 2).Info.Status)
}

func TestRollback_Sequenced_CleanupOnFail(t *testing.T) {
	client := newRecordingKubeClient()
	client.waitErrorOnCall = 2
	client.waitError = errors.New("timed out waiting for batch")

	rollback := newRollbackAction(t, client)
	rollback.CleanupOnFail = true

	targetChart := buildChartWithTemplates([]*common.File{
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
	}, withName("rollback-cleanup"))
	seedRollbackRelease(t, rollback, "rollback-cleanup", 1, rcommon.StatusSuperseded, targetChart, joinManifestDocs(
		configMapManifest("database", map[string]string{releaseutil.AnnotationResourceGroup: "database"}),
		configMapManifest("queue", map[string]string{
			releaseutil.AnnotationResourceGroup:           "queue",
			releaseutil.AnnotationDependsOnResourceGroups: `["database"]`,
		}),
		configMapManifest("app", map[string]string{
			releaseutil.AnnotationResourceGroup:           "app",
			releaseutil.AnnotationDependsOnResourceGroups: `["queue"]`,
		}),
	), &release.SequencingInfo{Enabled: true, Strategy: string(kube.OrderedWaitStrategy)})

	currentChart := buildChartWithTemplates([]*common.File{
		makeConfigMapTemplate("templates/database.yaml", "database", nil),
		makeConfigMapTemplate("templates/app.yaml", "app", nil),
	}, withName("rollback-cleanup"))
	seedRollbackRelease(t, rollback, "rollback-cleanup", 2, rcommon.StatusDeployed, currentChart, joinManifestDocs(
		configMapManifest("database", nil),
		configMapManifest("app", nil),
	), nil)

	rollback.Version = 1
	err := rollback.Run("rollback-cleanup")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "timed out waiting for batch")
	assert.Equal(t, [][]string{{"ConfigMap/queue"}}, client.deleteCalls)
	assert.Equal(t, rcommon.StatusFailed, latestRollbackRelease(t, rollback, "rollback-cleanup").Info.Status)
}

func TestRollback_Sequenced_DryRun(t *testing.T) {
	client := newRecordingKubeClient()
	rollback := newRollbackAction(t, client)
	rollback.DryRunStrategy = DryRunClient

	targetChart := buildChartWithTemplates([]*common.File{
		makeConfigMapTemplate("templates/database.yaml", "database", map[string]string{
			releaseutil.AnnotationResourceGroup: "database",
		}),
		makeConfigMapTemplate("templates/app.yaml", "app", map[string]string{
			releaseutil.AnnotationResourceGroup:           "app",
			releaseutil.AnnotationDependsOnResourceGroups: `["database"]`,
		}),
	}, withName("rollback-dry-run"))
	seedRollbackRelease(t, rollback, "rollback-dry-run", 1, rcommon.StatusSuperseded, targetChart, joinManifestDocs(
		configMapManifest("database", map[string]string{releaseutil.AnnotationResourceGroup: "database"}),
		configMapManifest("app", map[string]string{
			releaseutil.AnnotationResourceGroup:           "app",
			releaseutil.AnnotationDependsOnResourceGroups: `["database"]`,
		}),
	), &release.SequencingInfo{Enabled: true, Strategy: string(kube.OrderedWaitStrategy)})

	currentChart := buildChartWithTemplates([]*common.File{
		makeConfigMapTemplate("templates/database.yaml", "database", nil),
		makeConfigMapTemplate("templates/app.yaml", "app", nil),
	}, withName("rollback-dry-run"))
	seedRollbackRelease(t, rollback, "rollback-dry-run", 2, rcommon.StatusDeployed, currentChart, joinManifestDocs(
		configMapManifest("database", nil),
		configMapManifest("app", nil),
	), nil)

	rollback.Version = 1
	require.NoError(t, rollback.Run("rollback-dry-run"))

	assert.Empty(t, client.updateCalls)
	assert.Empty(t, client.waitCalls)
	assert.Empty(t, client.deleteCalls)
	assert.Equal(t, 2, latestRollbackRelease(t, rollback, "rollback-dry-run").Version)
}
