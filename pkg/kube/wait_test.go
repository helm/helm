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

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	appsv1beta1 "k8s.io/api/apps/v1beta1"
	appsv1beta2 "k8s.io/api/apps/v1beta2"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/resource"
)

func TestSelectorsForObject(t *testing.T) {
	tests := []struct {
		name           string
		object         interface{}
		expectError    bool
		errorContains  string
		expectedLabels map[string]string
	}{
		{
			name: "appsv1 ReplicaSet",
			object: &appsv1.ReplicaSet{
				Spec: appsv1.ReplicaSetSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"app": "test"},
					},
				},
			},
			expectError:    false,
			expectedLabels: map[string]string{"app": "test"},
		},
		{
			name: "extensionsv1beta1 ReplicaSet",
			object: &extensionsv1beta1.ReplicaSet{
				Spec: extensionsv1beta1.ReplicaSetSpec{
					Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "ext-rs"}},
				},
			},
			expectedLabels: map[string]string{"app": "ext-rs"},
		},
		{
			name: "appsv1beta2 ReplicaSet",
			object: &appsv1beta2.ReplicaSet{
				Spec: appsv1beta2.ReplicaSetSpec{
					Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "beta2-rs"}},
				},
			},
			expectedLabels: map[string]string{"app": "beta2-rs"},
		},
		{
			name: "corev1 ReplicationController",
			object: &corev1.ReplicationController{
				Spec: corev1.ReplicationControllerSpec{
					Selector: map[string]string{"rc": "test"},
				},
			},
			expectError:    false,
			expectedLabels: map[string]string{"rc": "test"},
		},
		{
			name: "appsv1 StatefulSet",
			object: &appsv1.StatefulSet{
				Spec: appsv1.StatefulSetSpec{
					Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "statefulset-v1"}},
				},
			},
			expectedLabels: map[string]string{"app": "statefulset-v1"},
		},
		{
			name: "appsv1beta1 StatefulSet",
			object: &appsv1beta1.StatefulSet{
				Spec: appsv1beta1.StatefulSetSpec{
					Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "statefulset-beta1"}},
				},
			},
			expectedLabels: map[string]string{"app": "statefulset-beta1"},
		},
		{
			name: "appsv1beta2 StatefulSet",
			object: &appsv1beta2.StatefulSet{
				Spec: appsv1beta2.StatefulSetSpec{
					Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "statefulset-beta2"}},
				},
			},
			expectedLabels: map[string]string{"app": "statefulset-beta2"},
		},
		{
			name: "extensionsv1beta1 DaemonSet",
			object: &extensionsv1beta1.DaemonSet{
				Spec: extensionsv1beta1.DaemonSetSpec{
					Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "daemonset-ext-beta1"}},
				},
			},
			expectedLabels: map[string]string{"app": "daemonset-ext-beta1"},
		},
		{
			name: "appsv1 DaemonSet",
			object: &appsv1.DaemonSet{
				Spec: appsv1.DaemonSetSpec{
					Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "daemonset-v1"}},
				},
			},
			expectedLabels: map[string]string{"app": "daemonset-v1"},
		},
		{
			name: "appsv1beta2 DaemonSet",
			object: &appsv1beta2.DaemonSet{
				Spec: appsv1beta2.DaemonSetSpec{
					Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "daemonset-beta2"}},
				},
			},
			expectedLabels: map[string]string{"app": "daemonset-beta2"},
		},
		{
			name: "extensionsv1beta1 Deployment",
			object: &extensionsv1beta1.Deployment{
				Spec: extensionsv1beta1.DeploymentSpec{
					Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "deployment-ext-beta1"}},
				},
			},
			expectedLabels: map[string]string{"app": "deployment-ext-beta1"},
		},
		{
			name: "appsv1 Deployment",
			object: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "deployment-v1"}},
				},
			},
			expectedLabels: map[string]string{"app": "deployment-v1"},
		},
		{
			name: "appsv1beta1 Deployment",
			object: &appsv1beta1.Deployment{
				Spec: appsv1beta1.DeploymentSpec{
					Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "deployment-beta1"}},
				},
			},
			expectedLabels: map[string]string{"app": "deployment-beta1"},
		},
		{
			name: "appsv1beta2 Deployment",
			object: &appsv1beta2.Deployment{
				Spec: appsv1beta2.DeploymentSpec{
					Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "deployment-beta2"}},
				},
			},
			expectedLabels: map[string]string{"app": "deployment-beta2"},
		},
		{
			name: "batchv1 Job",
			object: &batchv1.Job{
				Spec: batchv1.JobSpec{
					Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"job": "batch-job"}},
				},
			},
			expectedLabels: map[string]string{"job": "batch-job"},
		},
		{
			name: "corev1 Service with selector",
			object: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{Name: "svc"},
				Spec: corev1.ServiceSpec{
					Selector: map[string]string{"svc": "yes"},
				},
			},
			expectError:    false,
			expectedLabels: map[string]string{"svc": "yes"},
		},
		{
			name: "corev1 Service without selector",
			object: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{Name: "svc"},
				Spec:       corev1.ServiceSpec{Selector: map[string]string{}},
			},
			expectError:   true,
			errorContains: "invalid service 'svc': Service is defined without a selector",
		},
		{
			name: "invalid label selector",
			object: &appsv1.ReplicaSet{
				Spec: appsv1.ReplicaSetSpec{
					Selector: &metav1.LabelSelector{
						MatchExpressions: []metav1.LabelSelectorRequirement{
							{
								Key:      "foo",
								Operator: "InvalidOperator",
								Values:   []string{"bar"},
							},
						},
					},
				},
			},
			expectError:   true,
			errorContains: "invalid label selector:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			selector, err := SelectorsForObject(tt.object.(runtime.Object))
			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorContains)
			} else {
				assert.NoError(t, err)
				expected := labels.Set(tt.expectedLabels)
				assert.True(t, selector.Matches(expected), "expected selector to match")
			}
		})
	}
}

