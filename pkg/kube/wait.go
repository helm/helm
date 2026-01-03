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

package kube // import "helm.sh/helm/v4/pkg/kube"

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	appsv1beta1 "k8s.io/api/apps/v1beta1"
	appsv1beta2 "k8s.io/api/apps/v1beta2"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/kubernetes"
	cachetools "k8s.io/client-go/tools/cache"
	watchtools "k8s.io/client-go/tools/watch"

	"k8s.io/apimachinery/pkg/util/wait"
)

// legacyWaiter is the legacy implementation of the Waiter interface. This logic was used by default in Helm 3
// Helm 4 now uses the StatusWaiter implementation instead
type legacyWaiter struct {
	c          ReadyChecker
	kubeClient *kubernetes.Clientset
	ctx        context.Context
}

func (hw *legacyWaiter) Wait(resources ResourceList, timeout time.Duration) error {
	hw.c = NewReadyChecker(hw.kubeClient, PausedAsReady(true))
	return hw.waitForResources(resources, timeout)
}

func (hw *legacyWaiter) WaitWithJobs(resources ResourceList, timeout time.Duration) error {
	hw.c = NewReadyChecker(hw.kubeClient, PausedAsReady(true), CheckJobs(true))
	return hw.waitForResources(resources, timeout)
}

// waitForResources polls to get the current status of all pods, PVCs, Services and
// Jobs(optional) until all are ready or a timeout is reached
func (hw *legacyWaiter) waitForResources(created ResourceList, timeout time.Duration) error {
	slog.Debug("beginning wait for resources", "count", len(created), "timeout", timeout)

	ctx, cancel := hw.contextWithTimeout(timeout)
	defer cancel()

	numberOfErrors := make([]int, len(created))
	for i := range numberOfErrors {
		numberOfErrors[i] = 0
	}

	return wait.PollUntilContextCancel(ctx, 2*time.Second, true, func(ctx context.Context) (bool, error) {
		waitRetries := 30
		for i, v := range created {
			ready, err := hw.c.IsReady(ctx, v)

			if waitRetries > 0 && hw.isRetryableError(err, v) {
				numberOfErrors[i]++
				if numberOfErrors[i] > waitRetries {
					slog.Debug("max number of retries reached", "resource", v.Name, "retries", numberOfErrors[i])
					return false, err
				}
				slog.Debug("retrying resource readiness", "resource", v.Name, "currentRetries", numberOfErrors[i]-1, "maxRetries", waitRetries)
				return false, nil
			}
			numberOfErrors[i] = 0
			if !ready {
				return false, err
			}
		}
		return true, nil
	})
}

func (hw *legacyWaiter) isRetryableError(err error, resource *resource.Info) bool {
	if err == nil {
		return false
	}
	slog.Debug(
		"error received when checking resource status",
		slog.String("resource", resource.Name),
		slog.Any("error", err),
	)
	if ev, ok := err.(*apierrors.StatusError); ok {
		statusCode := ev.Status().Code
		retryable := hw.isRetryableHTTPStatusCode(statusCode)
		slog.Debug(
			"status code received",
			slog.String("resource", resource.Name),
			slog.Int("statusCode", int(statusCode)),
			slog.Bool("retryable", retryable),
		)
		return retryable
	}
	slog.Debug("retryable error assumed", "resource", resource.Name)
	return true
}

func (hw *legacyWaiter) isRetryableHTTPStatusCode(httpStatusCode int32) bool {
	return httpStatusCode == 0 || httpStatusCode == http.StatusTooManyRequests || (httpStatusCode >= 500 && httpStatusCode != http.StatusNotImplemented)
}

// WaitForDelete polls to check if all the resources are deleted or a timeout is reached
func (hw *legacyWaiter) WaitForDelete(deleted ResourceList, timeout time.Duration) error {
	slog.Debug("beginning wait for resources to be deleted", "count", len(deleted), "timeout", timeout)

	startTime := time.Now()
	ctx, cancel := hw.contextWithTimeout(timeout)
	defer cancel()

	err := wait.PollUntilContextCancel(ctx, 2*time.Second, true, func(_ context.Context) (bool, error) {
		for _, v := range deleted {
			err := v.Get()
			if err == nil || !apierrors.IsNotFound(err) {
				return false, err
			}
		}
		return true, nil
	})

	elapsed := time.Since(startTime).Round(time.Second)
	if err != nil {
		slog.Debug("wait for resources failed", slog.Duration("elapsed", elapsed), slog.Any("error", err))
	} else {
		slog.Debug("wait for resources succeeded", slog.Duration("elapsed", elapsed))
	}

	return err
}

