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
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fluxcd/cli-utils/pkg/kstatus/polling/collector"
	"github.com/fluxcd/cli-utils/pkg/kstatus/polling/engine"
	"github.com/fluxcd/cli-utils/pkg/kstatus/polling/event"
	"github.com/fluxcd/cli-utils/pkg/kstatus/status"
	"github.com/fluxcd/cli-utils/pkg/kstatus/watcher"
	"github.com/fluxcd/cli-utils/pkg/object"
	"github.com/fluxcd/cli-utils/pkg/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	clienttesting "k8s.io/client-go/testing"
	"k8s.io/kubectl/pkg/scheme"
)

var podCurrentManifest = `
apiVersion: v1
kind: Pod
metadata:
  name: current-pod
  namespace: ns
status:
  conditions:
  - type: Ready
    status: "True"
  phase: Running
`

var podNoStatusManifest = `
apiVersion: v1
kind: Pod
metadata:
  name: in-progress-pod
  namespace: ns
`

var jobNoStatusManifest = `
apiVersion: batch/v1
kind: Job
metadata:
   name: test
   namespace: qual
   generation: 1
`

var jobReadyManifest = `
apiVersion: batch/v1
kind: Job
metadata:
  name: ready-not-complete
  namespace: default
  generation: 1
status:
  startTime: 2025-02-06T16:34:20-05:00
  active: 1
  ready: 1
`

var jobCompleteManifest = `
apiVersion: batch/v1
kind: Job
metadata:
   name: test
   namespace: qual
   generation: 1
status:
   succeeded: 1
   active: 0
   conditions:
    - type: Complete
      status: "True"
`

var jobFailedManifest = `
apiVersion: batch/v1
kind: Job
metadata:
  name: failed-job
  namespace: default
  generation: 1
status:
  failed: 1
  active: 0
  conditions:
  - type: Failed
    status: "True"
    reason: BackoffLimitExceeded
    message: "Job has reached the specified backoff limit"
`

var podCompleteManifest = `
apiVersion: v1
kind: Pod
metadata:
  name: good-pod
  namespace: ns
status:
  phase: Succeeded
`

var pausedDeploymentManifest = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: paused
  namespace: ns-1
  generation: 1
spec:
  paused: true
  replicas: 1
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      labels:
        app: nginx
    spec:
      containers:
      - name: nginx
        image: nginx:1.19.6
        ports:
        - containerPort: 80
`

var notReadyDeploymentManifest = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: not-ready
  namespace: ns-1
  generation: 1
spec:
  replicas: 1
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      labels:
        app: nginx
    spec:
      containers:
      - name: nginx
        image: nginx:1.19.6
        ports:
        - containerPort: 80
`

var podNamespace1Manifest = `
apiVersion: v1
kind: Pod
metadata:
  name: pod-ns1
  namespace: namespace-1
status:
  conditions:
  - type: Ready
    status: "True"
  phase: Running
`

var podNamespace2Manifest = `
apiVersion: v1
kind: Pod
metadata:
  name: pod-ns2
  namespace: namespace-2
status:
  conditions:
  - type: Ready
    status: "True"
  phase: Running
`

var podNamespace1NoStatusManifest = `
apiVersion: v1
kind: Pod
metadata:
  name: pod-ns1
  namespace: namespace-1
`

var jobNamespace1CompleteManifest = `
apiVersion: batch/v1
kind: Job
metadata:
  name: job-ns1
  namespace: namespace-1
  generation: 1
status:
  succeeded: 1
  active: 0
  conditions:
  - type: Complete
    status: "True"
`

var podNamespace2SucceededManifest = `
apiVersion: v1
kind: Pod
metadata:
  name: pod-ns2
  namespace: namespace-2
status:
  phase: Succeeded
`

var clusterRoleManifest = `
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: test-cluster-role
rules:
- apiGroups: [""]
  resources: ["pods"]
  verbs: ["get", "list"]
`

var namespaceManifest = `
apiVersion: v1
kind: Namespace
metadata:
  name: test-namespace
`

func getGVR(t *testing.T, mapper meta.RESTMapper, obj *unstructured.Unstructured) schema.GroupVersionResource {
	t.Helper()
	gvk := obj.GroupVersionKind()
	mapping, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	require.NoError(t, err)
	return mapping.Resource
}

func getRuntimeObjFromManifests(t *testing.T, manifests []string) []runtime.Object {
	t.Helper()
	objects := []runtime.Object{}
	for _, manifest := range manifests {
		m := make(map[string]any)
		err := yaml.Unmarshal([]byte(manifest), &m)
		require.NoError(t, err)
		resource := &unstructured.Unstructured{Object: m}
		objects = append(objects, resource)
	}
	return objects
}

func getResourceListFromRuntimeObjs(t *testing.T, c *Client, objs []runtime.Object) ResourceList {
	t.Helper()
	resourceList := ResourceList{}
	for _, obj := range objs {
		list, err := c.Build(objBody(obj), false)
		require.NoError(t, err)
		resourceList = append(resourceList, list...)
	}
	return resourceList
}

func TestStatusWaitForDelete(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name              string
		manifestsToCreate []string
		manifestsToDelete []string
		expectErrs        []string
	}{
		{
			name:              "wait for pod to be deleted",
			manifestsToCreate: []string{podCurrentManifest},
			manifestsToDelete: []string{podCurrentManifest},
			expectErrs:        nil,
		},
		{
			name:              "error when not all objects are deleted",
			manifestsToCreate: []string{jobCompleteManifest, podCurrentManifest},
			manifestsToDelete: []string{jobCompleteManifest},
			expectErrs:        []string{"resource Pod/ns/current-pod still exists. status: Current", "context deadline exceeded"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c := newTestClient(t)
			timeout := time.Second
			timeUntilPodDelete := time.Millisecond * 500
			fakeClient := dynamicfake.NewSimpleDynamicClient(scheme.Scheme)
			fakeMapper := testutil.NewFakeRESTMapper(
				v1.SchemeGroupVersion.WithKind("Pod"),
				batchv1.SchemeGroupVersion.WithKind("Job"),
			)
			statusWaiter := statusWaiter{
				restMapper: fakeMapper,
				client:     fakeClient,
			}
			statusWaiter.SetLogger(slog.Default().Handler())
			objsToCreate := getRuntimeObjFromManifests(t, tt.manifestsToCreate)
			for _, objToCreate := range objsToCreate {
				u := objToCreate.(*unstructured.Unstructured)
				gvr := getGVR(t, fakeMapper, u)
				err := fakeClient.Tracker().Create(gvr, u, u.GetNamespace())
				require.NoError(t, err)
			}
			objsToDelete := getRuntimeObjFromManifests(t, tt.manifestsToDelete)
			for _, objToDelete := range objsToDelete {
				u := objToDelete.(*unstructured.Unstructured)
				gvr := getGVR(t, fakeMapper, u)
				go func(gvr schema.GroupVersionResource, u *unstructured.Unstructured) {
					time.Sleep(timeUntilPodDelete)
					err := fakeClient.Tracker().Delete(gvr, u.GetNamespace(), u.GetName())
					assert.NoError(t, err)
				}(gvr, u)
			}
			resourceList := getResourceListFromRuntimeObjs(t, c, objsToCreate)
			err := statusWaiter.WaitForDelete(resourceList, timeout)
			if tt.expectErrs != nil {
				require.Error(t, err)
				for _, expectedErrStr := range tt.expectErrs {
					require.ErrorContains(t, err, expectedErrStr)
				}
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestStatusWaitForDeleteNonExistentObject(t *testing.T) {
	t.Parallel()
	c := newTestClient(t)
	// timeout is a deadlock guard: if a never-created resource were (wrongly) waited
	// on, WaitForDelete would block until this deadline and return a deadline error,
	// which require.NoError below would catch. A correct wait returns in well under it.
	timeout := 2 * time.Second
	fakeClient := dynamicfake.NewSimpleDynamicClient(scheme.Scheme)
	fakeMapper := testutil.NewFakeRESTMapper(
		v1.SchemeGroupVersion.WithKind("Pod"),
	)
	statusWaiter := statusWaiter{
		restMapper: fakeMapper,
		client:     fakeClient,
	}
	statusWaiter.SetLogger(slog.Default().Handler())
	// Regression guard for #32214: a never-created resource must be confirmed gone by
	// the reconcile LIST and return, not hang until the timeout.
	objManifest := getRuntimeObjFromManifests(t, []string{podCurrentManifest})
	resourceList := getResourceListFromRuntimeObjs(t, c, objManifest)
	err := statusWaiter.WaitForDelete(resourceList, timeout)
	require.NoError(t, err)
}

// TestDeleteReconcilerUnknownIsNotComplete proves the anti-flake invariant on the
// delete path: while any target is still Unknown (as every target briefly is during
// informer sync), the reconciler does not report completion, so the observer will
// not cancel the watch and report a premature success. This is the delete-path
// replacement for the old observer-level check, since waitForDelete no longer routes
// through statusObserver. See https://github.com/helm/helm/issues/32261.
func TestDeleteReconcilerUnknownIsNotComplete(t *testing.T) {
	t.Parallel()
	a := object.ObjMetadata{GroupKind: schema.GroupKind{Kind: "Pod"}, Namespace: "ns", Name: "first"}
	b := object.ObjMetadata{GroupKind: schema.GroupKind{Kind: "Pod"}, Namespace: "ns", Name: "second"}
	w := &statusWaiter{}
	w.SetLogger(slog.Default().Handler())
	rec := newDeleteReconciler(w, []object.ObjMetadata{a, b}, func() {})

	// Both targets Unknown (informer still syncing): not complete.
	sc := collector.NewResourceStatusCollector(object.ObjMetadataSet{a, b})
	require.False(t, rec.observe(sc), "all-Unknown targets must not be reported complete")

	// One resolves to NotFound, the other stays Unknown: still not complete.
	sc.ResourceStatuses[a] = &event.ResourceStatus{Identifier: a, Status: status.NotFoundStatus}
	require.False(t, rec.observe(sc), "a still-Unknown target must keep the wait open")
}

func TestStatusWaitForDeleteAlreadyDeleted(t *testing.T) {
	t.Parallel()
	c := newTestClient(t)
	timeout := 2 * time.Second
	fakeClient := dynamicfake.NewSimpleDynamicClient(scheme.Scheme)
	fakeMapper := testutil.NewFakeRESTMapper(
		v1.SchemeGroupVersion.WithKind("Pod"),
	)
	statusWaiter := statusWaiter{
		restMapper: fakeMapper,
		client:     fakeClient,
	}
	statusWaiter.SetLogger(slog.Default().Handler())
	// A before-hook-creation hook is deleted by Helm and only then waited on, so it is
	// already gone here; the reconcile LIST must confirm it gone and return, not hang
	// until the timeout (#32214). timeout is only the deadlock guard: a hang would
	// surface as a deadline error caught by require.NoError.
	objs := getRuntimeObjFromManifests(t, []string{podCurrentManifest})
	for _, obj := range objs {
		u := obj.(*unstructured.Unstructured)
		gvr := getGVR(t, fakeMapper, u)
		require.NoError(t, fakeClient.Tracker().Create(gvr, u, u.GetNamespace()))
		require.NoError(t, fakeClient.Tracker().Delete(gvr, u.GetNamespace(), u.GetName()))
	}
	resourceList := getResourceListFromRuntimeObjs(t, c, objs)
	err := statusWaiter.WaitForDelete(resourceList, timeout)
	require.NoError(t, err)
}

// TestResourceGone covers the LIST-based existence check on the cases a unit test
// can decide unambiguously with the dynamic fake. The fake filters List by label
// only and ignores the field selector, so the "present" case keeps a single object
// in the namespace; the decoy case exploits that same blindness to drive the
// defensive "server did not honor the field selector" branch. A conforming API
// server's field-selector behaviour cannot be reproduced by the fake and is not
// unit-tested here; it was verified manually against a real cluster (see the PR).
func TestResourceGone(t *testing.T) {
	t.Parallel()
	podGVK := v1.SchemeGroupVersion.WithKind("Pod")
	id := object.ObjMetadata{GroupKind: podGVK.GroupKind(), Namespace: "ns", Name: "current-pod"}
	podsGVR := v1.SchemeGroupVersion.WithResource("pods")
	tests := []struct {
		name     string
		setup    func(*dynamicfake.FakeDynamicClient)
		wantGone bool
		wantErr  bool
	}{
		{
			name:     "empty list means gone",
			setup:    func(*dynamicfake.FakeDynamicClient) {},
			wantGone: true,
		},
		{
			name: "the named object present means not gone",
			setup: func(c *dynamicfake.FakeDynamicClient) {
				pod := getRuntimeObjFromManifests(t, []string{podCurrentManifest})[0].(*unstructured.Unstructured)
				require.NoError(t, c.Tracker().Create(podsGVR, pod, "ns"))
			},
			wantGone: false,
		},
		{
			// The fake ignores the field selector, so a decoy of the same GVR is
			// returned for a target that does not exist. resourceGone must detect the
			// name mismatch and surface it rather than misread the decoy as the target.
			name: "a different-named object surfaces a field-selector error",
			setup: func(c *dynamicfake.FakeDynamicClient) {
				decoy := getRuntimeObjFromManifests(t, []string{podCurrentManifest})[0].(*unstructured.Unstructured).DeepCopy()
				decoy.SetName("decoy-pod")
				require.NoError(t, c.Tracker().Create(podsGVR, decoy, "ns"))
			},
			wantGone: false,
			wantErr:  true,
		},
		{
			// A non-empty LIST error must be surfaced, not swallowed, so the caller
			// can classify it as transient (retry) or permanent (abort).
			name: "list error is surfaced",
			setup: func(c *dynamicfake.FakeDynamicClient) {
				c.PrependReactor("list", "pods", func(clienttesting.Action) (bool, runtime.Object, error) {
					return true, nil, apierrors.NewServiceUnavailable("boom")
				})
			},
			wantGone: false,
			wantErr:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fakeClient := dynamicfake.NewSimpleDynamicClient(scheme.Scheme)
			tt.setup(fakeClient)
			sw := statusWaiter{
				restMapper: testutil.NewFakeRESTMapper(podGVK),
				client:     fakeClient,
			}
			sw.SetLogger(slog.Default().Handler())
			gone, err := sw.resourceGone(context.Background(), id)
			assert.Equal(t, tt.wantGone, gone)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestRetriableReconcileError locks the transient-error allowlist: throttling,
// API/server timeouts, service unavailability and recognized transport failures are
// retried, while permanent request errors, a 500, and unrecognized errors are not.
func TestRetriableReconcileError(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil is not retriable", nil, false},
		{"too many requests", apierrors.NewTooManyRequests("slow down", 1), true},
		{"server timeout", apierrors.NewServerTimeout(schema.GroupResource{Resource: "pods"}, "list", 1), true},
		{"service unavailable", apierrors.NewServiceUnavailable("try later"), true},
		{"api timeout", &apierrors.StatusError{ErrStatus: metav1.Status{Reason: metav1.StatusReasonTimeout}}, true},
		{"http2 connection lost", errors.New("http2: client connection lost"), true},
		{"probable EOF", io.ErrUnexpectedEOF, true},
		{"net-level timeout", &net.DNSError{IsTimeout: true}, true},
		{"forbidden is permanent", apierrors.NewForbidden(schema.GroupResource{Resource: "pods"}, "p", errors.New("nope")), false},
		{"bad request is permanent", apierrors.NewBadRequest("malformed"), false},
		{"invalid is permanent", apierrors.NewInvalid(schema.GroupKind{Kind: "Pod"}, "p", nil), false},
		{"internal 500 is not retried", apierrors.NewInternalError(errors.New("boom")), false},
		{"not found is not retriable", apierrors.NewNotFound(schema.GroupResource{Resource: "pods"}, "p"), false},
		{"method not supported is permanent", apierrors.NewMethodNotSupported(schema.GroupResource{Resource: "pods"}, "list"), false},
		{"plain error is not retriable", errors.New("some other failure"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, retriableReconcileError(tt.err))
		})
	}
}

// TestServerSuggestedDelay verifies that a server-requested Retry-After is read from
// a retriable error (a 429 or ServerTimeout carrying retryAfterSeconds) and that
// errors without such a hint yield no delay.
func TestServerSuggestedDelay(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		err  error
		want time.Duration
	}{
		{"nil has no delay", nil, 0},
		{"429 with retry-after", apierrors.NewTooManyRequests("slow down", 3), 3 * time.Second},
		{"429 without retry-after", apierrors.NewTooManyRequests("slow down", 0), 0},
		{"server timeout with retry-after", apierrors.NewServerTimeout(schema.GroupResource{Resource: "pods"}, "list", 2), 2 * time.Second},
		{"service unavailable carries no hint", apierrors.NewServiceUnavailable("try later"), 0},
		{"non-status error carries no hint", errors.New("boom"), 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, serverSuggestedDelay(tt.err))
		})
	}
}

// TestReconcileWait verifies the between-rounds wait: the capped exponential backoff
// wins when no (or a smaller) server hint is present, a larger server hint is
// honored, and a pathological hint is clamped so it cannot consume the wait budget.
func TestReconcileWait(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		backoff     time.Duration
		serverDelay time.Duration
		want        time.Duration
	}{
		{"no server hint uses backoff", 40 * time.Millisecond, 0, 40 * time.Millisecond},
		{"server hint larger than backoff wins", 40 * time.Millisecond, 1 * time.Second, 1 * time.Second},
		{"backoff larger than hint wins", 500 * time.Millisecond, 100 * time.Millisecond, 500 * time.Millisecond},
		{"large server hint is clamped to the cap", 40 * time.Millisecond, 60 * time.Second, reconcileMaxServerDelay},
		{"hint exactly at the cap is honored", 40 * time.Millisecond, reconcileMaxServerDelay, reconcileMaxServerDelay},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, reconcileWait(tt.backoff, tt.serverDelay))
		})
	}
}

