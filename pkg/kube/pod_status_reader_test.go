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

package kube

// This file was copied and modified from https://github.com/fluxcd/kustomize-controller/blob/main/internal/statusreaders/job.go
import (
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/cli-utils/pkg/kstatus/status"
)

func TestPodConditions(t *testing.T) {
	t.Parallel()

	//TODO add some more tests here and parallelize

	t.Run("pod without status returns in progress", func(t *testing.T) {
		pod := &v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pod",
			},
			Spec:   v1.PodSpec{},
			Status: v1.PodStatus{},
		}
		us, err := toUnstructured(pod)
		assert.NoError(t, err)
		result, err := podConditions(us)
		assert.NoError(t, err)
		assert.Equal(t, status.InProgressStatus, result.Status)
	})

	t.Run("pod succeeded returns Current status", func(t *testing.T) {
		pod := &v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pod",
			},
			Spec:   v1.PodSpec{},
			Status: v1.PodStatus{
				Phase: v1.PodSucceeded,
			},
		}
		us, err := toUnstructured(pod)
		assert.NoError(t, err)
		result, err := podConditions(us)
		assert.NoError(t, err)
		assert.Equal(t, status.CurrentStatus, result.Status)
	})
}
