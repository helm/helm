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

	"github.com/fluxcd/cli-utils/pkg/kstatus/polling/clusterreader"
	"github.com/fluxcd/cli-utils/pkg/kstatus/polling/engine"
	"github.com/fluxcd/cli-utils/pkg/kstatus/polling/statusreaders"
	"github.com/fluxcd/cli-utils/pkg/kstatus/status"
	"github.com/fluxcd/cli-utils/pkg/object"
	"github.com/fluxcd/cli-utils/pkg/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/kubectl/pkg/scheme"
)

func TestCustomReadinessStatusReaderReadStatusForObject(t *testing.T) {
	u := customReadinessTestObject(t, "ready-config", map[string]string{
		AnnotationReadinessSuccess: `["{.phase} == \"Ready\""]`,
		AnnotationReadinessFailure: `["{.phase} == \"Failed\""]`,
	}, "Ready")
	reader := newCustomReadinessStatusReader(nil, customReadinessFallback(), eligibleReadinessObjects(t, u))

	result, err := reader.ReadStatusForObject(context.Background(), nil, u)
	require.NoError(t, err)
	assert.Equal(t, status.CurrentStatus, result.Status)
}

func TestCustomReadinessStatusReaderIneligibleUsesFallback(t *testing.T) {
	u := customReadinessTestObject(t, "ineligible-config", map[string]string{
		AnnotationReadinessSuccess: `["{.phase} == \"Ready\""]`,
		AnnotationReadinessFailure: `["{.phase} == \"Failed\""]`,
	}, "Waiting")
	fallback := customReadinessFallback()
	want, err := fallback.ReadStatusForObject(context.Background(), nil, u)
	require.NoError(t, err)
	reader := newCustomReadinessStatusReader(nil, fallback, nil)

	got, err := reader.ReadStatusForObject(context.Background(), nil, u)
	require.NoError(t, err)
	assert.Equal(t, want, got)
	assert.Equal(t, status.CurrentStatus, got.Status)
}

func TestCustomReadinessStatusReaderPartialPairWarnsOnceAndUsesFallback(t *testing.T) {
	var buf bytes.Buffer
	u := customReadinessTestObject(t, "partial-config", map[string]string{
		AnnotationReadinessSuccess: `["{.phase} == \"Ready\""]`,
	}, "Waiting")
	fallback := customReadinessFallback()
	want, err := fallback.ReadStatusForObject(context.Background(), nil, u)
	require.NoError(t, err)
	reader := newCustomReadinessStatusReader(
		slog.New(slog.NewTextHandler(&buf, nil)),
		fallback,
		eligibleReadinessObjects(t, u),
	)

	for range 2 {
		got, err := reader.ReadStatusForObject(context.Background(), nil, u)
		require.NoError(t, err)
		assert.Equal(t, want, got)
	}

	logged := buf.String()
	assert.Equal(t, 1, strings.Count(logged, "custom readiness annotations must be used together"))
	assert.Contains(t, logged, AnnotationReadinessSuccess)
	assert.Contains(t, logged, AnnotationReadinessFailure)
}

func TestCustomReadinessStatusReaderMalformedAnnotationIsResourceQualified(t *testing.T) {
	u := customReadinessTestObject(t, "malformed-config", map[string]string{
		AnnotationReadinessSuccess: `not-json`,
		AnnotationReadinessFailure: `["{.phase} == \"Failed\""]`,
	}, "Waiting")
	reader := newCustomReadinessStatusReader(nil, customReadinessFallback(), eligibleReadinessObjects(t, u))

	_, err := reader.ReadStatusForObject(context.Background(), nil, u)
	require.Error(t, err)
	assert.Contains(t, err.Error(), AnnotationReadinessSuccess)
	assert.Contains(t, err.Error(), "default")
	assert.Contains(t, err.Error(), "malformed-config")
}

func TestCustomReadinessStatusReaderWarnsOnceForIncomparableExpression(t *testing.T) {
	var buf bytes.Buffer
	u := customReadinessTestObject(t, "bad-ordering", map[string]string{
		AnnotationReadinessSuccess: `["{.phase} > \"Ready\""]`,
		AnnotationReadinessFailure: `["{.failed} >= 1"]`,
	}, "Running")
	reader := newCustomReadinessStatusReader(
		slog.New(slog.NewTextHandler(&buf, nil)),
		customReadinessFallback(),
		eligibleReadinessObjects(t, u),
	)

	for range 2 {
		result, err := reader.ReadStatusForObject(context.Background(), nil, u)
		require.NoError(t, err)
		assert.Equal(t, status.InProgressStatus, result.Status)
		assert.Contains(t, result.Message, "skipped")
		assert.Contains(t, result.Message, "ordering operators")
	}

	logged := buf.String()
	assert.Equal(t, 1, strings.Count(logged, "treating condition as not met"))
	assert.Contains(t, logged, "bad-ordering")
	assert.Contains(t, logged, "ordering operators")
}

