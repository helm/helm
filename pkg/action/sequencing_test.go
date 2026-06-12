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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	yamlutil "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/kubernetes/scheme"
	restfake "k8s.io/client-go/rest/fake"

	"helm.sh/helm/v4/pkg/chart/common"
	chart "helm.sh/helm/v4/pkg/chart/v2"
	"helm.sh/helm/v4/pkg/kube"
	kubefake "helm.sh/helm/v4/pkg/kube/fake"
	ri "helm.sh/helm/v4/pkg/release"
	rcommon "helm.sh/helm/v4/pkg/release/common"
	release "helm.sh/helm/v4/pkg/release/v1"
	releaseutil "helm.sh/helm/v4/pkg/release/v1/util"
)

type recordingKubeClient struct {
	kubefake.PrintingKubeClient
	createCalls          [][]string
	updateCalls          []updateCall
	waitCalls            [][]string
	watchUntilReadyCalls [][]string
	deleteWaitCalls      [][]string
	deleteCalls          [][]string
	operations           []string
	waitCallCount        int
	waitErrorOnCall      int
	waitError            error
	onBuild              func()
	onCreate             func()
}

type updateCall struct {
	current          []string
	target           []string
	created          []string
	currentResources kube.ResourceList
	targetResources  kube.ResourceList
}

type recordingKubeWaiter struct {
	client *recordingKubeClient
}

func newRecordingKubeClient() *recordingKubeClient {
	return &recordingKubeClient{
		PrintingKubeClient: kubefake.PrintingKubeClient{Out: io.Discard, LogOutput: io.Discard},
	}
}

func (c *recordingKubeClient) Create(resources kube.ResourceList, _ ...kube.ClientCreateOption) (*kube.Result, error) {
	if c.onCreate != nil {
		c.onCreate()
	}
	ids := resourceIDs(resources)
	c.createCalls = append(c.createCalls, ids)
	c.operations = append(c.operations, "create:"+strings.Join(ids, ","))
	return &kube.Result{Created: resources}, nil
}

func (c *recordingKubeClient) Delete(resources kube.ResourceList, deletionPropagation metav1.DeletionPropagation) (*kube.Result, []error) {
	ids := resourceIDs(resources)
	c.deleteCalls = append(c.deleteCalls, ids)
	c.operations = append(c.operations, "delete:"+strings.Join(ids, ","))
	return c.PrintingKubeClient.Delete(resources, deletionPropagation)
}

func (c *recordingKubeClient) Update(current, target kube.ResourceList, _ ...kube.ClientUpdateOption) (*kube.Result, error) {
	currentIDs := resourceIDs(current)
	targetIDs := resourceIDs(target)

	currentSet := make(map[string]struct{}, len(currentIDs))
	for _, id := range currentIDs {
		currentSet[id] = struct{}{}
	}

	var created kube.ResourceList
	var createdIDs []string
	for _, info := range target {
		id := fmt.Sprintf("%s/%s", info.Object.GetObjectKind().GroupVersionKind().Kind, info.Name)
		if _, exists := currentSet[id]; !exists {
			created = append(created, info)
			createdIDs = append(createdIDs, id)
		}
	}

	c.updateCalls = append(c.updateCalls, updateCall{
		current:          currentIDs,
		target:           targetIDs,
		created:          createdIDs,
		currentResources: current,
		targetResources:  target,
	})
	c.operations = append(c.operations, "update:"+strings.Join(targetIDs, ","))
	return &kube.Result{Updated: target, Created: created}, nil
}

func (c *recordingKubeClient) Build(reader io.Reader, _ bool) (kube.ResourceList, error) {
	if c.onBuild != nil {
		c.onBuild()
	}
	decoder := yamlutil.NewYAMLOrJSONDecoder(reader, 4096)
	var resources kube.ResourceList

	for {
		var obj map[string]any
		if err := decoder.Decode(&obj); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}
		if len(obj) == 0 {
			continue
		}

		u := &unstructured.Unstructured{Object: obj}
		gvk := u.GroupVersionKind()
		namespace := u.GetNamespace()
		if namespace == "" {
			namespace = "spaced"
			u.SetNamespace(namespace)
		}
		info := &resource.Info{
			Name:      u.GetName(),
			Namespace: namespace,
			Object:    u,
			Mapping: &meta.RESTMapping{
				Resource:         schema.GroupVersionResource{Group: gvk.Group, Version: gvk.Version, Resource: strings.ToLower(gvk.Kind) + "s"},
				GroupVersionKind: gvk,
				Scope:            meta.RESTScopeNamespace,
			},
			Client: newNotFoundRESTClient(u.GetName(), gvk),
		}
		resources.Append(info)
	}

	return resources, nil
}

func (c *recordingKubeClient) GetWaiter(ws kube.WaitStrategy) (kube.Waiter, error) {
	return c.GetWaiterWithOptions(ws)
}

func (c *recordingKubeClient) GetWaiterWithOptions(_ kube.WaitStrategy, _ ...kube.WaitOption) (kube.Waiter, error) {
	return &recordingKubeWaiter{client: c}, nil
}

func (w *recordingKubeWaiter) Wait(resources kube.ResourceList, _ time.Duration) error {
	return w.client.recordWait(resources)
}

func (w *recordingKubeWaiter) WaitWithJobs(resources kube.ResourceList, _ time.Duration) error {
	return w.client.recordWait(resources)
}

