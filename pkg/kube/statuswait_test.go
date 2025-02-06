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
	"sigs.k8s.io/cli-utils/pkg/kstatus/polling/collector"
	"sigs.k8s.io/cli-utils/pkg/kstatus/polling/event"
	"sigs.k8s.io/cli-utils/pkg/kstatus/status"
	"sigs.k8s.io/cli-utils/pkg/object"
	"sigs.k8s.io/cli-utils/pkg/testutil"
)

var podCurrentManifest = `
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
  name: sleep-job
  namespace: default
  uid: 5e7d8814-36fc-486f-9e6d-5b0a09351682
  resourceVersion: "568"
  generation: 1
  creationTimestamp: 2025-02-06T16:34:20-05:00
  labels:
    batch.kubernetes.io/controller-uid: 5e7d8814-36fc-486f-9e6d-5b0a09351682
    batch.kubernetes.io/job-name: sleep-job
    controller-uid: 5e7d8814-36fc-486f-9e6d-5b0a09351682
    job-name: sleep-job
  annotations:
    kubectl.kubernetes.io/last-applied-configuration: "{\"apiVersion\":\"batch/v1\",\"kind\":\"Job\",\"metadata\":{\"annotations\":{},\"name\":\"sleep-job\",\"namespace\":\"default\"},\"spec\":{\"template\":{\"metadata\":{\"name\":\"sleep-job\"},\"spec\":{\"containers\":[{\"command\":[\"sh\",\"-c\",\"sleep 100\"],\"image\":\"busybox\",\"name\":\"sleep\"}],\"restartPolicy\":\"Never\"}}}}\n"
  managedFields:
  - manager: kubectl-client-side-apply
    operation: Update
    apiVersion: batch/v1
    time: 2025-02-06T16:34:20-05:00
    fieldsType: FieldsV1
    fieldsV1: {}
  - manager: k3s
    operation: Update
    apiVersion: batch/v1
    time: 2025-02-06T16:34:23-05:00
    fieldsType: FieldsV1
    fieldsV1: {}
    subresource: status
spec:
  parallelism: 1
  completions: 1
  backoffLimit: 6
  selector:
    matchLabels:
      batch.kubernetes.io/controller-uid: 5e7d8814-36fc-486f-9e6d-5b0a09351682
  manualSelector: false
  template:
    metadata:
      name: sleep-job
      labels:
        batch.kubernetes.io/controller-uid: 5e7d8814-36fc-486f-9e6d-5b0a09351682
        batch.kubernetes.io/job-name: sleep-job
        controller-uid: 5e7d8814-36fc-486f-9e6d-5b0a09351682
        job-name: sleep-job
    spec:
      containers:
      - name: sleep
        image: busybox
        command:
        - sh
        - -c
        - sleep 100
        terminationMessagePath: /dev/termination-log
        terminationMessagePolicy: File
        imagePullPolicy: Always
      restartPolicy: Never
      terminationGracePeriodSeconds: 30
      dnsPolicy: ClusterFirst
      securityContext: {}
      schedulerName: default-scheduler
  completionMode: NonIndexed
  suspend: false
  podReplacementPolicy: TerminatingOrFailed
status:
  startTime: 2025-02-06T16:34:20-05:00
  active: 1
  terminating: 0
  uncountedTerminatedPods: {}
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

var pausedDeploymentManifest = `
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

func TestStatusLogger(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*1500)
	defer cancel()
	readyPod := object.ObjMetadata{
		Name:      "readyPod",
		GroupKind: schema.GroupKind{Kind: "Pod"},
	}
	notReadyPod := object.ObjMetadata{
		Name:      "notReadyPod",
		GroupKind: schema.GroupKind{Kind: "Pod"},
	}
	objs := []object.ObjMetadata{readyPod, notReadyPod}
	resourceStatusCollector := collector.NewResourceStatusCollector(objs)
	resourceStatusCollector.ResourceStatuses[readyPod] = &event.ResourceStatus{
		Identifier: readyPod,
		Status:     status.CurrentStatus,
	}
	expectedMessage := "waiting for resource, name: notReadyPod, kind: Pod, desired status: Current, actual status: Unknown"
	testLogger := func(message string, args ...interface{}) {
		assert.Equal(t, expectedMessage, fmt.Sprintf(message, args...))
	}
	logResourceStatus(ctx, objs, resourceStatusCollector, status.CurrentStatus, testLogger)
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
			expectErrs:        []error{errors.New("resource still exists, name: good-pod, kind: Pod, status: Current"), errors.New("context deadline exceeded")},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c := newTestClient(t)
			timeout := time.Second * 2
			timeUntilPodDelete := time.Second * 1
			fakeClient := dynamicfake.NewSimpleDynamicClient(scheme.Scheme)
			fakeMapper := testutil.NewFakeRESTMapper(
				v1.SchemeGroupVersion.WithKind("Pod"),
				appsv1.SchemeGroupVersion.WithKind("Deployment"),
				batchv1.SchemeGroupVersion.WithKind("Job"),
			)
			statusWaiter := statusWaiter{
				restMapper: fakeMapper,
				client:     fakeClient,
				log:        t.Logf,
			}
			createdObjs := []runtime.Object{}
			for _, manifest := range tt.manifestsToCreate {
				m := make(map[string]interface{})
				err := yaml.Unmarshal([]byte(manifest), &m)
				assert.NoError(t, err)
				resource := &unstructured.Unstructured{Object: m}
				createdObjs = append(createdObjs, resource)
				gvr := getGVR(t, fakeMapper, resource)
				err = fakeClient.Tracker().Create(gvr, resource, resource.GetNamespace())
				assert.NoError(t, err)
			}
			for _, manifest := range tt.manifestsToDelete {
				m := make(map[string]interface{})
				err := yaml.Unmarshal([]byte(manifest), &m)
				assert.NoError(t, err)
				resource := &unstructured.Unstructured{Object: m}
				gvr := getGVR(t, fakeMapper, resource)
				go func() {
					time.Sleep(timeUntilPodDelete)
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
		name         string
		objManifests []string
		expectErrs   []error
		waitForJobs  bool
	}{
		{
			name:         "Job is complete",
			objManifests: []string{jobCompleteManifest},
			expectErrs:   nil,
		},
		{
			name:         "Job is not complete",
			objManifests: []string{jobNoStatusManifest},
			expectErrs:   []error{errors.New("resource not ready, name: test, kind: Job, status: InProgress"), errors.New("context deadline exceeded")},
			waitForJobs:  true,
		},
		{
			name:         "Job is not ready but we pass wait anyway",
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
				log:        t.Logf,
			}
			objs := []runtime.Object{}

			for _, podYaml := range tt.objManifests {
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

			var err error
			if tt.waitForJobs {
				err = statusWaiter.WaitWithJobs(resourceList, time.Second*3)
			} else {				
				err = statusWaiter.Wait(resourceList, time.Second*3)
			}
			if tt.expectErrs != nil {
				assert.EqualError(t, err, errors.Join(tt.expectErrs...).Error())
				return
			}
			assert.NoError(t, err)
		})
	}
}
