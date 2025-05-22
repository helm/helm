/*
Copyright The Helm Authors.
This file was initially copied and modified from
    https://github.com/fluxcd/kustomize-controller/blob/main/internal/statusreaders/job.go
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
	"context"
	"fmt"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/fluxcd/cli-utils/pkg/kstatus/polling/engine"
	"github.com/fluxcd/cli-utils/pkg/kstatus/polling/event"
	"github.com/fluxcd/cli-utils/pkg/kstatus/polling/statusreaders"
	"github.com/fluxcd/cli-utils/pkg/kstatus/status"
	"github.com/fluxcd/cli-utils/pkg/object"
)

type customJobStatusReader struct {
	genericStatusReader engine.StatusReader
}

func NewCustomJobStatusReader(mapper meta.RESTMapper) engine.StatusReader {
	genericStatusReader := statusreaders.NewGenericStatusReader(mapper, jobConditions)
	return &customJobStatusReader{
		genericStatusReader: genericStatusReader,
	}
}

func (j *customJobStatusReader) Supports(gk schema.GroupKind) bool {
	return gk == batchv1.SchemeGroupVersion.WithKind("Job").GroupKind()
}

func (j *customJobStatusReader) ReadStatus(ctx context.Context, reader engine.ClusterReader, resource object.ObjMetadata) (*event.ResourceStatus, error) {
	return j.genericStatusReader.ReadStatus(ctx, reader, resource)
}

func (j *customJobStatusReader) ReadStatusForObject(ctx context.Context, reader engine.ClusterReader, resource *unstructured.Unstructured) (*event.ResourceStatus, error) {
	return j.genericStatusReader.ReadStatusForObject(ctx, reader, resource)
}

// Ref: https://github.com/kubernetes-sigs/cli-utils/blob/v0.29.4/pkg/kstatus/status/core.go
// Modified to return Current status only when the Job has completed as opposed to when it's in progress.
func jobConditions(u *unstructured.Unstructured) (*status.Result, error) {
	obj := u.UnstructuredContent()

	parallelism := status.GetIntField(obj, ".spec.parallelism", 1)
	completions := status.GetIntField(obj, ".spec.completions", parallelism)
	succeeded := status.GetIntField(obj, ".status.succeeded", 0)
	failed := status.GetIntField(obj, ".status.failed", 0)

	// Conditions
	// https://github.com/kubernetes/kubernetes/blob/master/pkg/controller/job/utils.go#L24
	objc, err := status.GetObjectWithConditions(obj)
	if err != nil {
		return nil, err
	}
	for _, c := range objc.Status.Conditions {
		switch c.Type {
		case "Complete":
			if c.Status == corev1.ConditionTrue {
				message := fmt.Sprintf("Job Completed. succeeded: %d/%d", succeeded, completions)
				return &status.Result{
					Status:     status.CurrentStatus,
					Message:    message,
					Conditions: []status.Condition{},
				}, nil
			}
		case "Failed":
			message := fmt.Sprintf("Job Failed. failed: %d/%d", failed, completions)
			if c.Status == corev1.ConditionTrue {
				return &status.Result{
					Status:  status.FailedStatus,
					Message: message,
					Conditions: []status.Condition{
						{
							Type:    status.ConditionStalled,
							Status:  corev1.ConditionTrue,
							Reason:  "JobFailed",
							Message: message,
						},
					},
				}, nil
			}
		}
	}

	message := "Job in progress"
	return &status.Result{
		Status:  status.InProgressStatus,
		Message: message,
		Conditions: []status.Condition{
			{
				Type:    status.ConditionReconciling,
				Status:  corev1.ConditionTrue,
				Reason:  "JobInProgress",
				Message: message,
			},
		},
	}, nil
}