func (w *recordingKubeWaiter) WaitForDelete(resources kube.ResourceList, _ time.Duration) error {
	ids := resourceIDs(resources)
	w.client.deleteWaitCalls = append(w.client.deleteWaitCalls, ids)
	w.client.operations = append(w.client.operations, "wait-delete:"+strings.Join(ids, ","))
	return nil
}

func (w *recordingKubeWaiter) WatchUntilReady(resources kube.ResourceList, _ time.Duration) error {
	ids := resourceIDs(resources)
	w.client.watchUntilReadyCalls = append(w.client.watchUntilReadyCalls, ids)
	w.client.operations = append(w.client.operations, "watch:"+strings.Join(ids, ","))
	return nil
}

func (c *recordingKubeClient) recordWait(resources kube.ResourceList) error {
	c.waitCallCount++
	ids := resourceIDs(resources)
	c.waitCalls = append(c.waitCalls, ids)
	c.operations = append(c.operations, "wait:"+strings.Join(ids, ","))
	if c.waitError != nil && c.waitCallCount == c.waitErrorOnCall {
		return c.waitError
	}
	return nil
}

func newNotFoundRESTClient(name string, gvk schema.GroupVersionKind) *restfake.RESTClient {
	body, _ := json.Marshal(metav1.Status{
		Status: metav1.StatusFailure,
		Reason: metav1.StatusReasonNotFound,
		Code:   http.StatusNotFound,
		Details: &metav1.StatusDetails{
			Name:  name,
			Group: gvk.Group,
			Kind:  gvk.Kind,
		},
	})

	return &restfake.RESTClient{
		GroupVersion:         gvk.GroupVersion(),
		NegotiatedSerializer: scheme.Codecs.WithoutConversion(),
		Client: restfake.CreateHTTPClient(func(_ *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusNotFound,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(bytes.NewReader(body)),
			}, nil
		}),
	}
}

func resourceIDs(resources kube.ResourceList) []string {
	ids := make([]string, 0, len(resources))
	for _, info := range resources {
		kind := info.Object.GetObjectKind().GroupVersionKind().Kind
		if kind == "" {
			kind = "Unknown"
		}
		ids = append(ids, fmt.Sprintf("%s/%s", kind, info.Name))
	}
	return ids
}

func newSequencedInstallAction(t *testing.T, kubeClient kube.Interface) *Install {
	t.Helper()
	cfg := actionConfigFixture(t)
	cfg.KubeClient = kubeClient

	install := NewInstall(cfg)
	install.Namespace = "spaced"
	install.ReleaseName = "test-install-release"
	install.Timeout = 5 * time.Minute
	install.ReadinessTimeout = time.Minute
	install.WaitStrategy = kube.OrderedWaitStrategy
	return install
}

func releaseFile(name, content string) *common.File {
	return &common.File{Name: name, ModTime: time.Now(), Data: []byte(content)}
}

func configMapManifest(name string, annotations map[string]string) string {
	var b strings.Builder
	b.WriteString("apiVersion: v1\nkind: ConfigMap\nmetadata:\n")
	_, _ = fmt.Fprintf(&b, "  name: %s\n", name)
	if len(annotations) > 0 {
		b.WriteString("  annotations:\n")
		for key, value := range annotations {
			_, _ = fmt.Fprintf(&b, "    %s: %q\n", key, value)
		}
	}
	b.WriteString("data:\n  key: value\n")
	return b.String()
}

func makeConfigMapTemplate(fileName, name string, annotations map[string]string) *common.File {
	return releaseFile(fileName, configMapManifest(name, annotations))
}

func makeHookTemplate(fileName, name, hook string, extraAnnotations map[string]string) *common.File {
	annotations := map[string]string{release.HookAnnotation: hook}
	maps.Copy(annotations, extraAnnotations)
	return makeConfigMapTemplate(fileName, name, annotations)
}

func mustRelease(t *testing.T, rel ri.Releaser) *release.Release {
	t.Helper()
	out, err := releaserToV1Release(rel)
	require.NoError(t, err)
	return out
}

func TestSequencing_GroupManifestsByDirectSubchart(t *testing.T) {
	manifests := []releaseutil.Manifest{
		makeTestManifest("parent", "parent/templates/one.yaml", nil),
		makeTestManifest("db", "parent/charts/database/templates/one.yaml", nil),
		makeTestManifest("cache", "parent/charts/database/charts/cache/templates/one.yaml", nil),
	}

	grouped := GroupManifestsByDirectSubchart(manifests, "parent")

	require.Len(t, grouped[""], 1)
	require.Len(t, grouped["database"], 2)
}

// TestSequencing_GroupManifestsByDirectSubchart_Nested verifies that when called
// with a deeper chartPath (i.e., during recursion into a subchart), nested
// grandchildren are routed to the correct subchart key instead of being merged
// into the parent batch.
func TestSequencing_GroupManifestsByDirectSubchart_Nested(t *testing.T) {
	manifests := []releaseutil.Manifest{
		makeTestManifest("db-own", "parent/charts/database/templates/one.yaml", nil),
		makeTestManifest("cache", "parent/charts/database/charts/cache/templates/one.yaml", nil),
	}

	grouped := GroupManifestsByDirectSubchart(manifests, "parent/charts/database")

	require.Len(t, grouped[""], 1, "database's own resources should be under the empty key")
	require.Len(t, grouped["cache"], 1, "nested cache subchart should be routed under its own key")
	require.Equal(t, "parent/charts/database/templates/one.yaml", grouped[""][0].Name)
	require.Equal(t, "parent/charts/database/charts/cache/templates/one.yaml", grouped["cache"][0].Name)
}

