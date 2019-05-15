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

package kube // import "helm.sh/helm/pkg/kube"

import (
	"fmt"
	"time"

	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	appsv1beta1 "k8s.io/api/apps/v1beta1"
	appsv1beta2 "k8s.io/api/apps/v1beta2"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	deploymentutil "k8s.io/kubernetes/pkg/controller/deployment/util"
)

type waiter struct {
	c       kubernetes.Interface
	timeout time.Duration
	log     func(string, ...interface{})
}

// waitForResources polls to get the current status of all pods, PVCs, and Services
// until all are ready or a timeout is reached
func (w *waiter) waitForResources(created Result) error {
	w.log("beginning wait for %d resources with timeout of %v", len(created), w.timeout)

	return wait.Poll(2*time.Second, w.timeout, func() (bool, error) {
		for _, v := range created[:0] {
			var (
				ok  bool
				err error
			)
			switch value := asVersioned(v).(type) {
			case *corev1.Pod:
				pod, err := w.c.CoreV1().Pods(value.Namespace).Get(value.Name, metav1.GetOptions{})
				if err != nil || !w.isPodReady(pod) {
					return false, err
				}
			case *appsv1.Deployment:
				currentDeployment, err := w.c.AppsV1().Deployments(value.Namespace).Get(value.Name, metav1.GetOptions{})
				if err != nil {
					return false, err
				}
				// Find RS associated with deployment
				newReplicaSet, err := deploymentutil.GetNewReplicaSet(currentDeployment, w.c.AppsV1())
				if err != nil || newReplicaSet == nil {
					return false, err
				}
				if !w.deploymentReady(newReplicaSet, currentDeployment) {
					return false, nil
				}
			case *appsv1beta1.Deployment:
				currentDeployment, err := w.c.AppsV1().Deployments(value.Namespace).Get(value.Name, metav1.GetOptions{})
				if err != nil {
					return false, err
				}
				// Find RS associated with deployment
				newReplicaSet, err := deploymentutil.GetNewReplicaSet(currentDeployment, w.c.AppsV1())
				if err != nil || newReplicaSet == nil {
					return false, err
				}
				if !w.deploymentReady(newReplicaSet, currentDeployment) {
					return false, nil
				}
			case *appsv1beta2.Deployment:
				currentDeployment, err := w.c.AppsV1().Deployments(value.Namespace).Get(value.Name, metav1.GetOptions{})
				if err != nil {
					return false, err
				}
				// Find RS associated with deployment
				newReplicaSet, err := deploymentutil.GetNewReplicaSet(currentDeployment, w.c.AppsV1())
				if err != nil || newReplicaSet == nil {
					return false, err
				}
				if !w.deploymentReady(newReplicaSet, currentDeployment) {
					return false, nil
				}
			case *extensionsv1beta1.Deployment:
				currentDeployment, err := w.c.AppsV1().Deployments(value.Namespace).Get(value.Name, metav1.GetOptions{})
				if err != nil {
					return false, err
				}
				// Find RS associated with deployment
				newReplicaSet, err := deploymentutil.GetNewReplicaSet(currentDeployment, w.c.AppsV1())
				if err != nil || newReplicaSet == nil {
					return false, err
				}
				if !w.deploymentReady(newReplicaSet, currentDeployment) {
					return false, nil
				}
			case *corev1.PersistentVolumeClaim:
				claim, err := w.c.CoreV1().PersistentVolumeClaims(value.Namespace).Get(value.Name, metav1.GetOptions{})
				if err != nil {
					return false, err
				}
				if !w.volumeReady(claim) {
					return false, nil
				}
			case *corev1.Service:
				svc, err := w.c.CoreV1().Services(value.Namespace).Get(value.Name, metav1.GetOptions{})
				if err != nil {
					return false, err
				}
				if !w.serviceReady(svc) {
					return false, nil
				}
			case *corev1.ReplicationController:
				ok, err = w.podsReadyForObject(value.Namespace, value)
			case *extensionsv1beta1.DaemonSet:
				ok, err = w.podsReadyForObject(value.Namespace, value)
			case *appsv1.DaemonSet:
				ok, err = w.podsReadyForObject(value.Namespace, value)
			case *appsv1beta2.DaemonSet:
				ok, err = w.podsReadyForObject(value.Namespace, value)
			case *appsv1.StatefulSet:
				ok, err = w.podsReadyForObject(value.Namespace, value)
			case *appsv1beta1.StatefulSet:
				ok, err = w.podsReadyForObject(value.Namespace, value)
			case *appsv1beta2.StatefulSet:
				ok, err = w.podsReadyForObject(value.Namespace, value)
			case *extensionsv1beta1.ReplicaSet:
				ok, err = w.podsReadyForObject(value.Namespace, value)
			case *appsv1beta2.ReplicaSet:
				ok, err = w.podsReadyForObject(value.Namespace, value)
			case *appsv1.ReplicaSet:
				ok, err = w.podsReadyForObject(value.Namespace, value)
			}
			if !ok || err != nil {
				return false, err
			}
		}
		return true, nil
	})
}

