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
	"time"

	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	appsv1beta1 "k8s.io/api/apps/v1beta1"
	appsv1beta2 "k8s.io/api/apps/v1beta2"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"

	deploymentutil "helm.sh/helm/v3/internal/third_party/k8s.io/kubernetes/deployment/util"
)

type waiter struct {
	c       kubernetes.Interface
	timeout time.Duration
	log     func(string, ...interface{})
}

// waitForResources polls to get the current status of all pods, PVCs, and Services
// until all are ready or a timeout is reached
func (w *waiter) waitForResources(created ResourceList) error {
	w.log("beginning wait for %d resources with timeout of %v", len(created), w.timeout)

	return wait.Poll(2*time.Second, w.timeout, func() (bool, error) {
		for _, v := range created {
			var (
				// This defaults to true, otherwise we get to a point where
				// things will always return false unless one of the objects
				// that manages pods has been hit
				ok  = true
				err error
			)
			switch value := AsVersioned(v).(type) {
			case *corev1.Pod:
				pod, err := w.c.CoreV1().Pods(v.Namespace).Get(context.Background(), v.Name, metav1.GetOptions{})
				if err != nil || !w.isPodReady(pod) {
					return false, err
				}
			case *appsv1.Deployment, *appsv1beta1.Deployment, *appsv1beta2.Deployment, *extensionsv1beta1.Deployment:
				currentDeployment, err := w.c.AppsV1().Deployments(v.Namespace).Get(context.Background(), v.Name, metav1.GetOptions{})
				if err != nil {
					return false, err
				}
				// If paused deployment will never be ready
				if currentDeployment.Spec.Paused {
					continue
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
				claim, err := w.c.CoreV1().PersistentVolumeClaims(v.Namespace).Get(context.Background(), v.Name, metav1.GetOptions{})
				if err != nil {
					return false, err
				}
				if !w.volumeReady(claim) {
					return false, nil
				}
			case *corev1.Service:
				svc, err := w.c.CoreV1().Services(v.Namespace).Get(context.Background(), v.Name, metav1.GetOptions{})
				if err != nil {
					return false, err
				}
				if !w.serviceReady(svc) {
					return false, nil
				}
			case *extensionsv1beta1.DaemonSet, *appsv1.DaemonSet, *appsv1beta2.DaemonSet:
				ds, err := w.c.AppsV1().DaemonSets(v.Namespace).Get(context.Background(), v.Name, metav1.GetOptions{})
				if err != nil {
					return false, err
				}
				if !w.daemonSetReady(ds) {
					return false, nil
				}
			case *apiextv1beta1.CustomResourceDefinition:
				if err := v.Get(); err != nil {
					return false, err
				}
				crd := &apiextv1beta1.CustomResourceDefinition{}
				if err := scheme.Scheme.Convert(v.Object, crd, nil); err != nil {
					return false, err
				}
				if !w.crdBetaReady(*crd) {
					return false, nil
				}
			case *apiextv1.CustomResourceDefinition:
				if err := v.Get(); err != nil {
					return false, err
				}
				crd := &apiextv1.CustomResourceDefinition{}
				if err := scheme.Scheme.Convert(v.Object, crd, nil); err != nil {
					return false, err
				}
				if !w.crdReady(*crd) {
					return false, nil
				}
			case *appsv1.StatefulSet, *appsv1beta1.StatefulSet, *appsv1beta2.StatefulSet:
				sts, err := w.c.AppsV1().StatefulSets(v.Namespace).Get(context.Background(), v.Name, metav1.GetOptions{})
				if err != nil {
					return false, err
				}
				if !w.statefulSetReady(sts) {
					return false, nil
				}
			case *corev1.ReplicationController, *extensionsv1beta1.ReplicaSet, *appsv1beta2.ReplicaSet, *appsv1.ReplicaSet:
				ok, err = w.podsReadyForObject(v.Namespace, value)
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
	selector, err := SelectorsForObject(obj)
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
	if s.Spec.ClusterIP != corev1.ClusterIPNone && s.Spec.ClusterIP == "" {
		w.log("Service does not have cluster IP address: %s/%s", s.GetNamespace(), s.GetName())
		return false
	}

	// This checks if the service has a LoadBalancer and that balancer has an Ingress defined
	if s.Spec.Type == corev1.ServiceTypeLoadBalancer {
		// do not wait when at least 1 external IP is set
		if len(s.Spec.ExternalIPs) > 0 {
			w.log("Service %s/%s has external IP addresses (%v), marking as ready", s.GetNamespace(), s.GetName(), s.Spec.ExternalIPs)
			return true
		}

		if s.Status.LoadBalancer.Ingress == nil {
			w.log("Service does not have load balancer ingress IP address: %s/%s", s.GetNamespace(), s.GetName())
			return false
		}
	}

	return true
}

func (w *waiter) volumeReady(v *corev1.PersistentVolumeClaim) bool {
	if v.Status.Phase != corev1.ClaimBound {
		w.log("PersistentVolumeClaim is not bound: %s/%s", v.GetNamespace(), v.GetName())
		return false
	}
	return true
}

func (w *waiter) deploymentReady(rs *appsv1.ReplicaSet, dep *appsv1.Deployment) bool {
	expectedReady := *dep.Spec.Replicas - deploymentutil.MaxUnavailable(*dep)
	if !(rs.Status.ReadyReplicas >= expectedReady) {
		w.log("Deployment is not ready: %s/%s. %d out of %d expected pods are ready", dep.Namespace, dep.Name, rs.Status.ReadyReplicas, expectedReady)
		return false
	}
	return true
}

func (w *waiter) daemonSetReady(ds *appsv1.DaemonSet) bool {
	// If the update strategy is not a rolling update, there will be nothing to wait for
	if ds.Spec.UpdateStrategy.Type != appsv1.RollingUpdateDaemonSetStrategyType {
		return true
	}

	// Make sure all the updated pods have been scheduled
	if ds.Status.UpdatedNumberScheduled != ds.Status.DesiredNumberScheduled {
		w.log("DaemonSet is not ready: %s/%s. %d out of %d expected pods have been scheduled", ds.Namespace, ds.Name, ds.Status.UpdatedNumberScheduled, ds.Status.DesiredNumberScheduled)
		return false
	}
	maxUnavailable, err := intstr.GetValueFromIntOrPercent(ds.Spec.UpdateStrategy.RollingUpdate.MaxUnavailable, int(ds.Status.DesiredNumberScheduled), true)
	if err != nil {
		// If for some reason the value is invalid, set max unavailable to the
		// number of desired replicas. This is the same behavior as the
		// `MaxUnavailable` function in deploymentutil
		maxUnavailable = int(ds.Status.DesiredNumberScheduled)
	}

	expectedReady := int(ds.Status.DesiredNumberScheduled) - maxUnavailable
	if !(int(ds.Status.NumberReady) >= expectedReady) {
		w.log("DaemonSet is not ready: %s/%s. %d out of %d expected pods are ready", ds.Namespace, ds.Name, ds.Status.NumberReady, expectedReady)
		return false
	}
	return true
}

// Because the v1 extensions API is not available on all supported k8s versions
// yet and because Go doesn't support generics, we need to have a duplicate
// function to support the v1beta1 types
func (w *waiter) crdBetaReady(crd apiextv1beta1.CustomResourceDefinition) bool {
	for _, cond := range crd.Status.Conditions {
		switch cond.Type {
		case apiextv1beta1.Established:
			if cond.Status == apiextv1beta1.ConditionTrue {
				return true
			}
		case apiextv1beta1.NamesAccepted:
			if cond.Status == apiextv1beta1.ConditionFalse {
				// This indicates a naming conflict, but it's probably not the
				// job of this function to fail because of that. Instead,
				// we treat it as a success, since the process should be able to
				// continue.
				return true
			}
		}
	}
	return false
}

func (w *waiter) crdReady(crd apiextv1.CustomResourceDefinition) bool {
	for _, cond := range crd.Status.Conditions {
		switch cond.Type {
		case apiextv1.Established:
			if cond.Status == apiextv1.ConditionTrue {
				return true
			}
		case apiextv1.NamesAccepted:
			if cond.Status == apiextv1.ConditionFalse {
				// This indicates a naming conflict, but it's probably not the
				// job of this function to fail because of that. Instead,
				// we treat it as a success, since the process should be able to
				// continue.
				return true
			}
		}
	}
	return false
}

func (w *waiter) statefulSetReady(sts *appsv1.StatefulSet) bool {
	// If the update strategy is not a rolling update, there will be nothing to wait for
	if sts.Spec.UpdateStrategy.Type != appsv1.RollingUpdateStatefulSetStrategyType {
		return true
	}

	// Dereference all the pointers because StatefulSets like them
	var partition int
	// 1 is the default for replicas if not set
	var replicas = 1
	// For some reason, even if the update strategy is a rolling update, the
	// actual rollingUpdate field can be nil. If it is, we can safely assume
	// there is no partition value
	if sts.Spec.UpdateStrategy.RollingUpdate != nil && sts.Spec.UpdateStrategy.RollingUpdate.Partition != nil {
		partition = int(*sts.Spec.UpdateStrategy.RollingUpdate.Partition)
	}
	if sts.Spec.Replicas != nil {
		replicas = int(*sts.Spec.Replicas)
	}

	// Because an update strategy can use partitioning, we need to calculate the
	// number of updated replicas we should have. For example, if the replicas
	// is set to 3 and the partition is 2, we'd expect only one pod to be
	// updated
	expectedReplicas := replicas - partition

	// Make sure all the updated pods have been scheduled
	if int(sts.Status.UpdatedReplicas) != expectedReplicas {
		w.log("StatefulSet is not ready: %s/%s. %d out of %d expected pods have been scheduled", sts.Namespace, sts.Name, sts.Status.UpdatedReplicas, expectedReplicas)
		return false
	}

	if int(sts.Status.ReadyReplicas) != replicas {
		w.log("StatefulSet is not ready: %s/%s. %d out of %d expected pods are ready", sts.Namespace, sts.Name, sts.Status.ReadyReplicas, replicas)
		return false
	}
	return true
}

func getPods(client kubernetes.Interface, namespace, selector string) ([]corev1.Pod, error) {
	list, err := client.CoreV1().Pods(namespace).List(context.Background(), metav1.ListOptions{
		LabelSelector: selector,
	})
	return list.Items, err
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
