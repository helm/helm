/*
Copyright 2016 The Kubernetes Authors All rights reserved.

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

package main

import (
	"testing"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/unversioned/testclient"
	"k8s.io/kubernetes/pkg/runtime"
)

func mockTillerPod() api.Pod {
	return api.Pod{
		ObjectMeta: api.ObjectMeta{
			Name:      "orca",
			Namespace: api.NamespaceDefault,
			Labels:    map[string]string{"app": "helm", "name": "tiller"},
		},
		Status: api.PodStatus{
			Phase: api.PodRunning,
			Conditions: []api.PodCondition{
				{
					Status: api.ConditionTrue,
					Type:   api.PodReady,
				},
			},
		},
	}
}

func mockTillerPodPending() api.Pod {
	p := mockTillerPod()
	p.Name = "blue"
	p.Status.Conditions[0].Status = api.ConditionFalse
	return p
}

func TestGetFirstPod(t *testing.T) {
	tests := []struct {
		name     string
		pods     []api.Pod
		expected string
		err      bool
	}{
		{
			name:     "with a ready pod",
			pods:     []api.Pod{mockTillerPod()},
			expected: "orca",
		},
		{
			name: "without a ready pod",
			pods: []api.Pod{mockTillerPodPending()},
			err:  true,
		},
		{
			name: "without a pod",
			pods: []api.Pod{},
			err:  true,
		},
	}

	for _, tt := range tests {
		client := &testclient.Fake{}
		client.PrependReactor("list", "pods", func(action testclient.Action) (handled bool, ret runtime.Object, err error) {
			return true, &api.PodList{Items: tt.pods}, nil
		})

		name, err := getTillerPodName(client, api.NamespaceDefault)
		if (err != nil) != tt.err {
			t.Errorf("%q. expected error: %v, got %v", tt.name, tt.err, err)
		}
		if name != tt.expected {
			t.Errorf("%q. expected %q, got %q", tt.name, tt.expected, name)
		}
	}
}