func TestSequencing_BuildManifestYAML(t *testing.T) {
	yaml := buildManifestYAML([]releaseutil.Manifest{
		makeTestManifest("one", "chart/templates/one.yaml", nil),
		makeTestManifest("two", "chart/templates/two.yaml", nil),
	})

	assert.Contains(t, yaml, "name: one")
	assert.Contains(t, yaml, "name: two")
}

func TestInstall_Sequenced_BasicResourceGroups(t *testing.T) {
	client := newRecordingKubeClient()
	install := newSequencedInstallAction(t, client)

	ch := buildChartWithTemplates([]*common.File{
		makeConfigMapTemplate("templates/database.yaml", "database", map[string]string{
			releaseutil.AnnotationResourceGroup: "database",
		}),
		makeConfigMapTemplate("templates/app.yaml", "app", map[string]string{
			releaseutil.AnnotationResourceGroup:           "app",
			releaseutil.AnnotationDependsOnResourceGroups: `["database"]`,
		}),
	})

	rel := mustRelease(t, mustRunInstall(t, install, ch))
	assert.Equal(t, [][]string{{"ConfigMap/database"}, {"ConfigMap/app"}}, client.createCalls)
	// Per HIP-0025 §Readiness: app has no dependents, so its readiness is not
	// waited on. database is depended-on by app, so it is waited on.
	assert.Equal(t, [][]string{{"ConfigMap/database"}}, client.waitCalls)
	assert.Equal(t, rcommon.StatusDeployed, rel.Info.Status)
	assert.NotNil(t, rel.SequencingInfo)
	assert.True(t, rel.SequencingInfo.Enabled)
}

func TestInstall_Sequenced_SubchartOrdering(t *testing.T) {
	client := newRecordingKubeClient()
	install := newSequencedInstallAction(t, client)

	parent := buildChartWithTemplates([]*common.File{
		makeConfigMapTemplate("templates/parent.yaml", "parent", nil),
	}, withName("parent"))
	database := buildChartWithTemplates([]*common.File{
		makeConfigMapTemplate("templates/database.yaml", "database", nil),
	}, withName("database"))
	api := buildChartWithTemplates([]*common.File{
		makeConfigMapTemplate("templates/api.yaml", "api", nil),
	}, withName("api"))

	parent.AddDependency(database, api)
	parent.Metadata.Dependencies = []*chart.Dependency{
		{Name: "database"},
		{Name: "api", DependsOn: []string{"database"}},
	}

	mustRunInstall(t, install, parent)

	assert.Equal(t, [][]string{{"ConfigMap/database"}, {"ConfigMap/api"}, {"ConfigMap/parent"}}, client.createCalls)
	assert.Equal(t, client.createCalls, client.waitCalls)
}

// TestInstall_Sequenced_UndeclaredVendoredSubchartDeployed is a regression for
// hip-0025: a subchart vendored into charts/ but absent from Chart.yaml
// dependencies is rendered, so under --wait=ordered its manifests must still be
// applied. Previously they were grouped by subchart yet never matched a DAG
// batch (the DAG is built from Chart.yaml dependencies only), so they were
// silently dropped. Undeclared subcharts are unsequenced: they deploy after the
// declared subchart batches and before the parent's own resources.
func TestInstall_Sequenced_UndeclaredVendoredSubchartDeployed(t *testing.T) {
	client := newRecordingKubeClient()
	install := newSequencedInstallAction(t, client)

	parent := buildChartWithTemplates([]*common.File{
		makeConfigMapTemplate("templates/parent.yaml", "parent", nil),
	}, withName("parent"))
	declared := buildChartWithTemplates([]*common.File{
		makeConfigMapTemplate("templates/database.yaml", "database", nil),
	}, withName("database"))
	vendored := buildChartWithTemplates([]*common.File{
		makeConfigMapTemplate("templates/vendored.yaml", "vendored", nil),
	}, withName("vendored"))

	parent.AddDependency(declared, vendored)
	// Only "database" is declared in Chart.yaml dependencies; "vendored" sits in
	// charts/ but is undeclared.
	parent.Metadata.Dependencies = []*chart.Dependency{
		{Name: "database"},
	}

	mustRunInstall(t, install, parent)

	assert.Equal(t, [][]string{{"ConfigMap/database"}, {"ConfigMap/vendored"}, {"ConfigMap/parent"}}, client.createCalls)
}

