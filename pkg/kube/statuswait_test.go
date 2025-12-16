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
	"strings"
	"testing"
	"time"

	"github.com/fluxcd/cli-utils/pkg/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/dynamic"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/kubectl/pkg/scheme"
)

var podCurrentManifest = `
apiVersion: v1
kind: Pod
metadata:
  name: current-pod
  namespace: ns
status:
  conditions:
  - type: Ready
    status: "True"
  phase: Running
`

var podNoStatusManifest = `
apiVersion: v1
kind: Pod
metadata:
  name: in-progress-pod
  namespace: ns
`

var jobNoStatusManifest = `
apiVersion: batch/v1
kind: Job
metadata:
   name: test
   namespace: qual
   generation: 1
`

var jobReadyManifest = `
apiVersion: batch/v1
kind: Job
metadata:
  name: ready-not-complete
  namespace: default
  generation: 1
status:
  startTime: 2025-02-06T16:34:20-05:00
  active: 1
  ready: 1
`

var jobCompleteManifest = `
apiVersion: batch/v1
kind: Job
metadata:
   name: test
   namespace: qual
   generation: 1
status:
   succeeded: 1
   active: 0
   conditions:
    - type: Complete 
      status: "True"
`

var podCompleteManifest = `
apiVersion: v1
kind: Pod
metadata:
  name: good-pod
  namespace: ns
status:
  phase: Succeeded
`

var pausedDeploymentManifest = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: paused
  namespace: ns-1
  generation: 1
spec:
  paused: true
  replicas: 1
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      labels:
        app: nginx
    spec:
      containers:
      - name: nginx
        image: nginx:1.19.6
        ports:
        - containerPort: 80
`

var notReadyDeploymentManifest = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: not-ready
  namespace: ns-1
  generation: 1
spec:
  replicas: 1
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      labels:
        app: nginx
    spec:
      containers:
      - name: nginx
        image: nginx:1.19.6
        ports:
        - containerPort: 80
`

var podNamespace1Manifest = `
apiVersion: v1
kind: Pod
metadata:
  name: pod-ns1
  namespace: namespace-1
status:
  conditions:
  - type: Ready
    status: "True"
  phase: Running
`

var podNamespace2Manifest = `
apiVersion: v1
kind: Pod
metadata:
  name: pod-ns2
  namespace: namespace-2
status:
  conditions:
  - type: Ready
    status: "True"
  phase: Running
`

var podNamespace1NoStatusManifest = `
apiVersion: v1
kind: Pod
metadata:
  name: pod-ns1
  namespace: namespace-1
`

var jobNamespace1CompleteManifest = `
apiVersion: batch/v1
kind: Job
metadata:
  name: job-ns1
  namespace: namespace-1
  generation: 1
status:
  succeeded: 1
  active: 0
  conditions:
  - type: Complete
    status: "True"
`

var podNamespace2SucceededManifest = `
apiVersion: v1
kind: Pod
metadata:
  name: pod-ns2
  namespace: namespace-2
status:
  phase: Succeeded
`

var clusterRoleManifest = `
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: test-cluster-role
rules:
- apiGroups: [""]
  resources: ["pods"]
  verbs: ["get", "list"]
`

var namespaceManifest = `
apiVersion: v1
kind: Namespace
metadata:
  name: test-namespace
`

func getGVR(t *testing.T, mapper meta.RESTMapper, obj *unstructured.Unstructured) schema.GroupVersionResource {
	t.Helper()
	gvk := obj.GroupVersionKind()
	mapping, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	require.NoError(t, err)
	return mapping.Resource
}

func getRuntimeObjFromManifests(t *testing.T, manifests []string) []runtime.Object {
	t.Helper()
	objects := []runtime.Object{}
	for _, manifest := range manifests {
		m := make(map[string]interface{})
		err := yaml.Unmarshal([]byte(manifest), &m)
		assert.NoError(t, err)
		resource := &unstructured.Unstructured{Object: m}
		objects = append(objects, resource)
	}
	return objects
}

func getResourceListFromRuntimeObjs(t *testing.T, c *Client, objs []runtime.Object) ResourceList {
	t.Helper()
	resourceList := ResourceList{}
	for _, obj := range objs {
		list, err := c.Build(objBody(obj), false)
		assert.NoError(t, err)
		resourceList = append(resourceList, list...)
	}
	return resourceList
}

