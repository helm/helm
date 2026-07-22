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
	"log/slog"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fluxcd/cli-utils/pkg/kstatus/polling/aggregator"
	"github.com/fluxcd/cli-utils/pkg/kstatus/polling/collector"
	"github.com/fluxcd/cli-utils/pkg/kstatus/polling/engine"
	"github.com/fluxcd/cli-utils/pkg/kstatus/polling/event"
	"github.com/fluxcd/cli-utils/pkg/kstatus/polling/statusreaders"
	"github.com/fluxcd/cli-utils/pkg/kstatus/status"
	"github.com/fluxcd/cli-utils/pkg/kstatus/watcher"
	"github.com/fluxcd/cli-utils/pkg/object"
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/fields"
	utilnet "k8s.io/apimachinery/pkg/util/net"
	"k8s.io/client-go/dynamic"
	watchtools "k8s.io/client-go/tools/watch"

	"helm.sh/helm/v4/internal/logging"
	helmStatusReaders "helm.sh/helm/v4/internal/statusreaders"
)

type statusWaiter struct {
	client               dynamic.Interface
	restMapper           meta.RESTMapper
	ctx                  context.Context
	watchUntilReadyCtx   context.Context
	waitCtx              context.Context
	waitWithJobsCtx      context.Context
	waitForDeleteCtx     context.Context
	readers              []engine.StatusReader
	statusComputeWorkers int
	logging.LogHolder
}

// DefaultStatusWatcherTimeout is the timeout used by the status waiter when a
// zero timeout is provided. This prevents callers from accidentally passing a
// zero value (which would immediately cancel the context) and getting
// "context deadline exceeded" errors. SDK callers can rely on this default
// when they don't set a timeout.
var DefaultStatusWatcherTimeout = 30 * time.Second

func alwaysReady(_ *unstructured.Unstructured) (*status.Result, error) {
	return &status.Result{
		Status:  status.CurrentStatus,
		Message: "Resource is current",
	}, nil
}

func getStatusWatcher(dynamicClient dynamic.Interface, mapper meta.RESTMapper) *watcher.DefaultStatusWatcher {
	sw := watcher.NewDefaultStatusWatcher(dynamicClient, mapper)
	sw.ResyncPeriod = 3 * time.Minute
	return sw
}

func (w *statusWaiter) WatchUntilReady(resourceList ResourceList, timeout time.Duration) error {
	if timeout == 0 {
		timeout = DefaultStatusWatcherTimeout
	}
	ctx, cancel := w.contextWithTimeout(w.watchUntilReadyCtx, timeout)
	defer cancel()
	w.Logger().Debug("waiting for resources", "count", len(resourceList), "timeout", timeout)
	sw := getStatusWatcher(w.client, w.restMapper)
	sw.StatusComputeWorkers = w.statusComputeWorkers
	jobSR := helmStatusReaders.NewCustomJobStatusReader(w.restMapper)
	podSR := helmStatusReaders.NewCustomPodStatusReader(w.restMapper)
	// We don't want to wait on any other resources as watchUntilReady is only for Helm hooks.
	// If custom readers are defined they can be used as Helm hooks support any resource.
	// We put them in front since the DelegatingStatusReader uses the first reader that matches.
	genericSR := statusreaders.NewGenericStatusReader(w.restMapper, alwaysReady)

	sr := &statusreaders.DelegatingStatusReader{
		StatusReaders: append(w.readers, jobSR, podSR, genericSR),
	}
	sw.StatusReader = sr
	return w.wait(ctx, resourceList, sw)
}

func (w *statusWaiter) Wait(resourceList ResourceList, timeout time.Duration) error {
	if timeout == 0 {
		timeout = DefaultStatusWatcherTimeout
	}
	ctx, cancel := w.contextWithTimeout(w.waitCtx, timeout)
	defer cancel()
	w.Logger().Debug("waiting for resources", "count", len(resourceList), "timeout", timeout)
	sw := getStatusWatcher(w.client, w.restMapper)
	sw.StatusComputeWorkers = w.statusComputeWorkers
	sw.StatusReader = statusreaders.NewStatusReader(w.restMapper, w.readers...)
	return w.wait(ctx, resourceList, sw)
}

