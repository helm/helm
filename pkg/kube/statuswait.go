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

	helmStatusReaders "helm.sh/helm/v4/internal/statusreaders"
)

type statusWaiter struct {
	client     dynamic.Interface
	restMapper meta.RESTMapper
}

func alwaysReady(_ *unstructured.Unstructured) (*status.Result, error) {
	return &status.Result{
		Status:  status.CurrentStatus,
		Message: "Resource is current",
	}, nil
}

func (w *statusWaiter) WatchUntilReady(resourceList ResourceList, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	slog.Debug("waiting for resources", "count", len(resourceList), "timeout", timeout)
	sw := watcher.NewDefaultStatusWatcher(w.client, w.restMapper)
	jobSR := helmStatusReaders.NewCustomJobStatusReader(w.restMapper)
	podSR := helmStatusReaders.NewCustomPodStatusReader(w.restMapper)
	// We don't want to wait on any other resources as watchUntilReady is only for Helm hooks
	genericSR := statusreaders.NewGenericStatusReader(w.restMapper, alwaysReady)

	sr := &statusreaders.DelegatingStatusReader{
		StatusReaders: []engine.StatusReader{
			jobSR,
			podSR,
			genericSR,
		},
	}
	sw.StatusReader = sr
	return w.wait(ctx, resourceList, sw)
}

func (w *statusWaiter) Wait(resourceList ResourceList, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.TODO(), timeout)
	defer cancel()
	slog.Debug("waiting for resources", "count", len(resourceList), "timeout", timeout)
	sw := watcher.NewDefaultStatusWatcher(w.client, w.restMapper)
	return w.wait(ctx, resourceList, sw)
}

func (w *statusWaiter) WaitWithJobs(resourceList ResourceList, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.TODO(), timeout)
	defer cancel()
	slog.Debug("waiting for resources", "count", len(resourceList), "timeout", timeout)
	sw := watcher.NewDefaultStatusWatcher(w.client, w.restMapper)
	newCustomJobStatusReader := helmStatusReaders.NewCustomJobStatusReader(w.restMapper)
	customSR := statusreaders.NewStatusReader(w.restMapper, newCustomJobStatusReader)
	sw.StatusReader = customSR
	return w.wait(ctx, resourceList, sw)
}

func (w *statusWaiter) WaitForDelete(resourceList ResourceList, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.TODO(), timeout)
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
	eventCh := sw.Watch(cancelCtx, resources, watcher.Options{})
	statusCollector := collector.NewResourceStatusCollector(resources)
	done := statusCollector.ListenWithObserver(eventCh, statusObserver(cancel, status.NotFoundStatus))
	<-done

	if statusCollector.Error != nil {
		return statusCollector.Error
	}

	// Only check parent context error, otherwise we would error when desired status is achieved.
	if ctx.Err() != nil {
		errs := []error{}
		for _, id := range resources {
			rs := statusCollector.ResourceStatuses[id]
			if rs.Status == status.NotFoundStatus {
				continue
			}
			errs = append(errs, fmt.Errorf("resource still exists, name: %s, kind: %s, status: %s", rs.Identifier.Name, rs.Identifier.GroupKind.Kind, rs.Status))
		}
		errs = append(errs, ctx.Err())
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

	eventCh := sw.Watch(cancelCtx, resources, watcher.Options{})
	statusCollector := collector.NewResourceStatusCollector(resources)
	done := statusCollector.ListenWithObserver(eventCh, statusObserver(cancel, status.CurrentStatus))
	<-done

	if statusCollector.Error != nil {
		return statusCollector.Error
	}

	// Only check parent context error, otherwise we would error when desired status is achieved.
	if ctx.Err() != nil {
		errs := []error{}
		for _, id := range resources {
			rs := statusCollector.ResourceStatuses[id]
			if rs.Status == status.CurrentStatus {
				continue
			}
			errs = append(errs, fmt.Errorf("resource not ready, name: %s, kind: %s, status: %s", rs.Identifier.Name, rs.Identifier.GroupKind.Kind, rs.Status))
		}
		errs = append(errs, ctx.Err())
		return errors.Join(errs...)
	}
	return nil
}

func statusObserver(cancel context.CancelFunc, desired status.Status) collector.ObserverFunc {
	return func(statusCollector *collector.ResourceStatusCollector, _ event.Event) {
		var rss []*event.ResourceStatus
		var nonDesiredResources []*event.ResourceStatus
		for _, rs := range statusCollector.ResourceStatuses {
			if rs == nil {
				continue
			}
			// If a resource is already deleted before waiting has started, it will show as unknown
			// this check ensures we don't wait forever for a resource that is already deleted
			if rs.Status == status.UnknownStatus && desired == status.NotFoundStatus {
				continue
			}
			rss = append(rss, rs)
			if rs.Status != desired {
				nonDesiredResources = append(nonDesiredResources, rs)
			}
		}

		if aggregator.AggregateStatus(rss, desired) == desired {
			cancel()
			return
		}

		if len(nonDesiredResources) > 0 {
			// Log a single resource so the user knows what they're waiting for without an overwhelming amount of output
			sort.Slice(nonDesiredResources, func(i, j int) bool {
				return nonDesiredResources[i].Identifier.Name < nonDesiredResources[j].Identifier.Name
			})
			first := nonDesiredResources[0]
			slog.Debug("waiting for resource", "name", first.Identifier.Name, "kind", first.Identifier.GroupKind.Kind, "expectedStatus", desired, "actualStatus", first.Status)
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
