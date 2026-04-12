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

func newSequencedUninstallAction(t *testing.T, kubeClient kube.Interface) *Uninstall {
	t.Helper()

	cfg := actionConfigFixture(t)
	cfg.KubeClient = kubeClient

	uninstall := NewUninstall(cfg)
	uninstall.DisableHooks = true
	uninstall.Timeout = 5 * time.Minute
	uninstall.WaitStrategy = kube.OrderedWaitStrategy

	return uninstall
}

func seedUninstallRelease(t *testing.T, uninstall *Uninstall, rel *release.Release) {
	t.Helper()
	require.NoError(t, uninstall.cfg.Releases.Create(rel))
}

func newUninstallRelease(name string, ch *chart.Chart, manifest string, sequencingInfo *release.SequencingInfo, hooks ...*release.Hook) *release.Release {
	now := time.Now()
	return &release.Release{
		Name:      name,
		Namespace: "spaced",
		Chart:     ch,
		Config:    map[string]interface{}{},
		Manifest:  manifest,
		Info: &release.Info{
			FirstDeployed: now,
			LastDeployed:  now,
			Status:        rcommon.StatusDeployed,
			Description:   "Seeded release",
		},
		Version:        1,
		Hooks:          hooks,
		SequencingInfo: sequencingInfo,
	}
}

func secretManifest(name string) string {
	return fmt.Sprintf(`apiVersion: v1
kind: Secret
metadata:
  name: %s
type: Opaque
data:
  password: cGFzc3dvcmQ=
`, name)
}

func serviceAccountManifest(name string) string {
	return fmt.Sprintf(`apiVersion: v1
kind: ServiceAccount
metadata:
  name: %s
`, name)
}

func expectedUninstallIDs(t *testing.T, manifest string) []string {
	t.Helper()

	_, files, err := releaseutil.SortManifests(releaseutil.SplitManifests(manifest), nil, releaseutil.UninstallOrder)
	require.NoError(t, err)

	ids := make([]string, 0, len(files))
	for _, file := range files {
		ids = append(ids, fmt.Sprintf("%s/%s", file.Head.Kind, file.Head.Metadata.Name))
	}
	return ids
}

func filterDeleteCalls(calls [][]string, wanted ...string) [][]string {
	want := make(map[string]struct{}, len(wanted))
	for _, id := range wanted {
		want[id] = struct{}{}
	}

	var filtered [][]string
	for _, call := range calls {
		if len(call) != 1 {
			continue
		}
		if _, ok := want[call[0]]; ok {
			filtered = append(filtered, call)
		}
	}
	return filtered
}

func operationIndex(operations []string, exact string) int {
	for i, operation := range operations {
		if operation == exact {
			return i
		}
	}
	return -1
}

func newDeleteHook(name string, event release.HookEvent) *release.Hook {
	return &release.Hook{
		Name:           name,
		Kind:           "ConfigMap",
		Path:           name,
		Manifest:       configMapManifest(name, nil),
		Events:         []release.HookEvent{event},
		DeletePolicies: []release.HookDeletePolicy{release.HookSucceeded},
	}
}

func TestUninstall_Sequenced_ReverseOrder(t *testing.T) {
	client := newRecordingKubeClient()
	uninstall := newSequencedUninstallAction(t, client)

	ch := buildChartWithTemplates([]*common.File{
		makeConfigMapTemplate("templates/database.yaml", "database", map[string]string{
			releaseutil.AnnotationResourceGroup: "database",
		}),
		makeConfigMapTemplate("templates/app.yaml", "app", map[string]string{
			releaseutil.AnnotationResourceGroup:           "app",
			releaseutil.AnnotationDependsOnResourceGroups: `["database"]`,
		}),
	}, withName("sequenced-uninstall"))

	rel := newUninstallRelease(
		"sequenced-uninstall",
		ch,
		joinManifestDocs(
			configMapManifest("database", map[string]string{releaseutil.AnnotationResourceGroup: "database"}),
			configMapManifest("app", map[string]string{
				releaseutil.AnnotationResourceGroup:           "app",
				releaseutil.AnnotationDependsOnResourceGroups: `["database"]`,
			}),
		),
		&release.SequencingInfo{Enabled: true, Strategy: string(kube.OrderedWaitStrategy)},
	)
	seedUninstallRelease(t, uninstall, rel)

	_, err := uninstall.Run(rel.Name)
	require.NoError(t, err)

	assert.Equal(t, [][]string{{"ConfigMap/app"}, {"ConfigMap/database"}}, client.deleteCalls)
	assert.Equal(t, client.deleteCalls, client.deleteWaitCalls)
}