func TestStatusWaitForDelete(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name              string
		manifestsToCreate []string
		manifestsToDelete []string
		expectErrs        []error
	}{
		{
			name:              "wait for pod to be deleted",
			manifestsToCreate: []string{podCurrentManifest},
			manifestsToDelete: []string{podCurrentManifest},
			expectErrs:        nil,
		},
		{
			name:              "error when not all objects are deleted",
			manifestsToCreate: []string{jobCompleteManifest, podCurrentManifest},
			manifestsToDelete: []string{jobCompleteManifest},
			expectErrs:        []error{errors.New("resource still exists, name: current-pod, kind: Pod, status: Current"), errors.New("context deadline exceeded")},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c := newTestClient(t)
			timeout := time.Second
			timeUntilPodDelete := time.Millisecond * 500
			fakeClient := dynamicfake.NewSimpleDynamicClient(scheme.Scheme)
			fakeMapper := testutil.NewFakeRESTMapper(
				v1.SchemeGroupVersion.WithKind("Pod"),
				batchv1.SchemeGroupVersion.WithKind("Job"),
			)
			statusWaiter := statusWaiter{
				restMapper: fakeMapper,
				client:     fakeClient,
			}
			objsToCreate := getRuntimeObjFromManifests(t, tt.manifestsToCreate)
			for _, objToCreate := range objsToCreate {
				u := objToCreate.(*unstructured.Unstructured)
				gvr := getGVR(t, fakeMapper, u)
				err := fakeClient.Tracker().Create(gvr, u, u.GetNamespace())
				assert.NoError(t, err)
			}
			objsToDelete := getRuntimeObjFromManifests(t, tt.manifestsToDelete)
			for _, objToDelete := range objsToDelete {
				u := objToDelete.(*unstructured.Unstructured)
				gvr := getGVR(t, fakeMapper, u)
				go func(gvr schema.GroupVersionResource, u *unstructured.Unstructured) {
					time.Sleep(timeUntilPodDelete)
					err := fakeClient.Tracker().Delete(gvr, u.GetNamespace(), u.GetName())
					assert.NoError(t, err)
				}(gvr, u)
			}
			resourceList := getResourceListFromRuntimeObjs(t, c, objsToCreate)
			err := statusWaiter.WaitForDelete(resourceList, timeout)
			if tt.expectErrs != nil {
				assert.EqualError(t, err, errors.Join(tt.expectErrs...).Error())
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestStatusWaitForDeleteNonExistentObject(t *testing.T) {
	t.Parallel()
	c := newTestClient(t)
	timeout := time.Second
	fakeClient := dynamicfake.NewSimpleDynamicClient(scheme.Scheme)
	fakeMapper := testutil.NewFakeRESTMapper(
		v1.SchemeGroupVersion.WithKind("Pod"),
	)
	statusWaiter := statusWaiter{
		restMapper: fakeMapper,
		client:     fakeClient,
	}
	// Don't create the object to test that the wait for delete works when the object doesn't exist
	objManifest := getRuntimeObjFromManifests(t, []string{podCurrentManifest})
	resourceList := getResourceListFromRuntimeObjs(t, c, objManifest)
	err := statusWaiter.WaitForDelete(resourceList, timeout)
	assert.NoError(t, err)
}

func TestStatusWait(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		objManifests []string
		expectErrs   []error
		waitForJobs  bool
	}{
		{
			name:         "Job is not complete",
			objManifests: []string{jobNoStatusManifest},
			expectErrs:   []error{errors.New("resource not ready, name: test, kind: Job, status: InProgress"), errors.New("context deadline exceeded")},
			waitForJobs:  true,
		},
		{
			name:         "Job is ready but not complete",
			objManifests: []string{jobReadyManifest},
			expectErrs:   nil,
			waitForJobs:  false,
		},
		{
			name:         "Pod is ready",
			objManifests: []string{podCurrentManifest},
			expectErrs:   nil,
		},
		{
			name:         "one of the pods never becomes ready",
			objManifests: []string{podNoStatusManifest, podCurrentManifest},
			expectErrs:   []error{errors.New("resource not ready, name: in-progress-pod, kind: Pod, status: InProgress"), errors.New("context deadline exceeded")},
		},
		{
			name:         "paused deployment passes",
			objManifests: []string{pausedDeploymentManifest},
			expectErrs:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c := newTestClient(t)
			fakeClient := dynamicfake.NewSimpleDynamicClient(scheme.Scheme)
			fakeMapper := testutil.NewFakeRESTMapper(
				v1.SchemeGroupVersion.WithKind("Pod"),
				appsv1.SchemeGroupVersion.WithKind("Deployment"),
				batchv1.SchemeGroupVersion.WithKind("Job"),
			)
			statusWaiter := statusWaiter{
				client:     fakeClient,
				restMapper: fakeMapper,
			}
			objs := getRuntimeObjFromManifests(t, tt.objManifests)
			for _, obj := range objs {
				u := obj.(*unstructured.Unstructured)
				gvr := getGVR(t, fakeMapper, u)
				err := fakeClient.Tracker().Create(gvr, u, u.GetNamespace())
				assert.NoError(t, err)
			}
			resourceList := getResourceListFromRuntimeObjs(t, c, objs)
			err := statusWaiter.Wait(resourceList, time.Second*3)
			if tt.expectErrs != nil {
				assert.EqualError(t, err, errors.Join(tt.expectErrs...).Error())
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestWaitForJobComplete(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		objManifests []string
		expectErrs   []error
	}{
		{
			name:         "Job is complete",
			objManifests: []string{jobCompleteManifest},
		},
		{
			name:         "Job is not ready",
			objManifests: []string{jobNoStatusManifest},
			expectErrs:   []error{errors.New("resource not ready, name: test, kind: Job, status: InProgress"), errors.New("context deadline exceeded")},
		},
		{
			name:         "Job is ready but not complete",
			objManifests: []string{jobReadyManifest},
			expectErrs:   []error{errors.New("resource not ready, name: ready-not-complete, kind: Job, status: InProgress"), errors.New("context deadline exceeded")},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c := newTestClient(t)
			fakeClient := dynamicfake.NewSimpleDynamicClient(scheme.Scheme)
			fakeMapper := testutil.NewFakeRESTMapper(
				batchv1.SchemeGroupVersion.WithKind("Job"),
			)
			statusWaiter := statusWaiter{
				client:     fakeClient,
				restMapper: fakeMapper,
			}
			objs := getRuntimeObjFromManifests(t, tt.objManifests)
			for _, obj := range objs {
				u := obj.(*unstructured.Unstructured)
				gvr := getGVR(t, fakeMapper, u)
				err := fakeClient.Tracker().Create(gvr, u, u.GetNamespace())
				assert.NoError(t, err)
			}
			resourceList := getResourceListFromRuntimeObjs(t, c, objs)
			err := statusWaiter.WaitWithJobs(resourceList, time.Second*3)
			if tt.expectErrs != nil {
				assert.EqualError(t, err, errors.Join(tt.expectErrs...).Error())
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestWatchForReady(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		objManifests []string
		expectErrs   []error
	}{
		{
			name:         "succeeds if pod and job are complete",
			objManifests: []string{jobCompleteManifest, podCompleteManifest},
		},
		{
			name:         "succeeds when a resource that's not a pod or job is not ready",
			objManifests: []string{notReadyDeploymentManifest},
		},
		{
			name:         "Fails if job is not complete",
			objManifests: []string{jobReadyManifest},
			expectErrs:   []error{errors.New("resource not ready, name: ready-not-complete, kind: Job, status: InProgress"), errors.New("context deadline exceeded")},
		},
		{
			name:         "Fails if pod is not complete",
			objManifests: []string{podCurrentManifest},
			expectErrs:   []error{errors.New("resource not ready, name: current-pod, kind: Pod, status: InProgress"), errors.New("context deadline exceeded")},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c := newTestClient(t)
			fakeClient := dynamicfake.NewSimpleDynamicClient(scheme.Scheme)
			fakeMapper := testutil.NewFakeRESTMapper(
				v1.SchemeGroupVersion.WithKind("Pod"),
				appsv1.SchemeGroupVersion.WithKind("Deployment"),
				batchv1.SchemeGroupVersion.WithKind("Job"),
			)
			statusWaiter := statusWaiter{
				client:     fakeClient,
				restMapper: fakeMapper,
			}
			objs := getRuntimeObjFromManifests(t, tt.objManifests)
			for _, obj := range objs {
				u := obj.(*unstructured.Unstructured)
				gvr := getGVR(t, fakeMapper, u)
				err := fakeClient.Tracker().Create(gvr, u, u.GetNamespace())
				assert.NoError(t, err)
			}
			resourceList := getResourceListFromRuntimeObjs(t, c, objs)
			err := statusWaiter.WatchUntilReady(resourceList, time.Second*3)
			if tt.expectErrs != nil {
				assert.EqualError(t, err, errors.Join(tt.expectErrs...).Error())
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestStatusWaitMultipleNamespaces(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		objManifests []string
		expectErrs   []error
		testFunc     func(statusWaiter, ResourceList, time.Duration) error
	}{
		{
			name:         "pods in multiple namespaces",
			objManifests: []string{podNamespace1Manifest, podNamespace2Manifest},
			testFunc: func(sw statusWaiter, rl ResourceList, timeout time.Duration) error {
				return sw.Wait(rl, timeout)
			},
		},
		{
			name:         "hooks in multiple namespaces",
			objManifests: []string{jobNamespace1CompleteManifest, podNamespace2SucceededManifest},
			testFunc: func(sw statusWaiter, rl ResourceList, timeout time.Duration) error {
				return sw.WatchUntilReady(rl, timeout)
			},
		},
		{
			name:         "error when resource not ready in one namespace",
			objManifests: []string{podNamespace1NoStatusManifest, podNamespace2Manifest},
			expectErrs:   []error{errors.New("resource not ready, name: pod-ns1, kind: Pod, status: InProgress"), errors.New("context deadline exceeded")},
			testFunc: func(sw statusWaiter, rl ResourceList, timeout time.Duration) error {
				return sw.Wait(rl, timeout)
			},
		},
		{
			name:         "delete resources in multiple namespaces",
			objManifests: []string{podNamespace1Manifest, podNamespace2Manifest},
			testFunc: func(sw statusWaiter, rl ResourceList, timeout time.Duration) error {
				return sw.WaitForDelete(rl, timeout)
			},
		},
		{
			name:         "cluster-scoped resources work correctly with unrestricted permissions",
			objManifests: []string{podNamespace1Manifest, clusterRoleManifest},
			testFunc: func(sw statusWaiter, rl ResourceList, timeout time.Duration) error {
				return sw.Wait(rl, timeout)
			},
		},
		{
			name:         "namespace-scoped and cluster-scoped resources work together",
			objManifests: []string{podNamespace1Manifest, podNamespace2Manifest, clusterRoleManifest},
			testFunc: func(sw statusWaiter, rl ResourceList, timeout time.Duration) error {
				return sw.Wait(rl, timeout)
			},
		},
		{
			name:         "delete cluster-scoped resources works correctly",
			objManifests: []string{podNamespace1Manifest, namespaceManifest},
			testFunc: func(sw statusWaiter, rl ResourceList, timeout time.Duration) error {
				return sw.WaitForDelete(rl, timeout)
			},
		},
		{
			name:         "watch cluster-scoped resources works correctly",
			objManifests: []string{clusterRoleManifest},
			testFunc: func(sw statusWaiter, rl ResourceList, timeout time.Duration) error {
				return sw.WatchUntilReady(rl, timeout)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c := newTestClient(t)
			fakeClient := dynamicfake.NewSimpleDynamicClient(scheme.Scheme)
			fakeMapper := testutil.NewFakeRESTMapper(
				v1.SchemeGroupVersion.WithKind("Pod"),
				batchv1.SchemeGroupVersion.WithKind("Job"),
				schema.GroupVersion{Group: "rbac.authorization.k8s.io", Version: "v1"}.WithKind("ClusterRole"),
				v1.SchemeGroupVersion.WithKind("Namespace"),
			)
			sw := statusWaiter{
				client:     fakeClient,
				restMapper: fakeMapper,
			}
			objs := getRuntimeObjFromManifests(t, tt.objManifests)
			for _, obj := range objs {
				u := obj.(*unstructured.Unstructured)
				gvr := getGVR(t, fakeMapper, u)
				err := fakeClient.Tracker().Create(gvr, u, u.GetNamespace())
				assert.NoError(t, err)
			}

			if strings.Contains(tt.name, "delete") {
				timeUntilDelete := time.Millisecond * 500
				for _, obj := range objs {
					u := obj.(*unstructured.Unstructured)
					gvr := getGVR(t, fakeMapper, u)
					go func(gvr schema.GroupVersionResource, u *unstructured.Unstructured) {
						time.Sleep(timeUntilDelete)
						err := fakeClient.Tracker().Delete(gvr, u.GetNamespace(), u.GetName())
						assert.NoError(t, err)
					}(gvr, u)
				}
			}

			resourceList := getResourceListFromRuntimeObjs(t, c, objs)
			err := tt.testFunc(sw, resourceList, time.Second*3)
			if tt.expectErrs != nil {
				assert.EqualError(t, err, errors.Join(tt.expectErrs...).Error())
				return
			}
			assert.NoError(t, err)
		})
	}
}

type restrictedDynamicClient struct {
	dynamic.Interface
	allowedNamespaces          map[string]bool
	clusterScopedListAttempted bool
}

func newRestrictedDynamicClient(baseClient dynamic.Interface, allowedNamespaces []string) *restrictedDynamicClient {
	allowed := make(map[string]bool)
	for _, ns := range allowedNamespaces {
		allowed[ns] = true
	}
	return &restrictedDynamicClient{
		Interface:         baseClient,
		allowedNamespaces: allowed,
	}
}

func (r *restrictedDynamicClient) Resource(resource schema.GroupVersionResource) dynamic.NamespaceableResourceInterface {
	return &restrictedNamespaceableResource{
		NamespaceableResourceInterface: r.Interface.Resource(resource),
		allowedNamespaces:              r.allowedNamespaces,
		clusterScopedListAttempted:     &r.clusterScopedListAttempted,
	}
}

type restrictedNamespaceableResource struct {
	dynamic.NamespaceableResourceInterface
	allowedNamespaces          map[string]bool
	clusterScopedListAttempted *bool
}

func (r *restrictedNamespaceableResource) Namespace(ns string) dynamic.ResourceInterface {
	return &restrictedResource{
		ResourceInterface:          r.NamespaceableResourceInterface.Namespace(ns),
		namespace:                  ns,
		allowedNamespaces:          r.allowedNamespaces,
		clusterScopedListAttempted: r.clusterScopedListAttempted,
	}
}

func (r *restrictedNamespaceableResource) List(_ context.Context, _ metav1.ListOptions) (*unstructured.UnstructuredList, error) {
	*r.clusterScopedListAttempted = true
	return nil, apierrors.NewForbidden(
		schema.GroupResource{Resource: "pods"},
		"",
		fmt.Errorf("user does not have cluster-wide LIST permissions for cluster-scoped resources"),
	)
}

type restrictedResource struct {
	dynamic.ResourceInterface
	namespace                  string
	allowedNamespaces          map[string]bool
	clusterScopedListAttempted *bool
}

func (r *restrictedResource) List(ctx context.Context, opts metav1.ListOptions) (*unstructured.UnstructuredList, error) {
	if r.namespace == "" {
		*r.clusterScopedListAttempted = true
		return nil, apierrors.NewForbidden(
			schema.GroupResource{Resource: "pods"},
			"",
			fmt.Errorf("user does not have cluster-wide LIST permissions for cluster-scoped resources"),
		)
	}
	if !r.allowedNamespaces[r.namespace] {
		return nil, apierrors.NewForbidden(
			schema.GroupResource{Resource: "pods"},
			"",
			fmt.Errorf("user does not have LIST permissions in namespace %q", r.namespace),
		)
	}
	return r.ResourceInterface.List(ctx, opts)
}

func TestStatusWaitRestrictedRBAC(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name              string
		objManifests      []string
		allowedNamespaces []string
		expectErrs        []error
		testFunc          func(statusWaiter, ResourceList, time.Duration) error
	}{
		{
			name:              "pods in multiple namespaces with namespace permissions",
			objManifests:      []string{podNamespace1Manifest, podNamespace2Manifest},
			allowedNamespaces: []string{"namespace-1", "namespace-2"},
			testFunc: func(sw statusWaiter, rl ResourceList, timeout time.Duration) error {
				return sw.Wait(rl, timeout)
			},
		},
		{
			name:              "delete pods in multiple namespaces with namespace permissions",
			objManifests:      []string{podNamespace1Manifest, podNamespace2Manifest},
			allowedNamespaces: []string{"namespace-1", "namespace-2"},
			testFunc: func(sw statusWaiter, rl ResourceList, timeout time.Duration) error {
				return sw.WaitForDelete(rl, timeout)
			},
		},
		{
			name:              "hooks in multiple namespaces with namespace permissions",
			objManifests:      []string{jobNamespace1CompleteManifest, podNamespace2SucceededManifest},
			allowedNamespaces: []string{"namespace-1", "namespace-2"},
			testFunc: func(sw statusWaiter, rl ResourceList, timeout time.Duration) error {
				return sw.WatchUntilReady(rl, timeout)
			},
		},
		{
			name:              "error when cluster-scoped resource included",
			objManifests:      []string{podNamespace1Manifest, clusterRoleManifest},
			allowedNamespaces: []string{"namespace-1"},
			expectErrs:        []error{fmt.Errorf("user does not have cluster-wide LIST permissions for cluster-scoped resources")},
			testFunc: func(sw statusWaiter, rl ResourceList, timeout time.Duration) error {
				return sw.Wait(rl, timeout)
			},
		},
		{
			name:              "error when deleting cluster-scoped resource",
			objManifests:      []string{podNamespace1Manifest, namespaceManifest},
			allowedNamespaces: []string{"namespace-1"},
			expectErrs:        []error{fmt.Errorf("user does not have cluster-wide LIST permissions for cluster-scoped resources")},
			testFunc: func(sw statusWaiter, rl ResourceList, timeout time.Duration) error {
				return sw.WaitForDelete(rl, timeout)
			},
		},
		{
			name:              "error when accessing disallowed namespace",
			objManifests:      []string{podNamespace1Manifest, podNamespace2Manifest},
			allowedNamespaces: []string{"namespace-1"},
			expectErrs:        []error{fmt.Errorf("user does not have LIST permissions in namespace %q", "namespace-2")},
			testFunc: func(sw statusWaiter, rl ResourceList, timeout time.Duration) error {
				return sw.Wait(rl, timeout)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c := newTestClient(t)
			baseFakeClient := dynamicfake.NewSimpleDynamicClient(scheme.Scheme)
			fakeMapper := testutil.NewFakeRESTMapper(
				v1.SchemeGroupVersion.WithKind("Pod"),
				batchv1.SchemeGroupVersion.WithKind("Job"),
				schema.GroupVersion{Group: "rbac.authorization.k8s.io", Version: "v1"}.WithKind("ClusterRole"),
				v1.SchemeGroupVersion.WithKind("Namespace"),
			)
			restrictedClient := newRestrictedDynamicClient(baseFakeClient, tt.allowedNamespaces)
			sw := statusWaiter{
				client:     restrictedClient,
				restMapper: fakeMapper,
			}
			objs := getRuntimeObjFromManifests(t, tt.objManifests)
			for _, obj := range objs {
				u := obj.(*unstructured.Unstructured)
				gvr := getGVR(t, fakeMapper, u)
				err := baseFakeClient.Tracker().Create(gvr, u, u.GetNamespace())
				assert.NoError(t, err)
			}

			if strings.Contains(tt.name, "delet") {
				timeUntilDelete := time.Millisecond * 500
				for _, obj := range objs {
					u := obj.(*unstructured.Unstructured)
					gvr := getGVR(t, fakeMapper, u)
					go func(gvr schema.GroupVersionResource, u *unstructured.Unstructured) {
						time.Sleep(timeUntilDelete)
						err := baseFakeClient.Tracker().Delete(gvr, u.GetNamespace(), u.GetName())
						assert.NoError(t, err)
					}(gvr, u)
				}
			}

			resourceList := getResourceListFromRuntimeObjs(t, c, objs)
			err := tt.testFunc(sw, resourceList, time.Second*3)
			if tt.expectErrs != nil {
				require.Error(t, err)
				for _, expectedErr := range tt.expectErrs {
					assert.Contains(t, err.Error(), expectedErr.Error())
				}
				return
			}
			assert.NoError(t, err)
			assert.False(t, restrictedClient.clusterScopedListAttempted)
		})
	}
}

func TestStatusWaitMixedResources(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name              string
		objManifests      []string
		allowedNamespaces []string
		expectErrs        []error
		testFunc          func(statusWaiter, ResourceList, time.Duration) error
	}{
		{
			name:              "wait succeeds with namespace-scoped resources only",
			objManifests:      []string{podNamespace1Manifest, podNamespace2Manifest},
			allowedNamespaces: []string{"namespace-1", "namespace-2"},
			testFunc: func(sw statusWaiter, rl ResourceList, timeout time.Duration) error {
				return sw.Wait(rl, timeout)
			},
		},
		{
			name:              "wait fails when cluster-scoped resource included",
			objManifests:      []string{podNamespace1Manifest, clusterRoleManifest},
			allowedNamespaces: []string{"namespace-1"},
			expectErrs:        []error{fmt.Errorf("user does not have cluster-wide LIST permissions for cluster-scoped resources")},
			testFunc: func(sw statusWaiter, rl ResourceList, timeout time.Duration) error {
				return sw.Wait(rl, timeout)
			},
		},
		{
			name:              "waitForDelete fails when cluster-scoped resource included",
			objManifests:      []string{podNamespace1Manifest, clusterRoleManifest},
			allowedNamespaces: []string{"namespace-1"},
			expectErrs:        []error{fmt.Errorf("user does not have cluster-wide LIST permissions for cluster-scoped resources")},
			testFunc: func(sw statusWaiter, rl ResourceList, timeout time.Duration) error {
				return sw.WaitForDelete(rl, timeout)
			},
		},
		{
			name:              "wait fails when namespace resource included",
			objManifests:      []string{podNamespace1Manifest, namespaceManifest},
			allowedNamespaces: []string{"namespace-1"},
			expectErrs:        []error{fmt.Errorf("user does not have cluster-wide LIST permissions for cluster-scoped resources")},
			testFunc: func(sw statusWaiter, rl ResourceList, timeout time.Duration) error {
				return sw.Wait(rl, timeout)
			},
		},
		{
			name:              "error when accessing disallowed namespace",
			objManifests:      []string{podNamespace1Manifest, podNamespace2Manifest},
			allowedNamespaces: []string{"namespace-1"},
			expectErrs:        []error{fmt.Errorf("user does not have LIST permissions in namespace %q", "namespace-2")},
			testFunc: func(sw statusWaiter, rl ResourceList, timeout time.Duration) error {
				return sw.Wait(rl, timeout)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c := newTestClient(t)
			baseFakeClient := dynamicfake.NewSimpleDynamicClient(scheme.Scheme)
			fakeMapper := testutil.NewFakeRESTMapper(
				v1.SchemeGroupVersion.WithKind("Pod"),
				batchv1.SchemeGroupVersion.WithKind("Job"),
				schema.GroupVersion{Group: "rbac.authorization.k8s.io", Version: "v1"}.WithKind("ClusterRole"),
				v1.SchemeGroupVersion.WithKind("Namespace"),
			)
			restrictedClient := newRestrictedDynamicClient(baseFakeClient, tt.allowedNamespaces)
			sw := statusWaiter{
				client:     restrictedClient,
				restMapper: fakeMapper,
			}
			objs := getRuntimeObjFromManifests(t, tt.objManifests)
			for _, obj := range objs {
				u := obj.(*unstructured.Unstructured)
				gvr := getGVR(t, fakeMapper, u)
				err := baseFakeClient.Tracker().Create(gvr, u, u.GetNamespace())
				assert.NoError(t, err)
			}

			if strings.Contains(tt.name, "delet") {
				timeUntilDelete := time.Millisecond * 500
				for _, obj := range objs {
					u := obj.(*unstructured.Unstructured)
					gvr := getGVR(t, fakeMapper, u)
					go func(gvr schema.GroupVersionResource, u *unstructured.Unstructured) {
						time.Sleep(timeUntilDelete)
						err := baseFakeClient.Tracker().Delete(gvr, u.GetNamespace(), u.GetName())
						assert.NoError(t, err)
					}(gvr, u)
				}
			}

			resourceList := getResourceListFromRuntimeObjs(t, c, objs)
			err := tt.testFunc(sw, resourceList, time.Second*3)
			if tt.expectErrs != nil {
				require.Error(t, err)
				for _, expectedErr := range tt.expectErrs {
					assert.Contains(t, err.Error(), expectedErr.Error())
				}
				return
			}
			assert.NoError(t, err)
			assert.False(t, restrictedClient.clusterScopedListAttempted)
		})
	}
}
