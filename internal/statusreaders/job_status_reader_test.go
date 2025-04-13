/*
Copyright The Helm Authors.
This file was initially copied and modified from
    https://github.com/fluxcd/kustomize-controller/blob/main/internal/statusreaders/job_test.go
Copyright 2022 The Flux authors

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
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/fluxcd/cli-utils/pkg/kstatus/status"
)

func toUnstructured(t *testing.T, obj runtime.Object) (*unstructured.Unstructured, error) {
	t.Helper()
	// If the incoming object is already unstructured, perform a deep copy first
	// otherwise DefaultUnstructuredConverter ends up returning the inner map without
	// making a copy.
	if _, ok := obj.(runtime.Unstructured); ok {
		obj = obj.DeepCopyObject()
	}
	rawMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return nil, err
	}
	return &unstructured.Unstructured{Object: rawMap}, nil
}

func TestJobConditions(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name           string
		job            *batchv1.Job
		expectedStatus status.Status
	}{
		{
			name: "job without Complete condition returns InProgress status",
			job: &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name: "job-no-condition",
				},
				Spec:   batchv1.JobSpec{},
				Status: batchv1.JobStatus{},
			},
			expectedStatus: status.InProgressStatus,
		},
		{
			name: "job with Complete condition as True returns Current status",
			job: &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name: "job-complete",
				},
				Spec: batchv1.JobSpec{},
				Status: batchv1.JobStatus{
					Conditions: []batchv1.JobCondition{
						{
							Type:   batchv1.JobComplete,
							Status: corev1.ConditionTrue,
						},
					},
				},
			},
			expectedStatus: status.CurrentStatus,
		},
		{
			name: "job with Failed condition as True returns Failed status",
			job: &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name: "job-failed",
				},
				Spec: batchv1.JobSpec{},
				Status: batchv1.JobStatus{
					Conditions: []batchv1.JobCondition{
						{
							Type:   batchv1.JobFailed,
							Status: corev1.ConditionTrue,
						},
					},
				},
			},
			expectedStatus: status.FailedStatus,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			us, err := toUnstructured(t, tc.job)
			assert.NoError(t, err)
			result, err := jobConditions(us)
			assert.NoError(t, err)
			assert.Equal(t, tc.expectedStatus, result.Status)
		})
	}
}
