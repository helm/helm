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
	logger *slog.Logger
	// fallback is the reader chain the enclosing wait method would use if
	// custom readiness were disabled. Resources that do not carry both
	// readiness annotations are delegated to it, so built-in readers (the
	// composite Deployment/ReplicaSet/StatefulSet readers, the Job reader in
	// WaitWithJobs) and caller-supplied readers keep their semantics.
	fallback engine.StatusReader
	// warnedExpressions dedups authoring-mistake warnings so a bad expression
	// logs once per resource rather than on every watch event. Keys are
	// "<ObjMetadata.String()>|<expression>". The reader is invoked
	// concurrently by the status watcher, hence sync.Map.
	warnedExpressions sync.Map
}

// newCustomReadinessStatusReader wraps fallback with per-resource custom
// readiness evaluation. fallback must be non-nil and must support every
// GroupKind this reader can receive (every chain built in statuswait.go
// terminates in a generic reader that supports all GroupKinds).
func newCustomReadinessStatusReader(logger *slog.Logger, fallback engine.StatusReader) engine.StatusReader {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	return &customReadinessStatusReader{logger: logger, fallback: fallback}
}

// Supports returns true for every GroupKind: annotated resources of any kind
// are evaluated here, and everything else is delegated to the fallback chain.
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
	annotations := resource.GetAnnotations()

	successExprs, err := parseReadinessExpressions(annotations[AnnotationReadinessSuccess])
	if err != nil {
		return nil, fmt.Errorf("parsing %s for %s/%s: %w", AnnotationReadinessSuccess, identifier.Namespace, identifier.Name, err)
	}
	failureExprs, err := parseReadinessExpressions(annotations[AnnotationReadinessFailure])
	if err != nil {
		return nil, fmt.Errorf("parsing %s for %s/%s: %w", AnnotationReadinessFailure, identifier.Namespace, identifier.Name, err)
	}

	result, useKstatus, warnings, err := EvaluateCustomReadiness(resource, successExprs, failureExprs)
	if err != nil {
		return nil, err
	}
	if useKstatus {
		// Missing or partial annotations: this resource does not opt in to
		// custom readiness, so the default reader chain decides its status.
		return r.fallback.ReadStatusForObject(ctx, reader, resource)
	}

	for _, w := range warnings {
		key := identifier.String() + "|" + w.Expression
		if _, alreadyWarned := r.warnedExpressions.LoadOrStore(key, struct{}{}); alreadyWarned {
			continue
		}
		r.logger.Warn("custom readiness expression cannot be evaluated; treating condition as not met",
			"kind", identifier.GroupKind.Kind,
			"namespace", identifier.Namespace,
			"name", identifier.Name,
			"expression", w.Expression,
			"detail", w.Detail,
		)
	}

	st, message := status.InProgressStatus, "waiting for custom readiness conditions"
	switch result {
	case ReadinessReady:
		st, message = status.CurrentStatus, "custom readiness conditions met"
	case ReadinessFailed:
		st, message = status.FailedStatus, "custom readiness failure condition met"
	default:
		// ReadinessPending (and any future status) keep the in-progress
		// defaults; name any skipped expression so an eventual wait timeout
		// error is self-explanatory.
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
