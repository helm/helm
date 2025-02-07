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
	"context"
	"fmt"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"sigs.k8s.io/cli-utils/pkg/kstatus/polling/engine"
	"sigs.k8s.io/cli-utils/pkg/kstatus/polling/event"
	"sigs.k8s.io/cli-utils/pkg/kstatus/polling/statusreaders"
	"sigs.k8s.io/cli-utils/pkg/kstatus/status"
	"sigs.k8s.io/cli-utils/pkg/object"
)

type customPodStatusReader struct {
	genericStatusReader engine.StatusReader
}

func NewCustomPodStatusReader(mapper meta.RESTMapper) engine.StatusReader {
	genericStatusReader := statusreaders.NewGenericStatusReader(mapper, podConditions)
	return &customJobStatusReader{
		genericStatusReader: genericStatusReader,
	}
}

func (j *customPodStatusReader) Supports(gk schema.GroupKind) bool {
	return gk == batchv1.SchemeGroupVersion.WithKind("Job").GroupKind()
}

func (j *customPodStatusReader) ReadStatus(ctx context.Context, reader engine.ClusterReader, resource object.ObjMetadata) (*event.ResourceStatus, error) {
	return j.genericStatusReader.ReadStatus(ctx, reader, resource)
}

func (j *customPodStatusReader) ReadStatusForObject(ctx context.Context, reader engine.ClusterReader, resource *unstructured.Unstructured) (*event.ResourceStatus, error) {
	return j.genericStatusReader.ReadStatusForObject(ctx, reader, resource)
}

func podConditions(u *unstructured.Unstructured) (*status.Result, error) {
	obj := u.UnstructuredContent()
	phase := status.GetStringField(obj, ".status.phase", "")
	switch phase {
	case string(v1.PodSucceeded):
		message := fmt.Sprintf("pod %s succeeded", u.GetName())
		return &status.Result{
			Status:  status.CurrentStatus,
			Message: message,
			Conditions: []status.Condition{
				{
					Type:    status.ConditionStalled,
					Status:  corev1.ConditionTrue,
					Message: message,
				},
			},
		}, nil
	case string(v1.PodFailed):
		message := fmt.Sprintf("pod %s failed", u.GetName())
		return &status.Result{
			Status:  status.FailedStatus,
			Message: message,
			Conditions: []status.Condition{
				{
					Type:    status.ConditionStalled,
					Status:  corev1.ConditionTrue,
					Reason:  "PodFailed",
					Message: message,
				},
			},
		}, nil
	case string(v1.PodPending):
	case string(v1.PodRunning):
	}

	message := "Pod in progress"
	return &status.Result{
		Status:  status.InProgressStatus,
		Message: message,
		Conditions: []status.Condition{
			{
				Type:    status.ConditionReconciling,
				Status:  corev1.ConditionTrue,
				Reason:  "PodInProgress",
				Message: message,
			},
		},
	}, nil
}
