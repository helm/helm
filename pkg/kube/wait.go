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
	"fmt"
	"net/http"
	"time"

	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	appsv1beta1 "k8s.io/api/apps/v1beta1"
	appsv1beta2 "k8s.io/api/apps/v1beta2"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/resource"

	"k8s.io/apimachinery/pkg/util/wait"
)

type waiter struct {
	c       ReadyChecker
	timeout time.Duration
	log     func(string, ...interface{})
}

// waitForResources polls to get the current status of all pods, PVCs, Services and
// Jobs(optional) until all are ready or a timeout is reached
func (w *waiter) waitForResources(created ResourceList) error {
	w.log("beginning wait for %d resources with timeout of %v", len(created), w.timeout)

	ctx, cancel := context.WithTimeout(context.Background(), w.timeout)
	defer cancel()

	numberOfErrors := make([]int, len(created))
	for i := range numberOfErrors {
		numberOfErrors[i] = 0
	}

	return wait.PollUntilContextCancel(ctx, 2*time.Second, true, func(ctx context.Context) (bool, error) {
		waitRetries := 30
		for i, v := range created {
			ready, err := w.c.IsReady(ctx, v)

			if waitRetries > 0 && w.isRetryableError(err, v) {
				numberOfErrors[i]++
				if numberOfErrors[i] > waitRetries {
					w.log("Max number of retries reached")
					return false, err
				}
				w.log("Retrying as current number of retries %d less than max number of retries %d", numberOfErrors[i]-1, waitRetries)
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

func (w *waiter) isRetryableError(err error, resource *resource.Info) bool {
	if err == nil {
		return false
	}
	w.log("Error received when checking status of resource %s. Error: '%s', Resource details: '%s'", resource.Name, err, resource)
	if ev, ok := err.(*apierrors.StatusError); ok {
		statusCode := ev.Status().Code
		retryable := w.isRetryableHTTPStatusCode(statusCode)
		w.log("Status code received: %d. Retryable error? %t", statusCode, retryable)
		return retryable
	}
	w.log("Retryable error? %t", true)
	return true
}

func (w *waiter) isRetryableHTTPStatusCode(httpStatusCode int32) bool {
	return httpStatusCode == 0 || httpStatusCode == http.StatusTooManyRequests || (httpStatusCode >= 500 && httpStatusCode != http.StatusNotImplemented)
}

// waitForDeletedResources polls to check if all the resources are deleted or a timeout is reached
func (w *waiter) waitForDeletedResources(deleted ResourceList) error {
	w.log("beginning wait for %d resources to be deleted with timeout of %v", len(deleted), w.timeout)

	ctx, cancel := context.WithTimeout(context.Background(), w.timeout)
	defer cancel()

	return wait.PollUntilContextCancel(ctx, 2*time.Second, true, func(_ context.Context) (bool, error) {
		for _, v := range deleted {
			err := v.Get()
			if err == nil || !apierrors.IsNotFound(err) {
				return false, err
			}
		}
		return true, nil
	})
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
		if t.Spec.Selector == nil || len(t.Spec.Selector) == 0 {
			return nil, fmt.Errorf("invalid service '%s': Service is defined without a selector", t.Name)
		}
		selector = labels.SelectorFromSet(t.Spec.Selector)

	default:
		return nil, fmt.Errorf("selector for %T not implemented", object)
	}

	return selector, errors.Wrap(err, "invalid label selector")
}
