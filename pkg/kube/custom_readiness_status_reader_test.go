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

package kube

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/fluxcd/cli-utils/pkg/kstatus/polling/clusterreader"
	"github.com/fluxcd/cli-utils/pkg/kstatus/polling/statusreaders"
	"github.com/fluxcd/cli-utils/pkg/kstatus/status"
	"github.com/fluxcd/cli-utils/pkg/object"
	"github.com/fluxcd/cli-utils/pkg/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/scheme"
)

func TestStatusWaitWithCustomReadinessReader(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		manifest         string
		expectErrStrs    []string
		notExpectErrStrs []string
	}{
		{
			name: "custom readiness makes resource current",
			manifest: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: ready-config
  namespace: default
  annotations:
    helm.sh/readiness-success: '["{.phase} == \"Ready\""]'
    helm.sh/readiness-failure: '["{.phase} == \"Failed\""]'
status:
  phase: Ready
`,
		},
		{
			name: "custom readiness failure stops wait",
			manifest: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: failed-config
  namespace: default
  annotations:
    helm.sh/readiness-success: '["{.phase} == \"Ready\""]'
    helm.sh/readiness-failure: '["{.phase} == \"Failed\""]'
status:
  phase: Failed
`,
			expectErrStrs: []string{"resource ConfigMap/default/failed-config not ready. status: Failed"},
		},
		{
			name: "resources without annotations fall back to kstatus",
			manifest: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: current-config
  namespace: default
`,
		},
		{
			// END-TO-END pin for hip-0025-gy6: an ordering operator on a
			// non-numeric value must leave the resource InProgress (and let
			// the wait time out per-resource) — NOT kill the status reporter
			// and abort the whole batch.
			name: "ordering operator on non-numeric value does not abort the wait",
			manifest: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: bad-ordering
  namespace: default
  annotations:
    helm.sh/readiness-success: '["{.phase} > \"Ready\""]'
    helm.sh/readiness-failure: '["{.phase} == \"Failed\""]'
status:
  phase: Running
`,
			expectErrStrs:    []string{"resource ConfigMap/default/bad-ordering not ready. status: InProgress", "skipped"},
			notExpectErrStrs: []string{"failed to compute object status"},
		},
		{
			name: "ordering on numeric string value compares numerically",
			manifest: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: numeric-string
  namespace: default
  annotations:
    helm.sh/readiness-success: '["{.phase} >= 5"]'
    helm.sh/readiness-failure: '["{.failed} >= 1"]'
status:
  phase: "5"
`,
		},
		{
			name: "incomparable expression skipped when another success condition is met",
			manifest: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: mixed-success
  namespace: default
  annotations:
    helm.sh/readiness-success: '["{.phase} > \"Ready\"", "{.phase} == \"Running\""]'
    helm.sh/readiness-failure: '["{.phase} == \"Failed\""]'
status:
  phase: Running
`,
		},
		{
			// Partial annotations (only one of success/failure): custom
			// evaluation must NOT engage; the resource is judged by the
			// default chain (a bare ConfigMap is Current), even though the
			// success expression is unmet.
			name: "partial annotation falls back to default readiness",
			manifest: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: partial-config
  namespace: default
  annotations:
    helm.sh/readiness-success: '["{.phase} == \"Ready\""]'
status:
  phase: NotReady
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c := newTestClient(t)
			fakeClient := fake.NewSimpleDynamicClient(scheme.Scheme)
			fakeMapper := testutil.NewFakeRESTMapper(v1.SchemeGroupVersion.WithKind("ConfigMap"))
			waiter := statusWaiter{
				client:          fakeClient,
				restMapper:      fakeMapper,
				customReadiness: true,
			}

			objs := getRuntimeObjFromManifests(t, []string{tt.manifest})
			for _, obj := range objs {
				u := obj.(*unstructured.Unstructured)
				gvr := getGVR(t, fakeMapper, u)
				require.NoError(t, fakeClient.Tracker().Create(gvr, u, u.GetNamespace()))
			}

			resourceList := getResourceListFromRuntimeObjs(t, c, objs)
			err := waiter.Wait(resourceList, time.Second)
			if tt.expectErrStrs == nil {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			for _, expected := range tt.expectErrStrs {
				assert.Contains(t, err.Error(), expected)
			}
			for _, unexpected := range tt.notExpectErrStrs {
				assert.NotContains(t, err.Error(), unexpected)
			}
		})
	}
}

func TestWithCustomReadinessStatusReader(t *testing.T) {
	opts := &waitOptions{}
	WithCustomReadinessStatusReader()(opts)
	assert.True(t, opts.enableCustomReadinessStatusReader)
}