func TestCustomReadinessReaderDelegatesToFallbackChain(t *testing.T) {
	t.Parallel()

	deploymentManifest := func(annotations string) string {
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
		name             string
		annotations      string
		wantStatus       status.Status
		wantGeneratedSet bool
	}{
		{
			name:             "unannotated deployment uses composite reader",
			wantStatus:       status.CurrentStatus,
			wantGeneratedSet: true,
		},
		{
			name: "partial pair uses composite reader",
			annotations: `
  annotations:
    helm.sh/readiness-success: '["{.readyReplicas} >= 1"]'`,
			wantStatus:       status.CurrentStatus,
			wantGeneratedSet: true,
		},
		{
			name: "complete pair bypasses composite reader",
			annotations: `
  annotations:
    helm.sh/readiness-success: '["{.readyReplicas} >= 2"]'
    helm.sh/readiness-failure: '["{.failed} >= 1"]'`,
			wantStatus: status.InProgressStatus,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			mapper := testutil.NewFakeRESTMapper(
				appsv1.SchemeGroupVersion.WithKind("Deployment"),
				appsv1.SchemeGroupVersion.WithKind("ReplicaSet"),
				v1.SchemeGroupVersion.WithKind("Pod"),
			)
			client := dynamicfake.NewSimpleDynamicClient(scheme.Scheme)
			objs := getRuntimeObjFromManifests(t, []string{deploymentManifest(tt.annotations), replicaSetManifest})
			for _, obj := range objs {
				u := obj.(*unstructured.Unstructured)
				require.NoError(t, client.Tracker().Create(getGVR(t, mapper, u), u, u.GetNamespace()))
			}
			clusterReader := &clusterreader.DynamicClusterReader{DynamicClient: client, Mapper: mapper}
			deployment := objs[0].(*unstructured.Unstructured)
			reader := newCustomReadinessStatusReader(
				nil,
				statusreaders.NewStatusReader(mapper),
				eligibleReadinessObjects(t, deployment),
			)

			got, err := reader.ReadStatusForObject(context.Background(), clusterReader, deployment)
			require.NoError(t, err)
			assert.Equal(t, tt.wantStatus, got.Status)
			assert.Equal(t, tt.wantGeneratedSet, len(got.GeneratedResources) > 0)
			if tt.wantGeneratedSet {
				assert.Equal(t, "web-abc123", got.GeneratedResources[0].Identifier.Name)
			} else {
				assert.Contains(t, got.Message, "custom readiness")
			}

			id, err := object.RuntimeToObjMeta(deployment)
			require.NoError(t, err)
			byID, err := reader.ReadStatus(context.Background(), clusterReader, id)
			require.NoError(t, err)
			assert.Equal(t, got.Status, byID.Status)
			assert.Equal(t, tt.wantGeneratedSet, len(byID.GeneratedResources) > 0)
		})
	}
}

func customReadinessTestObject(t *testing.T, name string, annotations map[string]string, phase string) *unstructured.Unstructured {
	t.Helper()
	u := &unstructured.Unstructured{}
	u.SetAPIVersion("v1")
	u.SetKind("ConfigMap")
	u.SetName(name)
	u.SetNamespace("default")
	u.SetAnnotations(annotations)
	require.NoError(t, unstructured.SetNestedField(u.Object, phase, "status", "phase"))
	return u
}

func customReadinessFallback() engine.StatusReader {
	return statusreaders.NewDefaultStatusReader(testutil.NewFakeRESTMapper(v1.SchemeGroupVersion.WithKind("ConfigMap")))
}

func eligibleReadinessObjects(t *testing.T, resources ...*unstructured.Unstructured) map[object.ObjMetadata]struct{} {
	t.Helper()
	eligible := make(map[object.ObjMetadata]struct{}, len(resources))
	for _, resource := range resources {
		identifier, err := object.RuntimeToObjMeta(resource)
		require.NoError(t, err)
		eligible[identifier] = struct{}{}
	}
	return eligible
}