func (w *statusWaiter) WaitWithJobs(resourceList ResourceList, timeout time.Duration) error {
	if timeout == 0 {
		timeout = DefaultStatusWatcherTimeout
	}
	ctx, cancel := w.contextWithTimeout(w.waitWithJobsCtx, timeout)
	defer cancel()
	w.Logger().Debug("waiting for resources", "count", len(resourceList), "timeout", timeout)
	sw := getStatusWatcher(w.client, w.restMapper)
	sw.StatusComputeWorkers = w.statusComputeWorkers
	newCustomJobStatusReader := helmStatusReaders.NewCustomJobStatusReader(w.restMapper)
	readers := append([]engine.StatusReader(nil), w.readers...)
	readers = append(readers, newCustomJobStatusReader)
	customSR := statusreaders.NewStatusReader(w.restMapper, readers...)
	sw.StatusReader = customSR
	return w.wait(ctx, resourceList, sw)
}

func (w *statusWaiter) WaitForDelete(resourceList ResourceList, timeout time.Duration) error {
	if timeout == 0 {
		timeout = DefaultStatusWatcherTimeout
	}
	ctx, cancel := w.contextWithTimeout(w.waitForDeleteCtx, timeout)
	defer cancel()
	w.Logger().Debug("waiting for resources to be deleted", "count", len(resourceList), "timeout", timeout)
	sw := getStatusWatcher(w.client, w.restMapper)
	return w.waitForDelete(ctx, resourceList, sw)
}

// errWatchEndedBeforeConfirmation is returned when the watcher stops (for example
// its stream closed) while the parent context is still live and one or more delete
// targets were neither observed deleted by the watcher nor confirmed absent by a
// live check. The wait fails closed rather than reporting an unverified success.
var errWatchEndedBeforeConfirmation = errors.New("watch ended before all resource deletions were confirmed")

func objMetaString(id object.ObjMetadata) string {
	return fmt.Sprintf("%s/%s/%s", id.GroupKind.Kind, id.Namespace, id.Name)
}

func (w *statusWaiter) waitForDelete(ctx context.Context, resourceList ResourceList, sw watcher.StatusWatcher) error {
	cancelCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// issue #32261: the delete path must not treat a target's transient Unknown status
	// as "already deleted". During informer sync every target is briefly Unknown; the
	// old observer skipped Unknown resources, so an all-Unknown set aggregated to
	// NotFound and cancelled the watch before any deletion was confirmed (the flake).
	// Simply not skipping Unknown would instead hang an already-gone resource forever,
	// because the watcher only reports NotFound from an observed delete event and never
	// emits one for an object absent from its initial LIST. So the watcher starts first
	// and, once it has synced, a bounded pool of per-target checker goroutines confirms
	// any still-Unknown target with a live LIST -- authoritative for both the gone and
	// the still-present case, and isolated so one slow target cannot block another.
	resources := []object.ObjMetadata{}
	for _, resource := range resourceList {
		obj, err := object.RuntimeToObjMeta(resource.Object)
		if err != nil {
			return err
		}
		resources = append(resources, obj)
	}
	if len(resources) == 0 {
		return nil
	}

	eventCh := sw.Watch(cancelCtx, resources, watcher.Options{
		RESTScopeStrategy: watcher.RESTScopeNamespace,
	})
	statusCollector := collector.NewResourceStatusCollector(resources)

	rec := newDeleteReconciler(w, resources, cancel)

	observer := func(sc *collector.ResourceStatusCollector, e event.Event) {
		done := rec.observe(sc)
		if e.Type == event.SyncEvent {
			rec.onSync(cancelCtx)
		}
		if done {
			cancel()
		}
	}
	done := statusCollector.ListenWithObserver(eventCh, collector.ObserverFunc(observer))
	<-done
	// The watcher has stopped. Cancel so every in-flight check unwinds, then wait for
	// all checker goroutines to return so nothing writes the reconciler's state while
	// result reads it.
	cancel()
	rec.wait()

	if n := rec.liveChecks.Load(); n > 0 {
		w.Logger().Debug("delete reconcile issued live existence checks", "listCalls", n, "targets", len(resources))
	}
	return rec.result(ctx, statusCollector)
}