// TestInstall_Sequenced_LeafReadinessIgnored locks cap-31 (hip-0025-4ce).
// Per HIP-0025 §Readiness, a resource whose group has no dependents in the
// sequencing DAG must be applied but its readiness must NOT be waited on,
// even if it carries helm.sh/readiness-success / helm.sh/readiness-failure
// annotations. Previously waitForResources called waiter.Wait over every
// resource in every batch, which made leaf resources block the install
// until their readiness condition fired or the deadline expired.
func TestInstall_Sequenced_LeafReadinessIgnored(t *testing.T) {
	client := newRecordingKubeClient()
	install := newSequencedInstallAction(t, client)

	ch := buildChartWithTemplates([]*common.File{
		makeConfigMapTemplate("templates/base.yaml", "base", map[string]string{
			releaseutil.AnnotationResourceGroup: "base",
		}),
		// leaf group: depends-on base, but nothing depends on it.
		// Carries readiness annotations that never match — would hang the
		// install if waitForResources still waited on leaves.
		makeConfigMapTemplate("templates/leaf.yaml", "leaf", map[string]string{
			releaseutil.AnnotationResourceGroup:           "leaf",
			releaseutil.AnnotationDependsOnResourceGroups: `["base"]`,
			kube.AnnotationReadinessSuccess:               `["{.never} == match"]`,
			kube.AnnotationReadinessFailure:               `["{.never} == fail"]`,
		}),
	})

	mustRunInstall(t, install, ch)
	assert.Equal(t, [][]string{{"ConfigMap/base"}, {"ConfigMap/leaf"}}, client.createCalls)
	// base has a dependent (leaf) → waited.
	// leaf has no dependents → readiness ignored, no wait call recorded.
	assert.Equal(t, [][]string{{"ConfigMap/base"}}, client.waitCalls)
}

// TestInstall_Sequenced_LeafWithoutReadinessAnnotationApplied confirms the
// pre-existing behavior for leaves with no readiness annotation still holds:
// the resource is applied, readiness is not waited on, install succeeds.
// Regression guard so the leaf-detection refactor does not change apply
// semantics for the "plain leaf" case.
func TestInstall_Sequenced_LeafWithoutReadinessAnnotationApplied(t *testing.T) {
	client := newRecordingKubeClient()
	install := newSequencedInstallAction(t, client)

	ch := buildChartWithTemplates([]*common.File{
		makeConfigMapTemplate("templates/base.yaml", "base", map[string]string{
			releaseutil.AnnotationResourceGroup: "base",
		}),
		makeConfigMapTemplate("templates/leaf.yaml", "leaf", map[string]string{
			releaseutil.AnnotationResourceGroup:           "leaf",
			releaseutil.AnnotationDependsOnResourceGroups: `["base"]`,
		}),
	})

	mustRunInstall(t, install, ch)
	assert.Equal(t, [][]string{{"ConfigMap/base"}, {"ConfigMap/leaf"}}, client.createCalls)
	assert.Equal(t, [][]string{{"ConfigMap/base"}}, client.waitCalls)
}

// TestInstall_Sequenced_NonLeafReadinessStillWaitsOnFailure is the regression
// guard for cap-04/cap-05/cap-32: when a resource HAS a dependent, its
// readiness IS waited on. Simulates the worker.Wait returning an error
// (e.g. readiness-failure JSONPath fired) and confirms the install aborts.
// Without this guard, a future "ignore-readiness" change could silently
// regress all custom-readiness behavior.
func TestInstall_Sequenced_NonLeafReadinessStillWaitsOnFailure(t *testing.T) {
	client := newRecordingKubeClient()
	client.waitErrorOnCall = 1
	client.waitError = assert.AnError
	install := newSequencedInstallAction(t, client)

	ch := buildChartWithTemplates([]*common.File{
		// bootstrap has a dependent (app), so its readiness must be waited on.
		makeConfigMapTemplate("templates/bootstrap.yaml", "bootstrap", map[string]string{
			releaseutil.AnnotationResourceGroup: "bootstrap",
			kube.AnnotationReadinessSuccess:     `["{.ready} == true"]`,
			kube.AnnotationReadinessFailure:     `["{.failed} == true"]`,
		}),
		makeConfigMapTemplate("templates/app.yaml", "app", map[string]string{
			releaseutil.AnnotationResourceGroup:           "app",
			releaseutil.AnnotationDependsOnResourceGroups: `["bootstrap"]`,
		}),
	})

	_, err := install.RunWithContext(context.Background(), ch, map[string]any{})
	require.Error(t, err, "bootstrap readiness wait failure must abort install")
	require.NotEmpty(t, client.waitCalls, "bootstrap wait call must have been recorded")
	assert.Equal(t, []string{"ConfigMap/bootstrap"}, client.waitCalls[0])
	// app group's batch never gets applied because bootstrap wait failed first.
	require.NotContains(t, client.operations, "create:ConfigMap/app", "app should not be created after bootstrap failed")
}

func TestInstall_Sequenced_Combined(t *testing.T) {
	client := newRecordingKubeClient()
	install := newSequencedInstallAction(t, client)

	parent := buildChartWithTemplates([]*common.File{
		makeConfigMapTemplate("templates/parent.yaml", "parent", nil),
	}, withName("parent"))
	database := buildChartWithTemplates([]*common.File{
		makeConfigMapTemplate("templates/setup.yaml", "db-setup", map[string]string{
			releaseutil.AnnotationResourceGroup: "setup",
		}),
		makeConfigMapTemplate("templates/data.yaml", "db-data", map[string]string{
			releaseutil.AnnotationResourceGroup:           "data",
			releaseutil.AnnotationDependsOnResourceGroups: `["setup"]`,
		}),
	}, withName("database"))
	api := buildChartWithTemplates([]*common.File{
		makeConfigMapTemplate("templates/api.yaml", "api", map[string]string{
			releaseutil.AnnotationResourceGroup: "api",
		}),
	}, withName("api"))

	parent.AddDependency(database, api)
	parent.Metadata.Dependencies = []*chart.Dependency{
		{Name: "database"},
		{Name: "api", DependsOn: []string{"database"}},
	}

	mustRunInstall(t, install, parent)

	assert.Equal(t, [][]string{
		{"ConfigMap/db-setup"},
		{"ConfigMap/db-data"},
		{"ConfigMap/api"},
		{"ConfigMap/parent"},
	}, client.createCalls)
}

