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
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/resource"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/kubectl/pkg/scheme"
)

const rolloutManifest = `
apiVersion: argoproj.io/v1alpha1
kind: Rollout
metadata:
  name: app
  namespace: default
  generation: 1
status:
  observedGeneration: "1"
  conditions:
  - type: Promoted
    status: "True"
    lastTransitionTime: "2026-01-01T00:00:00Z"
`

var rolloutGVR = schema.GroupVersionResource{
	Group:    "argoproj.io",
	Version:  "v1alpha1",
	Resource: "rollouts",
}

var rolloutGK = schema.GroupKind{Group: "argoproj.io", Kind: "Rollout"}

// delayedMapper simulates a CRD whose GroupKind is not registered in the
// RESTMapper until a short delay has passed (e.g. the CRD is created by the
// same Helm release and takes a moment to be established). Until then
// RESTMapping returns a NoMatchError.
type delayedMapper struct {
	meta.RESTMapper
	rolloutGK schema.GroupKind
	ready     atomic.Bool
}

func (d *delayedMapper) RESTMapping(gk schema.GroupKind, versions ...string) (*meta.RESTMapping, error) {
	if gk == d.rolloutGK && !d.ready.Load() {
		return nil, &meta.NoResourceMatchError{
			PartialResource: schema.GroupVersionResource{Group: gk.Group, Resource: "rollouts"},
		}
	}
	return d.RESTMapper.RESTMapping(gk, versions...)
}

func newDelayedMapper(ready bool) *delayedMapper {
	delegate := meta.NewDefaultRESTMapper([]schema.GroupVersion{
		{Group: "argoproj.io", Version: "v1alpha1"},
	})
	delegate.Add(schema.GroupVersionKind{Group: "argoproj.io", Version: "v1alpha1", Kind: "Rollout"}, meta.RESTScopeNamespace)
	mapper := &delayedMapper{
		RESTMapper: delegate,
		rolloutGK:  rolloutGK,
	}
	mapper.ready.Store(ready)
	return mapper
}

func newRolloutStatusWaiter(t *testing.T, mapper meta.RESTMapper) (*statusWaiter, ResourceList) {
	t.Helper()
	rollout := getRuntimeObjFromManifests(t, []string{rolloutManifest})[0].(*unstructured.Unstructured)

	fakeClient := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(
		scheme.Scheme,
		map[schema.GroupVersionResource]string{rolloutGVR: "RolloutList"},
		rollout,
	)

	sw := &statusWaiter{
		client:     fakeClient,
		restMapper: mapper,
	}
	sw.SetLogger(slog.Default().Handler())

	resourceList := ResourceList{
		&resource.Info{
			Object:    rollout,
			Namespace: rollout.GetNamespace(),
			Name:      rollout.GetName(),
		},
	}
	return sw, resourceList
}

// TestStatusWaitCustomResource ensures that waiting on a custom resource (such
// as an Argo Rollout) does not hang when its CRD is not yet registered in the
// RESTMapper when the wait starts. The wait must succeed as soon as the CRD
// becomes available, instead of leaving the resource in the Unknown status
// until the timeout.
func TestStatusWaitCustomResource(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		initReady  bool
		readyAfter time.Duration
	}{
		{
			name:      "CRD is already registered",
			initReady: true,
		},
		{
			// Simulate the CRD becoming established shortly after the wait
			// starts.
			name:       "CRD is registered while waiting",
			readyAfter: 500 * time.Millisecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			mapper := newDelayedMapper(tt.initReady)
			if tt.readyAfter > 0 {
				time.AfterFunc(tt.readyAfter, func() {
					mapper.ready.Store(true)
				})
			}

			sw, resourceList := newRolloutStatusWaiter(t, mapper)

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			done := make(chan error, 1)
			go func() {
				done <- sw.Wait(resourceList, 5*time.Second)
			}()

			select {
			case err := <-done:
				if err != nil {
					t.Fatalf("Wait failed: %v", err)
				}
			case <-ctx.Done():
				t.Fatal("Wait hung: custom resource remained in the Unknown status despite its CRD becoming available")
			}
		})
	}
}

// TestStatusWaitCustomResourceUncomputableStatus ensures that waiting on a custom
// resource whose status cannot be computed by the kstatus library (such as an
// Argo Rollout, which stores status.observedGeneration as a string) does not
// hang. Helm 3 considered kinds it could not evaluate as ready, so the wait must
// succeed instead of leaving the resource in the Unknown status until the timeout.
func TestStatusWaitCustomResourceUncomputableStatus(t *testing.T) {
	t.Parallel()
	mapper := newDelayedMapper(true)
	sw, resourceList := newRolloutStatusWaiter(t, mapper)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- sw.Wait(resourceList, 5*time.Second)
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Wait failed: %v", err)
		}
	case <-ctx.Done():
		t.Fatal("Wait hung: custom resource remained in the Unknown status because its status could not be computed")
	}
}