// deleteReconciler owns the per-WaitForDelete bookkeeping for confirming that a set
// of resources has been deleted. The cli-utils watcher drives statuses through the
// observer callback (which performs no API I/O); after the first Sync, one bounded
// pool of per-target checker goroutines performs the post-sync existence LISTs, so a
// slow or stuck check for one target cannot block another and each target retries on
// its own backoff. Every field below mu is guarded by it, and mu is never held across
// a LIST or a channel operation.
type deleteReconciler struct {
	w         *statusWaiter
	resources []object.ObjMetadata
	cancel    context.CancelFunc

	// sem bounds how many existence LISTs run concurrently.
	sem chan struct{}
	// launch starts the per-target checkers exactly once, on the first Sync.
	launch sync.Once
	// wg tracks the per-target checker goroutines so waitForDelete can join them.
	wg sync.WaitGroup

	// liveChecks counts existence LISTs issued (for operational visibility into the
	// extra request cost). Several checker goroutines write it concurrently, so it is
	// atomic.
	liveChecks atomic.Int64

	mu            sync.Mutex
	statuses      map[object.ObjMetadata]status.Status      // latest watcher status per target
	confirmedGone map[object.ObjMetadata]struct{}           // targets a live LIST confirmed absent
	reconcileErr  error                                     // first permanent reconcile failure
	targetCancel  map[object.ObjMetadata]context.CancelFunc // cancels a target's in-flight check when the watcher makes it terminal
}

func newDeleteReconciler(w *statusWaiter, resources []object.ObjMetadata, cancel context.CancelFunc) *deleteReconciler {
	r := &deleteReconciler{
		w:             w,
		resources:     resources,
		cancel:        cancel,
		sem:           make(chan struct{}, reconcileMaxConcurrentChecks),
		statuses:      make(map[object.ObjMetadata]status.Status, len(resources)),
		confirmedGone: make(map[object.ObjMetadata]struct{}, len(resources)),
		targetCancel:  make(map[object.ObjMetadata]context.CancelFunc, len(resources)),
	}
	for _, id := range resources {
		r.statuses[id] = status.UnknownStatus
	}
	return r
}

// observe records the collector's latest per-target status. It is called only from
// the ListenWithObserver goroutine, which updates the collector before invoking the
// observer, so reading sc here does not race. When the watcher supplies a definitive
// status for a target, any in-flight existence check for it is cancelled so it stops
// retrying and frees its concurrency slot. It returns whether every target is now
// terminal (observed NotFound or confirmed gone).
func (r *deleteReconciler) observe(sc *collector.ResourceStatusCollector) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, id := range r.resources {
		rs := sc.ResourceStatuses[id]
		if rs == nil {
			continue
		}
		r.statuses[id] = rs.Status
		if rs.Status != status.UnknownStatus {
			if tc := r.targetCancel[id]; tc != nil {
				tc()
				delete(r.targetCancel, id)
			}
		}
	}
	return r.completeLocked()
}

func (r *deleteReconciler) completeLocked() bool {
	for _, id := range r.resources {
		if r.statuses[id] == status.NotFoundStatus {
			continue
		}
		if _, ok := r.confirmedGone[id]; ok {
			continue
		}
		return false
	}
	return true
}

func (r *deleteReconciler) complete() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.completeLocked()
}

// pending returns the targets still Unknown and not yet confirmed gone.
func (r *deleteReconciler) pending() []object.ObjMetadata {
	r.mu.Lock()
	defer r.mu.Unlock()
	var todo []object.ObjMetadata
	for _, id := range r.resources {
		if _, ok := r.confirmedGone[id]; ok {
			continue
		}
		if r.statuses[id] != status.UnknownStatus {
			continue
		}
		todo = append(todo, id)
	}
	return todo
}

