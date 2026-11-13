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
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
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

// alwaysReadyGKs is the set of namespace-scoped GroupKinds whose status is
// always Current according to the kstatus library (status.Compute returns
// Current immediately for them, regardless of object content).
// Watching a namespace-wide informer for these kinds is wasteful — especially
// for Secrets, which in a shared namespace can include hundreds of large
// Helm release records (helm.sh/release.v1).
//
// Only kinds explicitly mapped to alwaysReady in kstatus legacyTypes are
// included here.  Kinds with type-specific handlers that may return InProgress
// (Service, PVC, Deployment, …) are intentionally excluded.
var alwaysReadyGKs = map[schema.GroupKind]struct{}{
	{Group: "", Kind: "Secret"}:        {},
	{Group: "", Kind: "ConfigMap"}:     {},
	{Group: "batch", Kind: "CronJob"}: {},
}

// isAlwaysReady returns true if the given GroupKind will always be reported as
// Current by the status reader, meaning no watcher informer is needed for it.
// Custom readers take precedence: if any custom reader supports this GK, it is
// NOT considered always-ready, so the watcher is still started for it.
func isAlwaysReady(gk schema.GroupKind, customReaders []engine.StatusReader) bool {
	for _, r := range customReaders {
		if r.Supports(gk) {
			return false
		}
	}
	_, ok := alwaysReadyGKs[gk]
	return ok
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

	// For hook waiting, only Job, Pod, and custom-reader resources need a real
	// watcher: every other kind is handled by the alwaysReady genericSR and is
	// immediately Current.  Skipping the watcher for those kinds avoids starting
	// a namespace-wide informer that would download all objects of that kind
	// (e.g. all Secrets, including every Helm release record in the namespace).
	specificReaders := append(append([]engine.StatusReader(nil), w.readers...), jobSR, podSR)
	watchList, skipList := splitBySpecificReaders(resourceList, specificReaders)

	w.Logger().Debug("skipping always-ready resources for hook watch",
		"skipped", len(skipList), "watched", len(watchList))

	return w.wait(ctx, watchList, sw)
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

	// Skip namespace-wide informers for kinds whose status.Compute always
	// returns Current (Secret, ConfigMap, batch/CronJob, …).  Custom readers
	// take precedence: if one supports the kind it is NOT skipped.
	watchList, skipList := splitByWatchNeed(resourceList, w.readers)
	w.Logger().Debug("skipping always-ready resources for wait",
		"skipped", len(skipList), "watched", len(watchList))

	return w.wait(ctx, watchList, sw)
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

	// Skip namespace-wide informers for kinds that are always Current.
	// The job reader is included so Job is not skipped.
	watchList, skipList := splitByWatchNeed(resourceList, readers)
	w.Logger().Debug("skipping always-ready resources for waitWithJobs",
		"skipped", len(skipList), "watched", len(watchList))

	return w.wait(ctx, watchList, sw)
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

func (w *statusWaiter) waitForDelete(ctx context.Context, resourceList ResourceList, sw watcher.StatusWatcher) error {
	cancelCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	resources := []object.ObjMetadata{}
	for _, resource := range resourceList {
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
	done := statusCollector.ListenWithObserver(eventCh, statusObserver(cancel, status.NotFoundStatus, w.Logger()))
	<-done

	if statusCollector.Error != nil {
		return statusCollector.Error
	}

	errs := []error{}
	for _, id := range resources {
		rs := statusCollector.ResourceStatuses[id]
		if rs.Status == status.NotFoundStatus || rs.Status == status.UnknownStatus {
			continue
		}
		errs = append(errs, fmt.Errorf("resource %s/%s/%s still exists. status: %s, message: %s",
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

// splitBySpecificReaders splits a ResourceList by whether the resource's
// GroupKind is handled by one of the given specific readers.
// Resources NOT covered by any specific reader are put in skipList because they
// will be served by the alwaysReady fallback reader and do not need a watcher.
// This is used by WatchUntilReady, where the fallback is always alwaysReady.
func splitBySpecificReaders(resourceList ResourceList, specificReaders []engine.StatusReader) (watchList, skipList ResourceList) {
	for _, resource := range resourceList {
		gk := resource.Mapping.GroupVersionKind.GroupKind()
		supported := false
		for _, r := range specificReaders {
			if r.Supports(gk) {
				supported = true
				break
			}
		}
		if supported {
			watchList = append(watchList, resource)
		} else {
			skipList = append(skipList, resource)
		}
	}
	return watchList, skipList
}

// splitByWatchNeed splits a ResourceList into two: resources that need a
// real watcher informer (watchList) and resources whose kind is always Current
// and therefore do not need a namespace-wide informer (skipList).
// specificReaders are status readers that override the default always-ready
// classification: any kind they support is kept in watchList.
// This is used by Wait and WaitWithJobs, where the fallback uses status.Compute.
func splitByWatchNeed(resourceList ResourceList, specificReaders []engine.StatusReader) (watchList, skipList ResourceList) {
	for _, resource := range resourceList {
		gk := resource.Mapping.GroupVersionKind.GroupKind()
		if isAlwaysReady(gk, specificReaders) {
			skipList = append(skipList, resource)
		} else {
			watchList = append(watchList, resource)
		}
	}
	return watchList, skipList
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
			// If a resource is already deleted before waiting has started, it will show as unknown.
			// This check ensures we don't wait forever for a resource that is already deleted.
			if rs.Status == status.UnknownStatus && desired == status.NotFoundStatus {
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