func TestCustomReadinessStatusReaderReadStatusForObject(t *testing.T) {
	reader := newCustomReadinessStatusReader(nil, statusreaders.NewDefaultStatusReader(testutil.NewFakeRESTMapper(v1.SchemeGroupVersion.WithKind("ConfigMap"))))
	u := &unstructured.Unstructured{}
	u.SetAPIVersion("v1")
	u.SetKind("ConfigMap")
	u.SetName("ready-config")
	u.SetNamespace("default")
	u.SetAnnotations(map[string]string{
		AnnotationReadinessSuccess: `["{.phase} == \"Ready\""]`,
		AnnotationReadinessFailure: `["{.phase} == \"Failed\""]`,
	})
	require.NoError(t, unstructured.SetNestedField(u.Object, "Ready", "status", "phase"))

	result, err := reader.ReadStatusForObject(context.Background(), nil, u)
	require.NoError(t, err)
	assert.Equal(t, status.CurrentStatus, result.Status)
}

func TestCustomReadinessStatusReaderWarnsOnceForIncomparableExpression(t *testing.T) {
	var buf bytes.Buffer
	reader := newCustomReadinessStatusReader(slog.New(slog.NewTextHandler(&buf, nil)), statusreaders.NewDefaultStatusReader(testutil.NewFakeRESTMapper(v1.SchemeGroupVersion.WithKind("ConfigMap"))))

	u := &unstructured.Unstructured{}
	u.SetAPIVersion("v1")
	u.SetKind("ConfigMap")
	u.SetName("bad-ordering")
	u.SetNamespace("default")
	u.SetAnnotations(map[string]string{
		AnnotationReadinessSuccess: `["{.phase} > \"Ready\""]`,
		AnnotationReadinessFailure: `["{.failed} >= 1"]`,
	})
	require.NoError(t, unstructured.SetNestedField(u.Object, "Running", "status", "phase"))

	for i := 0; i < 2; i++ {
		result, err := reader.ReadStatusForObject(context.Background(), nil, u)
		require.NoError(t, err)
		assert.Equal(t, status.InProgressStatus, result.Status)
		assert.Contains(t, result.Message, "skipped")
		assert.Contains(t, result.Message, "ordering operators")
	}

	logged := buf.String()
	assert.Equal(t, 1, strings.Count(logged, "treating condition as not met"),
		"warning must be deduplicated to once per resource+expression")
	assert.Contains(t, logged, "bad-ordering")
	assert.Contains(t, logged, "ordering operators")
}

