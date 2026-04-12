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
	"github.com/stretchr/testify/require"

	"helm.sh/helm/v4/pkg/chart/common"
	"helm.sh/helm/v4/pkg/kube"
	rcommon "helm.sh/helm/v4/pkg/release/common"
	release "helm.sh/helm/v4/pkg/release/v1"
	releaseutil "helm.sh/helm/v4/pkg/release/v1/util"
)

func TestBackwardCompat_OrderedWaitWithoutAnnotations(t *testing.T) {
	client := newRecordingKubeClient()
	install := newSequencedInstallAction(t, client)
	buf := captureWarningOutput(t)

	ch := buildChartWithTemplates([]*common.File{
		makeConfigMapTemplate("templates/first.yaml", "first", nil),
		makeConfigMapTemplate("templates/second.yaml", "second", nil),
	}, withName("backward-compat-no-annotations"))

	rel := mustRelease(t, mustRunInstall(t, install, ch))

	require.Len(t, client.createCalls, 1)
	assert.ElementsMatch(t, []string{"ConfigMap/first", "ConfigMap/second"}, client.createCalls[0])
	require.Len(t, client.waitCalls, 1)
	assert.ElementsMatch(t, client.createCalls[0], client.waitCalls[0])
	assert.Empty(t, buf.String())
	assert.Equal(t, rcommon.StatusDeployed, rel.Info.Status)
}

func TestBackwardCompat_RollbackWithoutSequencingInfo(t *testing.T) {
	client := newRecordingKubeClient()
	rollback := newRollbackAction(t, client)

	targetChart := buildChartWithTemplates([]*common.File{
		makeConfigMapTemplate("templates/database.yaml", "database", nil),
		makeConfigMapTemplate("templates/app.yaml", "app", nil),
	}, withName("backward-compat-rollback"))
	seedRollbackRelease(t, rollback, "backward-compat-rollback", 1, rcommon.StatusSuperseded, targetChart, joinManifestDocs(
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
	}, withName("backward-compat-rollback"))
	seedRollbackRelease(t, rollback, "backward-compat-rollback", 2, rcommon.StatusDeployed, currentChart, joinManifestDocs(
		configMapManifest("database", map[string]string{releaseutil.AnnotationResourceGroup: "database"}),
		configMapManifest("app", map[string]string{
			releaseutil.AnnotationResourceGroup:           "app",
			releaseutil.AnnotationDependsOnResourceGroups: `["database"]`,
		}),
	), &release.SequencingInfo{Enabled: true, Strategy: string(kube.OrderedWaitStrategy)})

	rollback.Version = 1

	var runErr error
	assert.NotPanics(t, func() {
		runErr = rollback.Run("backward-compat-rollback")
	})
	require.NoError(t, runErr)

	require.Len(t, client.updateCalls, 1)
	assert.ElementsMatch(t, []string{"ConfigMap/database", "ConfigMap/app"}, client.updateCalls[0].target)
	require.Len(t, client.waitCalls, 1)
	assert.ElementsMatch(t, []string{"ConfigMap/database", "ConfigMap/app"}, client.waitCalls[0])
	assert.Nil(t, latestRollbackRelease(t, rollback, "backward-compat-rollback").SequencingInfo)
}

func TestBackwardCompat_MixedAnnotationsUnsequencedLast(t *testing.T) {
	client := newRecordingKubeClient()
	install := newSequencedInstallAction(t, client)
	buf := captureWarningOutput(t)

	ch := buildChartWithTemplates([]*common.File{
		makeConfigMapTemplate("templates/database.yaml", "database", map[string]string{
			releaseutil.AnnotationResourceGroup: "database",
		}),
		makeConfigMapTemplate("templates/app.yaml", "app", map[string]string{
			releaseutil.AnnotationResourceGroup:           "app",
			releaseutil.AnnotationDependsOnResourceGroups: `["database"]`,
		}),
		makeConfigMapTemplate("templates/plain-one.yaml", "plain-one", nil),
		makeConfigMapTemplate("templates/plain-two.yaml", "plain-two", nil),
	}, withName("backward-compat-mixed"))

	rel := mustRelease(t, mustRunInstall(t, install, ch))

	require.Len(t, client.createCalls, 3)
	assert.Equal(t, []string{"ConfigMap/database"}, client.createCalls[0])
	assert.Equal(t, []string{"ConfigMap/app"}, client.createCalls[1])
	assert.ElementsMatch(t, []string{"ConfigMap/plain-one", "ConfigMap/plain-two"}, client.createCalls[2])
	require.Len(t, client.waitCalls, 3)
	assert.Equal(t, client.createCalls[:2], client.waitCalls[:2])
	assert.ElementsMatch(t, client.createCalls[2], client.waitCalls[2])
	assert.Empty(t, buf.String())
	assert.Equal(t, rcommon.StatusDeployed, rel.Info.Status)
}
