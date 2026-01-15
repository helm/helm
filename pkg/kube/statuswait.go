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

package kube // import "helm.sh/helm/v3/pkg/kube"

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
	"k8s.io/client-go/dynamic"
	watchtools "k8s.io/client-go/tools/watch"

	helmStatusReaders "helm.sh/helm/v4/internal/statusreaders"
)

type statusWaiter struct {
	client             dynamic.Interface
	restMapper         meta.RESTMapper
	ctx                context.Context
	watchUntilReadyCtx context.Context
	waitCtx            context.Context
	waitWithJobsCtx    context.Context
	waitForDeleteCtx   context.Context
	readers            []engine.StatusReader
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

func (w *statusWaiter) WatchUntilReady(resourceList ResourceList, timeout time.Duration) error {
	if timeout == 0 {
		timeout = DefaultStatusWatcherTimeout
	}
	ctx, cancel := w.contextWithTimeout(w.watchUntilReadyCtx, timeout)
	defer cancel()
	slog.Debug("waiting for resources", "count", len(resourceList), "timeout", timeout)
	sw := watcher.NewDefaultStatusWatcher(w.client, w.restMapper)
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
	slog.Debug("waiting for resources", "count", len(resourceList), "timeout", timeout)
	sw := watcher.NewDefaultStatusWatcher(w.client, w.restMapper)
	sw.StatusReader = statusreaders.NewStatusReader(w.restMapper, w.readers...)
	return w.wait(ctx, resourceList, sw)
}

func (w *statusWaiter) WaitWithJobs(resourceList ResourceList, timeout time.Duration) error {
	if timeout == 0 {
		timeout = DefaultStatusWatcherTimeout
	}
	ctx, cancel := w.contextWithTimeout(w.waitWithJobsCtx, timeout)
	defer cancel()
	slog.Debug("waiting for resources", "count", len(resourceList), "timeout", timeout)
	sw := watcher.NewDefaultStatusWatcher(w.client, w.restMapper)
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
	slog.Debug("waiting for resources to be deleted", "count", len(resourceList), "timeout", timeout)
	sw := watcher.NewDefaultStatusWatcher(w.client, w.restMapper)
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
	done := statusCollector.ListenWithObserver(eventCh, statusObserver(cancel, status.NotFoundStatus))
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

func (w *statusWaiter) wait(ctx context.Context, resourceList ResourceList, sw watcher.StatusWatcher) error {
	cancelCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	resources := []object.ObjMetadata{}
	for _, resource := range resourceList {
		switch value := AsVersioned(resource).(type) {
		case *appsv1.Deployment:
			if value.Spec.Paused {
				continue
			}
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
	done := statusCollector.ListenWithObserver(eventCh, statusObserver(cancel, status.CurrentStatus))
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

func statusObserver(cancel context.CancelFunc, desired status.Status) collector.ObserverFunc {
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
			slog.Debug("all resources achieved desired status", "desiredStatus", desired, "resourceCount", len(rss))
			cancel()
			return
		}

		if len(nonDesiredResources) > 0 {
			// Log a single resource so the user knows what they're waiting for without an overwhelming amount of output
			sort.Slice(nonDesiredResources, func(i, j int) bool {
				return nonDesiredResources[i].Identifier.Name < nonDesiredResources[j].Identifier.Name
			})
			first := nonDesiredResources[0]
			slog.Debug("waiting for resource", "namespace", first.Identifier.Namespace, "name", first.Identifier.Name, "kind", first.Identifier.GroupKind.Kind, "expectedStatus", desired, "actualStatus", first.Status)
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