func TestUninstall_NonSequenced_Unchanged(t *testing.T) {
	client := newRecordingKubeClient()
	uninstall := newSequencedUninstallAction(t, client)

	manifest := joinManifestDocs(
		configMapManifest("config", nil),
		secretManifest("secret"),
		serviceAccountManifest("service-account"),
	)
	rel := newUninstallRelease("standard-uninstall", buildChartWithTemplates(nil, withName("standard-uninstall")), manifest, nil)
	seedUninstallRelease(t, uninstall, rel)

	_, err := uninstall.Run(rel.Name)
	require.NoError(t, err)

	require.Len(t, client.deleteCalls, 1)
	assert.Equal(t, expectedUninstallIDs(t, manifest), client.deleteCalls[0])
	require.Len(t, client.deleteWaitCalls, 1)
	assert.Equal(t, client.deleteCalls[0], client.deleteWaitCalls[0])
}

func TestUninstall_Sequenced_WithSubcharts(t *testing.T) {
	client := newRecordingKubeClient()
	uninstall := newSequencedUninstallAction(t, client)

	parent := buildChartWithTemplates([]*common.File{
		makeConfigMapTemplate("templates/parent.yaml", "parent", nil),
	}, withName("parent"))
	bar := buildChartWithTemplates([]*common.File{
		makeConfigMapTemplate("templates/bar.yaml", "bar", nil),
	}, withName("bar"))
	nginx := buildChartWithTemplates([]*common.File{
		makeConfigMapTemplate("templates/nginx.yaml", "nginx", nil),
	}, withName("nginx"))

	bar.AddDependency(nginx)
	bar.Metadata.Dependencies = []*chart.Dependency{{Name: "nginx"}}
	parent.AddDependency(bar)
	parent.Metadata.Dependencies = []*chart.Dependency{{Name: "bar"}}

	rel := newUninstallRelease(
		"subchart-uninstall",
		parent,
		joinManifestDocs(
			configMapManifest("nginx", nil),
			configMapManifest("bar", nil),
			configMapManifest("parent", nil),
		),
		&release.SequencingInfo{Enabled: true, Strategy: string(kube.OrderedWaitStrategy)},
	)
	seedUninstallRelease(t, uninstall, rel)

	_, err := uninstall.Run(rel.Name)
	require.NoError(t, err)

	assert.Equal(t, [][]string{{"ConfigMap/parent"}, {"ConfigMap/bar"}, {"ConfigMap/nginx"}}, client.deleteCalls)
	assert.Equal(t, client.deleteCalls, client.deleteWaitCalls)
}

func TestUninstall_Sequenced_KeepPolicy(t *testing.T) {
	client := newRecordingKubeClient()
	uninstall := newSequencedUninstallAction(t, client)

	ch := buildChartWithTemplates([]*common.File{
		makeConfigMapTemplate("templates/database-live.yaml", "database-live", map[string]string{
			releaseutil.AnnotationResourceGroup: "database",
		}),
		makeConfigMapTemplate("templates/database-keep.yaml", "database-keep", map[string]string{
			releaseutil.AnnotationResourceGroup: "database",
			kube.ResourcePolicyAnno:             kube.KeepPolicy,
		}),
		makeConfigMapTemplate("templates/app.yaml", "app", map[string]string{
			releaseutil.AnnotationResourceGroup:           "app",
			releaseutil.AnnotationDependsOnResourceGroups: `["database"]`,
		}),
	}, withName("keep-policy"))

	rel := newUninstallRelease(
		"keep-policy",
		ch,
		joinManifestDocs(
			configMapManifest("database-live", map[string]string{releaseutil.AnnotationResourceGroup: "database"}),
			configMapManifest("database-keep", map[string]string{
				releaseutil.AnnotationResourceGroup: "database",
				kube.ResourcePolicyAnno:             kube.KeepPolicy,
			}),
			configMapManifest("app", map[string]string{
				releaseutil.AnnotationResourceGroup:           "app",
				releaseutil.AnnotationDependsOnResourceGroups: `["database"]`,
			}),
		),
		&release.SequencingInfo{Enabled: true, Strategy: string(kube.OrderedWaitStrategy)},
	)
	seedUninstallRelease(t, uninstall, rel)

	res, err := uninstall.Run(rel.Name)
	require.NoError(t, err)

	assert.Equal(t, [][]string{{"ConfigMap/app"}, {"ConfigMap/database-live"}}, client.deleteCalls)
	assert.Equal(t, client.deleteCalls, client.deleteWaitCalls)
	assert.NotContains(t, client.deleteCalls, []string{"ConfigMap/database-keep"})
	assert.Contains(t, res.Info, "[ConfigMap] database-keep")
}