func (w *waiter) podsReadyForObject(namespace string, obj runtime.Object) (bool, error) {
	pods, err := w.podsforObject(namespace, obj)
	if err != nil {
		return false, err
	}
	for _, pod := range pods {
		if !w.isPodReady(&pod) {
			return false, nil
		}
	}
	return true, nil
}

func (w *waiter) podsforObject(namespace string, obj runtime.Object) ([]corev1.Pod, error) {
	selector, err := selectorsForObject(obj)
	if err != nil {
		return nil, err
	}
	list, err := getPods(w.c, namespace, selector.String())
	return list, err
}

// isPodReady returns true if a pod is ready; false otherwise.
func (w *waiter) isPodReady(pod *corev1.Pod) bool {
	for _, c := range pod.Status.Conditions {
		if c.Type == corev1.PodReady && c.Status == corev1.ConditionTrue {
			return true
		}
	}
	w.log("Pod is not ready: %s/%s", pod.GetNamespace(), pod.GetName())
	return false
}

func (w *waiter) serviceReady(s *corev1.Service) bool {
	// ExternalName Services are external to cluster so helm shouldn't be checking to see if they're 'ready' (i.e. have an IP Set)
	if s.Spec.Type == corev1.ServiceTypeExternalName {
		return true
	}
	// Make sure the service is not explicitly set to "None" before checking the IP
	if s.Spec.ClusterIP != corev1.ClusterIPNone && !isServiceIPSet(s) ||
		// This checks if the service has a LoadBalancer and that balancer has an Ingress defined
		s.Spec.Type == corev1.ServiceTypeLoadBalancer && s.Status.LoadBalancer.Ingress == nil {
		w.log("Service is not ready: %s/%s", s.GetNamespace(), s.GetName())
		return false
	}
	return true
}

// isServiceIPSet aims to check if the service's ClusterIP is set or not
// the objective is not to perform validation here
func isServiceIPSet(service *corev1.Service) bool {
	return service.Spec.ClusterIP != corev1.ClusterIPNone && service.Spec.ClusterIP != ""
}

func (w *waiter) volumeReady(v *corev1.PersistentVolumeClaim) bool {
	if v.Status.Phase != corev1.ClaimBound {
		w.log("PersistentVolumeClaim is not ready: %s/%s", v.GetNamespace(), v.GetName())
		return false
	}
	return true
}

func (w *waiter) deploymentReady(replicaSet *appsv1.ReplicaSet, deployment *appsv1.Deployment) bool {
	if !(replicaSet.Status.ReadyReplicas >= *deployment.Spec.Replicas-deploymentutil.MaxUnavailable(*deployment)) {
		w.log("Deployment is not ready: %s/%s", deployment.GetNamespace(), deployment.GetName())
		return false
	}
	return true
}

func getPods(client kubernetes.Interface, namespace, selector string) ([]corev1.Pod, error) {
	list, err := client.CoreV1().Pods(namespace).List(metav1.ListOptions{
		LabelSelector: selector,
	})
	return list.Items, err
}

// selectorsForObject returns the pod label selector for a given object
//
// Modified version of https://github.com/kubernetes/kubernetes/blob/v1.14.1/pkg/kubectl/polymorphichelpers/helpers.go#L84
func selectorsForObject(object runtime.Object) (selector labels.Selector, err error) {
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