func TestInstall_Sequenced_NoAnnotations(t *testing.T) {
	client := newRecordingKubeClient()
	install := newSequencedInstallAction(t, client)

	ch := buildChartWithTemplates([]*common.File{
		makeConfigMapTemplate("templates/first.yaml", "first", nil),
		makeConfigMapTemplate("templates/second.yaml", "second", nil),
	})

	rel := mustRelease(t, mustRunInstall(t, install, ch))

	require.Len(t, client.createCalls, 1)
	assert.ElementsMatch(t, []string{"ConfigMap/first", "ConfigMap/second"}, client.createCalls[0])
	assert.Equal(t, rcommon.StatusDeployed, rel.Info.Status)
}

func TestInstall_Sequenced_UnsequencedLast(t *testing.T) {
	client := newRecordingKubeClient()
	install := newSequencedInstallAction(t, client)

	ch := buildChartWithTemplates([]*common.File{
		makeConfigMapTemplate("templates/database.yaml", "database", map[string]string{
			releaseutil.AnnotationResourceGroup: "database",
		}),
		makeConfigMapTemplate("templates/orphan.yaml", "orphan", map[string]string{
			releaseutil.AnnotationResourceGroup:           "orphan",
			releaseutil.AnnotationDependsOnResourceGroups: `["missing"]`,
		}),
		makeConfigMapTemplate("templates/plain.yaml", "plain", nil),
	})

	mustRunInstall(t, install, ch)

	require.Len(t, client.createCalls, 2)
	assert.Equal(t, []string{"ConfigMap/database"}, client.createCalls[0])
	assert.ElementsMatch(t, []string{"ConfigMap/orphan", "ConfigMap/plain"}, client.createCalls[1])
}

func TestInstall_Sequenced_ReadinessTimeout(t *testing.T) {
	client := newRecordingKubeClient()
	client.waitErrorOnCall = 1
	client.waitError = errors.New("timed out waiting for batch")

	install := newSequencedInstallAction(t, client)
	ch := buildChartWithTemplates([]*common.File{
		makeConfigMapTemplate("templates/database.yaml", "database", map[string]string{
			releaseutil.AnnotationResourceGroup: "database",
		}),
		makeConfigMapTemplate("templates/app.yaml", "app", map[string]string{
			releaseutil.AnnotationResourceGroup:           "app",
			releaseutil.AnnotationDependsOnResourceGroups: `["database"]`,
		}),
	})

	rel, err := install.Run(ch, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "timed out waiting for batch")
	assert.Len(t, client.createCalls, 1)

	last, getErr := install.cfg.Releases.Last(install.ReleaseName)
	require.NoError(t, getErr)
	assert.Equal(t, rcommon.StatusFailed, mustRelease(t, last).Info.Status)
	assert.Equal(t, rcommon.StatusFailed, mustRelease(t, rel).Info.Status)
}

func TestInstall_Sequenced_AtomicRollback(t *testing.T) {
	client := newRecordingKubeClient()
	client.waitErrorOnCall = 1
	client.waitError = errors.New("timed out waiting for batch")

	install := newSequencedInstallAction(t, client)
	install.RollbackOnFailure = true

	ch := buildChartWithTemplates([]*common.File{
		makeConfigMapTemplate("templates/database.yaml", "database", map[string]string{
			releaseutil.AnnotationResourceGroup: "database",
		}),
		makeConfigMapTemplate("templates/app.yaml", "app", map[string]string{
			releaseutil.AnnotationResourceGroup:           "app",
			releaseutil.AnnotationDependsOnResourceGroups: `["database"]`,
		}),
	})

	_, err := install.Run(ch, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rollback-on-failure")
	assert.NotEmpty(t, client.deleteCalls)
}

func TestInstall_Sequenced_DryRun(t *testing.T) {
	client := newRecordingKubeClient()
	install := newSequencedInstallAction(t, client)
	install.DryRunStrategy = DryRunServer

	ch := buildChartWithTemplates([]*common.File{
		makeConfigMapTemplate("templates/database.yaml", "database", map[string]string{
			releaseutil.AnnotationResourceGroup: "database",
		}),
		makeConfigMapTemplate("templates/app.yaml", "app", map[string]string{
			releaseutil.AnnotationResourceGroup:           "app",
			releaseutil.AnnotationDependsOnResourceGroups: `["database"]`,
		}),
	})

	rel := mustRelease(t, mustRunInstall(t, install, ch))
	assert.Empty(t, client.createCalls)
	assert.Empty(t, client.waitCalls)
	assert.Equal(t, "Dry run complete", rel.Info.Description)
}

func TestInstall_Sequenced_CyclicDependency(t *testing.T) {
	client := newRecordingKubeClient()
	install := newSequencedInstallAction(t, client)

	ch := buildChartWithTemplates([]*common.File{
		makeConfigMapTemplate("templates/a.yaml", "a", map[string]string{
			releaseutil.AnnotationResourceGroup:           "a",
			releaseutil.AnnotationDependsOnResourceGroups: `["b"]`,
		}),
		makeConfigMapTemplate("templates/b.yaml", "b", map[string]string{
			releaseutil.AnnotationResourceGroup:           "b",
			releaseutil.AnnotationDependsOnResourceGroups: `["a"]`,
		}),
	})

	_, err := install.Run(ch, nil)
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "cycle") || strings.Contains(err.Error(), "circular"))
	assert.Empty(t, client.createCalls)
}

