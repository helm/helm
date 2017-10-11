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

package portforwarder

import (
	"testing"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func mockTillerPod() v1.Pod {
	return v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "orca",
			Namespace: v1.NamespaceDefault,
			Labels:    tillerPodLabels,
		},
		Status: v1.PodStatus{
			Phase: v1.PodRunning,
			Conditions: []v1.PodCondition{
				{
					Status: v1.ConditionTrue,
					Type:   v1.PodReady,
				},
			},
		},
	}
}

func mockTillerPodPending() v1.Pod {
	p := mockTillerPod()
	p.Name = "blue"
	p.Status.Conditions[0].Status = v1.ConditionFalse
	return p
}

func TestGetFirstPod(t *testing.T) {
	tests := []struct {
		name     string
		pods     []v1.Pod
		expected string
		err      bool
	}{
		{
			name:     "with a ready pod",
			pods:     []v1.Pod{mockTillerPod()},
			expected: "orca",
		},
		{
			name: "without a ready pod",
			pods: []v1.Pod{mockTillerPodPending()},
			err:  true,
		},
		{
			name: "without a pod",
			pods: []v1.Pod{},
			err:  true,
		},
	}

	for _, tt := range tests {
		client := fake.NewSimpleClientset(&v1.PodList{Items: tt.pods})
		name, err := getTillerPodName(client.Core(), v1.NamespaceDefault)
		if (err != nil) != tt.err {
			t.Errorf("%q. expected error: %v, got %v", tt.name, tt.err, err)
		}
		if name != tt.expected {
			t.Errorf("%q. expected %q, got %q", tt.name, tt.expected, name)
		}
	}
}
