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
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/yaml"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/kubectl/pkg/scheme"
	"sigs.k8s.io/cli-utils/pkg/kstatus/watcher"
	"sigs.k8s.io/cli-utils/pkg/testutil"
)

var podCurrentYaml = `
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

var podYaml = `
apiVersion: v1
kind: Pod
metadata:
  name: in-progress-pod
  namespace: ns
`

func TestRunHealthChecks(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		podYamls   []string
		expectErrs []error
	}{
		{
			name:       "Pod is ready",
			podYamls:   []string{podCurrentYaml},
			expectErrs: nil,
		},
		{
			name:     "one of the pods never becomes ready",
			podYamls: []string{podYaml, podCurrentYaml},
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
			)
			pods := []runtime.Object{}
			statusWatcher := watcher.NewDefaultStatusWatcher(fakeClient, fakeMapper)
			for _, podYaml := range tt.podYamls {
				m := make(map[string]interface{})
				err := yaml.Unmarshal([]byte(podYaml), &m)
				require.NoError(t, err)
				pod := &unstructured.Unstructured{Object: m}
				pods = append(pods, pod)
				fmt.Println(pod.GetName())
				podGVR := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
				err = fakeClient.Tracker().Create(podGVR, pod, pod.GetNamespace())
				require.NoError(t, err)
			}
			c.Waiter = &kstatusWaiter{
				sw:  statusWatcher,
				log: c.Log,
			}

			resourceList := ResourceList{}
			for _, pod := range pods {
				list, err := c.Build(objBody(pod), false)
				if err != nil {
					t.Fatal(err)
				}
				resourceList = append(resourceList, list...)
			}

			err := c.Wait(resourceList, time.Second*5)
			if tt.expectErrs != nil {
				require.EqualError(t, err, errors.Join(tt.expectErrs...).Error())
				return
			}
			require.NoError(t, err)
		})
	}
}