func TestInstall_Sequenced_SequencingInfoStored(t *testing.T) {
	client := newRecordingKubeClient()
	install := newSequencedInstallAction(t, client)

	ch := buildChartWithTemplates([]*common.File{
		makeConfigMapTemplate("templates/config.yaml", "config", nil),
	})

	rel := mustRelease(t, mustRunInstall(t, install, ch))

	require.NotNil(t, rel.SequencingInfo)
	assert.True(t, rel.SequencingInfo.Enabled)
	assert.Equal(t, string(kube.OrderedWaitStrategy), rel.SequencingInfo.Strategy)

	stored, err := install.cfg.Releases.Get(rel.Name, rel.Version)
	require.NoError(t, err)
	assert.Equal(t, string(kube.OrderedWaitStrategy), mustRelease(t, stored).SequencingInfo.Strategy)
}

func TestInstall_NonSequenced_Unchanged(t *testing.T) {
	client := newRecordingKubeClient()
	install := newSequencedInstallAction(t, client)
	install.WaitStrategy = kube.StatusWatcherStrategy

	ch := buildChartWithTemplates([]*common.File{
		makeConfigMapTemplate("templates/database.yaml", "database", map[string]string{
			releaseutil.AnnotationResourceGroup: "database",
		}),
		makeConfigMapTemplate("templates/app.yaml", "app", map[string]string{
			releaseutil.AnnotationResourceGroup:           "app",
			releaseutil.AnnotationDependsOnResourceGroups: `["database"]`,
		}),
	})

	rel := mustRelease(t, mustRunInstall(t, install, ch))

	require.Len(t, client.createCalls, 1)
	assert.ElementsMatch(t, []string{"ConfigMap/database", "ConfigMap/app"}, client.createCalls[0])
	assert.Nil(t, rel.SequencingInfo)
}

func TestInstall_Sequenced_HookResourcesExcludedFromDAG(t *testing.T) {
	client := newRecordingKubeClient()
	install := newSequencedInstallAction(t, client)

	ch := buildChartWithTemplates([]*common.File{
		makeHookTemplate("templates/pre-hook.yaml", "pre-hook", release.HookPreInstall.String(), map[string]string{
			releaseutil.AnnotationResourceGroup: "database",
		}),
		makeConfigMapTemplate("templates/database.yaml", "database", map[string]string{
			releaseutil.AnnotationResourceGroup: "database",
		}),
	})

	mustRunInstall(t, install, ch)

	assert.Equal(t, [][]string{{"ConfigMap/pre-hook"}, {"ConfigMap/database"}}, client.createCalls)
	assert.Equal(t, [][]string{{"ConfigMap/pre-hook"}}, client.watchUntilReadyCalls)
}

func TestInstall_Sequenced_HooksAtStandardPositions(t *testing.T) {
	client := newRecordingKubeClient()
	install := newSequencedInstallAction(t, client)

	ch := buildChartWithTemplates([]*common.File{
		makeHookTemplate("templates/pre-hook.yaml", "pre-hook", release.HookPreInstall.String(), map[string]string{
			releaseutil.AnnotationResourceGroup: "database",
		}),
		makeConfigMapTemplate("templates/database.yaml", "database", map[string]string{
			releaseutil.AnnotationResourceGroup: "database",
		}),
		makeHookTemplate("templates/post-hook.yaml", "post-hook", release.HookPostInstall.String(), map[string]string{
			releaseutil.AnnotationResourceGroup: "database",
		}),
	})

	mustRunInstall(t, install, ch)

	assert.Equal(t, [][]string{{"ConfigMap/pre-hook"}, {"ConfigMap/database"}, {"ConfigMap/post-hook"}}, client.createCalls)
	assert.Equal(t, [][]string{{"ConfigMap/pre-hook"}, {"ConfigMap/post-hook"}}, client.watchUntilReadyCalls)
}

func TestSequencing_WarnIfPartialReadinessAnnotations(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn})
	oldLogger := slog.Default()
	slog.SetDefault(slog.New(handler))
	t.Cleanup(func() { slog.SetDefault(oldLogger) })

	warnIfPartialReadinessAnnotations(slog.Default(), []releaseutil.Manifest{
		makeTestManifest("cm", "chart/templates/cm.yaml", map[string]string{
			kube.AnnotationReadinessSuccess: `["{.ready} == true"]`,
		}),
	})

	assert.Contains(t, buf.String(), "readiness")
}