// stillUnknown reports whether a target still needs a live check: not yet confirmed
// gone and no watcher status has arrived for it.
func (r *deleteReconciler) stillUnknown(id object.ObjMetadata) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.confirmedGone[id]; ok {
		return false
	}
	return r.statuses[id] == status.UnknownStatus
}

func (r *deleteReconciler) markGone(id object.ObjMetadata) {
	r.mu.Lock()
	r.confirmedGone[id] = struct{}{}
	r.mu.Unlock()
}

func (r *deleteReconciler) failClosed(id object.ObjMetadata, err error) {
	r.mu.Lock()
	if r.reconcileErr == nil {
		r.reconcileErr = fmt.Errorf("confirming deletion of resource %s: %w", objMetaString(id), err)
	}
	r.mu.Unlock()
}

// onSync launches one checker goroutine per still-Unknown target, exactly once, when
// the watcher first syncs. Targets the watcher already gave a status (present, or
// deleted) are left to the watch and get no checker.
func (r *deleteReconciler) onSync(ctx context.Context) {
	r.launch.Do(func() {
		if ctx.Err() != nil {
			return // the wait already ended (e.g. a watch error before sync); nothing to reconcile
		}
		for _, id := range r.pending() {
			tctx, tcancel := context.WithCancel(ctx)
			r.mu.Lock()
			r.targetCancel[id] = tcancel
			r.mu.Unlock()
			r.wg.Add(1)
			go r.check(tctx, tcancel, id)
		}
	})
}

// wait blocks until every checker goroutine has returned, so result can read the
// reconciler's state without racing a live check.
func (r *deleteReconciler) wait() {
	r.wg.Wait()
}

// check is one target's independent reconcile loop. It confirms the target gone with a
// live LIST, retries transient failures on its own backoff (honoring a bounded server
// Retry-After for this target alone, so one target's throttling never defers another),
// fails the whole wait closed on a permanent or discovery error, and returns as soon
// as the target is confirmed gone, found present (left to the watch), or its check is
// cancelled (the watcher resolved it, or the wait ended). Retries are bounded only by
// ctx, so no arbitrary per-request timeout is imposed on a slow aggregated API.
func (r *deleteReconciler) check(ctx context.Context, cancel context.CancelFunc, id object.ObjMetadata) {
	defer r.wg.Done()
	defer cancel()
	backoff := reconcileMinBackoff
	for {
		if !r.stillUnknown(id) {
			return // the watch supplied a status, or it is already confirmed
		}
		gone, err := r.checkOnce(ctx, id)
		switch {
		case err == nil:
			if gone {
				r.markGone(id)
				r.w.Logger().Debug("resource confirmed deleted by live check", "kind", id.GroupKind.Kind, "namespace", id.Namespace, "name", id.Name)
				r.checkComplete()
			}
			return // gone, or present (rely on the watch)
		case ctx.Err() != nil:
			return // this target's check was cancelled, or the wait ended
		case retriableReconcileError(err):
			select {
			case <-ctx.Done():
				return
			case <-time.After(reconcileWait(backoff, serverSuggestedDelay(err))):
			}
			backoff *= 2
			if backoff > reconcileMaxBackoff {
				backoff = reconcileMaxBackoff
			}
		case meta.IsNoMatchError(err):
			// The resource type is no longer served (e.g. its CRD was removed
			// mid-wait) and the REST mapper cannot resolve it. Fail closed rather
			// than assume every instance is gone or retry a discovery gap forever.
			r.failClosed(id, err)
			r.cancel()
			return
		default:
			// Permanent or malformed request error (Forbidden, Unauthorized,
			// BadRequest, Invalid, MethodNotSupported, a field-selector-not-
			// honored error, ...): fail closed with the original error.
			r.failClosed(id, err)
			r.cancel()
			return
		}
	}
}

