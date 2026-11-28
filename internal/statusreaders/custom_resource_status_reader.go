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
	"context"
	"strings"
	"sync"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/discovery"
	kubescheme "k8s.io/client-go/kubernetes/scheme"

	"github.com/fluxcd/cli-utils/pkg/kstatus/polling/engine"
	"github.com/fluxcd/cli-utils/pkg/kstatus/polling/event"
	"github.com/fluxcd/cli-utils/pkg/kstatus/polling/statusreaders"
	"github.com/fluxcd/cli-utils/pkg/kstatus/status"
	"github.com/fluxcd/cli-utils/pkg/object"
)

const (
	// noStatusReportedReason is the condition reason used while a custom
	// resource has not reported any status yet.
	noStatusReportedReason = "NoStatusReported"
	// noStatusReportedMessage explains why a custom resource without status is
	// treated as still in progress.
	noStatusReportedMessage = "Custom resource has not yet reported status"
	statusSubresourceSuffix = "/status"
)

// builtInGroupKinds contains every GroupKind registered in the standard
// Kubernetes client-go scheme. Building a private scheme once isolates this
// check from callers that register CRDs into the global scheme.Scheme.
var builtInGroupKinds = func() map[schema.GroupKind]struct{} {
	s := runtime.NewScheme()
	utilruntime.Must(kubescheme.AddToScheme(s))
	gks := make(map[schema.GroupKind]struct{})
	for gvk := range s.AllKnownTypes() {
		gks[gvk.GroupKind()] = struct{}{}
	}
	return gks
}()

// NewCustomResourceStatusReader returns a status reader for custom resources
// that expose a /status subresource. It delegates status computation to
// kstatus, except that a freshly created custom resource whose controller has
// not yet written any status is reported as InProgress instead of Current.
//
// Without this override kstatus falls back to assuming an unknown resource with
// no known conditions is Current, which makes `--wait` return before the
// controller has reconciled a newly created custom resource. Gating on the
// /status subresource via API discovery ensures status-less config CRDs (such
// as ServiceMonitor or IngressRoute) keep the immediate-Current behavior.
// See https://github.com/helm/helm/issues/32066 and
// https://github.com/fluxcd/cli-utils/blob/v1.4.0/pkg/kstatus/README.md#the-ready-condition
func NewCustomResourceStatusReader(mapper meta.RESTMapper, discoveryClient discovery.ServerResourcesInterface) engine.StatusReader {
	return &customResourceStatusReader{
		genericStatusReader: statusreaders.NewGenericStatusReader(mapper, customResourceConditions),
		mapper:              mapper,
		discoveryClient:     discoveryClient,
		supportsCache:       make(map[schema.GroupKind]bool),
		gvStatusResources:   make(map[string]map[string]struct{}),
	}
}

type customResourceStatusReader struct {
	genericStatusReader engine.StatusReader
	mapper              meta.RESTMapper
	discoveryClient     discovery.ServerResourcesInterface

	mu                sync.Mutex
	supportsCache     map[schema.GroupKind]bool
	gvStatusResources map[string]map[string]struct{} // groupVersion -> resource names having /status
}

// Supports matches custom resource kinds that:
//  1. are not built-in Kubernetes kinds and have no built-in kstatus rules, and
//  2. declare a /status subresource according to Kubernetes API discovery.
//
// If mapper or discovery is unavailable or returns an error, Supports fails
// open (returns false without caching the error) so status-less resources are
// never falsely blocked.
func (c *customResourceStatusReader) Supports(gk schema.GroupKind) bool {
	if _, isBuiltIn := builtInGroupKinds[gk]; isBuiltIn {
		return false
	}
	// GetLegacyConditionsFn only looks at the resource's GroupKind, so a
	// lightweight unstructured carrying just the GVK is enough to tell whether
	// kstatus has built-in rules for this kind (e.g. CustomResourceDefinition).
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(schema.GroupVersionKind{Group: gk.Group, Kind: gk.Kind})
	if status.GetLegacyConditionsFn(u) != nil {
		return false
	}
	if c.mapper == nil || c.discoveryClient == nil {
		return false
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if supported, ok := c.supportsCache[gk]; ok {
		return supported
	}

	mapping, err := c.mapper.RESTMapping(gk)
	if err != nil {
		return false
	}
	gv := mapping.GroupVersionKind.GroupVersion().String()

	statusResources, ok := c.gvStatusResources[gv]
	if !ok {
		resourceList, err := c.discoveryClient.ServerResourcesForGroupVersion(gv)
		if err != nil || resourceList == nil {
			return false
		}
		statusResources = extractStatusResources(resourceList)
		c.gvStatusResources[gv] = statusResources
	}

	_, hasStatus := statusResources[mapping.Resource.Resource]
	c.supportsCache[gk] = hasStatus
	return hasStatus
}

func extractStatusResources(resourceList *metav1.APIResourceList) map[string]struct{} {
	res := make(map[string]struct{})
	for _, r := range resourceList.APIResources {
		if base, ok := strings.CutSuffix(r.Name, statusSubresourceSuffix); ok && base != "" {
			res[base] = struct{}{}
		}
	}
	return res
}

func (c *customResourceStatusReader) ReadStatus(ctx context.Context, reader engine.ClusterReader, resource object.ObjMetadata) (*event.ResourceStatus, error) {
	return c.genericStatusReader.ReadStatus(ctx, reader, resource)
}

func (c *customResourceStatusReader) ReadStatusForObject(ctx context.Context, reader engine.ClusterReader, resource *unstructured.Unstructured) (*event.ResourceStatus, error) {
	return c.genericStatusReader.ReadStatusForObject(ctx, reader, resource)
}

// customResourceConditions wraps kstatus.Compute so that a custom resource
// without any status is not prematurely considered Current. As soon as the
// controller writes anything to .status, the resource is handed back to the
// regular kstatus computation, so this heuristic can never block a resource on
// its own past the configured wait timeout.
func customResourceConditions(u *unstructured.Unstructured) (*status.Result, error) {
	result, err := status.Compute(u)
	if err != nil {
		return nil, err
	}
	if result.Status == status.CurrentStatus && hasEmptyStatus(u) {
		return &status.Result{
			Status:  status.InProgressStatus,
			Message: noStatusReportedMessage,
			Conditions: []status.Condition{
				{
					Type:    status.ConditionReconciling,
					Status:  corev1.ConditionTrue,
					Reason:  noStatusReportedReason,
					Message: noStatusReportedMessage,
				},
			},
		}, nil
	}
	return result, nil
}

// hasEmptyStatus reports whether the resource does not have any field under
// .status yet (absent, null, or empty map), which means its controller has not
// reported reconciliation state. A malformed (non-object) status is treated as
// non-empty so that the regular kstatus computation surfaces the problem
// instead of this heuristic.
func hasEmptyStatus(u *unstructured.Unstructured) bool {
	val, found, err := unstructured.NestedFieldNoCopy(u.Object, "status")
	if err != nil {
		return false
	}
	if !found || val == nil {
		return true
	}
	statusMap, ok := val.(map[string]any)
	if !ok {
		return false
	}
	return len(statusMap) == 0
}