// The composite Deployment reader is the fallback observable: it populates
// GeneratedResources with the child ReplicaSets it aggregated, which neither
// the generic kstatus reader nor a bare status.Compute ever does. (In
// cli-utils v1.2.2 the composite readers compute the same top-level status as
// status.Compute, so GeneratedResources is the distinguishing signal that the
// default chain — not an internal kstatus shortcut — handled the resource.)
func TestCustomReadinessReaderDelegatesToFallbackChain(t *testing.T) {
	t.Parallel()

	deploymentYAML := func(annotations string) string {
		return `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
  namespace: default
  generation: 1` + annotations + `
spec:
  replicas: 1
  selector:
    matchLabels:
      app: web
status:
  observedGeneration: 1
  replicas: 1
  updatedReplicas: 1
  readyReplicas: 1
  availableReplicas: 1
  conditions:
  - type: Available
    status: "True"
    reason: MinimumReplicasAvailable
`
	}
	replicaSetManifest := `
apiVersion: apps/v1
kind: ReplicaSet
metadata:
  name: web-abc123
  namespace: default
  generation: 1
  labels:
    app: web
spec:
  replicas: 1
  selector:
    matchLabels:
      app: web
status:
  observedGeneration: 1
  replicas: 1
  readyReplicas: 1
  availableReplicas: 1
`

	tests := []struct {
		name                   string
		annotations            string
		expectStatus           status.Status
		expectGeneratedResults bool
	}{
		{
			// No annotations: must be delegated to the composite chain.
			name:                   "non-annotated deployment handled by composite reader",
			annotations:            "",
			expectStatus:           status.CurrentStatus,
			expectGeneratedResults: true,
		},
		{
			// One annotation only: partial config must also delegate.
			name: "partial-annotated deployment handled by composite reader",
			annotations: `
  annotations:
    helm.sh/readiness-success: '["{.readyReplicas} >= 1"]'`,
			expectStatus:           status.CurrentStatus,
			expectGeneratedResults: true,
		},
		{
			// Both annotations: custom evaluation engages (unmet expression
			// keeps it InProgress) and the fallback chain is bypassed.
			name: "fully annotated deployment evaluated by expressions",
			annotations: `
  annotations:
    helm.sh/readiness-success: '["{.readyReplicas} >= 2"]'
    helm.sh/readiness-failure: '["{.failed} >= 1"]'`,
			expectStatus:           status.InProgressStatus,
			expectGeneratedResults: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			fakeMapper := testutil.NewFakeRESTMapper(
				appsv1.SchemeGroupVersion.WithKind("Deployment"),
				appsv1.SchemeGroupVersion.WithKind("ReplicaSet"),
				v1.SchemeGroupVersion.WithKind("Pod"),
			)
			fakeClient := fake.NewSimpleDynamicClient(scheme.Scheme)
			objs := getRuntimeObjFromManifests(t, []string{deploymentYAML(tt.annotations), replicaSetManifest})
			for _, obj := range objs {
				u := obj.(*unstructured.Unstructured)
				gvr := getGVR(t, fakeMapper, u)
				require.NoError(t, fakeClient.Tracker().Create(gvr, u, u.GetNamespace()))
			}
			clusterReader := &clusterreader.DynamicClusterReader{
				DynamicClient: fakeClient,
				Mapper:        fakeMapper,
			}
			reader := newCustomReadinessStatusReader(nil, statusreaders.NewStatusReader(fakeMapper))

			deployment := objs[0].(*unstructured.Unstructured)
			result, err := reader.ReadStatusForObject(context.Background(), clusterReader, deployment)
			require.NoError(t, err)
			assert.Equal(t, tt.expectStatus, result.Status)
			if tt.expectGeneratedResults {
				require.NotEmpty(t, result.GeneratedResources,
					"fallback composite reader must aggregate the child ReplicaSet")
				assert.Equal(t, "web-abc123", result.GeneratedResources[0].Identifier.Name)
			} else {
				assert.Empty(t, result.GeneratedResources)
				assert.Contains(t, result.Message, "custom readiness")
			}

			// The identifier-based entry point must route identically.
			id, err := object.RuntimeToObjMeta(deployment)
			require.NoError(t, err)
			byID, err := reader.ReadStatus(context.Background(), clusterReader, id)
			require.NoError(t, err)
			assert.Equal(t, tt.expectStatus, byID.Status)
			assert.Equal(t, tt.expectGeneratedResults, len(byID.GeneratedResources) > 0)
		})
	}
}

// A plain (non-annotated) Job in a batch that enables custom readiness must
// keep helm's completion gating: helm's job reader holds a started Job
// InProgress until a Complete/Failed condition, while plain kstatus reports a
// started Job as Current. Before the fix the custom readiness reader shadowed
// the job reader and the wait returned early.
func TestStatusWaitWithJobsMixedCustomReadiness(t *testing.T) {
	t.Parallel()

	startedJobManifest := `
apiVersion: batch/v1
kind: Job
metadata:
  name: slow-job
  namespace: default
  generation: 1
status:
  startTime: 2025-02-06T16:34:20-05:00
  active: 1
  ready: 1
`
	completeJobManifest := `
apiVersion: batch/v1
kind: Job
metadata:
  name: slow-job
  namespace: default
  generation: 1
status:
  succeeded: 1
  conditions:
  - type: Complete
    status: "True"
`
	readyConfigMapManifest := `
apiVersion: v1
kind: ConfigMap
metadata:
  name: gated-config
  namespace: default
  annotations:
    helm.sh/readiness-success: '["{.phase} == \"Ready\""]'
    helm.sh/readiness-failure: '["{.phase} == \"Failed\""]'
status:
  phase: Ready
`
	pendingConfigMapManifest := `
apiVersion: v1
kind: ConfigMap
metadata:
  name: gated-config
  namespace: default
  annotations:
    helm.sh/readiness-success: '["{.phase} == \"Ready\""]'
    helm.sh/readiness-failure: '["{.phase} == \"Failed\""]'
status:
  phase: Pending
`

	tests := []struct {
		name             string
		manifests        []string
		expectErrStrs    []string
		notExpectErrStrs []string
	}{
		{
			name:      "plain job keeps completion gating in mixed batch",
			manifests: []string{startedJobManifest, readyConfigMapManifest},
			expectErrStrs: []string{
				"resource Job/default/slow-job not ready. status: InProgress",
			},
			notExpectErrStrs: []string{"ConfigMap"},
		},
		{
			name:      "annotated resource keeps expression gating in mixed batch",
			manifests: []string{completeJobManifest, pendingConfigMapManifest},
			expectErrStrs: []string{
				"resource ConfigMap/default/gated-config not ready. status: InProgress",
			},
			notExpectErrStrs: []string{"Job/default/slow-job"},
		},
		{
			name:      "batch completes when job is complete and expression met",
			manifests: []string{completeJobManifest, readyConfigMapManifest},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c := newTestClient(t)
			fakeClient := fake.NewSimpleDynamicClient(scheme.Scheme)
			fakeMapper := testutil.NewFakeRESTMapper(
				batchv1.SchemeGroupVersion.WithKind("Job"),
				v1.SchemeGroupVersion.WithKind("ConfigMap"),
			)
			waiter := statusWaiter{
				client:          fakeClient,
				restMapper:      fakeMapper,
				customReadiness: true,
			}
			objs := getRuntimeObjFromManifests(t, tt.manifests)
			for _, obj := range objs {
				u := obj.(*unstructured.Unstructured)
				gvr := getGVR(t, fakeMapper, u)
				require.NoError(t, fakeClient.Tracker().Create(gvr, u, u.GetNamespace()))
			}
			resourceList := getResourceListFromRuntimeObjs(t, c, objs)
			err := waiter.WaitWithJobs(resourceList, 2*time.Second)
			if tt.expectErrStrs == nil {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			for _, expected := range tt.expectErrStrs {
				assert.Contains(t, err.Error(), expected)
			}
			for _, unexpected := range tt.notExpectErrStrs {
				assert.NotContains(t, err.Error(), unexpected)
			}
		})
	}
}

