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

	appsv1 "k8s.io/api/apps/v1"
	appsv1beta1 "k8s.io/api/apps/v1beta1"
	appsv1beta2 "k8s.io/api/apps/v1beta2"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"

	deploymentutil "helm.sh/helm/v3/internal/third_party/k8s.io/kubernetes/deployment/util"
)

// ReadyCheckerOption is a function that configures a ReadyChecker.
type ReadyCheckerOption func(*ReadyChecker)

// PausedAsReady returns a ReadyCheckerOption that configures a ReadyChecker
// to consider paused resources to be ready. For example a Deployment
// with spec.paused equal to true would be considered ready.
func PausedAsReady(pausedAsReady bool) ReadyCheckerOption {
	return func(c *ReadyChecker) {
		c.pausedAsReady = pausedAsReady
	}
}

// CheckJobs returns a ReadyCheckerOption that configures a ReadyChecker
// to consider readiness of Job resources.
func CheckJobs(checkJobs bool) ReadyCheckerOption {
	return func(c *ReadyChecker) {
		c.checkJobs = checkJobs
	}
}

// NewReadyChecker creates a new checker. Passed ReadyCheckerOptions can
// be used to override defaults.
func NewReadyChecker(cl kubernetes.Interface, log func(string, ...interface{}), opts ...ReadyCheckerOption) ReadyChecker {
	c := ReadyChecker{
		client: cl,
		log:    log,
	}
	if c.log == nil {
		c.log = nopLogger
	}
	for _, opt := range opts {
		opt(&c)
	}
	return c
}

// ReadyChecker is a type that can check core Kubernetes types for readiness.
type ReadyChecker struct {
	client        kubernetes.Interface
	log           func(string, ...interface{})
	checkJobs     bool
	pausedAsReady bool
}