func TestSequencedDeployment_CreateAndWait_RespectsContextCancellation(t *testing.T) {
	newSequencedDeploymentForTest := func(client kube.Interface) *sequencedDeployment {
		t.Helper()

		cfg := actionConfigFixture(t)
		cfg.KubeClient = client

		return &sequencedDeployment{
			cfg:              cfg,
			releaseName:      "demo",
			releaseNamespace: "spaced",
			waitStrategy:     kube.OrderedWaitStrategy,
			readinessTimeout: time.Minute,
		}
	}

	t.Run("context canceled before build", func(t *testing.T) {
		client := newRecordingKubeClient()
		sd := newSequencedDeploymentForTest(client)

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		err := sd.createAndWait(ctx, []releaseutil.Manifest{
			makeTestManifest("cm", "chart/templates/cm.yaml", nil),
		})
		require.ErrorIs(t, err, context.Canceled)
		assert.Empty(t, client.createCalls)
		assert.Empty(t, client.waitCalls)
	})

	t.Run("context canceled after build before create", func(t *testing.T) {
		client := newRecordingKubeClient()
		ctx, cancel := context.WithCancel(context.Background())
		client.onBuild = cancel

		sd := newSequencedDeploymentForTest(client)

		err := sd.createAndWait(ctx, []releaseutil.Manifest{
			makeTestManifest("cm", "chart/templates/cm.yaml", nil),
		})
		require.ErrorIs(t, err, context.Canceled)
		assert.Empty(t, client.createCalls)
		assert.Empty(t, client.waitCalls)
	})

	t.Run("context canceled after create before wait", func(t *testing.T) {
		client := newRecordingKubeClient()
		ctx, cancel := context.WithCancel(context.Background())
		client.onCreate = cancel

		sd := newSequencedDeploymentForTest(client)

		err := sd.createAndWait(ctx, []releaseutil.Manifest{
			makeTestManifest("cm", "chart/templates/cm.yaml", nil),
		})
		require.ErrorIs(t, err, context.Canceled)
		require.Len(t, client.createCalls, 1)
		assert.Empty(t, client.waitCalls)
	})
}

func TestFindSubchart(t *testing.T) {
	// makeSubchart constructs a chart with the given chart-name (its own
	// Metadata.Name — what BuildSubchartDAG and findSubchart resolve against).
	makeSubchart := func(chartName string) *chart.Chart {
		return &chart.Chart{
			Metadata: &chart.Metadata{
				APIVersion: "v1",
				Name:       chartName,
				Version:    "0.1.0",
			},
		}
	}

	// makeParent attaches subcharts as dependencies and declares the
	// parent's Metadata.Dependencies (which carry the Alias field).
	makeParent := func(deps []*chart.Chart, metaDeps []*chart.Dependency) *chart.Chart {
		parent := &chart.Chart{
			Metadata: &chart.Metadata{
				APIVersion:   "v1",
				Name:         "parent",
				Version:      "0.1.0",
				Dependencies: metaDeps,
			},
		}
		for _, d := range deps {
			parent.AddDependency(d)
		}
		return parent
	}

	t.Run("resolves by chart name when no alias is declared", func(t *testing.T) {
		db := makeSubchart("database")
		parent := makeParent(
			[]*chart.Chart{db},
			[]*chart.Dependency{{Name: "database"}},
		)

		got := findSubchart(parent, "database")
		require.NotNil(t, got)
		assert.Equal(t, "database", got.Name())
	})

	t.Run("resolves by alias when alias is declared", func(t *testing.T) {
		postgres := makeSubchart("postgres")
		parent := makeParent(
			[]*chart.Chart{postgres},
			[]*chart.Dependency{{Name: "postgres", Alias: "db"}},
		)

		got := findSubchart(parent, "db")
		require.NotNil(t, got, "alias lookup should resolve to the underlying chart")
		assert.Equal(t, "postgres", got.Name())
	})

	t.Run("resolves by underlying chart name even when an alias is declared", func(t *testing.T) {
		// An alias does not hide the chart's real name — manifests rendered
		// under the chart's actual chart-name path should still resolve.
		postgres := makeSubchart("postgres")
		parent := makeParent(
			[]*chart.Chart{postgres},
			[]*chart.Dependency{{Name: "postgres", Alias: "db"}},
		)

		got := findSubchart(parent, "postgres")
		require.NotNil(t, got)
		assert.Equal(t, "postgres", got.Name())
	})

	t.Run("alias collides with another chart's real name — first match wins", func(t *testing.T) {
		// dep1: chart "foo" aliased as "bar".
		// dep2: chart "bar" with no alias.
		// Query "bar" must resolve deterministically. Current contract:
		// iteration order over Dependencies() is preserved, so the first
		// dep whose effective name (alias or real) matches the query wins.
		foo := makeSubchart("foo")
		bar := makeSubchart("bar")
		parent := makeParent(
			[]*chart.Chart{foo, bar},
			[]*chart.Dependency{
				{Name: "foo", Alias: "bar"},
				{Name: "bar"},
			},
		)

		got := findSubchart(parent, "bar")
		require.NotNil(t, got, "collision must resolve, not return nil")
		assert.Equal(t, "foo", got.Name(),
			"first matching dependency wins; aliased 'foo' is declared before raw 'bar'")

		// And the raw name "foo" must still resolve to chart "foo" even
		// though its effective name has been shifted by the alias.
		gotFoo := findSubchart(parent, "foo")
		require.NotNil(t, gotFoo)
		assert.Equal(t, "foo", gotFoo.Name())
	})

	t.Run("returns nil when not found", func(t *testing.T) {
		db := makeSubchart("database")
		parent := makeParent(
			[]*chart.Chart{db},
			[]*chart.Dependency{{Name: "database"}},
		)

		assert.Nil(t, findSubchart(parent, "nonexistent"))
	})

	t.Run("returns nil when parent has no dependencies", func(t *testing.T) {
		parent := &chart.Chart{
			Metadata: &chart.Metadata{APIVersion: "v1", Name: "parent", Version: "0.1.0"},
		}
		assert.Nil(t, findSubchart(parent, "anything"))
	})
}

