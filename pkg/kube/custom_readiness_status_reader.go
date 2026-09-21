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
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/fluxcd/cli-utils/pkg/kstatus/polling/engine"
	"github.com/fluxcd/cli-utils/pkg/kstatus/polling/event"
	"github.com/fluxcd/cli-utils/pkg/kstatus/status"
	"github.com/fluxcd/cli-utils/pkg/object"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type customReadinessStatusReader struct {
	logger   *slog.Logger
	fallback engine.StatusReader
	eligible map[object.ObjMetadata]struct{}
	// warnedExpressions deduplicates warnings across concurrent watch events.
	// Keys are "<ObjMetadata.String()>|<expression>"; partial annotation pairs
	// use the reserved expression suffix "partial".
	warnedExpressions sync.Map
}

// newCustomReadinessStatusReader wraps the complete fallback reader chain with
// custom readiness evaluation for resources in eligible.
func newCustomReadinessStatusReader(logger *slog.Logger, fallback engine.StatusReader, eligible map[object.ObjMetadata]struct{}) engine.StatusReader {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	return &customReadinessStatusReader{logger: logger, fallback: fallback, eligible: eligible}
}

// Supports returns true for every GroupKind so ineligible and unannotated
// resources can be delegated to the complete fallback chain.
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
	return r.readStatus(ctx, reader, resource, u)
}

func (r *customReadinessStatusReader) ReadStatusForObject(ctx context.Context, reader engine.ClusterReader, resource *unstructured.Unstructured) (*event.ResourceStatus, error) {
	identifier, err := object.RuntimeToObjMeta(resource)
	if err != nil {
		return nil, err
	}
	return r.readStatus(ctx, reader, identifier, resource)
}

func (r *customReadinessStatusReader) readStatus(ctx context.Context, reader engine.ClusterReader, identifier object.ObjMetadata, resource *unstructured.Unstructured) (*event.ResourceStatus, error) {
	if _, ok := r.eligible[identifier]; !ok {
		return r.fallback.ReadStatusForObject(ctx, reader, resource)
	}

	annotations := resource.GetAnnotations()
	successExprs, err := ParseReadinessExpressions(annotations[AnnotationReadinessSuccess])
	if err != nil {
		return nil, fmt.Errorf("parsing %s for %s/%s: %w", AnnotationReadinessSuccess, identifier.Namespace, identifier.Name, err)
	}
	failureExprs, err := ParseReadinessExpressions(annotations[AnnotationReadinessFailure])
	if err != nil {
		return nil, fmt.Errorf("parsing %s for %s/%s: %w", AnnotationReadinessFailure, identifier.Namespace, identifier.Name, err)
	}

	successPresent, failurePresent := len(successExprs) > 0, len(failureExprs) > 0
	if successPresent != failurePresent {
		key := identifier.String() + "|partial"
		if _, alreadyWarned := r.warnedExpressions.LoadOrStore(key, struct{}{}); !alreadyWarned {
			r.logger.Warn("custom readiness annotations must be used together; falling back to default readiness",
				"kind", identifier.GroupKind.Kind,
				"namespace", identifier.Namespace,
				"name", identifier.Name,
				"successAnnotation", AnnotationReadinessSuccess,
				"failureAnnotation", AnnotationReadinessFailure,
			)
		}
		return r.fallback.ReadStatusForObject(ctx, reader, resource)
	}
	if !successPresent {
		return r.fallback.ReadStatusForObject(ctx, reader, resource)
	}

	result, useKstatus, warnings, err := EvaluateCustomReadiness(resource, successExprs, failureExprs)
	if err != nil {
		return nil, err
	}
	if useKstatus {
		return r.fallback.ReadStatusForObject(ctx, reader, resource)
	}

	for _, warning := range warnings {
		key := identifier.String() + "|" + warning.Expression
		if _, alreadyWarned := r.warnedExpressions.LoadOrStore(key, struct{}{}); alreadyWarned {
			continue
		}
		r.logger.Warn("custom readiness expression cannot be evaluated; treating condition as not met",
			"kind", identifier.GroupKind.Kind,
			"namespace", identifier.Namespace,
			"name", identifier.Name,
			"expression", warning.Expression,
			"detail", warning.Detail,
		)
	}

	st, message := status.InProgressStatus, "waiting for custom readiness conditions"
	switch result {
	case ReadinessReady:
		st, message = status.CurrentStatus, "custom readiness conditions met"
	case ReadinessFailed:
		st, message = status.FailedStatus, "custom readiness failure condition met"
	default:
		if len(warnings) > 0 {
			message = fmt.Sprintf("waiting for custom readiness conditions (%d expression(s) skipped: %s)", len(warnings), warnings[0].Detail)
		}
	}
	return &event.ResourceStatus{
		Identifier: identifier,
		Status:     st,
		Message:    message,
	}, nil
}