// SelectorsForObject returns the pod label selector for a given object
//
// Modified version of https://github.com/kubernetes/kubernetes/blob/v1.14.1/pkg/kubectl/polymorphichelpers/helpers.go#L84
func SelectorsForObject(object runtime.Object) (selector labels.Selector, err error) {
	switch t := object.(type) {
	case *extensionsv1beta1.ReplicaSet:
		selector, err = metav1.LabelSelectorAsSelector(t.Spec.Selector)
	case *appsv1.ReplicaSet:
		selector, err = metav1.LabelSelectorAsSelector(t.Spec.Selector)
	case *appsv1beta2.ReplicaSet:
		selector, err = metav1.LabelSelectorAsSelector(t.Spec.Selector)
	case *corev1.ReplicationController:
		selector = labels.SelectorFromSet(t.Spec.Selector)
	case *appsv1.StatefulSet:
		selector, err = metav1.LabelSelectorAsSelector(t.Spec.Selector)
	case *appsv1beta1.StatefulSet:
		selector, err = metav1.LabelSelectorAsSelector(t.Spec.Selector)
	case *appsv1beta2.StatefulSet:
		selector, err = metav1.LabelSelectorAsSelector(t.Spec.Selector)
	case *extensionsv1beta1.DaemonSet:
		selector, err = metav1.LabelSelectorAsSelector(t.Spec.Selector)
	case *appsv1.DaemonSet:
		selector, err = metav1.LabelSelectorAsSelector(t.Spec.Selector)
	case *appsv1beta2.DaemonSet:
		selector, err = metav1.LabelSelectorAsSelector(t.Spec.Selector)
	case *extensionsv1beta1.Deployment:
		selector, err = metav1.LabelSelectorAsSelector(t.Spec.Selector)
	case *appsv1.Deployment:
		selector, err = metav1.LabelSelectorAsSelector(t.Spec.Selector)
	case *appsv1beta1.Deployment:
		selector, err = metav1.LabelSelectorAsSelector(t.Spec.Selector)
	case *appsv1beta2.Deployment:
		selector, err = metav1.LabelSelectorAsSelector(t.Spec.Selector)
	case *batchv1.Job:
		selector, err = metav1.LabelSelectorAsSelector(t.Spec.Selector)
	case *corev1.Service:
		if len(t.Spec.Selector) == 0 {
			return nil, fmt.Errorf("invalid service '%s': Service is defined without a selector", t.Name)
		}
		selector = labels.SelectorFromSet(t.Spec.Selector)

	default:
		return nil, fmt.Errorf("selector for %T not implemented", object)
	}

	if err != nil {
		return selector, fmt.Errorf("invalid label selector: %w", err)
	}

	return selector, nil
}

func (hw *legacyWaiter) watchTimeout(t time.Duration) func(*resource.Info) error {
	return func(info *resource.Info) error {
		return hw.watchUntilReady(t, info)
	}
}

// WatchUntilReady watches the resources given and waits until it is ready.
//
// This method is mainly for hook implementations. It watches for a resource to
// hit a particular milestone. The milestone depends on the Kind.
//
// For most kinds, it checks to see if the resource is marked as Added or Modified
// by the Kubernetes event stream. For some kinds, it does more:
//
//   - Jobs: A job is marked "Ready" when it has successfully completed. This is
//     ascertained by watching the Status fields in a job's output.
//   - Pods: A pod is marked "Ready" when it has successfully completed. This is
//     ascertained by watching the status.phase field in a pod's output.
//
// Handling for other kinds will be added as necessary.
func (hw *legacyWaiter) WatchUntilReady(resources ResourceList, timeout time.Duration) error {
	// For jobs, there's also the option to do poll c.Jobs(namespace).Get():
	// https://github.com/adamreese/kubernetes/blob/master/test/e2e/job.go#L291-L300
	return perform(resources, hw.watchTimeout(timeout))
}

