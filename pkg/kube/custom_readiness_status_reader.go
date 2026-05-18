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

package kube // import "helm.sh/helm/v4/pkg/kube"

import (
	"context"
	"fmt"

	"github.com/fluxcd/cli-utils/pkg/kstatus/polling/engine"
	"github.com/fluxcd/cli-utils/pkg/kstatus/polling/event"
	"github.com/fluxcd/cli-utils/pkg/kstatus/status"
	"github.com/fluxcd/cli-utils/pkg/object"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type customReadinessStatusReader struct{}

func newCustomReadinessStatusReader() engine.StatusReader {
	return &customReadinessStatusReader{}
}

func (*customReadinessStatusReader) Supports(schema.GroupKind) bool {
	return true
}

func (r *customReadinessStatusReader) ReadStatus(ctx context.Context, reader engine.ClusterReader, resource object.ObjMetadata) (*event.ResourceStatus, error) {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(schema.GroupVersionKind{
		Group: resource.GroupKind.Group,
		Kind:  resource.GroupKind.Kind,
	})
	if err := reader.Get(ctx, client.ObjectKey{Namespace: resource.Namespace, Name: resource.Name}, u); err != nil {
		return nil, err
	}
	return r.readStatus(resource, u)
}

func (r *customReadinessStatusReader) ReadStatusForObject(_ context.Context, _ engine.ClusterReader, resource *unstructured.Unstructured) (*event.ResourceStatus, error) {
	identifier, err := object.RuntimeToObjMeta(resource)
	if err != nil {
		return nil, err
	}
	return r.readStatus(identifier, resource)
}

func (r *customReadinessStatusReader) readStatus(identifier object.ObjMetadata, resource *unstructured.Unstructured) (*event.ResourceStatus, error) {
	annotations := resource.GetAnnotations()

	successExprs, err := parseReadinessExpressions(annotations[AnnotationReadinessSuccess])
	if err != nil {
		return nil, fmt.Errorf("parsing %s for %s/%s: %w", AnnotationReadinessSuccess, identifier.Namespace, identifier.Name, err)
	}
	failureExprs, err := parseReadinessExpressions(annotations[AnnotationReadinessFailure])
	if err != nil {
		return nil, fmt.Errorf("parsing %s for %s/%s: %w", AnnotationReadinessFailure, identifier.Namespace, identifier.Name, err)
	}

	result, useKstatus, err := EvaluateCustomReadiness(resource, successExprs, failureExprs)
	if err != nil {
		return nil, err
	}
	if useKstatus {
		return computeKstatusStatus(identifier, resource)
	}

	switch result {
	case ReadinessReady:
		return &event.ResourceStatus{
			Identifier: identifier,
			Status:     status.CurrentStatus,
			Message:    "custom readiness conditions met",
		}, nil
	case ReadinessFailed:
		return &event.ResourceStatus{
			Identifier: identifier,
			Status:     status.FailedStatus,
			Message:    "custom readiness failure condition met",
		}, nil
	default:
		return &event.ResourceStatus{
			Identifier: identifier,
			Status:     status.InProgressStatus,
			Message:    "waiting for custom readiness conditions",
		}, nil
	}
}

func computeKstatusStatus(identifier object.ObjMetadata, resource *unstructured.Unstructured) (*event.ResourceStatus, error) {
	result, err := status.Compute(resource)
	if err != nil {
		return nil, err
	}
	return &event.ResourceStatus{
		Identifier: identifier,
		Status:     result.Status,
		Message:    result.Message,
	}, nil
}
