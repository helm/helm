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
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
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

func getGVR(t *testing.T, mapper meta.RESTMapper, obj *unstructured.Unstructured) schema.GroupVersionResource {
	gvk := obj.GroupVersionKind()
	mapping, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	require.NoError(t, err)
	return mapping.Resource
}

func TestKWaitJob(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		objYamls   []string
		expectErrs []error
	}{
		{
			name:       "Job is complete",
			objYamls:   []string{jobComplete},
			expectErrs: nil,
		},
		{
			name:       "Job is not complete",
			objYamls:   []string{jobNoStatus},
			expectErrs: []error{errors.New("not all resources ready: context deadline exceeded: test: Job not ready, status: InProgress")},
		},
		{
			name:       "Pod is ready",
			objYamls:   []string{podCurrent},
			expectErrs: nil,
		},
		{
			name:     "one of the pods never becomes ready",
			objYamls: []string{podNoStatus, podCurrent},
			// TODO, make this better
			expectErrs: []error{errors.New("not all resources ready: context deadline exceeded: in-progress-pod: Pod not ready, status: InProgress")},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c := newTestClient(t)
			fakeClient := dynamicfake.NewSimpleDynamicClient(scheme.Scheme)
			fakeMapper := testutil.NewFakeRESTMapper(
				v1.SchemeGroupVersion.WithKind("Pod"),
				schema.GroupVersionKind{
					Group:   "batch",
					Version: "v1",
					Kind:    "Job",
				},
			)
			objs := []runtime.Object{}
			statusWatcher := watcher.NewDefaultStatusWatcher(fakeClient, fakeMapper)
			for _, podYaml := range tt.objYamls {
				m := make(map[string]interface{})
				err := yaml.Unmarshal([]byte(podYaml), &m)
				require.NoError(t, err)
				resource := &unstructured.Unstructured{Object: m}
				objs = append(objs, resource)
				gvr := getGVR(t, fakeMapper, resource)
				err = fakeClient.Tracker().Create(gvr, resource, resource.GetNamespace())
				require.NoError(t, err)
			}
			c.Waiter = &kstatusWaiter{
				sw:  statusWatcher,
				log: c.Log,
			}

			resourceList := ResourceList{}
			for _, obj := range objs {
				list, err := c.Build(objBody(obj), false)
				if err != nil {
					t.Fatal(err)
				}
				resourceList = append(resourceList, list...)
			}

			err := c.Wait(resourceList, time.Second*3)
			if tt.expectErrs != nil {
        //TODO remove require
				require.EqualError(t, err, errors.Join(tt.expectErrs...).Error())
				return
			}
			require.NoError(t, err)
		})
	}
}