// WatchUntilReady is the hook path: hook readiness must be governed by the
// hook readers (helm's job/pod readers + always-ready for everything else),
// never by the readiness annotations, even when custom readiness is enabled
// on the waiter. Two directions:
//   - a started-but-incomplete hook Job must still time out (kstatus would
//     wrongly report a started Job as Current);
//   - an annotated non-Job hook must complete even though its success
//     expression is unmet (annotations are ignored for hooks).
func TestWatchUntilReadyIgnoresCustomReadiness(t *testing.T) {
	t.Parallel()

	startedJobManifest := `
apiVersion: batch/v1
kind: Job
metadata:
  name: hook-job
  namespace: default
  generation: 1
status:
  startTime: 2025-02-06T16:34:20-05:00
  active: 1
`
	completeJobManifest := `
apiVersion: batch/v1
kind: Job
metadata:
  name: hook-job
  namespace: default
  generation: 1
status:
  succeeded: 1
  conditions:
  - type: Complete
    status: "True"
`
	annotatedConfigMapManifest := `
apiVersion: v1
kind: ConfigMap
metadata:
  name: hook-config
  namespace: default
  annotations:
    helm.sh/readiness-success: '["{.phase} == \"Ready\""]'
    helm.sh/readiness-failure: '["{.phase} == \"Failed\""]'
status:
  phase: Pending
`

	tests := []struct {
		name          string
		manifests     []string
		expectErrStrs []string
	}{
		{
			name:      "incomplete hook job still gated by hook job reader",
			manifests: []string{startedJobManifest},
			expectErrStrs: []string{
				"resource Job/default/hook-job not ready. status: InProgress",
			},
		},
		{
			name:      "annotated hook resource ignores readiness annotations",
			manifests: []string{annotatedConfigMapManifest},
		},
		{
			name:      "complete hook job succeeds",
			manifests: []string{completeJobManifest},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c := newTestClient(t)
			fakeClient := fake.NewSimpleDynamicClient(scheme.Scheme)
			fakeMapper := testutil.NewFakeRESTMapper(
				batchv1.SchemeGroupVersion.WithKind("Job"),
				v1.SchemeGroupVersion.WithKind("ConfigMap"),
			)
			waiter := statusWaiter{
				client:          fakeClient,
				restMapper:      fakeMapper,
				customReadiness: true,
			}
			objs := getRuntimeObjFromManifests(t, tt.manifests)
			for _, obj := range objs {
				u := obj.(*unstructured.Unstructured)
				gvr := getGVR(t, fakeMapper, u)
				require.NoError(t, fakeClient.Tracker().Create(gvr, u, u.GetNamespace()))
			}
			resourceList := getResourceListFromRuntimeObjs(t, c, objs)
			err := waiter.WatchUntilReady(resourceList, 2*time.Second)
			if tt.expectErrStrs == nil {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			for _, expected := range tt.expectErrStrs {
				assert.Contains(t, err.Error(), expected)
			}
		})
	}
}