// IsReady checks if v is ready. It supports checking readiness for pods,
// deployments, persistent volume claims, services, daemon sets, custom
// resource definitions, stateful sets, replication controllers, jobs (optional),
// and replica sets. All other resource kinds are always considered ready.
//
// IsReady will fetch the latest state of the object from the server prior to
// performing readiness checks, and it will return any error encountered.
func (c *ReadyChecker) IsReady(ctx context.Context, v *resource.Info) (bool, error) {
	var (
		// This defaults to true, otherwise we get to a point where
		// things will always return false unless one of the objects
		// that manages pods has been hit
		ok  = true
		err error
	)
	switch value := AsVersioned(v).(type) {
	case *corev1.Pod:
		pod, err := c.client.CoreV1().Pods(v.Namespace).Get(ctx, v.Name, metav1.GetOptions{})
		if err != nil || !c.isPodReady(pod) {
			return false, err
		}
	case *batchv1.Job:
		if c.checkJobs {
			job, err := c.client.BatchV1().Jobs(v.Namespace).Get(ctx, v.Name, metav1.GetOptions{})
			if err != nil {
				return false, err
			}
			ready, err := c.jobReady(job)
			return ready, err
		}
	case *appsv1.Deployment, *appsv1beta1.Deployment, *appsv1beta2.Deployment, *extensionsv1beta1.Deployment:
		currentDeployment, err := c.client.AppsV1().Deployments(v.Namespace).Get(ctx, v.Name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		// If paused deployment will never be ready
		if currentDeployment.Spec.Paused {
			return c.pausedAsReady, nil
		}
		// Find RS associated with deployment
		newReplicaSet, err := deploymentutil.GetNewReplicaSet(currentDeployment, c.client.AppsV1())
		if err != nil || newReplicaSet == nil {
			return false, err
		}
		if !c.deploymentReady(newReplicaSet, currentDeployment) {
			return false, nil
		}
	case *corev1.PersistentVolumeClaim:
		claim, err := c.client.CoreV1().PersistentVolumeClaims(v.Namespace).Get(ctx, v.Name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		if !c.volumeReady(claim) {
			return false, nil
		}
	case *corev1.Service:
		svc, err := c.client.CoreV1().Services(v.Namespace).Get(ctx, v.Name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		if !c.serviceReady(svc) {
			return false, nil
		}
	case *extensionsv1beta1.DaemonSet, *appsv1.DaemonSet, *appsv1beta2.DaemonSet:
		ds, err := c.client.AppsV1().DaemonSets(v.Namespace).Get(ctx, v.Name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		if !c.daemonSetReady(ds) {
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
		if !c.crdBetaReady(*crd) {
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
		if !c.crdReady(*crd) {
			return false, nil
		}
	case *appsv1.StatefulSet, *appsv1beta1.StatefulSet, *appsv1beta2.StatefulSet:
		sts, err := c.client.AppsV1().StatefulSets(v.Namespace).Get(ctx, v.Name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		if !c.statefulSetReady(sts) {
			return false, nil
		}
	case *corev1.ReplicationController, *extensionsv1beta1.ReplicaSet, *appsv1beta2.ReplicaSet, *appsv1.ReplicaSet:
		ok, err = c.podsReadyForObject(ctx, v.Namespace, value)
	}
	if !ok || err != nil {
		return false, err
	}
	return true, nil
}

func (c *ReadyChecker) podsReadyForObject(ctx context.Context, namespace string, obj runtime.Object) (bool, error) {
	pods, err := c.podsforObject(ctx, namespace, obj)
	if err != nil {
		return false, err
	}
	for _, pod := range pods {
		if !c.isPodReady(&pod) {
			return false, nil
		}
	}
	return true, nil
}

func (c *ReadyChecker) podsforObject(ctx context.Context, namespace string, obj runtime.Object) ([]corev1.Pod, error) {
	selector, err := SelectorsForObject(obj)
	if err != nil {
		return nil, err
	}
	list, err := getPods(ctx, c.client, namespace, selector.String())
	return list, err
}

// isPodReady returns true if a pod is ready; false otherwise.
func (c *ReadyChecker) isPodReady(pod *corev1.Pod) bool {
	for _, c := range pod.Status.Conditions {
		if c.Type == corev1.PodReady && c.Status == corev1.ConditionTrue {
			return true
		}
	}
	c.log("Pod is not ready: %s/%s", pod.GetNamespace(), pod.GetName())
	return false
}

func (c *ReadyChecker) jobReady(job *batchv1.Job) (bool, error) {
	if job.Status.Failed > *job.Spec.BackoffLimit {
		c.log("Job is failed: %s/%s", job.GetNamespace(), job.GetName())
		// If a job is failed, it can't recover, so throw an error
		return false, fmt.Errorf("job is failed: %s/%s", job.GetNamespace(), job.GetName())
	}
	if job.Spec.Completions != nil && job.Status.Succeeded < *job.Spec.Completions {
		c.log("Job is not completed: %s/%s", job.GetNamespace(), job.GetName())
		return false, nil
	}
	return true, nil
}

func (c *ReadyChecker) serviceReady(s *corev1.Service) bool {
	// ExternalName Services are external to cluster so helm shouldn't be checking to see if they're 'ready' (i.e. have an IP Set)
	if s.Spec.Type == corev1.ServiceTypeExternalName {
		return true
	}

	// Ensure that the service cluster IP is not empty
	if s.Spec.ClusterIP == "" {
		c.log("Service does not have cluster IP address: %s/%s", s.GetNamespace(), s.GetName())
		return false
	}

	// This checks if the service has a LoadBalancer and that balancer has an Ingress defined
	if s.Spec.Type == corev1.ServiceTypeLoadBalancer {
		// do not wait when at least 1 external IP is set
		if len(s.Spec.ExternalIPs) > 0 {
			c.log("Service %s/%s has external IP addresses (%v), marking as ready", s.GetNamespace(), s.GetName(), s.Spec.ExternalIPs)
			return true
		}

		if s.Status.LoadBalancer.Ingress == nil {
			c.log("Service does not have load balancer ingress IP address: %s/%s", s.GetNamespace(), s.GetName())
			return false
		}
	}

	return true
}

func (c *ReadyChecker) volumeReady(v *corev1.PersistentVolumeClaim) bool {
	if v.Status.Phase != corev1.ClaimBound {
		c.log("PersistentVolumeClaim is not bound: %s/%s", v.GetNamespace(), v.GetName())
		return false
	}
	return true
}

func (c *ReadyChecker) deploymentReady(rs *appsv1.ReplicaSet, dep *appsv1.Deployment) bool {
	expectedReady := *dep.Spec.Replicas - deploymentutil.MaxUnavailable(*dep)
	if !(rs.Status.ReadyReplicas >= expectedReady) {
		c.log("Deployment is not ready: %s/%s. %d out of %d expected pods are ready", dep.Namespace, dep.Name, rs.Status.ReadyReplicas, expectedReady)
		return false
	}
	return true
}

func (c *ReadyChecker) daemonSetReady(ds *appsv1.DaemonSet) bool {
	// If the update strategy is not a rolling update, there will be nothing to wait for
	if ds.Spec.UpdateStrategy.Type != appsv1.RollingUpdateDaemonSetStrategyType {
		return true
	}

	// Make sure all the updated pods have been scheduled
	if ds.Status.UpdatedNumberScheduled != ds.Status.DesiredNumberScheduled {
		c.log("DaemonSet is not ready: %s/%s. %d out of %d expected pods have been scheduled", ds.Namespace, ds.Name, ds.Status.UpdatedNumberScheduled, ds.Status.DesiredNumberScheduled)
		return false
	}
	maxUnavailable, err := intstr.GetScaledValueFromIntOrPercent(ds.Spec.UpdateStrategy.RollingUpdate.MaxUnavailable, int(ds.Status.DesiredNumberScheduled), true)
	if err != nil {
		// If for some reason the value is invalid, set max unavailable to the
		// number of desired replicas. This is the same behavior as the
		// `MaxUnavailable` function in deploymentutil
		maxUnavailable = int(ds.Status.DesiredNumberScheduled)
	}

	expectedReady := int(ds.Status.DesiredNumberScheduled) - maxUnavailable
	if !(int(ds.Status.NumberReady) >= expectedReady) {
		c.log("DaemonSet is not ready: %s/%s. %d out of %d expected pods are ready", ds.Namespace, ds.Name, ds.Status.NumberReady, expectedReady)
		return false
	}
	return true
}

// Because the v1 extensions API is not available on all supported k8s versions
// yet and because Go doesn't support generics, we need to have a duplicate
// function to support the v1beta1 types
func (c *ReadyChecker) crdBetaReady(crd apiextv1beta1.CustomResourceDefinition) bool {
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

func (c *ReadyChecker) crdReady(crd apiextv1.CustomResourceDefinition) bool {
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

func (c *ReadyChecker) statefulSetReady(sts *appsv1.StatefulSet) bool {
	// If the update strategy is not a rolling update, there will be nothing to wait for
	if sts.Spec.UpdateStrategy.Type != appsv1.RollingUpdateStatefulSetStrategyType {
		c.log("StatefulSet skipped ready check: %s/%s. updateStrategy is %v", sts.Namespace, sts.Name, sts.Spec.UpdateStrategy.Type)
		return true
	}

	// Make sure the status is up-to-date with the StatefulSet changes
	if sts.Status.ObservedGeneration < sts.Generation {
		c.log("StatefulSet is not ready: %s/%s. update has not yet been observed", sts.Namespace, sts.Name)
		return false
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
	if int(sts.Status.UpdatedReplicas) < expectedReplicas {
		c.log("StatefulSet is not ready: %s/%s. %d out of %d expected pods have been scheduled", sts.Namespace, sts.Name, sts.Status.UpdatedReplicas, expectedReplicas)
		return false
	}

	if int(sts.Status.ReadyReplicas) != replicas {
		c.log("StatefulSet is not ready: %s/%s. %d out of %d expected pods are ready", sts.Namespace, sts.Name, sts.Status.ReadyReplicas, replicas)
		return false
	}
	// This check only makes sense when all partitions are being upgraded otherwise during a
	// partioned rolling upgrade, this condition will never evaluate to true, leading to
	// error.
	if partition == 0 && sts.Status.CurrentRevision != sts.Status.UpdateRevision {
		c.log("StatefulSet is not ready: %s/%s. currentRevision %s does not yet match updateRevision %s", sts.Namespace, sts.Name, sts.Status.CurrentRevision, sts.Status.UpdateRevision)
		return false
	}

	c.log("StatefulSet is ready: %s/%s. %d out of %d expected pods are ready", sts.Namespace, sts.Name, sts.Status.ReadyReplicas, replicas)
	return true
}

func getPods(ctx context.Context, client kubernetes.Interface, namespace, selector string) ([]corev1.Pod, error) {
	list, err := client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: selector,
	})
	return list.Items, err
}