func TestLegacyWaiter_waitForPodSuccess(t *testing.T) {
	lw := &legacyWaiter{}

	tests := []struct {
		name       string
		obj        runtime.Object
		wantDone   bool
		wantErr    bool
		errMessage string
	}{
		{
			name: "pod succeeded",
			obj: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "pod1"},
				Status:     corev1.PodStatus{Phase: corev1.PodSucceeded},
			},
			wantDone: true,
			wantErr:  false,
		},
		{
			name: "pod failed",
			obj: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "pod2"},
				Status:     corev1.PodStatus{Phase: corev1.PodFailed},
			},
			wantDone:   true,
			wantErr:    true,
			errMessage: "pod pod2 failed",
		},
		{
			name: "pod pending",
			obj: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "pod3"},
				Status:     corev1.PodStatus{Phase: corev1.PodPending},
			},
			wantDone: false,
			wantErr:  false,
		},
		{
			name: "pod running",
			obj: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "pod4"},
				Status:     corev1.PodStatus{Phase: corev1.PodRunning},
			},
			wantDone: false,
			wantErr:  false,
		},
		{
			name:       "wrong object type",
			obj:        &metav1.Status{},
			wantDone:   true,
			wantErr:    true,
			errMessage: "expected foo to be a *v1.Pod, got *v1.Status",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			done, err := lw.waitForPodSuccess(tt.obj, "foo")
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got none")
				} else if !strings.Contains(err.Error(), tt.errMessage) {
					t.Errorf("expected error to contain %q, got %q", tt.errMessage, err.Error())
				}
			} else if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if done != tt.wantDone {
				t.Errorf("got done=%v, want %v", done, tt.wantDone)
			}
		})
	}
}

func TestLegacyWaiter_waitForJob(t *testing.T) {
	lw := &legacyWaiter{}

	tests := []struct {
		name       string
		obj        runtime.Object
		wantDone   bool
		wantErr    bool
		errMessage string
	}{
		{
			name: "job complete",
			obj: &batchv1.Job{
				Status: batchv1.JobStatus{
					Conditions: []batchv1.JobCondition{
						{
							Type:   batchv1.JobComplete,
							Status: "True",
						},
					},
				},
			},
			wantDone: true,
			wantErr:  false,
		},
		{
			name: "job failed",
			obj: &batchv1.Job{
				Status: batchv1.JobStatus{
					Conditions: []batchv1.JobCondition{
						{
							Type:   batchv1.JobFailed,
							Status: "True",
							Reason: "FailedReason",
						},
					},
				},
			},
			wantDone:   true,
			wantErr:    true,
			errMessage: "job test-job failed: FailedReason",
		},
		{
			name: "job in progress",
			obj: &batchv1.Job{
				Status: batchv1.JobStatus{
					Active:    1,
					Failed:    0,
					Succeeded: 0,
					Conditions: []batchv1.JobCondition{
						{
							Type:   batchv1.JobComplete,
							Status: "False",
						},
						{
							Type:   batchv1.JobFailed,
							Status: "False",
						},
					},
				},
			},
			wantDone: false,
			wantErr:  false,
		},
		{
			name:       "wrong object type",
			obj:        &metav1.Status{},
			wantDone:   true,
			wantErr:    true,
			errMessage: "expected test-job to be a *batch.Job, got *v1.Status",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			done, err := lw.waitForJob(tt.obj, "test-job")
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got none")
				} else if !strings.Contains(err.Error(), tt.errMessage) {
					t.Errorf("expected error to contain %q, got %q", tt.errMessage, err.Error())
				}
			} else if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if done != tt.wantDone {
				t.Errorf("got done=%v, want %v", done, tt.wantDone)
			}
		})
	}
}

func TestLegacyWaiter_isRetryableError(t *testing.T) {
	lw := &legacyWaiter{}

	info := &resource.Info{
		Name: "test-resource",
	}

	tests := []struct {
		name        string
		err         error
		wantRetry   bool
		description string
	}{
		{
			name:      "nil error",
			err:       nil,
			wantRetry: false,
		},
		{
			name:      "status error - 0 code",
			err:       &apierrors.StatusError{ErrStatus: metav1.Status{Code: 0}},
			wantRetry: true,
		},
		{
			name:      "status error - 429 (TooManyRequests)",
			err:       &apierrors.StatusError{ErrStatus: metav1.Status{Code: http.StatusTooManyRequests}},
			wantRetry: true,
		},
		{
			name:      "status error - 503",
			err:       &apierrors.StatusError{ErrStatus: metav1.Status{Code: http.StatusServiceUnavailable}},
			wantRetry: true,
		},
		{
			name:      "status error - 501 (NotImplemented)",
			err:       &apierrors.StatusError{ErrStatus: metav1.Status{Code: http.StatusNotImplemented}},
			wantRetry: false,
		},
		{
			name:      "status error - 400 (Bad Request)",
			err:       &apierrors.StatusError{ErrStatus: metav1.Status{Code: http.StatusBadRequest}},
			wantRetry: false,
		},
		{
			name:      "non-status error",
			err:       fmt.Errorf("some generic error"),
			wantRetry: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := lw.isRetryableError(tt.err, info)
			if got != tt.wantRetry {
				t.Errorf("isRetryableError() = %v, want %v", got, tt.wantRetry)
			}
		})
	}
}
