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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/yaml"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/kubectl/pkg/scheme"
	"sigs.k8s.io/cli-utils/pkg/kstatus/watcher"
	"sigs.k8s.io/cli-utils/pkg/testutil"
)

var podCurrent = `
apiVersion: v1
kind: Pod
metadata:
  name: good-pod
  namespace: ns
status:
  conditions:
  - type: Ready
    status: "True"
  phase: Running
`

var podNoStatus = `
apiVersion: v1
kind: Pod
metadata:
  name: in-progress-pod
  namespace: ns
`

var jobNoStatus = `
apiVersion: batch/v1
kind: Job
metadata:
   name: test
   namespace: qual
   generation: 1
`

var jobComplete = `
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

var pausedDeploymentYaml = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx
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

func getGVR(t *testing.T, mapper meta.RESTMapper, obj *unstructured.Unstructured) schema.GroupVersionResource {
	gvk := obj.GroupVersionKind()
	mapping, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	require.NoError(t, err)
	return mapping.Resource
}
func testLogger(message string, args ...interface{}) {
	fmt.Printf(message, args...)
}

func TestStatusWaitForDelete(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		objToCreate []string
		toDelete    []string
		expectErrs  []error
	}{
		{
			name:        "wait for pod to be deleted",
			objToCreate: []string{podCurrent},
			toDelete:    []string{podCurrent},
			expectErrs:  nil,
		},
		{
			name:        "error when not all objects are deleted",
			objToCreate: []string{jobComplete, podCurrent},
			toDelete:    []string{jobComplete},
			expectErrs:  []error{errors.New("resource still exists, name: good-pod, kind: Pod, status: Current"), errors.New("context deadline exceeded")},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c := newTestClient(t)
			timeout := time.Second * 3
			timeToDeletePod := time.Second * 2
			fakeClient := dynamicfake.NewSimpleDynamicClient(scheme.Scheme)
			fakeMapper := testutil.NewFakeRESTMapper(
				v1.SchemeGroupVersion.WithKind("Pod"),
				appsv1.SchemeGroupVersion.WithKind("Deployment"),
				batchv1.SchemeGroupVersion.WithKind("Job"),
			)
			statusWatcher := watcher.NewDefaultStatusWatcher(fakeClient, fakeMapper)
			statusWaiter := statusWaiter{
				sw:  statusWatcher,
				log: testLogger,
			}
			createdObjs := []runtime.Object{}
			for _, objYaml := range tt.objToCreate {
				m := make(map[string]interface{})
				err := yaml.Unmarshal([]byte(objYaml), &m)
				assert.NoError(t, err)
				resource := &unstructured.Unstructured{Object: m}
				createdObjs = append(createdObjs, resource)
				gvr := getGVR(t, fakeMapper, resource)
				err = fakeClient.Tracker().Create(gvr, resource, resource.GetNamespace())
				assert.NoError(t, err)
			}
			for _, objYaml := range tt.toDelete {
				m := make(map[string]interface{})
				err := yaml.Unmarshal([]byte(objYaml), &m)
				assert.NoError(t, err)
				resource := &unstructured.Unstructured{Object: m}
				gvr := getGVR(t, fakeMapper, resource)
				go func() {
					time.Sleep(timeToDeletePod)
					err = fakeClient.Tracker().Delete(gvr, resource.GetNamespace(), resource.GetName())
					assert.NoError(t, err)
				}()
			}
			resourceList := ResourceList{}
			for _, obj := range createdObjs {
				list, err := c.Build(objBody(obj), false)
				assert.NoError(t, err)
				resourceList = append(resourceList, list...)
			}
			err := statusWaiter.WaitForDelete(resourceList, timeout)
			if tt.expectErrs != nil {
				assert.EqualError(t, err, errors.Join(tt.expectErrs...).Error())
				return
			}
			assert.NoError(t, err)
		})
	}

}

func TestStatusWait(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		objYamls    []string
		expectErrs  []error
		waitForJobs bool
	}{
		{
			name:       "Job is complete",
			objYamls:   []string{jobComplete},
			expectErrs: nil,
		},
		{
			name:        "Job is not complete",
			objYamls:    []string{jobNoStatus},
			expectErrs:  []error{errors.New("resource not ready, name: test, kind: Job, status: InProgress"), errors.New("context deadline exceeded")},
			waitForJobs: true,
		},
		{
			name:        "Job is not ready, but we pass wait anyway",
			objYamls:    []string{jobNoStatus},
			expectErrs:  nil,
			waitForJobs: false,
		},
		{
			name:       "Pod is ready",
			objYamls:   []string{podCurrent},
			expectErrs: nil,
		},
		{
			name:       "one of the pods never becomes ready",
			objYamls:   []string{podNoStatus, podCurrent},
			expectErrs: []error{errors.New("resource not ready, name: in-progress-pod, kind: Pod, status: InProgress"), errors.New("context deadline exceeded")},
		},
		{
			name:       "paused deployment passes",
			objYamls:   []string{pausedDeploymentYaml},
			expectErrs: nil,
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
			statusWatcher := watcher.NewDefaultStatusWatcher(fakeClient, fakeMapper)
			statusWaiter := statusWaiter{
				sw:  statusWatcher,
				log: testLogger,
			}
			objs := []runtime.Object{}

			for _, podYaml := range tt.objYamls {
				m := make(map[string]interface{})
				err := yaml.Unmarshal([]byte(podYaml), &m)
				assert.NoError(t, err)
				resource := &unstructured.Unstructured{Object: m}
				objs = append(objs, resource)
				gvr := getGVR(t, fakeMapper, resource)
				err = fakeClient.Tracker().Create(gvr, resource, resource.GetNamespace())
				assert.NoError(t, err)
			}
			resourceList := ResourceList{}
			for _, obj := range objs {
				list, err := c.Build(objBody(obj), false)
				assert.NoError(t, err)
				resourceList = append(resourceList, list...)
			}

			ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
			defer cancel()
			err := statusWaiter.wait(ctx, resourceList, tt.waitForJobs)
			if tt.expectErrs != nil {
				assert.EqualError(t, err, errors.Join(tt.expectErrs...).Error())
				return
			}
			assert.NoError(t, err)
		})
	}
}