// deleteBeforeSyncWatcher is a fake watcher.StatusWatcher that reproduces the exact
// scenario the reconcile fixes: the target is already gone by the time the informer's
// initial LIST runs, so it is absent from that LIST. It emits a single synthetic
// SyncEvent and no ResourceUpdateEvent for the target, modelling a kstatus watcher
// whose initial LIST does not contain the object: the watcher therefore never emits
// a Deleted/NotFound status for it, and the collector keeps it Unknown. Only the
// post-sync reconcile LIST can confirm such a target gone.
//
// The fake runs in its own goroutine and never fails the test directly; it
// reports through channels observed by the main test goroutine, which provides
// positive proof that the intended interleaving actually occurred (so a green
// result cannot be a false positive from a mis-wired harness).
type deleteBeforeSyncWatcher struct {
	// del removes the target from the cluster. It is called at the start of the Watch
	// goroutine (as the informer would be running its initial LIST) and strictly
	// before the SyncEvent, so the target is already absent when sync is observed.
	del func() error

	watchInvoked   chan struct{} // closed when Watch is called
	deleteErr      chan error    // receives the result of del()
	syncReceived   chan struct{} // closed once the unbuffered SyncEvent send returns (collector consumed it)
	updatesEmitted atomic.Int64  // number of ResourceUpdateEvents fed to the collector; must stay 0
}

var _ watcher.StatusWatcher = (*deleteBeforeSyncWatcher)(nil)

func newDeleteBeforeSyncWatcher(del func() error) *deleteBeforeSyncWatcher {
	return &deleteBeforeSyncWatcher{
		del:          del,
		watchInvoked: make(chan struct{}),
		deleteErr:    make(chan error, 1),
		syncReceived: make(chan struct{}),
	}
}

func (w *deleteBeforeSyncWatcher) Watch(ctx context.Context, _ object.ObjMetadataSet, _ watcher.Options) <-chan event.Event {
	close(w.watchInvoked)
	eventCh := make(chan event.Event)
	go func() {
		defer close(eventCh)
		// Ordering is explicit, not timing-based: delete strictly before SyncEvent.
		w.deleteErr <- w.del()
		// Emit only SyncEvent -- never a ResourceUpdateEvent, so a post-fix green
		// result can only come from production reconciliation, not from a status
		// supplied here. The send is unbuffered: it returns once the collector has
		// consumed the SyncEvent, i.e. sync is observed with the target Unknown.
		if !w.emit(ctx, eventCh, event.Event{Type: event.SyncEvent}) {
			return
		}
		close(w.syncReceived)
		// A real informer keeps watching but delivers nothing further for a target
		// absent from the initial LIST. Stay alive until the context is cancelled.
		<-ctx.Done()
	}()
	return eventCh
}

func (w *deleteBeforeSyncWatcher) emit(ctx context.Context, eventCh chan event.Event, ev event.Event) bool {
	if ev.Type == event.ResourceUpdateEvent {
		w.updatesEmitted.Add(1)
	}
	select {
	case eventCh <- ev:
		return true
	case <-ctx.Done():
		return false
	}
}

// TestStatusWaitForDeleteRaceDeletedBeforeInitialList proves the reconcile handles a
// target absent from the watcher's initial LIST: an object gone before the informer
// syncs is never reported NotFound (no delete event is generated for an object absent
// from the initial cache), so it stays Unknown for the life of the watch. Because
// waitForDelete no longer treats Unknown as deleted, only the post-sync reconcile LIST
// can confirm it gone; without that, the wait would run to the deadline.
//
// The interleaving enforced here is exactly:
//
//	target deleted  ->  informer initial LIST misses it  ->  target stays Unknown
//	                    (SyncEvent, no ResourceUpdateEvent)
//
// The only result assertion is the desired external behavior (no hang). The channel
// checks are positive proof that the interleaving occurred; they do not encode the
// result, so the test is a genuine reproduction of the reconcile's job.
func TestStatusWaitForDeleteRaceDeletedBeforeInitialList(t *testing.T) {
	t.Parallel()
	c := newTestClient(t)
	// Deadlock guard only. It does not create the ordering (enforced by control
	// flow and the fake's explicit event sequence); it just bounds the hang so the
	// test fails fast instead of blocking forever.
	timeout := 2 * time.Second

	fakeClient := dynamicfake.NewSimpleDynamicClient(scheme.Scheme)
	fakeMapper := testutil.NewFakeRESTMapper(v1.SchemeGroupVersion.WithKind("Pod"))

	// The target exists when the wait begins; the fake deletes it before the SyncEvent.
	u := getRuntimeObjFromManifests(t, []string{podCurrentManifest})[0].(*unstructured.Unstructured)
	gvr := getGVR(t, fakeMapper, u)
	require.NoError(t, fakeClient.Tracker().Create(gvr, u, u.GetNamespace()))

	statusWaiter := statusWaiter{
		restMapper: fakeMapper,
		client:     fakeClient,
	}
	statusWaiter.SetLogger(slog.Default().Handler())

	sw := newDeleteBeforeSyncWatcher(func() error { return fakeClient.Tracker().Delete(gvr, u.GetNamespace(), u.GetName()) })

	resourceList := getResourceListFromRuntimeObjs(t, c, []runtime.Object{u})

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Run waitForDelete concurrently so the main goroutine can observe the
	// interleaving as it happens. The fake is injected directly; the public
	// WaitForDelete runs this same path after building a real watcher.
	resultCh := make(chan error, 1)
	go func() {
		resultCh <- statusWaiter.waitForDelete(ctx, resourceList, sw)
	}()

	// Positive proof the intended interleaving occurred (all checked in the main
	// goroutine; the fake reports through channels and never fails the test itself).
	<-sw.watchInvoked
	require.NoError(t, <-sw.deleteErr, "fake failed to delete the target before SyncEvent")
	<-sw.syncReceived

	err := <-resultCh
	require.Zero(t, sw.updatesEmitted.Load(), "fake must not emit any ResourceUpdateEvent for the target")

	// The single result assertion: the desired external behavior. Without the post-sync
	// reconcile this would hang to the deadline; with it, the still-Unknown target is
	// confirmed gone and the wait returns.
	require.NoError(t, err,
		"a target absent from the watcher's initial LIST must be confirmed gone by the reconcile, not hang WaitForDelete")
}

// TestDeleteReconcilerConfirmedGoneWinsOverLateStatus is the direct state-machine
// proof for the stale-event concern: once a live LIST has confirmed a target gone
// (markGone), a later, stale watcher status for that same target -- e.g. a delayed
// initial Current from the informer -- must not resurrect it. confirmedGone is
// terminal and takes precedence over the collector's status in both completion and
// the assembled result.
//
// This is tested at the reconciler directly rather than through a fake watcher: with
// a single target the wait cancels the instant markGone completes, so a fake could
// not reliably deliver a post-confirmation status through the collector at all. Here
// a second, unresolved target keeps the state machine live while the stale Current
// for the first target is applied, and the invariant is asserted deterministically.
func TestDeleteReconcilerConfirmedGoneWinsOverLateStatus(t *testing.T) {
	t.Parallel()
	a := object.ObjMetadata{GroupKind: schema.GroupKind{Kind: "Pod"}, Namespace: "ns", Name: "a"}
	b := object.ObjMetadata{GroupKind: schema.GroupKind{Kind: "Pod"}, Namespace: "ns", Name: "b"}
	w := &statusWaiter{}
	w.SetLogger(slog.Default().Handler())
	var cancelled atomic.Bool
	rec := newDeleteReconciler(w, []object.ObjMetadata{a, b}, func() { cancelled.Store(true) })

	// A live LIST confirms A gone; B is not yet resolved, so the wait stays open.
	rec.markGone(a)

	sc := collector.NewResourceStatusCollector(object.ObjMetadataSet{a, b})
	// A stale, late Current for A arrives after it was confirmed gone; B still Unknown.
	sc.ResourceStatuses[a] = &event.ResourceStatus{Identifier: a, Status: status.CurrentStatus}
	require.False(t, rec.observe(sc), "A is gone but B is unresolved, so the wait must stay open")

	// B is then observed deleted. With A confirmed gone (despite its stale Current)
	// and B NotFound, every target is terminal.
	sc.ResourceStatuses[b] = &event.ResourceStatus{Identifier: b, Status: status.NotFoundStatus}
	require.True(t, rec.observe(sc), "confirmedGone A + NotFound B must complete despite the stale Current for A")

	// The assembled result is success: A is skipped via confirmedGone (not reported as
	// still-existing from its stale Current), B was observed NotFound.
	require.NoError(t, rec.result(context.Background(), sc),
		"a live-confirmed deletion must not be overridden by a delayed Current event")
}