// checkOnce runs a single existence LIST for one target under the concurrency bound.
func (r *deleteReconciler) checkOnce(ctx context.Context, id object.ObjMetadata) (bool, error) {
	select {
	case r.sem <- struct{}{}:
	case <-ctx.Done():
		return false, ctx.Err()
	}
	defer func() { <-r.sem }()
	r.liveChecks.Add(1)
	return r.w.resourceGone(ctx, id)
}

// checkComplete cancels the wait once every target is terminal (observed NotFound or
// confirmed gone), so a checker that confirms the last target ends the wait.
func (r *deleteReconciler) checkComplete() {
	if r.complete() {
		r.cancel()
	}
}

// result assembles the final error after the watcher and all checker goroutines have
// stopped. Precedence: a permanent reconcile error; then a context error (returned
// alone, never joined with the watch-ended error); then "still exists" diagnostics
// for targets the watcher observed as present; then the watch-ended-before-
// confirmation error.
func (r *deleteReconciler) result(ctx context.Context, sc *collector.ResourceStatusCollector) error {
	r.mu.Lock()
	rErr := r.reconcileErr
	r.mu.Unlock()
	if rErr != nil {
		return rErr
	}
	if sc.Error != nil {
		return sc.Error
	}

	var stillExists []error
	var unconfirmed []object.ObjMetadata
	r.mu.Lock()
	for _, id := range r.resources {
		if _, ok := r.confirmedGone[id]; ok {
			continue
		}
		rs := sc.ResourceStatuses[id]
		switch {
		case rs != nil && rs.Status == status.NotFoundStatus:
			// deletion observed by the watcher
		case rs == nil || rs.Status == status.UnknownStatus:
			unconfirmed = append(unconfirmed, id)
		default:
			stillExists = append(stillExists, fmt.Errorf("resource %s/%s/%s still exists. status: %s, message: %s",
				rs.Identifier.GroupKind.Kind, rs.Identifier.Namespace, rs.Identifier.Name, rs.Status, rs.Message))
		}
	}
	r.mu.Unlock()

	// Every target is confirmed gone by a live check or observed NotFound: the
	// deletion succeeded. Report success even if the parent context has since
	// expired -- a deadline that fires as the last confirmation lands must not turn
	// a completed deletion into a failure.
	if len(stillExists) == 0 && len(unconfirmed) == 0 {
		return nil
	}

	// A context error (deadline or cancellation) is the real cause and is returned
	// on its own, never joined with the watch-ended error. "still exists"
	// diagnostics remain informative and accompany a context error.
	if ctxErr := ctx.Err(); ctxErr != nil {
		if len(stillExists) > 0 {
			return errors.Join(append(stillExists, ctxErr)...)
		}
		return ctxErr
	}
	if len(stillExists) > 0 {
		return errors.Join(stillExists...)
	}
	if len(unconfirmed) > 0 {
		parts := make([]error, 0, len(unconfirmed)+1)
		parts = append(parts, errWatchEndedBeforeConfirmation)
		for _, id := range unconfirmed {
			parts = append(parts, fmt.Errorf("resource %s deletion unconfirmed", objMetaString(id)))
		}
		return errors.Join(parts...)
	}
	return nil
}

