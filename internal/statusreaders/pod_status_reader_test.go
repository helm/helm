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

package statusreaders

import (
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/fluxcd/cli-utils/pkg/kstatus/status"
)

func TestPodConditions(t *testing.T) {
	tests := []struct {
		name           string
		pod            *v1.Pod
		expectedStatus status.Status
	}{
		{
			name: "pod without status returns in progress",
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "pod-no-status"},
				Spec:       v1.PodSpec{},
				Status:     v1.PodStatus{},
			},
			expectedStatus: status.InProgressStatus,
		},
		{
			name: "pod succeeded returns current status",
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "pod-succeeded"},
				Spec:       v1.PodSpec{},
				Status: v1.PodStatus{
					Phase: v1.PodSucceeded,
				},
			},
			expectedStatus: status.CurrentStatus,
		},
		{
			name: "pod failed returns failed status",
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "pod-failed"},
				Spec:       v1.PodSpec{},
				Status: v1.PodStatus{
					Phase: v1.PodFailed,
				},
			},
			expectedStatus: status.FailedStatus,
		},
		{
			name: "pod pending returns in progress status",
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "pod-pending"},
				Spec:       v1.PodSpec{},
				Status: v1.PodStatus{
					Phase: v1.PodPending,
				},
			},
			expectedStatus: status.InProgressStatus,
		},
		{
			name: "pod running returns in progress status",
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "pod-running"},
				Spec:       v1.PodSpec{},
				Status: v1.PodStatus{
					Phase: v1.PodRunning,
				},
			},
			expectedStatus: status.InProgressStatus,
		},
		{
			name: "pod with unknown phase returns in progress status",
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "pod-unknown"},
				Spec:       v1.PodSpec{},
				Status: v1.PodStatus{
					Phase: v1.PodUnknown,
				},
			},
			expectedStatus: status.InProgressStatus,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			us, err := toUnstructured(t, tc.pod)
			assert.NoError(t, err)
			result, err := podConditions(us)
			assert.NoError(t, err)
			assert.Equal(t, tc.expectedStatus, result.Status)
		})
	}
}