// TestDeleteReconcilerResultSuccessBeatsExpiredContext proves that a fully confirmed
// deletion is reported as success even if the parent context expired at the moment
// the last confirmation landed: a completed deletion must not be turned into a
// context-error failure.
func TestDeleteReconcilerResultSuccessBeatsExpiredContext(t *testing.T) {
	t.Parallel()
	a := object.ObjMetadata{GroupKind: schema.GroupKind{Kind: "Pod"}, Namespace: "ns", Name: "a"}
	b := object.ObjMetadata{GroupKind: schema.GroupKind{Kind: "Pod"}, Namespace: "ns", Name: "b"}
	w := &statusWaiter{}
	w.SetLogger(slog.Default().Handler())
	rec := newDeleteReconciler(w, []object.ObjMetadata{a, b}, func() {})

	// A confirmed gone by a live LIST; B observed NotFound by the watcher.
	rec.markGone(a)
	sc := collector.NewResourceStatusCollector(object.ObjMetadataSet{a, b})
	sc.ResourceStatuses[b] = &event.ResourceStatus{Identifier: b, Status: status.NotFoundStatus}

	// The parent context has already expired when result runs.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	require.NoError(t, rec.result(ctx, sc),
		"a deletion fully confirmed must report success even if the context expired as the last confirmation landed")
}

// TestStatusWaitForDeleteReconcileRetriesTransientError: the first reconciliation
// LIST fails with a retriable error and a later retry returns an empty list (gone).
// WaitForDelete must retry (bounded by context) and succeed rather than treat the
// transient error as fatal and hang.
func TestStatusWaitForDeleteReconcileRetriesTransientError(t *testing.T) {
	t.Parallel()
	c := newTestClient(t)
	timeout := 2 * time.Second
	fakeClient := dynamicfake.NewSimpleDynamicClient(scheme.Scheme)
	fakeMapper := testutil.NewFakeRESTMapper(v1.SchemeGroupVersion.WithKind("Pod"))
	u := getRuntimeObjFromManifests(t, []string{podCurrentManifest})[0].(*unstructured.Unstructured)
	gvr := getGVR(t, fakeMapper, u)
	require.NoError(t, fakeClient.Tracker().Create(gvr, u, u.GetNamespace()))

	// The first reconciliation LIST fails with a retriable (allowlisted) error that
	// carries no Retry-After hint, so it is retried on the fast backoff; later LISTs
	// fall through to the tracker, which returns an empty list once the target has
	// been deleted. (Retry-After honoring is covered deterministically by
	// TestServerSuggestedDelay, not by wall-clock timing here.)
	var lists atomic.Int64
	fakeClient.PrependReactor("list", "pods", func(clienttesting.Action) (bool, runtime.Object, error) {
		if lists.Add(1) == 1 {
			return true, nil, apierrors.NewServiceUnavailable("slow down")
		}
		return false, nil, nil
	})

	statusWaiter := statusWaiter{restMapper: fakeMapper, client: fakeClient}
	statusWaiter.SetLogger(slog.Default().Handler())

	sw := newDeleteBeforeSyncWatcher(func() error { return fakeClient.Tracker().Delete(gvr, u.GetNamespace(), u.GetName()) })

	resourceList := getResourceListFromRuntimeObjs(t, c, []runtime.Object{u})
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	resultCh := make(chan error, 1)
	go func() { resultCh <- statusWaiter.waitForDelete(ctx, resourceList, sw) }()
	<-sw.watchInvoked
	require.NoError(t, <-sw.deleteErr, "fake failed to delete the target before SyncEvent")
	<-sw.syncReceived

	err := <-resultCh
	require.NoError(t, err,
		"a transient reconciliation error must be retried until the target is confirmed gone, not hang WaitForDelete")
}

// closeAfterSyncWatcher emits a single SyncEvent and then closes its event
// channel immediately, modelling a watch that ends (stream closed / restart)
// before any target's deletion has been observed. It uses a buffered channel and
// no goroutine, so it cannot itself leak.
type closeAfterSyncWatcher struct {
	watchInvoked chan struct{}
}

var _ watcher.StatusWatcher = (*closeAfterSyncWatcher)(nil)

func (w *closeAfterSyncWatcher) Watch(_ context.Context, _ object.ObjMetadataSet, _ watcher.Options) <-chan event.Event {
	close(w.watchInvoked)
	ch := make(chan event.Event, 1)
	ch <- event.Event{Type: event.SyncEvent}
	close(ch)
	return ch
}

// TestStatusWaitForDeleteWatcherClosesWithUnknownTarget: the watch closes right after
// sync while the target still exists, so its deletion is never observed and never
// confirmed by a live check. The defined result is a non-success (deletion could not
// be confirmed), returned promptly rather than as a timeout, and with no leaked
// goroutine -- the wait must not silently report success for an unconfirmed target.
func TestStatusWaitForDeleteWatcherClosesWithUnknownTarget(t *testing.T) {
	t.Parallel()
	c := newTestClient(t)
	timeout := 2 * time.Second // deadlock guard only
	fakeClient := dynamicfake.NewSimpleDynamicClient(scheme.Scheme)
	fakeMapper := testutil.NewFakeRESTMapper(v1.SchemeGroupVersion.WithKind("Pod"))

	// The target genuinely still exists: a live existence check finds it present,
	// so it cannot be marked confirmedGone and remains Unknown when the watch ends.
	u := getRuntimeObjFromManifests(t, []string{podCurrentManifest})[0].(*unstructured.Unstructured)
	gvr := getGVR(t, fakeMapper, u)
	require.NoError(t, fakeClient.Tracker().Create(gvr, u, u.GetNamespace()))

	statusWaiter := statusWaiter{restMapper: fakeMapper, client: fakeClient}
	statusWaiter.SetLogger(slog.Default().Handler())

	sw := &closeAfterSyncWatcher{watchInvoked: make(chan struct{})}
	resourceList := getResourceListFromRuntimeObjs(t, c, []runtime.Object{u})
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	resultCh := make(chan error, 1)
	go func() { resultCh <- statusWaiter.waitForDelete(ctx, resourceList, sw) }()
	<-sw.watchInvoked

	var err error
	select {
	case err = <-resultCh:
	case <-time.After(timeout):
		t.Fatal("waitForDelete did not return after the watch closed (hang / goroutine leak)")
	}
	require.Error(t, err,
		"when the watch closes with a target still unconfirmed, WaitForDelete must not report success")
	require.ErrorIs(t, err, errWatchEndedBeforeConfirmation,
		"the failure must be the explicit watch-ended-before-confirmation error")
	require.NotErrorIs(t, err, context.DeadlineExceeded,
		"the watch closed while the parent context was active, so this must not be a timeout")
	require.NotErrorIs(t, err, context.Canceled,
		"the watch closed while the parent context was active, so this must not be a cancellation")
}

// syncThenBlockWatcher emits a SyncEvent and then blocks until its context is
// cancelled. It records watchDone so a test can prove the watcher goroutine
// terminated (no leak).
type syncThenBlockWatcher struct {
	watchInvoked chan struct{}
	syncReceived chan struct{}
	watchDone    chan struct{}
}

var _ watcher.StatusWatcher = (*syncThenBlockWatcher)(nil)

func (w *syncThenBlockWatcher) Watch(ctx context.Context, _ object.ObjMetadataSet, _ watcher.Options) <-chan event.Event {
	close(w.watchInvoked)
	ch := make(chan event.Event)
	go func() {
		defer close(w.watchDone)
		defer close(ch)
		select {
		case ch <- event.Event{Type: event.SyncEvent}:
		case <-ctx.Done():
			return
		}
		close(w.syncReceived)
		<-ctx.Done()
	}()
	return ch
}

// blockingListClient wraps a dynamic.Interface so every List blocks until the
// call's context is cancelled and then returns ctx.Err(), modelling an in-flight
// API request interrupted by cancellation (a real client-go REST client cancels
// the underlying request when its context ends). It closes entered exactly once,
// when the first List begins to block.
type blockingListClient struct {
	dynamic.Interface
	entered chan struct{}
	once    sync.Once
}

func (c *blockingListClient) Resource(gvr schema.GroupVersionResource) dynamic.NamespaceableResourceInterface {
	return &blockingListNamespaceable{c.Interface.Resource(gvr), c}
}

type blockingListNamespaceable struct {
	dynamic.NamespaceableResourceInterface
	c *blockingListClient
}

func (n *blockingListNamespaceable) Namespace(ns string) dynamic.ResourceInterface {
	return &blockingListResource{n.NamespaceableResourceInterface.Namespace(ns), n.c}
}

func (n *blockingListNamespaceable) List(ctx context.Context, _ metav1.ListOptions) (*unstructured.UnstructuredList, error) {
	return n.c.block(ctx)
}

type blockingListResource struct {
	dynamic.ResourceInterface
	c *blockingListClient
}

func (r *blockingListResource) List(ctx context.Context, _ metav1.ListOptions) (*unstructured.UnstructuredList, error) {
	return r.c.block(ctx)
}

func (c *blockingListClient) block(ctx context.Context) (*unstructured.UnstructuredList, error) {
	c.once.Do(func() { close(c.entered) })
	<-ctx.Done()
	return nil, ctx.Err()
}

// TestStatusWaitForDeleteContextCancelledDuringReconcile: the context is cancelled
// while a reconciliation LIST is blocked in-flight. Both the waiter and its reconcile
// goroutine must unwind deterministically -- no hang, no leak -- and the wait must
// return an error carrying context.Canceled.
func TestStatusWaitForDeleteContextCancelledDuringReconcile(t *testing.T) {
	t.Parallel()
	c := newTestClient(t)
	timeout := 5 * time.Second // deadlock guard only
	blocking := &blockingListClient{
		Interface: dynamicfake.NewSimpleDynamicClient(scheme.Scheme),
		entered:   make(chan struct{}),
	}
	fakeMapper := testutil.NewFakeRESTMapper(v1.SchemeGroupVersion.WithKind("Pod"))
	u := getRuntimeObjFromManifests(t, []string{podCurrentManifest})[0].(*unstructured.Unstructured)

	statusWaiter := statusWaiter{restMapper: fakeMapper, client: blocking}
	statusWaiter.SetLogger(slog.Default().Handler())

	sw := &syncThenBlockWatcher{
		watchInvoked: make(chan struct{}),
		syncReceived: make(chan struct{}),
		watchDone:    make(chan struct{}),
	}
	resourceList := getResourceListFromRuntimeObjs(t, c, []runtime.Object{u})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	waitClosed := func(ch <-chan struct{}, msg string) {
		t.Helper()
		select {
		case <-ch:
		case <-time.After(timeout):
			t.Fatal(msg)
		}
	}

	resultCh := make(chan error, 1)
	go func() { resultCh <- statusWaiter.waitForDelete(ctx, resourceList, sw) }()

	waitClosed(sw.watchInvoked, "watcher was never invoked")
	waitClosed(sw.syncReceived, "SyncEvent was never observed")
	waitClosed(blocking.entered, "the reconciliation LIST never started (nothing to interrupt)")

	// Cancel while the LIST is blocked in-flight.
	cancel()

	var err error
	select {
	case err = <-resultCh:
	case <-time.After(timeout):
		t.Fatal("waitForDelete did not return after cancellation during a blocked reconcile LIST")
	}
	waitClosed(sw.watchDone, "watcher goroutine did not terminate after cancellation")
	require.Error(t, err, "a cancelled delete wait must return an error, not success")
	require.ErrorIs(t, err, context.Canceled,
		"caller cancellation must be preserved as context.Canceled")
	require.NotErrorIs(t, err, errWatchEndedBeforeConfirmation,
		"a context error must take precedence over and not be joined with the watch-ended error")
}

// TestStatusWaitForDeleteReconcileOutageLongerThanRetryWindow is the extended
// outage test: the reconciliation existence check fails for many consecutive
// attempts -- deliberately more than any small fixed retry cap -- before the API
// recovers and reports the target gone. A robust waiter must ride the outage out
// (within its timeout) rather than give up after a fixed number of attempts and
// hang. The outage length is expressed as a failure count that is not tied to the
// implementation's backoff constants; the test asserts only eventual success.
func TestStatusWaitForDeleteReconcileOutageLongerThanRetryWindow(t *testing.T) {
	t.Parallel()
	c := newTestClient(t)
	timeout := 5 * time.Second // deadlock guard only
	fakeClient := dynamicfake.NewSimpleDynamicClient(scheme.Scheme)
	fakeMapper := testutil.NewFakeRESTMapper(v1.SchemeGroupVersion.WithKind("Pod"))
	u := getRuntimeObjFromManifests(t, []string{podCurrentManifest})[0].(*unstructured.Unstructured)
	gvr := getGVR(t, fakeMapper, u)
	require.NoError(t, fakeClient.Tracker().Create(gvr, u, u.GetNamespace()))

	const outageLists = 6 // > any small fixed retry cap, without referencing its value
	var lists atomic.Int64
	fakeClient.PrependReactor("list", "pods", func(clienttesting.Action) (bool, runtime.Object, error) {
		if lists.Add(1) <= outageLists {
			// retryAfterSeconds 0: exercise the retry loop on the fast backoff without
			// a server-suggested delay stretching the outage past the deadlock guard.
			return true, nil, apierrors.NewServerTimeout(schema.GroupResource{Resource: "pods"}, "list", 0)
		}
		return false, nil, nil // pass through: tracker returns an empty list for the deleted target
	})

	statusWaiter := statusWaiter{restMapper: fakeMapper, client: fakeClient}
	statusWaiter.SetLogger(slog.Default().Handler())

	sw := newDeleteBeforeSyncWatcher(func() error { return fakeClient.Tracker().Delete(gvr, u.GetNamespace(), u.GetName()) })
	resourceList := getResourceListFromRuntimeObjs(t, c, []runtime.Object{u})
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	resultCh := make(chan error, 1)
	go func() { resultCh <- statusWaiter.waitForDelete(ctx, resourceList, sw) }()
	<-sw.watchInvoked
	require.NoError(t, <-sw.deleteErr, "fake failed to delete the target before SyncEvent")
	<-sw.syncReceived

	err := <-resultCh
	require.NoError(t, err,
		"an API outage longer than a fixed retry window must be ridden out until the target is confirmed gone")
}