// resourceGone reports whether the resource is absent from the cluster, confirmed
// with a filtered collection LIST (metadata.name field selector) rather than a
// point GET. A LIST is authorized as the `list` verb -- already required by the
// informer that drives the watch -- so this preserves the pre-existing RBAC
// surface, whereas a GET would additionally require `get`. A missing object yields
// an empty list (no NotFound error), so absence is len(items) == 0. The response is
// validated defensively: a returned object with a different name, or more than one
// object, means the server or client did not honor the field selector and is
// surfaced as an error rather than misread as present/absent.
func (w *statusWaiter) resourceGone(ctx context.Context, id object.ObjMetadata) (bool, error) {
	mapping, err := w.restMapper.RESTMapping(id.GroupKind)
	if err != nil {
		return false, err
	}
	var ri dynamic.ResourceInterface
	if mapping.Scope.Name() == meta.RESTScopeNameNamespace {
		ri = w.client.Resource(mapping.Resource).Namespace(id.Namespace)
	} else {
		ri = w.client.Resource(mapping.Resource)
	}
	// An exact metadata.name selector matches at most one object within the GVR and
	// namespace, so no Limit/pagination is needed against a conforming API server.
	list, err := ri.List(ctx, metav1.ListOptions{
		FieldSelector: fields.OneTermEqualSelector("metadata.name", id.Name).String(),
	})
	if err != nil {
		return false, err
	}
	switch len(list.Items) {
	case 0:
		return true, nil
	case 1:
		if name := list.Items[0].GetName(); name != id.Name {
			return false, fmt.Errorf("existence check for %s/%s/%s returned a different object %q: the API server or client did not honor the metadata.name field selector",
				id.GroupKind.Kind, id.Namespace, id.Name, name)
		}
		return false, nil
	default:
		return false, fmt.Errorf("existence check for %s/%s/%s returned %d objects: the API server or client did not honor the metadata.name field selector",
			id.GroupKind.Kind, id.Namespace, id.Name, len(list.Items))
	}
}

// retriableReconcileError reports whether a reconciliation existence-check error is
// a transient failure worth retrying: throttling, API/server timeouts, service
// unavailability, and recognized temporary transport failures (connection
// refused/reset, a lost HTTP/2 client connection, a probable EOF, and net-level
// timeouts -- the same transport classes the watcher itself reconnects on). It is an
// explicit allowlist; every other error -- permanent request errors (Forbidden,
// Unauthorized, BadRequest, Invalid, MethodNotSupported), a 500 InternalError,
// REST-mapping NoMatch, and anything unrecognized -- is surfaced immediately rather
// than retried. The existence check is a read-only LIST, so retrying is always safe.
func retriableReconcileError(err error) bool {
	return apierrors.IsTooManyRequests(err) ||
		apierrors.IsServerTimeout(err) ||
		apierrors.IsTimeout(err) ||
		apierrors.IsServiceUnavailable(err) ||
		utilnet.IsConnectionRefused(err) ||
		utilnet.IsConnectionReset(err) ||
		utilnet.IsHTTP2ConnectionLost(err) ||
		utilnet.IsProbableEOF(err) ||
		utilnet.IsTimeout(err)
}

// serverSuggestedDelay returns the Retry-After the API server requested for a
// retriable existence-check error, or 0 when the error carries no such hint.
// Honoring it keeps the reconcile from hammering a server that has explicitly asked
// the client to back off (a 429 or a ServerTimeout carrying retryAfterSeconds).
func serverSuggestedDelay(err error) time.Duration {
	if secs, ok := apierrors.SuggestsClientDelay(err); ok && secs > 0 {
		return time.Duration(secs) * time.Second
	}
	return 0
}

const (
	reconcileMinBackoff = 10 * time.Millisecond
	reconcileMaxBackoff = 500 * time.Millisecond
	// reconcileMaxServerDelay caps how long a single server-suggested Retry-After may
	// stall one target's retry loop. Realistic hints (typically 1-2s) are honored in
	// full so the client stays polite under throttling, but a pathological hint must
	// not consume the whole wait budget: a reconcile-only target (absent from the
	// watcher's initial LIST) is confirmed deleted solely by these LISTs, and the
	// watcher will never emit its deletion, so the target has to keep re-checking.
	reconcileMaxServerDelay = 5 * time.Second
	// reconcileMaxConcurrentChecks bounds how many existence LISTs run at once. It caps
	// the connection/goroutine fan-out for a large release while staying above one, so
	// a slow or stuck check for one target does not block another target's check.
	reconcileMaxConcurrentChecks = 8
)

// reconcileWait returns how long a target waits before its next existence check: the
// capped exponential backoff, raised to a server-suggested Retry-After but never
// beyond reconcileMaxServerDelay. The delay is per target, so one target's Retry-After
// never defers another target's retry.
func reconcileWait(backoff, serverDelay time.Duration) time.Duration {
	if serverDelay > reconcileMaxServerDelay {
		serverDelay = reconcileMaxServerDelay
	}
	if serverDelay > backoff {
		return serverDelay
	}
	return backoff
}

