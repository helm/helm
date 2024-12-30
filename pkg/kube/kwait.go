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
	"time"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/cli-utils/pkg/kstatus/polling/aggregator"
	"sigs.k8s.io/cli-utils/pkg/kstatus/polling/collector"
	"sigs.k8s.io/cli-utils/pkg/kstatus/polling/event"
	"sigs.k8s.io/cli-utils/pkg/kstatus/status"
	"sigs.k8s.io/cli-utils/pkg/kstatus/watcher"
	"sigs.k8s.io/cli-utils/pkg/object"
)

type kstatusWaiter struct {
	sw  watcher.StatusWatcher
	log func(string, ...interface{})
}

func (w *kstatusWaiter) Wait(resourceList ResourceList, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.TODO(), timeout)
	defer cancel()
	return w.wait(ctx, resourceList, false)
}

func (w *kstatusWaiter) WaitWithJobs(resourceList ResourceList, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.TODO(), timeout)
	defer cancel()
	return w.wait(ctx, resourceList, true)
}

func (w *kstatusWaiter) waitForDelete(ctx context.Context, resourceList ResourceList) error {
	cancelCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	runtimeObjs := []runtime.Object{}
	for _, resource := range resourceList {
		runtimeObjs = append(runtimeObjs, resource.Object)
	}
	resources := []object.ObjMetadata{}
	for _, runtimeObj := range runtimeObjs {
		obj, err := object.RuntimeToObjMeta(runtimeObj)
		if err != nil {
			return err
		}
		resources = append(resources, obj)
	}
	eventCh := w.sw.Watch(cancelCtx, resources, watcher.Options{})
	statusCollector := collector.NewResourceStatusCollector(resources)
	done := statusCollector.ListenWithObserver(eventCh, collector.ObserverFunc(
		func(statusCollector *collector.ResourceStatusCollector, _ event.Event) {
			rss := []*event.ResourceStatus{}
			for _, rs := range statusCollector.ResourceStatuses {
				if rs == nil {
					continue
				}
				rss = append(rss, rs)
			}
			desired := status.NotFoundStatus
			if aggregator.AggregateStatus(rss, desired) == desired {
				cancel()
				return
			}
		}),
	)
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
			errs = append(errs, fmt.Errorf("%s: %s not ready, status: %s", rs.Identifier.Name, rs.Identifier.GroupKind.Kind, rs.Status))
		}
		errs = append(errs, ctx.Err())
		return errors.Join(errs...)
	}
	return nil
	defer cancel()
	return nil
}

func (w *kstatusWaiter) wait(ctx context.Context, resourceList ResourceList, waitForJobs bool) error {
	cancelCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	runtimeObjs := []runtime.Object{}
	for _, resource := range resourceList {
		switch value := AsVersioned(resource).(type) {
		case *batchv1.Job:
			if !waitForJobs {
				continue
			}
		case *appsv1.Deployment:
			if value.Spec.Paused {
				continue
			}
		}
		runtimeObjs = append(runtimeObjs, resource.Object)
	}
	resources := []object.ObjMetadata{}
	for _, runtimeObj := range runtimeObjs {
		obj, err := object.RuntimeToObjMeta(runtimeObj)
		if err != nil {
			return err
		}
		resources = append(resources, obj)
	}
	eventCh := w.sw.Watch(cancelCtx, resources, watcher.Options{})
	statusCollector := collector.NewResourceStatusCollector(resources)
	done := statusCollector.ListenWithObserver(eventCh, collector.ObserverFunc(
		func(statusCollector *collector.ResourceStatusCollector, _ event.Event) {
			rss := []*event.ResourceStatus{}
			for _, rs := range statusCollector.ResourceStatuses {
				if rs == nil {
					continue
				}
				rss = append(rss, rs)
			}
			desired := status.CurrentStatus
			if aggregator.AggregateStatus(rss, desired) == desired {
				cancel()
				return
			}
		}),
	)
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
			errs = append(errs, fmt.Errorf("%s: %s not ready, status: %s", rs.Identifier.Name, rs.Identifier.GroupKind.Kind, rs.Status))
		}
		errs = append(errs, ctx.Err())
		return errors.Join(errs...)
	}
	return nil
}