// TestStatusWaitForDeleteParentDeadlinePropagates is a context-semantic test: when
// the parent context's deadline expires while a target is still present, the
// returned error must carry context.DeadlineExceeded -- the unconfirmed-deletion
// error must not replace it.
func TestStatusWaitForDeleteParentDeadlinePropagates(t *testing.T) {
	t.Parallel()
	c := newTestClient(t)
	timeout := 300 * time.Millisecond
	fakeClient := dynamicfake.NewSimpleDynamicClient(scheme.Scheme)
	fakeMapper := testutil.NewFakeRESTMapper(v1.SchemeGroupVersion.WithKind("Pod"))
	// The target stays present, so no live check can confirm its deletion and the
	// wait can only end at the parent deadline.
	u := getRuntimeObjFromManifests(t, []string{podCurrentManifest})[0].(*unstructured.Unstructured)
	gvr := getGVR(t, fakeMapper, u)
	require.NoError(t, fakeClient.Tracker().Create(gvr, u, u.GetNamespace()))

	statusWaiter := statusWaiter{restMapper: fakeMapper, client: fakeClient}
	statusWaiter.SetLogger(slog.Default().Handler())

	sw := &syncThenBlockWatcher{
		watchInvoked: make(chan struct{}),
		syncReceived: make(chan struct{}),
		watchDone:    make(chan struct{}),
	}
	resourceList := getResourceListFromRuntimeObjs(t, c, []runtime.Object{u})
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	err := statusWaiter.waitForDelete(ctx, resourceList, sw)
	require.Error(t, err)
	require.ErrorIs(t, err, context.DeadlineExceeded,
		"a parent deadline must be preserved as context.DeadlineExceeded")
	require.NotErrorIs(t, err, errWatchEndedBeforeConfirmation,
		"a context error must take precedence over and not be joined with the watch-ended error")
	<-sw.watchDone // the watcher goroutine terminated (no leak)
}

// TestStatusWaitForDeleteReconcileDoesNotStarveOtherTargets proves round-based
// reconciliation across two different GVRs: target A's existence LIST always fails
// with a retriable error, target B is absent and its LIST succeeds. B must be
// checked and confirmed gone despite A continuing to fail -- a per-target
// retry-until-context loop would let A (listed first) starve B forever. The overall
// wait still fails because A is never confirmed. Ordering is enforced by channels,
// not timing.
func TestStatusWaitForDeleteReconcileDoesNotStarveOtherTargets(t *testing.T) {
	t.Parallel()
	c := newTestClient(t)
	timeout := 400 * time.Millisecond // deadlock guard; also bounds A's retry rounds
	fakeClient := dynamicfake.NewSimpleDynamicClient(scheme.Scheme)
	fakeMapper := testutil.NewFakeRESTMapper(
		v1.SchemeGroupVersion.WithKind("Pod"),
		v1.SchemeGroupVersion.WithKind("ConfigMap"),
	)

	// Target A (a pod) always fails its existence LIST with a retriable error.
	podA := getRuntimeObjFromManifests(t, []string{podCurrentManifest})[0].(*unstructured.Unstructured)
	// Target B (a configmap) is absent, so its LIST returns an empty list -> gone.
	cmB := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1", "kind": "ConfigMap",
		"metadata": map[string]any{"name": "b", "namespace": "ns"},
	}}

	var aLists, bLists atomic.Int64
	var bOnce sync.Once
	bQueried := make(chan struct{})
	fakeClient.PrependReactor("list", "pods", func(clienttesting.Action) (bool, runtime.Object, error) {
		aLists.Add(1)
		return true, nil, apierrors.NewServiceUnavailable("target A is unavailable")
	})
	fakeClient.PrependReactor("list", "configmaps", func(clienttesting.Action) (bool, runtime.Object, error) {
		bLists.Add(1)
		bOnce.Do(func() { close(bQueried) })
		return false, nil, nil // pass through: B is absent -> empty list -> gone
	})

	statusWaiter := statusWaiter{restMapper: fakeMapper, client: fakeClient}
	statusWaiter.SetLogger(slog.Default().Handler())

	sw := &syncThenBlockWatcher{
		watchInvoked: make(chan struct{}),
		syncReceived: make(chan struct{}),
		watchDone:    make(chan struct{}),
	}
	// A is listed before B, so a sequential reconcile that retries A until the
	// context ends would never reach B.
	resourceList := getResourceListFromRuntimeObjs(t, c, []runtime.Object{podA, cmB})
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	resultCh := make(chan error, 1)
	go func() { resultCh <- statusWaiter.waitForDelete(ctx, resourceList, sw) }()

	<-sw.watchInvoked
	<-sw.syncReceived
	select {
	case <-bQueried:
	case <-time.After(timeout):
		t.Fatal("target B was never queried: a failing target starved the reconciliation")
	}

	err := <-resultCh
	require.Error(t, err, "the overall wait must still fail because A was never confirmed gone")
	assert.Equal(t, int64(1), bLists.Load(),
		"B must be checked exactly once (confirmed gone in round one, then not re-polled)")
	assert.GreaterOrEqual(t, aLists.Load(), int64(2),
		"A must be retried across rounds, proving it did not monopolize the reconcile goroutine")
}

// syncThenNotFoundWatcher emits a SyncEvent, then -- once the test closes release --
// emits a NotFound status for a single target, modelling the watcher observing that
// target's deletion. It stays alive until its context is cancelled.
type syncThenNotFoundWatcher struct {
	id           object.ObjMetadata
	release      chan struct{}
	watchInvoked chan struct{}
	syncReceived chan struct{}
	watchDone    chan struct{}
}

var _ watcher.StatusWatcher = (*syncThenNotFoundWatcher)(nil)

func (w *syncThenNotFoundWatcher) Watch(ctx context.Context, _ object.ObjMetadataSet, _ watcher.Options) <-chan event.Event {
	close(w.watchInvoked)
	ch := make(chan event.Event)
	go func() {
		defer close(w.watchDone)
		defer close(ch)
		select {
		case ch <- event.Event{Type: event.SyncEvent}:
		case <-ctx.Done():
			return
		}
		close(w.syncReceived)
		select {
		case <-w.release:
		case <-ctx.Done():
			return
		}
		select {
		case ch <- event.Event{Type: event.ResourceUpdateEvent, Resource: &event.ResourceStatus{Identifier: w.id, Status: status.NotFoundStatus}}:
		case <-ctx.Done():
			return
		}
		<-ctx.Done()
	}()
	return ch
}

// blockOneGVRClient wraps a dynamic.Interface so that List on exactly one GVR blocks
// until the call's context is cancelled (signalling entry once via entered), while
// List on every other GVR passes through to the wrapped client. It models one target
// whose existence LIST is slow/stuck while another target's LIST returns promptly.
type blockOneGVRClient struct {
	dynamic.Interface
	blockGVR schema.GroupVersionResource
	entered  chan struct{}
	once     sync.Once
}

func (c *blockOneGVRClient) Resource(gvr schema.GroupVersionResource) dynamic.NamespaceableResourceInterface {
	inner := c.Interface.Resource(gvr)
	if gvr == c.blockGVR {
		return &blockOneGVRNamespaceable{inner, c}
	}
	return inner
}

type blockOneGVRNamespaceable struct {
	dynamic.NamespaceableResourceInterface
	c *blockOneGVRClient
}

func (n *blockOneGVRNamespaceable) Namespace(ns string) dynamic.ResourceInterface {
	return &blockOneGVRResource{n.NamespaceableResourceInterface.Namespace(ns), n.c}
}

func (n *blockOneGVRNamespaceable) List(ctx context.Context, _ metav1.ListOptions) (*unstructured.UnstructuredList, error) {
	return n.c.block(ctx)
}

type blockOneGVRResource struct {
	dynamic.ResourceInterface
	c *blockOneGVRClient
}

func (r *blockOneGVRResource) List(ctx context.Context, _ metav1.ListOptions) (*unstructured.UnstructuredList, error) {
	return r.c.block(ctx)
}

func (c *blockOneGVRClient) block(ctx context.Context) (*unstructured.UnstructuredList, error) {
	c.once.Do(func() { close(c.entered) })
	<-ctx.Done()
	return nil, ctx.Err()
}

// TestStatusWaitForDeleteInflightListDoesNotBlockOtherTargets is a failure-isolation
// test: one target's existence LIST is stuck in flight while a second target is
// already absent. The second target must be confirmed gone concurrently -- a single
// in-flight LIST must not block every later target -- and once the watcher reports the
// stuck target deleted, the wait must succeed without hitting the deadline.
//
// RED against a sequential reconcile: the one goroutine blocks inside target A's LIST
// and never reaches target B, even after the watcher makes A terminal.
func TestStatusWaitForDeleteInflightListDoesNotBlockOtherTargets(t *testing.T) {
	t.Parallel()
	c := newTestClient(t)
	timeout := 3 * time.Second   // deadlock guard only
	hardStop := 30 * time.Second // harness safety net, well above the guard
	base := dynamicfake.NewSimpleDynamicClient(scheme.Scheme)
	fakeMapper := testutil.NewFakeRESTMapper(
		v1.SchemeGroupVersion.WithKind("Pod"),
		v1.SchemeGroupVersion.WithKind("ConfigMap"),
	)
	podsGVR := v1.SchemeGroupVersion.WithResource("pods")

	// A = pod: its existence LIST blocks until cancelled. B = configmap: absent, so its
	// LIST returns empty (gone) immediately.
	podA := getRuntimeObjFromManifests(t, []string{podCurrentManifest})[0].(*unstructured.Unstructured)
	cmB := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1", "kind": "ConfigMap",
		"metadata": map[string]any{"name": "b", "namespace": "ns"},
	}}
	idA, err := object.RuntimeToObjMeta(podA)
	require.NoError(t, err)

	blocking := &blockOneGVRClient{Interface: base, blockGVR: podsGVR, entered: make(chan struct{})}
	sw := &syncThenNotFoundWatcher{
		id:           idA,
		release:      make(chan struct{}),
		watchInvoked: make(chan struct{}),
		syncReceived: make(chan struct{}),
		watchDone:    make(chan struct{}),
	}

	statusWaiter := statusWaiter{restMapper: fakeMapper, client: blocking}
	statusWaiter.SetLogger(slog.Default().Handler())

	// A before B, so a sequential reconcile checks (and blocks on) A first.
	resourceList := getResourceListFromRuntimeObjs(t, c, []runtime.Object{podA, cmB})
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	resultCh := make(chan error, 1)
	go func() { resultCh <- statusWaiter.waitForDelete(ctx, resourceList, sw) }()

	<-sw.watchInvoked
	<-sw.syncReceived
	select {
	case <-blocking.entered:
	case <-time.After(hardStop):
		t.Fatal("target A's existence LIST never entered")
	}
	// A's LIST is stuck. Make A terminal via the watcher; B must still be confirmed.
	close(sw.release)

	var werr error
	select {
	case werr = <-resultCh:
	case <-time.After(hardStop):
		t.Fatal("WaitForDelete hung: a stuck in-flight LIST blocked confirmation of other targets")
	}
	<-sw.watchDone
	require.NoError(t, werr,
		"B must be confirmed gone while A's LIST is stuck, and the wait must succeed without waiting for the deadline")
}

// TestStatusWaitForDeletePerTargetRetryAfterIsolation is a failure-isolation test for
// retry timing: one target returns 429 with a long Retry-After (and is then resolved
// by the watcher), while a second target returns a transient transport error once and
// would succeed on its own short-backoff retry. The long Retry-After of the first
// target must not delay the second target's retry.
//
// The deadlock guard is deliberately below the (capped) server delay a whole-round
// coupled reconcile would impose and far above the few milliseconds an isolated
// reconcile needs, so a coupled implementation fails to confirm B within the guard.
//
// RED against a reconcile that applies one server delay to the whole round.
func TestStatusWaitForDeletePerTargetRetryAfterIsolation(t *testing.T) {
	t.Parallel()
	c := newTestClient(t)
	timeout := 3 * time.Second // deadlock guard; see the doc comment above for why this value
	hardStop := 30 * time.Second
	base := dynamicfake.NewSimpleDynamicClient(scheme.Scheme)
	fakeMapper := testutil.NewFakeRESTMapper(
		v1.SchemeGroupVersion.WithKind("Pod"),
		v1.SchemeGroupVersion.WithKind("ConfigMap"),
	)

	podA := getRuntimeObjFromManifests(t, []string{podCurrentManifest})[0].(*unstructured.Unstructured)
	cmB := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1", "kind": "ConfigMap",
		"metadata": map[string]any{"name": "b", "namespace": "ns"},
	}}
	idA, err := object.RuntimeToObjMeta(podA)
	require.NoError(t, err)

	// A's LIST always fails with 429 carrying a long Retry-After; A is resolved by the
	// watcher, not by its LIST.
	var aOnce sync.Once
	aQueried := make(chan struct{})
	base.PrependReactor("list", "pods", func(clienttesting.Action) (bool, runtime.Object, error) {
		aOnce.Do(func() { close(aQueried) })
		return true, nil, apierrors.NewTooManyRequests("A is throttled", 100)
	})
	// B's LIST fails once with a transient transport error, then passes through (absent
	// -> empty -> gone) on its next retry.
	var bCalls atomic.Int64
	base.PrependReactor("list", "configmaps", func(clienttesting.Action) (bool, runtime.Object, error) {
		if bCalls.Add(1) == 1 {
			return true, nil, io.ErrUnexpectedEOF
		}
		return false, nil, nil
	})

	sw := &syncThenNotFoundWatcher{
		id:           idA,
		release:      make(chan struct{}),
		watchInvoked: make(chan struct{}),
		syncReceived: make(chan struct{}),
		watchDone:    make(chan struct{}),
	}

	statusWaiter := statusWaiter{restMapper: fakeMapper, client: base}
	statusWaiter.SetLogger(slog.Default().Handler())

	resourceList := getResourceListFromRuntimeObjs(t, c, []runtime.Object{podA, cmB})
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	resultCh := make(chan error, 1)
	go func() { resultCh <- statusWaiter.waitForDelete(ctx, resourceList, sw) }()

	<-sw.watchInvoked
	<-sw.syncReceived
	select {
	case <-aQueried:
	case <-time.After(hardStop):
		t.Fatal("target A was never queried")
	}
	// A has been throttled with a long Retry-After. Make A terminal via the watcher.
	close(sw.release)

	var werr error
	select {
	case werr = <-resultCh:
	case <-time.After(hardStop):
		t.Fatal("WaitForDelete hung")
	}
	<-sw.watchDone
	require.NoError(t, werr,
		"B must be retried on its own short backoff and confirmed gone; A's long Retry-After must not delay it")
	require.GreaterOrEqual(t, bCalls.Load(), int64(2),
		"B must have been retried (its second LIST confirms it gone) rather than waiting behind A's Retry-After")
}

