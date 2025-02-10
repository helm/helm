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
	"sort"
	"time"

	helmStatusReaders "helm.sh/helm/v4/internal/statusreaders"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
	"sigs.k8s.io/cli-utils/pkg/kstatus/polling/aggregator"
	"sigs.k8s.io/cli-utils/pkg/kstatus/polling/collector"
	"sigs.k8s.io/cli-utils/pkg/kstatus/polling/engine"
	"sigs.k8s.io/cli-utils/pkg/kstatus/polling/event"
	"sigs.k8s.io/cli-utils/pkg/kstatus/polling/statusreaders"
	"sigs.k8s.io/cli-utils/pkg/kstatus/status"
	"sigs.k8s.io/cli-utils/pkg/kstatus/watcher"
	"sigs.k8s.io/cli-utils/pkg/object"
)

type statusWaiter struct {
	client     dynamic.Interface
	restMapper meta.RESTMapper
	log        func(string, ...interface{})
}

func alwaysReady(u *unstructured.Unstructured) (*status.Result, error) {
	return &status.Result{
		Status:  status.CurrentStatus,
		Message: "Resource is current",
	}, nil
}

func (w *statusWaiter) WatchUntilReady(resourceList ResourceList, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	w.log("waiting for %d pods and jobs to complete with a timeout of %s", len(resourceList), timeout)
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
	w.log("beginning wait for %d resources with timeout of %s", len(resourceList), timeout)
	sw := watcher.NewDefaultStatusWatcher(w.client, w.restMapper)
	return w.wait(ctx, resourceList, sw)
}

func (w *statusWaiter) WaitWithJobs(resourceList ResourceList, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.TODO(), timeout)
	defer cancel()
	w.log("beginning wait for %d resources with timeout of %s", len(resourceList), timeout)
	sw := watcher.NewDefaultStatusWatcher(w.client, w.restMapper)
	newCustomJobStatusReader := helmStatusReaders.NewCustomJobStatusReader(w.restMapper)
	customSR := statusreaders.NewStatusReader(w.restMapper, newCustomJobStatusReader)
	sw.StatusReader = customSR
	return w.wait(ctx, resourceList, sw)
}

func (w *statusWaiter) WaitForDelete(resourceList ResourceList, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.TODO(), timeout)
	defer cancel()
	w.log("beginning wait for %d resources to be deleted with timeout of %s", len(resourceList), timeout)
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
	done := statusCollector.ListenWithObserver(eventCh, statusObserver(cancel, status.NotFoundStatus, w.log))
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
	done := statusCollector.ListenWithObserver(eventCh, statusObserver(cancel, status.CurrentStatus, w.log))
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

func statusObserver(cancel context.CancelFunc, desired status.Status, logFn func(string, ...interface{})) collector.ObserverFunc {
	return func(statusCollector *collector.ResourceStatusCollector, e event.Event) {
		var rss []*event.ResourceStatus
		var nonDesiredResources []*event.ResourceStatus
		for _, rs := range statusCollector.ResourceStatuses {
			if rs == nil {
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
			// Log only the first resource so the user knows what they're waiting for without being overwhelmed
			sort.Slice(nonDesiredResources, func(i, j int) bool {
				return nonDesiredResources[i].Identifier.Name < nonDesiredResources[j].Identifier.Name
			})
			first := nonDesiredResources[0]
			logFn("waiting for resource: name: %s, kind: %s, desired status: %s, actual status: %s",
				first.Identifier.Name, first.Identifier.GroupKind.Kind, desired, first.Status)
		}
	}
}