func (hw *legacyWaiter) watchUntilReady(timeout time.Duration, info *resource.Info) error {
	kind := info.Mapping.GroupVersionKind.Kind
	switch kind {
	case "Job", "Pod":
	default:
		return nil
	}

	slog.Debug("watching for resource changes", "kind", kind, "resource", info.Name, "timeout", timeout)

	// Use a selector on the name of the resource. This should be unique for the
	// given version and kind
	selector, err := fields.ParseSelector(fmt.Sprintf("metadata.name=%s", info.Name))
	if err != nil {
		return err
	}
	lw := cachetools.NewListWatchFromClient(info.Client, info.Mapping.Resource.Resource, info.Namespace, selector)

	// What we watch for depends on the Kind.
	// - For a Job, we watch for completion.
	// - For all else, we watch until Ready.
	// In the future, we might want to add some special logic for types
	// like Ingress, Volume, etc.

	ctx, cancel := hw.contextWithTimeout(timeout)
	defer cancel()
	_, err = watchtools.UntilWithSync(ctx, lw, &unstructured.Unstructured{}, nil, func(e watch.Event) (bool, error) {
		// Make sure the incoming object is versioned as we use unstructured
		// objects when we build manifests
		obj := convertWithMapper(e.Object, info.Mapping)
		switch e.Type {
		case watch.Added, watch.Modified:
			// For things like a secret or a config map, this is the best indicator
			// we get. We care mostly about jobs, where what we want to see is
			// the status go into a good state. For other types, like ReplicaSet
			// we don't really do anything to support these as hooks.
			slog.Debug("add/modify event received", "resource", info.Name, "eventType", e.Type)

			switch kind {
			case "Job":
				return hw.waitForJob(obj, info.Name)
			case "Pod":
				return hw.waitForPodSuccess(obj, info.Name)
			}
			return true, nil
		case watch.Deleted:
			slog.Debug("deleted event received", "resource", info.Name)
			return true, nil
		case watch.Error:
			// Handle error and return with an error.
			slog.Error("error event received", "resource", info.Name)
			return true, fmt.Errorf("failed to deploy %s", info.Name)
		default:
			return false, nil
		}
	})
	return err
}

// waitForJob is a helper that waits for a job to complete.
//
// This operates on an event returned from a watcher.
func (hw *legacyWaiter) waitForJob(obj runtime.Object, name string) (bool, error) {
	o, ok := obj.(*batchv1.Job)
	if !ok {
		return true, fmt.Errorf("expected %s to be a *batch.Job, got %T", name, obj)
	}

	for _, c := range o.Status.Conditions {
		if c.Type == batchv1.JobComplete && c.Status == "True" {
			return true, nil
		} else if c.Type == batchv1.JobFailed && c.Status == "True" {
			slog.Error("job failed", "job", name, "reason", c.Reason)
			return true, fmt.Errorf("job %s failed: %s", name, c.Reason)
		}
	}

	slog.Debug("job status update", "job", name, "active", o.Status.Active, "failed", o.Status.Failed, "succeeded", o.Status.Succeeded)
	return false, nil
}

// waitForPodSuccess is a helper that waits for a pod to complete.
//
// This operates on an event returned from a watcher.
func (hw *legacyWaiter) waitForPodSuccess(obj runtime.Object, name string) (bool, error) {
	o, ok := obj.(*corev1.Pod)
	if !ok {
		return true, fmt.Errorf("expected %s to be a *v1.Pod, got %T", name, obj)
	}

	switch o.Status.Phase {
	case corev1.PodSucceeded:
		slog.Debug("pod succeeded", "pod", o.Name)
		return true, nil
	case corev1.PodFailed:
		slog.Error("pod failed", "pod", o.Name)
		return true, fmt.Errorf("pod %s failed", o.Name)
	case corev1.PodPending:
		slog.Debug("pod pending", "pod", o.Name)
	case corev1.PodRunning:
		slog.Debug("pod running", "pod", o.Name)
	case corev1.PodUnknown:
		slog.Debug("pod unknown", "pod", o.Name)
	}

	return false, nil
}

func (hw *legacyWaiter) contextWithTimeout(timeout time.Duration) (context.Context, context.CancelFunc) {
	return contextWithTimeout(hw.ctx, timeout)
}