func TestStatusWait(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		objManifests  []string
		expectErrStrs []string
		waitForJobs   bool
	}{
		{
			name:          "Job is not complete",
			objManifests:  []string{jobNoStatusManifest},
			expectErrStrs: []string{"resource Job/qual/test not ready. status: InProgress", "context deadline exceeded"},
			waitForJobs:   true,
		},
		{
			name:          "Job is ready but not complete",
			objManifests:  []string{jobReadyManifest},
			expectErrStrs: nil,
			waitForJobs:   false,
		},
		{
			name:         "Pod is ready",
			objManifests: []string{podCurrentManifest},
		},
		{
			name:          "one of the pods never becomes ready",
			objManifests:  []string{podNoStatusManifest, podCurrentManifest},
			expectErrStrs: []string{"resource Pod/ns/in-progress-pod not ready. status: InProgress", "context deadline exceeded"},
		},
		{
			name:         "paused deployment passes",
			objManifests: []string{pausedDeploymentManifest},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c := newTestClient(t)
			fakeClient := dynamicfake.NewSimpleDynamicClient(scheme.Scheme)
			fakeMapper := testutil.NewFakeRESTMapper(
				v1.SchemeGroupVersion.WithKind("Pod"),
				appsv1.SchemeGroupVersion.WithKind("Deployment"),
				batchv1.SchemeGroupVersion.WithKind("Job"),
			)
			statusWaiter := statusWaiter{
				client:     fakeClient,
				restMapper: fakeMapper,
			}
			statusWaiter.SetLogger(slog.Default().Handler())
			objs := getRuntimeObjFromManifests(t, tt.objManifests)
			for _, obj := range objs {
				u := obj.(*unstructured.Unstructured)
				gvr := getGVR(t, fakeMapper, u)
				err := fakeClient.Tracker().Create(gvr, u, u.GetNamespace())
				require.NoError(t, err)
			}
			resourceList := getResourceListFromRuntimeObjs(t, c, objs)
			err := statusWaiter.Wait(resourceList, time.Second*3)
			if tt.expectErrStrs != nil {
				require.Error(t, err)
				for _, expectedErrStr := range tt.expectErrStrs {
					require.ErrorContains(t, err, expectedErrStr)
				}
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestWaitForJobComplete(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		objManifests  []string
		expectErrStrs []string
	}{
		{
			name:         "Job is complete",
			objManifests: []string{jobCompleteManifest},
		},
		{
			name:          "Job is not ready",
			objManifests:  []string{jobNoStatusManifest},
			expectErrStrs: []string{"resource Job/qual/test not ready. status: InProgress", "context deadline exceeded"},
		},
		{
			name:          "Job is ready but not complete",
			objManifests:  []string{jobReadyManifest},
			expectErrStrs: []string{"resource Job/default/ready-not-complete not ready. status: InProgress", "context deadline exceeded"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c := newTestClient(t)
			fakeClient := dynamicfake.NewSimpleDynamicClient(scheme.Scheme)
			fakeMapper := testutil.NewFakeRESTMapper(
				batchv1.SchemeGroupVersion.WithKind("Job"),
			)
			statusWaiter := statusWaiter{
				client:     fakeClient,
				restMapper: fakeMapper,
			}
			statusWaiter.SetLogger(slog.Default().Handler())
			objs := getRuntimeObjFromManifests(t, tt.objManifests)
			for _, obj := range objs {
				u := obj.(*unstructured.Unstructured)
				gvr := getGVR(t, fakeMapper, u)
				err := fakeClient.Tracker().Create(gvr, u, u.GetNamespace())
				require.NoError(t, err)
			}
			resourceList := getResourceListFromRuntimeObjs(t, c, objs)
			err := statusWaiter.WaitWithJobs(resourceList, time.Second*3)
			if tt.expectErrStrs != nil {
				require.Error(t, err)
				for _, expectedErrStr := range tt.expectErrStrs {
					require.ErrorContains(t, err, expectedErrStr)
				}
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestWatchForReady(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		objManifests  []string
		expectErrStrs []string
	}{
		{
			name:         "succeeds if pod and job are complete",
			objManifests: []string{jobCompleteManifest, podCompleteManifest},
		},
		{
			name:         "succeeds when a resource that's not a pod or job is not ready",
			objManifests: []string{notReadyDeploymentManifest},
		},
		{
			name:          "Fails if job is not complete",
			objManifests:  []string{jobReadyManifest},
			expectErrStrs: []string{"resource Job/default/ready-not-complete not ready. status: InProgress", "context deadline exceeded"},
		},
		{
			name:          "Fails if pod is not complete",
			objManifests:  []string{podCurrentManifest},
			expectErrStrs: []string{"resource Pod/ns/current-pod not ready. status: InProgress", "context deadline exceeded"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c := newTestClient(t)
			fakeClient := dynamicfake.NewSimpleDynamicClient(scheme.Scheme)
			fakeMapper := testutil.NewFakeRESTMapper(
				v1.SchemeGroupVersion.WithKind("Pod"),
				appsv1.SchemeGroupVersion.WithKind("Deployment"),
				batchv1.SchemeGroupVersion.WithKind("Job"),
			)
			statusWaiter := statusWaiter{
				client:     fakeClient,
				restMapper: fakeMapper,
			}
			statusWaiter.SetLogger(slog.Default().Handler())
			objs := getRuntimeObjFromManifests(t, tt.objManifests)
			for _, obj := range objs {
				u := obj.(*unstructured.Unstructured)
				gvr := getGVR(t, fakeMapper, u)
				err := fakeClient.Tracker().Create(gvr, u, u.GetNamespace())
				require.NoError(t, err)
			}
			resourceList := getResourceListFromRuntimeObjs(t, c, objs)
			err := statusWaiter.WatchUntilReady(resourceList, time.Second*3)
			if tt.expectErrStrs != nil {
				require.Error(t, err)
				for _, expectedErrStr := range tt.expectErrStrs {
					require.ErrorContains(t, err, expectedErrStr)
				}
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestStatusWaitMultipleNamespaces(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		objManifests  []string
		expectErrStrs []string
		testFunc      func(*statusWaiter, ResourceList, time.Duration) error
	}{
		{
			name:         "pods in multiple namespaces",
			objManifests: []string{podNamespace1Manifest, podNamespace2Manifest},
			testFunc: func(sw *statusWaiter, rl ResourceList, timeout time.Duration) error {
				return sw.Wait(rl, timeout)
			},
		},
		{
			name:         "hooks in multiple namespaces",
			objManifests: []string{jobNamespace1CompleteManifest, podNamespace2SucceededManifest},
			testFunc: func(sw *statusWaiter, rl ResourceList, timeout time.Duration) error {
				return sw.WatchUntilReady(rl, timeout)
			},
		},
		{
			name:          "error when resource not ready in one namespace",
			objManifests:  []string{podNamespace1NoStatusManifest, podNamespace2Manifest},
			expectErrStrs: []string{"resource Pod/namespace-1/pod-ns1 not ready. status: InProgress", "context deadline exceeded"},
			testFunc: func(sw *statusWaiter, rl ResourceList, timeout time.Duration) error {
				return sw.Wait(rl, timeout)
			},
		},
		{
			name:         "delete resources in multiple namespaces",
			objManifests: []string{podNamespace1Manifest, podNamespace2Manifest},
			testFunc: func(sw *statusWaiter, rl ResourceList, timeout time.Duration) error {
				return sw.WaitForDelete(rl, timeout)
			},
		},
		{
			name:         "cluster-scoped resources work correctly with unrestricted permissions",
			objManifests: []string{podNamespace1Manifest, clusterRoleManifest},
			testFunc: func(sw *statusWaiter, rl ResourceList, timeout time.Duration) error {
				return sw.Wait(rl, timeout)
			},
		},
		{
			name:         "namespace-scoped and cluster-scoped resources work together",
			objManifests: []string{podNamespace1Manifest, podNamespace2Manifest, clusterRoleManifest},
			testFunc: func(sw *statusWaiter, rl ResourceList, timeout time.Duration) error {
				return sw.Wait(rl, timeout)
			},
		},
		{
			name:         "delete cluster-scoped resources works correctly",
			objManifests: []string{podNamespace1Manifest, namespaceManifest},
			testFunc: func(sw *statusWaiter, rl ResourceList, timeout time.Duration) error {
				return sw.WaitForDelete(rl, timeout)
			},
		},
		{
			name:         "watch cluster-scoped resources works correctly",
			objManifests: []string{clusterRoleManifest},
			testFunc: func(sw *statusWaiter, rl ResourceList, timeout time.Duration) error {
				return sw.WatchUntilReady(rl, timeout)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c := newTestClient(t)
			fakeClient := dynamicfake.NewSimpleDynamicClient(scheme.Scheme)
			fakeMapper := testutil.NewFakeRESTMapper(
				v1.SchemeGroupVersion.WithKind("Pod"),
				batchv1.SchemeGroupVersion.WithKind("Job"),
				schema.GroupVersion{Group: "rbac.authorization.k8s.io", Version: "v1"}.WithKind("ClusterRole"),
				v1.SchemeGroupVersion.WithKind("Namespace"),
			)
			sw := statusWaiter{
				client:     fakeClient,
				restMapper: fakeMapper,
			}
			sw.SetLogger(slog.Default().Handler())
			objs := getRuntimeObjFromManifests(t, tt.objManifests)
			for _, obj := range objs {
				u := obj.(*unstructured.Unstructured)
				gvr := getGVR(t, fakeMapper, u)
				err := fakeClient.Tracker().Create(gvr, u, u.GetNamespace())
				require.NoError(t, err)
			}

			if strings.Contains(tt.name, "delete") {
				timeUntilDelete := time.Millisecond * 500
				for _, obj := range objs {
					u := obj.(*unstructured.Unstructured)
					gvr := getGVR(t, fakeMapper, u)
					go func(gvr schema.GroupVersionResource, u *unstructured.Unstructured) {
						time.Sleep(timeUntilDelete)
						err := fakeClient.Tracker().Delete(gvr, u.GetNamespace(), u.GetName())
						assert.NoError(t, err)
					}(gvr, u)
				}
			}

			resourceList := getResourceListFromRuntimeObjs(t, c, objs)
			err := tt.testFunc(&sw, resourceList, time.Second*3)
			if tt.expectErrStrs != nil {
				require.Error(t, err)
				for _, expectedErrStr := range tt.expectErrStrs {
					require.ErrorContains(t, err, expectedErrStr)
				}
				return
			}
			assert.NoError(t, err)
		})
	}
}

// restrictedClientConfig holds the configuration for RBAC simulation on a fake dynamic client
type restrictedClientConfig struct {
	allowedNamespaces          map[string]bool
	clusterScopedListAttempted bool
}

// setupRestrictedClient configures a fake dynamic client to simulate RBAC restrictions
// by using PrependReactor and PrependWatchReactor to intercept list/watch operations.
func setupRestrictedClient(fakeClient *dynamicfake.FakeDynamicClient, allowedNamespaces []string) *restrictedClientConfig {
	allowed := make(map[string]bool)
	for _, ns := range allowedNamespaces {
		allowed[ns] = true
	}
	config := &restrictedClientConfig{
		allowedNamespaces: allowed,
	}

	// Intercept list operations
	fakeClient.PrependReactor("list", "*", func(action clienttesting.Action) (bool, runtime.Object, error) {
		listAction := action.(clienttesting.ListAction)
		ns := listAction.GetNamespace()
		if ns == "" {
			// Cluster-scoped list
			config.clusterScopedListAttempted = true
			return true, nil, apierrors.NewForbidden(
				action.GetResource().GroupResource(),
				"",
				errors.New("user does not have cluster-wide LIST permissions for cluster-scoped resources"),
			)
		}
		if !config.allowedNamespaces[ns] {
			return true, nil, apierrors.NewForbidden(
				action.GetResource().GroupResource(),
				"",
				fmt.Errorf("user does not have LIST permissions in namespace %q", ns),
			)
		}
		// Fall through to the default handler
		return false, nil, nil
	})

	// Intercept watch operations
	fakeClient.PrependWatchReactor("*", func(action clienttesting.Action) (bool, watch.Interface, error) {
		watchAction := action.(clienttesting.WatchAction)
		ns := watchAction.GetNamespace()
		if ns == "" {
			// Cluster-scoped watch
			config.clusterScopedListAttempted = true
			return true, nil, apierrors.NewForbidden(
				action.GetResource().GroupResource(),
				"",
				errors.New("user does not have cluster-wide WATCH permissions for cluster-scoped resources"),
			)
		}
		if !config.allowedNamespaces[ns] {
			return true, nil, apierrors.NewForbidden(
				action.GetResource().GroupResource(),
				"",
				fmt.Errorf("user does not have WATCH permissions in namespace %q", ns),
			)
		}
		// Fall through to the default handler
		return false, nil, nil
	})

	return config
}

func TestStatusWaitRestrictedRBAC(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name              string
		objManifests      []string
		allowedNamespaces []string
		expectErrs        []error
		testFunc          func(*statusWaiter, ResourceList, time.Duration) error
	}{
		{
			name:              "pods in multiple namespaces with namespace permissions",
			objManifests:      []string{podNamespace1Manifest, podNamespace2Manifest},
			allowedNamespaces: []string{"namespace-1", "namespace-2"},
			testFunc: func(sw *statusWaiter, rl ResourceList, timeout time.Duration) error {
				return sw.Wait(rl, timeout)
			},
		},
		{
			name:              "delete pods in multiple namespaces with namespace permissions",
			objManifests:      []string{podNamespace1Manifest, podNamespace2Manifest},
			allowedNamespaces: []string{"namespace-1", "namespace-2"},
			testFunc: func(sw *statusWaiter, rl ResourceList, timeout time.Duration) error {
				return sw.WaitForDelete(rl, timeout)
			},
		},
		{
			name:              "hooks in multiple namespaces with namespace permissions",
			objManifests:      []string{jobNamespace1CompleteManifest, podNamespace2SucceededManifest},
			allowedNamespaces: []string{"namespace-1", "namespace-2"},
			testFunc: func(sw *statusWaiter, rl ResourceList, timeout time.Duration) error {
				return sw.WatchUntilReady(rl, timeout)
			},
		},
		{
			name:              "error when cluster-scoped resource included",
			objManifests:      []string{podNamespace1Manifest, clusterRoleManifest},
			allowedNamespaces: []string{"namespace-1"},
			expectErrs:        []error{errors.New("user does not have cluster-wide LIST permissions for cluster-scoped resources")},
			testFunc: func(sw *statusWaiter, rl ResourceList, timeout time.Duration) error {
				return sw.Wait(rl, timeout)
			},
		},
		{
			name:              "error when deleting cluster-scoped resource",
			objManifests:      []string{podNamespace1Manifest, namespaceManifest},
			allowedNamespaces: []string{"namespace-1"},
			expectErrs:        []error{errors.New("user does not have cluster-wide LIST permissions for cluster-scoped resources")},
			testFunc: func(sw *statusWaiter, rl ResourceList, timeout time.Duration) error {
				return sw.WaitForDelete(rl, timeout)
			},
		},
		{
			name:              "error when accessing disallowed namespace",
			objManifests:      []string{podNamespace1Manifest, podNamespace2Manifest},
			allowedNamespaces: []string{"namespace-1"},
			expectErrs:        []error{fmt.Errorf("user does not have LIST permissions in namespace %q", "namespace-2")},
			testFunc: func(sw *statusWaiter, rl ResourceList, timeout time.Duration) error {
				return sw.Wait(rl, timeout)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c := newTestClient(t)
			baseFakeClient := dynamicfake.NewSimpleDynamicClient(scheme.Scheme)
			fakeMapper := testutil.NewFakeRESTMapper(
				v1.SchemeGroupVersion.WithKind("Pod"),
				batchv1.SchemeGroupVersion.WithKind("Job"),
				schema.GroupVersion{Group: "rbac.authorization.k8s.io", Version: "v1"}.WithKind("ClusterRole"),
				v1.SchemeGroupVersion.WithKind("Namespace"),
			)
			restrictedConfig := setupRestrictedClient(baseFakeClient, tt.allowedNamespaces)
			sw := statusWaiter{
				client:     baseFakeClient,
				restMapper: fakeMapper,
			}
			sw.SetLogger(slog.Default().Handler())
			objs := getRuntimeObjFromManifests(t, tt.objManifests)
			for _, obj := range objs {
				u := obj.(*unstructured.Unstructured)
				gvr := getGVR(t, fakeMapper, u)
				err := baseFakeClient.Tracker().Create(gvr, u, u.GetNamespace())
				require.NoError(t, err)
			}

			if strings.Contains(tt.name, "delet") {
				timeUntilDelete := time.Millisecond * 500
				for _, obj := range objs {
					u := obj.(*unstructured.Unstructured)
					gvr := getGVR(t, fakeMapper, u)
					go func(gvr schema.GroupVersionResource, u *unstructured.Unstructured) {
						time.Sleep(timeUntilDelete)
						err := baseFakeClient.Tracker().Delete(gvr, u.GetNamespace(), u.GetName())
						assert.NoError(t, err)
					}(gvr, u)
				}
			}

			resourceList := getResourceListFromRuntimeObjs(t, c, objs)
			err := tt.testFunc(&sw, resourceList, time.Second*3)
			if tt.expectErrs != nil {
				require.Error(t, err)
				for _, expectedErr := range tt.expectErrs {
					require.ErrorContains(t, err, expectedErr.Error())
				}
				return
			}
			require.NoError(t, err)
			assert.False(t, restrictedConfig.clusterScopedListAttempted)
		})
	}
}

func TestStatusWaitMixedResources(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name              string
		objManifests      []string
		allowedNamespaces []string
		expectErrs        []error
		testFunc          func(*statusWaiter, ResourceList, time.Duration) error
	}{
		{
			name:              "wait succeeds with namespace-scoped resources only",
			objManifests:      []string{podNamespace1Manifest, podNamespace2Manifest},
			allowedNamespaces: []string{"namespace-1", "namespace-2"},
			testFunc: func(sw *statusWaiter, rl ResourceList, timeout time.Duration) error {
				return sw.Wait(rl, timeout)
			},
		},
		{
			name:              "wait fails when cluster-scoped resource included",
			objManifests:      []string{podNamespace1Manifest, clusterRoleManifest},
			allowedNamespaces: []string{"namespace-1"},
			expectErrs:        []error{errors.New("user does not have cluster-wide LIST permissions for cluster-scoped resources")},
			testFunc: func(sw *statusWaiter, rl ResourceList, timeout time.Duration) error {
				return sw.Wait(rl, timeout)
			},
		},
		{
			name:              "waitForDelete fails when cluster-scoped resource included",
			objManifests:      []string{podNamespace1Manifest, clusterRoleManifest},
			allowedNamespaces: []string{"namespace-1"},
			expectErrs:        []error{errors.New("user does not have cluster-wide LIST permissions for cluster-scoped resources")},
			testFunc: func(sw *statusWaiter, rl ResourceList, timeout time.Duration) error {
				return sw.WaitForDelete(rl, timeout)
			},
		},
		{
			name:              "wait fails when namespace resource included",
			objManifests:      []string{podNamespace1Manifest, namespaceManifest},
			allowedNamespaces: []string{"namespace-1"},
			expectErrs:        []error{errors.New("user does not have cluster-wide LIST permissions for cluster-scoped resources")},
			testFunc: func(sw *statusWaiter, rl ResourceList, timeout time.Duration) error {
				return sw.Wait(rl, timeout)
			},
		},
		{
			name:              "error when accessing disallowed namespace",
			objManifests:      []string{podNamespace1Manifest, podNamespace2Manifest},
			allowedNamespaces: []string{"namespace-1"},
			expectErrs:        []error{fmt.Errorf("user does not have LIST permissions in namespace %q", "namespace-2")},
			testFunc: func(sw *statusWaiter, rl ResourceList, timeout time.Duration) error {
				return sw.Wait(rl, timeout)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c := newTestClient(t)
			baseFakeClient := dynamicfake.NewSimpleDynamicClient(scheme.Scheme)
			fakeMapper := testutil.NewFakeRESTMapper(
				v1.SchemeGroupVersion.WithKind("Pod"),
				batchv1.SchemeGroupVersion.WithKind("Job"),
				schema.GroupVersion{Group: "rbac.authorization.k8s.io", Version: "v1"}.WithKind("ClusterRole"),
				v1.SchemeGroupVersion.WithKind("Namespace"),
			)
			restrictedConfig := setupRestrictedClient(baseFakeClient, tt.allowedNamespaces)
			sw := statusWaiter{
				client:     baseFakeClient,
				restMapper: fakeMapper,
			}
			sw.SetLogger(slog.Default().Handler())
			objs := getRuntimeObjFromManifests(t, tt.objManifests)
			for _, obj := range objs {
				u := obj.(*unstructured.Unstructured)
				gvr := getGVR(t, fakeMapper, u)
				err := baseFakeClient.Tracker().Create(gvr, u, u.GetNamespace())
				require.NoError(t, err)
			}

			if strings.Contains(tt.name, "delet") {
				timeUntilDelete := time.Millisecond * 500
				for _, obj := range objs {
					u := obj.(*unstructured.Unstructured)
					gvr := getGVR(t, fakeMapper, u)
					go func(gvr schema.GroupVersionResource, u *unstructured.Unstructured) {
						time.Sleep(timeUntilDelete)
						err := baseFakeClient.Tracker().Delete(gvr, u.GetNamespace(), u.GetName())
						assert.NoError(t, err)
					}(gvr, u)
				}
			}

			resourceList := getResourceListFromRuntimeObjs(t, c, objs)
			err := tt.testFunc(&sw, resourceList, time.Second*3)
			if tt.expectErrs != nil {
				require.Error(t, err)
				for _, expectedErr := range tt.expectErrs {
					require.ErrorContains(t, err, expectedErr.Error())
				}
				return
			}
			require.NoError(t, err)
			assert.False(t, restrictedConfig.clusterScopedListAttempted)
		})
	}
}

// mockStatusReader is a custom status reader for testing that tracks when it's used
// and returns a configurable status for resources it supports.
type mockStatusReader struct {
	supportedGK schema.GroupKind
	status      status.Status
	callCount   atomic.Int32
}

func (m *mockStatusReader) Supports(gk schema.GroupKind) bool {
	return gk == m.supportedGK
}

func (m *mockStatusReader) ReadStatus(_ context.Context, _ engine.ClusterReader, id object.ObjMetadata) (*event.ResourceStatus, error) {
	m.callCount.Add(1)
	return &event.ResourceStatus{
		Identifier: id,
		Status:     m.status,
		Message:    "mock status reader",
	}, nil
}

func (m *mockStatusReader) ReadStatusForObject(_ context.Context, _ engine.ClusterReader, u *unstructured.Unstructured) (*event.ResourceStatus, error) {
	m.callCount.Add(1)
	id := object.ObjMetadata{
		Namespace: u.GetNamespace(),
		Name:      u.GetName(),
		GroupKind: u.GroupVersionKind().GroupKind(),
	}
	return &event.ResourceStatus{
		Identifier: id,
		Status:     m.status,
		Message:    "mock status reader",
	}, nil
}

func TestStatusWaitWithCustomReaders(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		objManifests  []string
		customReader  *mockStatusReader
		expectErrStrs []string
	}{
		{
			name:         "custom reader makes pod immediately current",
			objManifests: []string{podNoStatusManifest},
			customReader: &mockStatusReader{
				supportedGK: v1.SchemeGroupVersion.WithKind("Pod").GroupKind(),
				status:      status.CurrentStatus,
			},
		},
		{
			name:         "custom reader returns in-progress status",
			objManifests: []string{podCurrentManifest},
			customReader: &mockStatusReader{
				supportedGK: v1.SchemeGroupVersion.WithKind("Pod").GroupKind(),
				status:      status.InProgressStatus,
			},
			expectErrStrs: []string{"resource Pod/ns/current-pod not ready. status: InProgress", "context deadline exceeded"},
		},
		{
			name:         "custom reader for different resource type is not used",
			objManifests: []string{podCurrentManifest},
			customReader: &mockStatusReader{
				supportedGK: batchv1.SchemeGroupVersion.WithKind("Job").GroupKind(),
				status:      status.InProgressStatus,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c := newTestClient(t)
			fakeClient := dynamicfake.NewSimpleDynamicClient(scheme.Scheme)
			fakeMapper := testutil.NewFakeRESTMapper(
				v1.SchemeGroupVersion.WithKind("Pod"),
				batchv1.SchemeGroupVersion.WithKind("Job"),
			)
			statusWaiter := statusWaiter{
				client:     fakeClient,
				restMapper: fakeMapper,
				readers:    []engine.StatusReader{tt.customReader},
			}
			objs := getRuntimeObjFromManifests(t, tt.objManifests)
			for _, obj := range objs {
				u := obj.(*unstructured.Unstructured)
				gvr := getGVR(t, fakeMapper, u)
				err := fakeClient.Tracker().Create(gvr, u, u.GetNamespace())
				require.NoError(t, err)
			}
			resourceList := getResourceListFromRuntimeObjs(t, c, objs)
			err := statusWaiter.Wait(resourceList, time.Second*3)
			if tt.expectErrStrs != nil {
				require.Error(t, err)
				for _, expectedErrStr := range tt.expectErrStrs {
					require.ErrorContains(t, err, expectedErrStr)
				}
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestStatusWaitWithJobsAndCustomReaders(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		objManifests []string
		customReader *mockStatusReader
		expectErrs   []error
	}{
		{
			name:         "custom reader makes job immediately current",
			objManifests: []string{jobNoStatusManifest},
			customReader: &mockStatusReader{
				supportedGK: batchv1.SchemeGroupVersion.WithKind("Job").GroupKind(),
				status:      status.CurrentStatus,
			},
			expectErrs: nil,
		},
		{
			name:         "custom reader for pod works with WaitWithJobs",
			objManifests: []string{podNoStatusManifest},
			customReader: &mockStatusReader{
				supportedGK: v1.SchemeGroupVersion.WithKind("Pod").GroupKind(),
				status:      status.CurrentStatus,
			},
			expectErrs: nil,
		},
		{
			name:         "built-in job reader is still appended after custom readers",
			objManifests: []string{jobCompleteManifest},
			customReader: &mockStatusReader{
				supportedGK: v1.SchemeGroupVersion.WithKind("Pod").GroupKind(),
				status:      status.CurrentStatus,
			},
			expectErrs: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c := newTestClient(t)
			fakeClient := dynamicfake.NewSimpleDynamicClient(scheme.Scheme)
			fakeMapper := testutil.NewFakeRESTMapper(
				v1.SchemeGroupVersion.WithKind("Pod"),
				batchv1.SchemeGroupVersion.WithKind("Job"),
			)
			statusWaiter := statusWaiter{
				client:     fakeClient,
				restMapper: fakeMapper,
				readers:    []engine.StatusReader{tt.customReader},
			}
			objs := getRuntimeObjFromManifests(t, tt.objManifests)
			for _, obj := range objs {
				u := obj.(*unstructured.Unstructured)
				gvr := getGVR(t, fakeMapper, u)
				err := fakeClient.Tracker().Create(gvr, u, u.GetNamespace())
				require.NoError(t, err)
			}
			resourceList := getResourceListFromRuntimeObjs(t, c, objs)
			err := statusWaiter.WaitWithJobs(resourceList, time.Second*3)
			if tt.expectErrs != nil {
				assert.EqualError(t, err, errors.Join(tt.expectErrs...).Error())
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestStatusWaitWithFailedResources(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		objManifests  []string
		customReader  *mockStatusReader
		expectErrStrs []string
		testFunc      func(*statusWaiter, ResourceList, time.Duration) error
	}{
		{
			name:         "Wait returns error when resource has failed",
			objManifests: []string{podNoStatusManifest},
			customReader: &mockStatusReader{
				supportedGK: v1.SchemeGroupVersion.WithKind("Pod").GroupKind(),
				status:      status.FailedStatus,
			},
			expectErrStrs: []string{"resource Pod/ns/in-progress-pod not ready. status: Failed, message: mock status reader"},
			testFunc: func(sw *statusWaiter, rl ResourceList, timeout time.Duration) error {
				return sw.Wait(rl, timeout)
			},
		},
		{
			name:         "WaitWithJobs returns error when job has failed",
			objManifests: []string{jobFailedManifest},
			customReader: nil, // Use the built-in job status reader
			expectErrStrs: []string{
				"resource Job/default/failed-job not ready. status: Failed",
			},
			testFunc: func(sw *statusWaiter, rl ResourceList, timeout time.Duration) error {
				return sw.WaitWithJobs(rl, timeout)
			},
		},
		{
			name:         "Wait returns errors when multiple resources fail",
			objManifests: []string{podNoStatusManifest, podCurrentManifest},
			customReader: &mockStatusReader{
				supportedGK: v1.SchemeGroupVersion.WithKind("Pod").GroupKind(),
				status:      status.FailedStatus,
			},
			// The mock reader will make both pods return FailedStatus
			expectErrStrs: []string{
				"resource Pod/ns/in-progress-pod not ready. status: Failed, message: mock status reader",
				"resource Pod/ns/current-pod not ready. status: Failed, message: mock status reader",
			},
			testFunc: func(sw *statusWaiter, rl ResourceList, timeout time.Duration) error {
				return sw.Wait(rl, timeout)
			},
		},
		{
			name:         "WatchUntilReady returns error when resource has failed",
			objManifests: []string{podNoStatusManifest},
			customReader: &mockStatusReader{
				supportedGK: v1.SchemeGroupVersion.WithKind("Pod").GroupKind(),
				status:      status.FailedStatus,
			},
			// WatchUntilReady also waits for CurrentStatus, so failed resources should return error
			expectErrStrs: []string{"resource Pod/ns/in-progress-pod not ready. status: Failed, message: mock status reader"},
			testFunc: func(sw *statusWaiter, rl ResourceList, timeout time.Duration) error {
				return sw.WatchUntilReady(rl, timeout)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c := newTestClient(t)
			fakeClient := dynamicfake.NewSimpleDynamicClient(scheme.Scheme)
			fakeMapper := testutil.NewFakeRESTMapper(
				v1.SchemeGroupVersion.WithKind("Pod"),
				batchv1.SchemeGroupVersion.WithKind("Job"),
			)
			var readers []engine.StatusReader
			if tt.customReader != nil {
				readers = []engine.StatusReader{tt.customReader}
			}
			sw := statusWaiter{
				client:     fakeClient,
				restMapper: fakeMapper,
				readers:    readers,
			}
			objs := getRuntimeObjFromManifests(t, tt.objManifests)
			for _, obj := range objs {
				u := obj.(*unstructured.Unstructured)
				gvr := getGVR(t, fakeMapper, u)
				err := fakeClient.Tracker().Create(gvr, u, u.GetNamespace())
				require.NoError(t, err)
			}
			resourceList := getResourceListFromRuntimeObjs(t, c, objs)
			err := tt.testFunc(&sw, resourceList, time.Second*3)
			if tt.expectErrStrs != nil {
				require.Error(t, err)
				for _, expectedErrStr := range tt.expectErrStrs {
					require.ErrorContains(t, err, expectedErrStr)
				}
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestWaitOptionFunctions(t *testing.T) {
	t.Parallel()

	t.Run("WithWatchUntilReadyMethodContext sets watchUntilReadyCtx", func(t *testing.T) {
		t.Parallel()
		type contextKey struct{}
		ctx := context.WithValue(context.Background(), contextKey{}, "test")
		opts := &waitOptions{}
		WithWatchUntilReadyMethodContext(ctx)(opts)
		assert.Equal(t, ctx, opts.watchUntilReadyCtx)
	})

	t.Run("WithWaitMethodContext sets waitCtx", func(t *testing.T) {
		t.Parallel()
		type contextKey struct{}
		ctx := context.WithValue(context.Background(), contextKey{}, "test")
		opts := &waitOptions{}
		WithWaitMethodContext(ctx)(opts)
		assert.Equal(t, ctx, opts.waitCtx)
	})

	t.Run("WithWaitWithJobsMethodContext sets waitWithJobsCtx", func(t *testing.T) {
		t.Parallel()
		type contextKey struct{}
		ctx := context.WithValue(context.Background(), contextKey{}, "test")
		opts := &waitOptions{}
		WithWaitWithJobsMethodContext(ctx)(opts)
		assert.Equal(t, ctx, opts.waitWithJobsCtx)
	})

	t.Run("WithWaitForDeleteMethodContext sets waitForDeleteCtx", func(t *testing.T) {
		t.Parallel()
		type contextKey struct{}
		ctx := context.WithValue(context.Background(), contextKey{}, "test")
		opts := &waitOptions{}
		WithWaitForDeleteMethodContext(ctx)(opts)
		assert.Equal(t, ctx, opts.waitForDeleteCtx)
	})
}

func TestMethodSpecificContextCancellation(t *testing.T) {
	t.Parallel()

	t.Run("WatchUntilReady uses method-specific context", func(t *testing.T) {
		t.Parallel()
		c := newTestClient(t)
		fakeClient := dynamicfake.NewSimpleDynamicClient(scheme.Scheme)
		fakeMapper := testutil.NewFakeRESTMapper(
			v1.SchemeGroupVersion.WithKind("Pod"),
		)

		// Create a cancelled method-specific context
		methodCtx, methodCancel := context.WithCancel(context.Background())
		methodCancel() // Cancel immediately

		sw := statusWaiter{
			client:             fakeClient,
			restMapper:         fakeMapper,
			ctx:                context.Background(), // General context is not cancelled
			watchUntilReadyCtx: methodCtx,            // Method context is cancelled
		}

		objs := getRuntimeObjFromManifests(t, []string{podCompleteManifest})
		for _, obj := range objs {
			u := obj.(*unstructured.Unstructured)
			gvr := getGVR(t, fakeMapper, u)
			err := fakeClient.Tracker().Create(gvr, u, u.GetNamespace())
			require.NoError(t, err)
		}
		resourceList := getResourceListFromRuntimeObjs(t, c, objs)

		err := sw.WatchUntilReady(resourceList, time.Second*3)
		// Should fail due to cancelled method context
		assert.ErrorContains(t, err, "context canceled")
	})

	t.Run("Wait uses method-specific context", func(t *testing.T) {
		t.Parallel()
		c := newTestClient(t)
		fakeClient := dynamicfake.NewSimpleDynamicClient(scheme.Scheme)
		fakeMapper := testutil.NewFakeRESTMapper(
			v1.SchemeGroupVersion.WithKind("Pod"),
		)

		// Create a cancelled method-specific context
		methodCtx, methodCancel := context.WithCancel(context.Background())
		methodCancel() // Cancel immediately

		sw := statusWaiter{
			client:     fakeClient,
			restMapper: fakeMapper,
			ctx:        context.Background(), // General context is not cancelled
			waitCtx:    methodCtx,            // Method context is cancelled
		}

		objs := getRuntimeObjFromManifests(t, []string{podCurrentManifest})
		for _, obj := range objs {
			u := obj.(*unstructured.Unstructured)
			gvr := getGVR(t, fakeMapper, u)
			err := fakeClient.Tracker().Create(gvr, u, u.GetNamespace())
			require.NoError(t, err)
		}
		resourceList := getResourceListFromRuntimeObjs(t, c, objs)

		err := sw.Wait(resourceList, time.Second*3)
		// Should fail due to cancelled method context
		assert.ErrorContains(t, err, "context canceled")
	})

	t.Run("WaitWithJobs uses method-specific context", func(t *testing.T) {
		t.Parallel()
		c := newTestClient(t)
		fakeClient := dynamicfake.NewSimpleDynamicClient(scheme.Scheme)
		fakeMapper := testutil.NewFakeRESTMapper(
			batchv1.SchemeGroupVersion.WithKind("Job"),
		)

		// Create a cancelled method-specific context
		methodCtx, methodCancel := context.WithCancel(context.Background())
		methodCancel() // Cancel immediately

		sw := statusWaiter{
			client:          fakeClient,
			restMapper:      fakeMapper,
			ctx:             context.Background(), // General context is not cancelled
			waitWithJobsCtx: methodCtx,            // Method context is cancelled
		}

		objs := getRuntimeObjFromManifests(t, []string{jobCompleteManifest})
		for _, obj := range objs {
			u := obj.(*unstructured.Unstructured)
			gvr := getGVR(t, fakeMapper, u)
			err := fakeClient.Tracker().Create(gvr, u, u.GetNamespace())
			require.NoError(t, err)
		}
		resourceList := getResourceListFromRuntimeObjs(t, c, objs)

		err := sw.WaitWithJobs(resourceList, time.Second*3)
		// Should fail due to cancelled method context
		assert.ErrorContains(t, err, "context canceled")
	})

	t.Run("WaitForDelete uses method-specific context", func(t *testing.T) {
		t.Parallel()
		c := newTestClient(t)
		fakeClient := dynamicfake.NewSimpleDynamicClient(scheme.Scheme)
		fakeMapper := testutil.NewFakeRESTMapper(
			v1.SchemeGroupVersion.WithKind("Pod"),
		)

		// Create a cancelled method-specific context
		methodCtx, methodCancel := context.WithCancel(context.Background())
		methodCancel() // Cancel immediately

		sw := statusWaiter{
			client:           fakeClient,
			restMapper:       fakeMapper,
			ctx:              context.Background(), // General context is not cancelled
			waitForDeleteCtx: methodCtx,            // Method context is cancelled
		}

		objs := getRuntimeObjFromManifests(t, []string{podCurrentManifest})
		for _, obj := range objs {
			u := obj.(*unstructured.Unstructured)
			gvr := getGVR(t, fakeMapper, u)
			err := fakeClient.Tracker().Create(gvr, u, u.GetNamespace())
			require.NoError(t, err)
		}
		resourceList := getResourceListFromRuntimeObjs(t, c, objs)

		err := sw.WaitForDelete(resourceList, time.Second*3)
		// Should fail due to cancelled method context
		assert.ErrorContains(t, err, "context canceled")
	})
}

func TestMethodContextFallbackToGeneralContext(t *testing.T) {
	t.Parallel()

	t.Run("WatchUntilReady falls back to general context when method context is nil", func(t *testing.T) {
		t.Parallel()
		c := newTestClient(t)
		fakeClient := dynamicfake.NewSimpleDynamicClient(scheme.Scheme)
		fakeMapper := testutil.NewFakeRESTMapper(
			v1.SchemeGroupVersion.WithKind("Pod"),
		)

		// Create a cancelled general context
		generalCtx, generalCancel := context.WithCancel(context.Background())
		generalCancel() // Cancel immediately

		sw := statusWaiter{
			client:             fakeClient,
			restMapper:         fakeMapper,
			ctx:                generalCtx, // General context is cancelled
			watchUntilReadyCtx: nil,        // Method context is nil, should fall back
		}

		objs := getRuntimeObjFromManifests(t, []string{podCompleteManifest})
		for _, obj := range objs {
			u := obj.(*unstructured.Unstructured)
			gvr := getGVR(t, fakeMapper, u)
			err := fakeClient.Tracker().Create(gvr, u, u.GetNamespace())
			require.NoError(t, err)
		}
		resourceList := getResourceListFromRuntimeObjs(t, c, objs)

		err := sw.WatchUntilReady(resourceList, time.Second*3)
		// Should fail due to cancelled general context
		assert.ErrorContains(t, err, "context canceled")
	})

	t.Run("Wait falls back to general context when method context is nil", func(t *testing.T) {
		t.Parallel()
		c := newTestClient(t)
		fakeClient := dynamicfake.NewSimpleDynamicClient(scheme.Scheme)
		fakeMapper := testutil.NewFakeRESTMapper(
			v1.SchemeGroupVersion.WithKind("Pod"),
		)

		// Create a cancelled general context
		generalCtx, generalCancel := context.WithCancel(context.Background())
		generalCancel() // Cancel immediately

		sw := statusWaiter{
			client:     fakeClient,
			restMapper: fakeMapper,
			ctx:        generalCtx, // General context is cancelled
			waitCtx:    nil,        // Method context is nil, should fall back
		}

		objs := getRuntimeObjFromManifests(t, []string{podCurrentManifest})
		for _, obj := range objs {
			u := obj.(*unstructured.Unstructured)
			gvr := getGVR(t, fakeMapper, u)
			err := fakeClient.Tracker().Create(gvr, u, u.GetNamespace())
			require.NoError(t, err)
		}
		resourceList := getResourceListFromRuntimeObjs(t, c, objs)

		err := sw.Wait(resourceList, time.Second*3)
		// Should fail due to cancelled general context
		assert.ErrorContains(t, err, "context canceled")
	})

	t.Run("WaitWithJobs falls back to general context when method context is nil", func(t *testing.T) {
		t.Parallel()
		c := newTestClient(t)
		fakeClient := dynamicfake.NewSimpleDynamicClient(scheme.Scheme)
		fakeMapper := testutil.NewFakeRESTMapper(
			batchv1.SchemeGroupVersion.WithKind("Job"),
		)

		// Create a cancelled general context
		generalCtx, generalCancel := context.WithCancel(context.Background())
		generalCancel() // Cancel immediately

		sw := statusWaiter{
			client:          fakeClient,
			restMapper:      fakeMapper,
			ctx:             generalCtx, // General context is cancelled
			waitWithJobsCtx: nil,        // Method context is nil, should fall back
		}

		objs := getRuntimeObjFromManifests(t, []string{jobCompleteManifest})
		for _, obj := range objs {
			u := obj.(*unstructured.Unstructured)
			gvr := getGVR(t, fakeMapper, u)
			err := fakeClient.Tracker().Create(gvr, u, u.GetNamespace())
			require.NoError(t, err)
		}
		resourceList := getResourceListFromRuntimeObjs(t, c, objs)

		err := sw.WaitWithJobs(resourceList, time.Second*3)
		// Should fail due to cancelled general context
		assert.ErrorContains(t, err, "context canceled")
	})

	t.Run("WaitForDelete falls back to general context when method context is nil", func(t *testing.T) {
		t.Parallel()
		c := newTestClient(t)
		fakeClient := dynamicfake.NewSimpleDynamicClient(scheme.Scheme)
		fakeMapper := testutil.NewFakeRESTMapper(
			v1.SchemeGroupVersion.WithKind("Pod"),
		)

		// Create a cancelled general context
		generalCtx, generalCancel := context.WithCancel(context.Background())
		generalCancel() // Cancel immediately

		sw := statusWaiter{
			client:           fakeClient,
			restMapper:       fakeMapper,
			ctx:              generalCtx, // General context is cancelled
			waitForDeleteCtx: nil,        // Method context is nil, should fall back
		}

		objs := getRuntimeObjFromManifests(t, []string{podCurrentManifest})
		for _, obj := range objs {
			u := obj.(*unstructured.Unstructured)
			gvr := getGVR(t, fakeMapper, u)
			err := fakeClient.Tracker().Create(gvr, u, u.GetNamespace())
			require.NoError(t, err)
		}
		resourceList := getResourceListFromRuntimeObjs(t, c, objs)

		err := sw.WaitForDelete(resourceList, time.Second*3)
		// Should fail due to cancelled general context
		assert.ErrorContains(t, err, "context canceled")
	})
}

func TestMethodContextOverridesGeneralContext(t *testing.T) {
	t.Parallel()

	t.Run("method-specific context overrides general context for WatchUntilReady", func(t *testing.T) {
		t.Parallel()
		c := newTestClient(t)
		fakeClient := dynamicfake.NewSimpleDynamicClient(scheme.Scheme)
		fakeMapper := testutil.NewFakeRESTMapper(
			v1.SchemeGroupVersion.WithKind("Pod"),
		)

		// General context is cancelled, but method context is not
		generalCtx, generalCancel := context.WithCancel(context.Background())
		generalCancel()

		sw := statusWaiter{
			client:             fakeClient,
			restMapper:         fakeMapper,
			ctx:                generalCtx,           // Cancelled
			watchUntilReadyCtx: context.Background(), // Not cancelled - should be used
		}

		objs := getRuntimeObjFromManifests(t, []string{podCompleteManifest})
		for _, obj := range objs {
			u := obj.(*unstructured.Unstructured)
			gvr := getGVR(t, fakeMapper, u)
			err := fakeClient.Tracker().Create(gvr, u, u.GetNamespace())
			require.NoError(t, err)
		}
		resourceList := getResourceListFromRuntimeObjs(t, c, objs)

		err := sw.WatchUntilReady(resourceList, time.Second*3)
		// Should succeed because method context is used and it's not cancelled
		assert.NoError(t, err)
	})

	t.Run("method-specific context overrides general context for Wait", func(t *testing.T) {
		t.Parallel()
		c := newTestClient(t)
		fakeClient := dynamicfake.NewSimpleDynamicClient(scheme.Scheme)
		fakeMapper := testutil.NewFakeRESTMapper(
			v1.SchemeGroupVersion.WithKind("Pod"),
		)

		// General context is cancelled, but method context is not
		generalCtx, generalCancel := context.WithCancel(context.Background())
		generalCancel()

		sw := statusWaiter{
			client:     fakeClient,
			restMapper: fakeMapper,
			ctx:        generalCtx,           // Cancelled
			waitCtx:    context.Background(), // Not cancelled - should be used
		}

		objs := getRuntimeObjFromManifests(t, []string{podCurrentManifest})
		for _, obj := range objs {
			u := obj.(*unstructured.Unstructured)
			gvr := getGVR(t, fakeMapper, u)
			err := fakeClient.Tracker().Create(gvr, u, u.GetNamespace())
			require.NoError(t, err)
		}
		resourceList := getResourceListFromRuntimeObjs(t, c, objs)

		err := sw.Wait(resourceList, time.Second*3)
		// Should succeed because method context is used and it's not cancelled
		assert.NoError(t, err)
	})

	t.Run("method-specific context overrides general context for WaitWithJobs", func(t *testing.T) {
		t.Parallel()
		c := newTestClient(t)
		fakeClient := dynamicfake.NewSimpleDynamicClient(scheme.Scheme)
		fakeMapper := testutil.NewFakeRESTMapper(
			batchv1.SchemeGroupVersion.WithKind("Job"),
		)

		// General context is cancelled, but method context is not
		generalCtx, generalCancel := context.WithCancel(context.Background())
		generalCancel()

		sw := statusWaiter{
			client:          fakeClient,
			restMapper:      fakeMapper,
			ctx:             generalCtx,           // Cancelled
			waitWithJobsCtx: context.Background(), // Not cancelled - should be used
		}

		objs := getRuntimeObjFromManifests(t, []string{jobCompleteManifest})
		for _, obj := range objs {
			u := obj.(*unstructured.Unstructured)
			gvr := getGVR(t, fakeMapper, u)
			err := fakeClient.Tracker().Create(gvr, u, u.GetNamespace())
			require.NoError(t, err)
		}
		resourceList := getResourceListFromRuntimeObjs(t, c, objs)

		err := sw.WaitWithJobs(resourceList, time.Second*3)
		// Should succeed because method context is used and it's not cancelled
		assert.NoError(t, err)
	})

	t.Run("method-specific context overrides general context for WaitForDelete", func(t *testing.T) {
		t.Parallel()
		c := newTestClient(t)
		fakeClient := dynamicfake.NewSimpleDynamicClient(scheme.Scheme)
		fakeMapper := testutil.NewFakeRESTMapper(
			v1.SchemeGroupVersion.WithKind("Pod"),
		)

		// General context is cancelled, but method context is not
		generalCtx, generalCancel := context.WithCancel(context.Background())
		generalCancel()

		sw := statusWaiter{
			client:           fakeClient,
			restMapper:       fakeMapper,
			ctx:              generalCtx,           // Cancelled
			waitForDeleteCtx: context.Background(), // Not cancelled - should be used
		}

		// Use a non-existent resource: WaitForDelete should return immediately since
		// the pod is already in the desired "deleted" state.
		// This also validates context selection: if generalCtx (cancelled) were
		// incorrectly used instead of waitForDeleteCtx, the watch context would be
		// immediately cancelled and the call would return a context error.
		objs := getRuntimeObjFromManifests(t, []string{podCurrentManifest})
		resourceList := getResourceListFromRuntimeObjs(t, c, objs)
		err := sw.WaitForDelete(resourceList, time.Second)
		// Should succeed because method context is used and it's not cancelled
		assert.NoError(t, err)
	})
}

func TestWatchUntilReadyWithCustomReaders(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		objManifests  []string
		customReader  *mockStatusReader
		expectErrStrs []string
	}{
		{
			name:         "custom reader makes job immediately current for hooks",
			objManifests: []string{jobNoStatusManifest},
			customReader: &mockStatusReader{
				supportedGK: batchv1.SchemeGroupVersion.WithKind("Job").GroupKind(),
				status:      status.CurrentStatus,
			},
		},
		{
			name:         "custom reader makes pod immediately current for hooks",
			objManifests: []string{podCurrentManifest},
			customReader: &mockStatusReader{
				supportedGK: v1.SchemeGroupVersion.WithKind("Pod").GroupKind(),
				status:      status.CurrentStatus,
			},
		},
		{
			name:         "custom reader takes precedence over built-in pod reader",
			objManifests: []string{podCompleteManifest},
			customReader: &mockStatusReader{
				supportedGK: v1.SchemeGroupVersion.WithKind("Pod").GroupKind(),
				status:      status.InProgressStatus,
			},
			expectErrStrs: []string{"resource Pod/ns/good-pod not ready. status: InProgress", "context deadline exceeded"},
		},
		{
			name:         "custom reader takes precedence over built-in job reader",
			objManifests: []string{jobCompleteManifest},
			customReader: &mockStatusReader{
				supportedGK: batchv1.SchemeGroupVersion.WithKind("Job").GroupKind(),
				status:      status.InProgressStatus,
			},
			expectErrStrs: []string{"resource Job/qual/test not ready. status: InProgress", "context deadline exceeded"},
		},
		{
			name:         "custom reader for different resource type does not affect pods",
			objManifests: []string{podCompleteManifest},
			customReader: &mockStatusReader{
				supportedGK: batchv1.SchemeGroupVersion.WithKind("Job").GroupKind(),
				status:      status.InProgressStatus,
			},
		},
		{
			name:         "built-in readers still work when custom reader does not match",
			objManifests: []string{jobCompleteManifest},
			customReader: &mockStatusReader{
				supportedGK: v1.SchemeGroupVersion.WithKind("Pod").GroupKind(),
				status:      status.InProgressStatus,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c := newTestClient(t)
			fakeClient := dynamicfake.NewSimpleDynamicClient(scheme.Scheme)
			fakeMapper := testutil.NewFakeRESTMapper(
				v1.SchemeGroupVersion.WithKind("Pod"),
				batchv1.SchemeGroupVersion.WithKind("Job"),
			)
			statusWaiter := statusWaiter{
				client:     fakeClient,
				restMapper: fakeMapper,
				readers:    []engine.StatusReader{tt.customReader},
			}
			objs := getRuntimeObjFromManifests(t, tt.objManifests)
			for _, obj := range objs {
				u := obj.(*unstructured.Unstructured)
				gvr := getGVR(t, fakeMapper, u)
				err := fakeClient.Tracker().Create(gvr, u, u.GetNamespace())
				require.NoError(t, err)
			}
			resourceList := getResourceListFromRuntimeObjs(t, c, objs)
			err := statusWaiter.WatchUntilReady(resourceList, time.Second*3)
			if tt.expectErrStrs != nil {
				require.Error(t, err)
				for _, expectedErrStr := range tt.expectErrStrs {
					require.ErrorContains(t, err, expectedErrStr)
				}
				return
			}
			assert.NoError(t, err)
		})
	}
}