func TestUninstall_Sequenced_KeepHistory(t *testing.T) {
	client := newRecordingKubeClient()
	uninstall := newSequencedUninstallAction(t, client)
	uninstall.KeepHistory = true

	ch := buildChartWithTemplates([]*common.File{
		makeConfigMapTemplate("templates/database.yaml", "database", map[string]string{
			releaseutil.AnnotationResourceGroup: "database",
		}),
		makeConfigMapTemplate("templates/app.yaml", "app", map[string]string{
			releaseutil.AnnotationResourceGroup:           "app",
			releaseutil.AnnotationDependsOnResourceGroups: `["database"]`,
		}),
	}, withName("keep-history"))

	rel := newUninstallRelease(
		"keep-history",
		ch,
		joinManifestDocs(
			configMapManifest("database", map[string]string{releaseutil.AnnotationResourceGroup: "database"}),
			configMapManifest("app", map[string]string{
				releaseutil.AnnotationResourceGroup:           "app",
				releaseutil.AnnotationDependsOnResourceGroups: `["database"]`,
			}),
		),
		&release.SequencingInfo{Enabled: true, Strategy: string(kube.OrderedWaitStrategy)},
	)
	seedUninstallRelease(t, uninstall, rel)

	_, err := uninstall.Run(rel.Name)
	require.NoError(t, err)

	reli, err := uninstall.cfg.Releases.Get(rel.Name, rel.Version)
	require.NoError(t, err)
	stored, err := releaserToV1Release(reli)
	require.NoError(t, err)

	assert.Equal(t, [][]string{{"ConfigMap/app"}, {"ConfigMap/database"}}, client.deleteCalls)
	assert.Equal(t, rcommon.StatusUninstalled, stored.Info.Status)
}

func TestUninstall_Sequenced_DryRun(t *testing.T) {
	client := newRecordingKubeClient()
	uninstall := newSequencedUninstallAction(t, client)
	uninstall.DryRun = true

	ch := buildChartWithTemplates([]*common.File{
		makeConfigMapTemplate("templates/database.yaml", "database", map[string]string{
			releaseutil.AnnotationResourceGroup: "database",
		}),
		makeConfigMapTemplate("templates/app.yaml", "app", map[string]string{
			releaseutil.AnnotationResourceGroup:           "app",
			releaseutil.AnnotationDependsOnResourceGroups: `["database"]`,
		}),
	}, withName("dry-run-uninstall"))

	rel := newUninstallRelease(
		"dry-run-uninstall",
		ch,
		joinManifestDocs(
			configMapManifest("database", map[string]string{releaseutil.AnnotationResourceGroup: "database"}),
			configMapManifest("app", map[string]string{
				releaseutil.AnnotationResourceGroup:           "app",
				releaseutil.AnnotationDependsOnResourceGroups: `["database"]`,
			}),
		),
		&release.SequencingInfo{Enabled: true, Strategy: string(kube.OrderedWaitStrategy)},
	)
	seedUninstallRelease(t, uninstall, rel)

	res, err := uninstall.Run(rel.Name)
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.Empty(t, client.deleteCalls)
	assert.Empty(t, client.deleteWaitCalls)
}

func TestUninstall_Sequenced_Hooks(t *testing.T) {
	client := newRecordingKubeClient()
	uninstall := newSequencedUninstallAction(t, client)
	uninstall.DisableHooks = false

	ch := buildChartWithTemplates([]*common.File{
		makeConfigMapTemplate("templates/database.yaml", "database", map[string]string{
			releaseutil.AnnotationResourceGroup: "database",
		}),
		makeConfigMapTemplate("templates/app.yaml", "app", map[string]string{
			releaseutil.AnnotationResourceGroup:           "app",
			releaseutil.AnnotationDependsOnResourceGroups: `["database"]`,
		}),
	}, withName("hooked-uninstall"))

	rel := newUninstallRelease(
		"hooked-uninstall",
		ch,
		joinManifestDocs(
			configMapManifest("database", map[string]string{releaseutil.AnnotationResourceGroup: "database"}),
			configMapManifest("app", map[string]string{
				releaseutil.AnnotationResourceGroup:           "app",
				releaseutil.AnnotationDependsOnResourceGroups: `["database"]`,
			}),
		),
		&release.SequencingInfo{Enabled: true, Strategy: string(kube.OrderedWaitStrategy)},
		newDeleteHook("pre-hook", release.HookPreDelete),
		newDeleteHook("post-hook", release.HookPostDelete),
	)
	seedUninstallRelease(t, uninstall, rel)

	_, err := uninstall.Run(rel.Name)
	require.NoError(t, err)

	assert.Equal(t, [][]string{{"ConfigMap/pre-hook"}, {"ConfigMap/post-hook"}}, client.createCalls)
	assert.Equal(t, [][]string{{"ConfigMap/pre-hook"}, {"ConfigMap/post-hook"}}, client.watchUntilReadyCalls)
	assert.Equal(t, [][]string{{"ConfigMap/app"}, {"ConfigMap/database"}}, filterDeleteCalls(client.deleteCalls, "ConfigMap/app", "ConfigMap/database"))

	preHookCreate := operationIndex(client.operations, "create:ConfigMap/pre-hook")
	appDelete := operationIndex(client.operations, "delete:ConfigMap/app")
	databaseWaitDelete := operationIndex(client.operations, "wait-delete:ConfigMap/database")
	postHookCreate := operationIndex(client.operations, "create:ConfigMap/post-hook")

	require.NotEqual(t, -1, preHookCreate)
	require.NotEqual(t, -1, appDelete)
	require.NotEqual(t, -1, databaseWaitDelete)
	require.NotEqual(t, -1, postHookCreate)
	assert.Less(t, preHookCreate, appDelete)
	assert.Greater(t, postHookCreate, databaseWaitDelete)
}