func TestStripSequencingAnnotations(t *testing.T) {
	// makeInfo constructs a minimal *resource.Info backed by an unstructured
	// ConfigMap. stripSequencingAnnotations only needs meta.Accessor to work,
	// so Name + Object are sufficient — no Mapping/Client required.
	makeInfo := func(name string, annotations map[string]string) *resource.Info {
		u := &unstructured.Unstructured{}
		u.SetAPIVersion("v1")
		u.SetKind("ConfigMap")
		u.SetName(name)
		if annotations != nil {
			u.SetAnnotations(annotations)
		}
		return &resource.Info{Name: name, Object: u}
	}

	annotationsOf := func(info *resource.Info) map[string]string {
		acc, err := meta.Accessor(info.Object)
		require.NoError(t, err)
		return acc.GetAnnotations()
	}

	t.Run("strips depends-on annotation, preserves siblings", func(t *testing.T) {
		info := makeInfo("cm", map[string]string{
			releaseutil.AnnotationDependsOnResourceGroups: `["foo"]`,
			releaseutil.AnnotationResourceGroup:           "app",
			"other.example.com/keep":                      "yes",
		})
		resources := kube.ResourceList{info}

		require.NoError(t, stripSequencingAnnotations(resources))

		ann := annotationsOf(info)
		assert.NotContains(t, ann, releaseutil.AnnotationDependsOnResourceGroups,
			"helm-internal depends-on annotation must be removed")
		assert.Equal(t, "app", ann[releaseutil.AnnotationResourceGroup],
			"single-slash helm.sh/resource-group is a valid k8s key and must be preserved")
		assert.Equal(t, "yes", ann["other.example.com/keep"],
			"unrelated annotations must be preserved")
	})

	t.Run("no-op when resource has no annotations", func(t *testing.T) {
		info := makeInfo("cm", nil)
		resources := kube.ResourceList{info}

		require.NoError(t, stripSequencingAnnotations(resources))
		assert.Empty(t, annotationsOf(info))
	})

	t.Run("no-op when resource has only non-sequencing annotations", func(t *testing.T) {
		info := makeInfo("cm", map[string]string{
			"keep.example.com/one": "1",
			"keep.example.com/two": "2",
		})
		resources := kube.ResourceList{info}

		require.NoError(t, stripSequencingAnnotations(resources))

		ann := annotationsOf(info)
		assert.Equal(t, "1", ann["keep.example.com/one"])
		assert.Equal(t, "2", ann["keep.example.com/two"])
	})

	t.Run("strips across a multi-resource batch", func(t *testing.T) {
		a := makeInfo("a", map[string]string{
			releaseutil.AnnotationDependsOnResourceGroups: `["x"]`,
			releaseutil.AnnotationResourceGroup:           "g1",
		})
		b := makeInfo("b", map[string]string{
			releaseutil.AnnotationDependsOnResourceGroups: `["y"]`,
			releaseutil.AnnotationResourceGroup:           "g2",
		})
		c := makeInfo("c", map[string]string{
			"unrelated": "value",
		})
		resources := kube.ResourceList{a, b, c}

		require.NoError(t, stripSequencingAnnotations(resources))

		for _, info := range []*resource.Info{a, b} {
			ann := annotationsOf(info)
			assert.NotContains(t, ann, releaseutil.AnnotationDependsOnResourceGroups,
				"resource %s should have depends-on stripped", info.Name)
			assert.Contains(t, ann, releaseutil.AnnotationResourceGroup,
				"resource %s should retain resource-group", info.Name)
		}
		assert.Equal(t, "value", annotationsOf(c)["unrelated"],
			"resources without sequencing annotations are untouched")
	})

	t.Run("strips every key listed in HelmInternalSequencingAnnotations", func(t *testing.T) {
		// Lock the invariant: the function must remove EVERY key in the
		// HelmInternalSequencingAnnotations list, not just depends-on.
		ann := map[string]string{"keep": "yes"}
		for _, k := range releaseutil.HelmInternalSequencingAnnotations {
			ann[k] = "internal"
		}
		info := makeInfo("cm", ann)

		require.NoError(t, stripSequencingAnnotations(kube.ResourceList{info}))

		got := annotationsOf(info)
		for _, k := range releaseutil.HelmInternalSequencingAnnotations {
			assert.NotContains(t, got, k, "internal key %q must be stripped", k)
		}
		assert.Equal(t, "yes", got["keep"])
	})
}

func makeTestManifest(name, sourcePath string, annotations map[string]string) releaseutil.Manifest {
	content := configMapManifest(name, annotations)
	head := &releaseutil.SimpleHead{
		Version: v1.SchemeGroupVersion.String(),
		Kind:    "ConfigMap",
	}
	head.Metadata = &struct {
		Name        string            `json:"name"`
		Namespace   string            `json:"namespace,omitempty"`
		Annotations map[string]string `json:"annotations"`
	}{
		Name:        name,
		Annotations: annotations,
	}
	return releaseutil.Manifest{Name: sourcePath, Content: content, Head: head}
}

func mustRunInstall(t *testing.T, install *Install, ch *chart.Chart) ri.Releaser {
	t.Helper()
	rel, err := install.RunWithContext(context.Background(), ch, map[string]any{})
	require.NoError(t, err)
	return rel
}