func (w *statusWaiter) wait(ctx context.Context, resourceList ResourceList, sw watcher.StatusWatcher) error {
	cancelCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	resources := []object.ObjMetadata{}
	for _, resource := range resourceList {
		if value, ok := AsVersioned(resource).(*appsv1.Deployment); ok && value.Spec.Paused {
			continue
		}
		obj, err := object.RuntimeToObjMeta(resource.Object)
		if err != nil {
			return err
		}
		resources = append(resources, obj)
	}

	eventCh := sw.Watch(cancelCtx, resources, watcher.Options{
		RESTScopeStrategy: watcher.RESTScopeNamespace,
	})
	statusCollector := collector.NewResourceStatusCollector(resources)
	done := statusCollector.ListenWithObserver(eventCh, statusObserver(cancel, status.CurrentStatus, w.Logger()))
	<-done

	if statusCollector.Error != nil {
		return statusCollector.Error
	}

	errs := []error{}
	for _, id := range resources {
		rs := statusCollector.ResourceStatuses[id]
		if rs.Status == status.CurrentStatus {
			continue
		}
		errs = append(errs, fmt.Errorf("resource %s/%s/%s not ready. status: %s, message: %s",
			rs.Identifier.GroupKind.Kind, rs.Identifier.Namespace, rs.Identifier.Name, rs.Status, rs.Message))
	}
	if err := ctx.Err(); err != nil {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

func (w *statusWaiter) contextWithTimeout(methodCtx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if methodCtx == nil {
		methodCtx = w.ctx
	}
	return contextWithTimeout(methodCtx, timeout)
}

func contextWithTimeout(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	return watchtools.ContextWithOptionalTimeout(ctx, timeout)
}

func statusObserver(cancel context.CancelFunc, desired status.Status, logger *slog.Logger) collector.ObserverFunc {
	return func(statusCollector *collector.ResourceStatusCollector, _ event.Event) {
		var rss []*event.ResourceStatus
		var nonDesiredResources []*event.ResourceStatus
		for _, rs := range statusCollector.ResourceStatuses {
			if rs == nil {
				continue
			}
			// Failed is a terminal state. This check ensures we don't wait forever for a resource
			// that has already failed, as intervention is required to resolve the failure.
			if rs.Status == status.FailedStatus && desired == status.CurrentStatus {
				continue
			}
			rss = append(rss, rs)
			if rs.Status != desired {
				nonDesiredResources = append(nonDesiredResources, rs)
			}
		}

		if aggregator.AggregateStatus(rss, desired) == desired {
			logger.Debug("all resources achieved desired status", "desiredStatus", desired, "resourceCount", len(rss))
			cancel()
			return
		}

		if len(nonDesiredResources) > 0 {
			// Log a single resource so the user knows what they're waiting for without an overwhelming amount of output
			sort.Slice(nonDesiredResources, func(i, j int) bool {
				return nonDesiredResources[i].Identifier.Name < nonDesiredResources[j].Identifier.Name
			})
			first := nonDesiredResources[0]
			logger.Debug("waiting for resource", "namespace", first.Identifier.Namespace, "name", first.Identifier.Name, "kind", first.Identifier.GroupKind.Kind, "expectedStatus", desired, "actualStatus", first.Status)
		}
	}
}

type hookOnlyWaiter struct {
	sw *statusWaiter
}

func (w *hookOnlyWaiter) WatchUntilReady(resourceList ResourceList, timeout time.Duration) error {
	return w.sw.WatchUntilReady(resourceList, timeout)
}

func (w *hookOnlyWaiter) Wait(_ ResourceList, _ time.Duration) error {
	return nil
}

func (w *hookOnlyWaiter) WaitWithJobs(_ ResourceList, _ time.Duration) error {
	return nil
}

func (w *hookOnlyWaiter) WaitForDelete(_ ResourceList, _ time.Duration) error {
	return nil
}
