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
}

type updateCall struct {
	current []string
	target  []string
	created []string
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
		current: currentIDs,
		target:  targetIDs,
		created: createdIDs,
	})
	c.operations = append(c.operations, "update:"+strings.Join(targetIDs, ","))
	return &kube.Result{Updated: target, Created: created}, nil
}

func (c *recordingKubeClient) Build(reader io.Reader, _ bool) (kube.ResourceList, error) {
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
	for key, value := range extraAnnotations {
		annotations[key] = value
	}
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
	assert.Equal(t, [][]string{{"ConfigMap/database"}, {"ConfigMap/app"}}, client.waitCalls)
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

	warnIfPartialReadinessAnnotations([]releaseutil.Manifest{
		makeTestManifest("cm", "chart/templates/cm.yaml", map[string]string{
			kube.AnnotationReadinessSuccess: `["{.ready} == true"]`,
		}),
	})

	assert.Contains(t, buf.String(), "readiness")
}

func makeTestManifest(name, sourcePath string, annotations map[string]string) releaseutil.Manifest {
	content := configMapManifest(name, annotations)
	head := &releaseutil.SimpleHead{
		Version: v1.SchemeGroupVersion.String(),
		Kind:    "ConfigMap",
	}
	head.Metadata = &struct {
		Name        string            `json:"name"`
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
