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
	"context"
	"testing"
	"time"

	"github.com/fluxcd/cli-utils/pkg/kstatus/polling/engine"
	"github.com/fluxcd/cli-utils/pkg/kstatus/status"
	"github.com/fluxcd/cli-utils/pkg/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/scheme"
)

func TestStatusWaitWithCustomReadinessReader(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		manifest      string
		expectErrStrs []string
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c := newTestClient(t)
			fakeClient := fake.NewSimpleDynamicClient(scheme.Scheme)
			fakeMapper := testutil.NewFakeRESTMapper(v1.SchemeGroupVersion.WithKind("ConfigMap"))
			waiter := statusWaiter{
				client:     fakeClient,
				restMapper: fakeMapper,
				readers:    []engine.StatusReader{newCustomReadinessStatusReader()},
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
		})
	}
}

func TestWithCustomReadinessStatusReader(t *testing.T) {
	opts := &waitOptions{}
	WithCustomReadinessStatusReader()(opts)
	assert.True(t, opts.enableCustomReadinessStatusReader)
}

func TestCustomReadinessStatusReaderReadStatusForObject(t *testing.T) {
	reader := newCustomReadinessStatusReader()
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
