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
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/discovery"
	discoveryfake "k8s.io/client-go/discovery/fake"
	clienttesting "k8s.io/client-go/testing"

	"github.com/fluxcd/cli-utils/pkg/kstatus/status"
	"github.com/fluxcd/cli-utils/pkg/testutil"
)

func mustUnstructured(t *testing.T, manifest string) *unstructured.Unstructured {
	t.Helper()
	obj := map[string]any{}
	require.NoError(t, yaml.Unmarshal([]byte(manifest), &obj))
	return &unstructured.Unstructured{Object: obj}
}

type stubDiscovery struct {
	discovery.ServerResourcesInterface
	fn    func(groupVersion string) (*metav1.APIResourceList, error)
	calls atomic.Int32
}

func (s *stubDiscovery) ServerResourcesForGroupVersion(groupVersion string) (*metav1.APIResourceList, error) {
	s.calls.Add(1)
	return s.fn(groupVersion)
}

func TestCustomResourceStatusReaderSupports(t *testing.T) {
	t.Parallel()

	knativeServiceGVK := schema.GroupVersionKind{Group: "serving.knative.dev", Version: "v1", Kind: "Service"}
	knativeRouteGVK := schema.GroupVersionKind{Group: "serving.knative.dev", Version: "v1", Kind: "Route"}
	serviceMonitorGVK := schema.GroupVersionKind{Group: "monitoring.coreos.com", Version: "v1", Kind: "ServiceMonitor"}
	ingressGVK := networkingv1.SchemeGroupVersion.WithKind("Ingress")

	fakeMapper := testutil.NewFakeRESTMapper(
		knativeServiceGVK,
		knativeRouteGVK,
		serviceMonitorGVK,
		ingressGVK,
	)
	fakeDiscovery := &discoveryfake.FakeDiscovery{
		Fake: &clienttesting.Fake{
			Resources: []*metav1.APIResourceList{
				{
					GroupVersion: "serving.knative.dev/v1",
					APIResources: []metav1.APIResource{
						{Name: "services", Kind: "Service", Namespaced: true},
						{Name: "services/status", Kind: "Service", Namespaced: true},
						{Name: "routes", Kind: "Route", Namespaced: true},
						{Name: "routes/status", Kind: "Route", Namespaced: true},
					},
				},
				{
					GroupVersion: "monitoring.coreos.com/v1",
					APIResources: []metav1.APIResource{
						{Name: "servicemonitors", Kind: "ServiceMonitor", Namespaced: true},
					},
				},
				{
					GroupVersion: "networking.k8s.io/v1",
					APIResources: []metav1.APIResource{
						{Name: "ingresses", Kind: "Ingress", Namespaced: true},
						{Name: "ingresses/status", Kind: "Ingress", Namespaced: true},
					},
				},
			},
		},
	}

	reader := NewCustomResourceStatusReader(fakeMapper, fakeDiscovery)

	tests := []struct {
		name     string
		gk       schema.GroupKind
		expected bool
	}{
		{
			name:     "core Pod keeps its built-in reader",
			gk:       corev1.SchemeGroupVersion.WithKind("Pod").GroupKind(),
			expected: false,
		},
		{
			name:     "core Service keeps its built-in reader despite a CR sharing the Kind name",
			gk:       corev1.SchemeGroupVersion.WithKind("Service").GroupKind(),
			expected: false,
		},
		{
			name:     "custom resource with status subresource is supported",
			gk:       knativeServiceGVK.GroupKind(),
			expected: true,
		},
		{
			name:     "second custom resource in same group with status subresource is supported",
			gk:       knativeRouteGVK.GroupKind(),
			expected: true,
		},
		{
			name:     "status-less custom resource keeps generic immediate-Current behavior",
			gk:       serviceMonitorGVK.GroupKind(),
			expected: false,
		},
		{
			name:     "built-in Ingress keeps generic behavior even though it has a status subresource",
			gk:       ingressGVK.GroupKind(),
			expected: false,
		},
		{
			name:     "core ConfigMap keeps its built-in reader",
			gk:       corev1.SchemeGroupVersion.WithKind("ConfigMap").GroupKind(),
			expected: false,
		},
		{
			name:     "batch Job keeps its built-in reader",
			gk:       batchv1.SchemeGroupVersion.WithKind("Job").GroupKind(),
			expected: false,
		},
		{
			name:     "apps Deployment keeps its built-in reader",
			gk:       appsv1.SchemeGroupVersion.WithKind("Deployment").GroupKind(),
			expected: false,
		},
		{
			name:     "CustomResourceDefinition keeps its built-in reader",
			gk:       apiextv1.SchemeGroupVersion.WithKind("CustomResourceDefinition").GroupKind(),
			expected: false,
		},
		{
			name:     "RBAC ClusterRole without a status subresource keeps generic behavior",
			gk:       rbacv1.SchemeGroupVersion.WithKind("ClusterRole").GroupKind(),
			expected: false,
		},
		{
			name:     "ServiceAccount without a status subresource keeps generic behavior",
			gk:       corev1.SchemeGroupVersion.WithKind("ServiceAccount").GroupKind(),
			expected: false,
		},
		{
			name:     "Namespace without a status subresource keeps generic behavior",
			gk:       corev1.SchemeGroupVersion.WithKind("Namespace").GroupKind(),
			expected: false,
		},
		{
			name:     "Lease without a status subresource keeps generic behavior",
			gk:       coordinationv1.SchemeGroupVersion.WithKind("Lease").GroupKind(),
			expected: false,
		},
		{
			name:     "ValidatingWebhookConfiguration keeps generic behavior",
			gk:       admissionv1.SchemeGroupVersion.WithKind("ValidatingWebhookConfiguration").GroupKind(),
			expected: false,
		},
		{
			name:     "NetworkPolicy keeps generic behavior",
			gk:       networkingv1.SchemeGroupVersion.WithKind("NetworkPolicy").GroupKind(),
			expected: false,
		},
		{
			name:     "unmapped custom resource fails open",
			gk:       schema.GroupKind{Group: "unknown.example.com", Kind: "Widget"},
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			// Evaluate twice to cover both the cold and memoized code paths.
			assert.Equal(t, tc.expected, reader.Supports(tc.gk))
			assert.Equal(t, tc.expected, reader.Supports(tc.gk))
		})
	}
}

func TestCustomResourceStatusReaderSupportsNilDependencies(t *testing.T) {
	t.Parallel()
	gk := schema.GroupKind{Group: "serving.knative.dev", Kind: "Service"}
	fakeMapper := testutil.NewFakeRESTMapper(schema.GroupVersionKind{Group: "serving.knative.dev", Version: "v1", Kind: "Service"})
	fakeDiscovery := &discoveryfake.FakeDiscovery{Fake: &clienttesting.Fake{}}

	assert.False(t, NewCustomResourceStatusReader(nil, nil).Supports(gk))
	assert.False(t, NewCustomResourceStatusReader(fakeMapper, nil).Supports(gk))
	assert.False(t, NewCustomResourceStatusReader(nil, fakeDiscovery).Supports(gk))
}

func TestCustomResourceStatusReaderSupportsRetriesTransientDiscoveryError(t *testing.T) {
	t.Parallel()
	gvk := schema.GroupVersionKind{Group: "serving.knative.dev", Version: "v1", Kind: "Service"}
	fakeMapper := testutil.NewFakeRESTMapper(gvk)

	var attempt atomic.Int32
	disc := &stubDiscovery{
		fn: func(gv string) (*metav1.APIResourceList, error) {
			if attempt.Add(1) == 1 {
				return nil, errors.New("temporary discovery failure")
			}
			return &metav1.APIResourceList{
				GroupVersion: gv,
				APIResources: []metav1.APIResource{
					{Name: "services", Kind: "Service", Namespaced: true},
					{Name: "services/status", Kind: "Service", Namespaced: true},
				},
			}, nil
		},
	}

	reader := NewCustomResourceStatusReader(fakeMapper, disc)
	// First call fails open without caching the transient error.
	assert.False(t, reader.Supports(gvk.GroupKind()))
	// Subsequent call succeeds and caches the result.
	assert.True(t, reader.Supports(gvk.GroupKind()))
	assert.True(t, reader.Supports(gvk.GroupKind()))
	assert.Equal(t, int32(2), disc.calls.Load())
}

func TestCustomResourceStatusReaderSupportsCachesPerGroupVersion(t *testing.T) {
	t.Parallel()
	serviceGVK := schema.GroupVersionKind{Group: "serving.knative.dev", Version: "v1", Kind: "Service"}
	routeGVK := schema.GroupVersionKind{Group: "serving.knative.dev", Version: "v1", Kind: "Route"}
	configGVK := schema.GroupVersionKind{Group: "serving.knative.dev", Version: "v1", Kind: "Configuration"}

	fakeMapper := testutil.NewFakeRESTMapper(serviceGVK, routeGVK, configGVK)
	disc := &stubDiscovery{
		fn: func(gv string) (*metav1.APIResourceList, error) {
			return &metav1.APIResourceList{
				GroupVersion: gv,
				APIResources: []metav1.APIResource{
					{Name: "services", Kind: "Service", Namespaced: true},
					{Name: "services/status", Kind: "Service", Namespaced: true},
					{Name: "routes", Kind: "Route", Namespaced: true},
					{Name: "configurations", Kind: "Configuration", Namespaced: true},
				},
			}, nil
		},
	}

	reader := NewCustomResourceStatusReader(fakeMapper, disc)
	assert.True(t, reader.Supports(serviceGVK.GroupKind()))
	assert.False(t, reader.Supports(routeGVK.GroupKind()))
	assert.False(t, reader.Supports(configGVK.GroupKind()))
	// All three kinds share one GroupVersion, so discovery is called once.
	assert.Equal(t, int32(1), disc.calls.Load())
}

func TestCustomResourceConditions(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name           string
		manifest       string
		expectedStatus status.Status
		expectedReason string
	}{
		{
			name: "custom resource without status is in progress",
			manifest: `
apiVersion: serving.knative.dev/v1
kind: Service
metadata:
  name: hello-knative
  namespace: ns
  generation: 1
`,
			expectedStatus: status.InProgressStatus,
			expectedReason: noStatusReportedReason,
		},
		{
			name: "custom resource with null status is in progress",
			manifest: `
apiVersion: serving.knative.dev/v1
kind: Service
metadata:
  name: hello-knative
  namespace: ns
  generation: 1
status: null
`,
			expectedStatus: status.InProgressStatus,
			expectedReason: noStatusReportedReason,
		},
		{
			name: "custom resource with an empty status map is in progress",
			manifest: `
apiVersion: example.com/v1
kind: Widget
metadata:
  name: pending-widget
  namespace: ns
status: {}
`,
			expectedStatus: status.InProgressStatus,
			expectedReason: noStatusReportedReason,
		},
		{
			name: "custom resource with a Ready True condition is current",
			manifest: `
apiVersion: serving.knative.dev/v1
kind: Service
metadata:
  name: hello-knative
  namespace: ns
  generation: 1
status:
  observedGeneration: 1
  conditions:
  - type: Ready
    status: "True"
`,
			expectedStatus: status.CurrentStatus,
		},
		{
			name: "custom resource with a Ready False condition stays in progress via kstatus",
			manifest: `
apiVersion: serving.knative.dev/v1
kind: Service
metadata:
  name: hello-knative
  namespace: ns
  generation: 1
status:
  conditions:
  - type: Ready
    status: "False"
    reason: Reconciling
`,
			expectedStatus: status.InProgressStatus,
		},
		{
			name: "custom resource with an unobserved generation stays in progress via kstatus",
			manifest: `
apiVersion: example.com/v1
kind: Widget
metadata:
  name: widget
  namespace: ns
  generation: 2
status:
  observedGeneration: 1
`,
			expectedStatus: status.InProgressStatus,
		},
		{
			name: "custom resource with any non-empty status defers to kstatus",
			manifest: `
apiVersion: example.com/v1
kind: Widget
metadata:
  name: widget
  namespace: ns
status:
  phase: Deployed
`,
			expectedStatus: status.CurrentStatus,
		},
		{
			name: "custom resource with a Reconciling condition stays in progress via kstatus",
			manifest: `
apiVersion: example.com/v1
kind: Widget
metadata:
  name: widget
  namespace: ns
status:
  conditions:
  - type: Reconciling
    status: "True"
`,
			expectedStatus: status.InProgressStatus,
		},
		{
			name: "custom resource scheduled for deletion is terminating",
			manifest: `
apiVersion: example.com/v1
kind: Widget
metadata:
  name: widget
  namespace: ns
  deletionTimestamp: "2026-01-01T00:00:00Z"
`,
			expectedStatus: status.TerminatingStatus,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			u := mustUnstructured(t, tc.manifest)
			result, err := customResourceConditions(u)
			require.NoError(t, err)
			assert.Equal(t, tc.expectedStatus, result.Status)
			if tc.expectedReason != "" {
				require.Len(t, result.Conditions, 1)
				assert.Equal(t, tc.expectedReason, result.Conditions[0].Reason)
			}
		})
	}
}

func TestHasEmptyStatus(t *testing.T) {
	t.Parallel()
	t.Run("missing status", func(t *testing.T) {
		t.Parallel()
		u := mustUnstructured(t, `
apiVersion: example.com/v1
kind: Widget
metadata:
  name: widget
`)
		assert.True(t, hasEmptyStatus(u))
	})
	t.Run("null status", func(t *testing.T) {
		t.Parallel()
		u := mustUnstructured(t, `
apiVersion: example.com/v1
kind: Widget
metadata:
  name: widget
status: null
`)
		assert.True(t, hasEmptyStatus(u))
	})
	t.Run("empty status map", func(t *testing.T) {
		t.Parallel()
		u := mustUnstructured(t, `
apiVersion: example.com/v1
kind: Widget
metadata:
  name: widget
status: {}
`)
		assert.True(t, hasEmptyStatus(u))
	})
	t.Run("populated status", func(t *testing.T) {
		t.Parallel()
		u := mustUnstructured(t, `
apiVersion: example.com/v1
kind: Widget
metadata:
  name: widget
status:
  phase: Deployed
`)
		assert.False(t, hasEmptyStatus(u))
	})
	t.Run("malformed non-object status is not considered empty", func(t *testing.T) {
		t.Parallel()
		u := mustUnstructured(t, `
apiVersion: example.com/v1
kind: Widget
metadata:
  name: widget
status: not-an-object
`)
		assert.False(t, hasEmptyStatus(u))
	})
	t.Run("empty status array is not treated as an empty map", func(t *testing.T) {
		t.Parallel()
		// A non-map status, even an empty-looking one, must defer to kstatus
		// instead of triggering the empty-status InProgress heuristic.
		u := mustUnstructured(t, `
apiVersion: example.com/v1
kind: Widget
metadata:
  name: widget
status: []
`)
		assert.False(t, hasEmptyStatus(u))
	})
}

func TestCustomResourceConditionsMalformedStatus(t *testing.T) {
	t.Parallel()
	// A malformed .status must surface kstatus's error rather than being
	// swallowed by the empty-status heuristic or misreported as in progress.
	u := mustUnstructured(t, `
apiVersion: example.com/v1
kind: Widget
metadata:
  name: widget
  namespace: ns
status: not-an-object
`)
	_, err := customResourceConditions(u)
	require.Error(t, err)
}

func TestCustomResourceStatusReaderSupportsConcurrent(t *testing.T) {
	t.Parallel()
	knativeGVK := schema.GroupVersionKind{Group: "serving.knative.dev", Version: "v1", Kind: "Service"}
	serviceMonitorGVK := schema.GroupVersionKind{Group: "monitoring.coreos.com", Version: "v1", Kind: "ServiceMonitor"}
	fakeMapper := testutil.NewFakeRESTMapper(knativeGVK, serviceMonitorGVK)
	disc := &stubDiscovery{
		fn: func(gv string) (*metav1.APIResourceList, error) {
			switch gv {
			case "serving.knative.dev/v1":
				return &metav1.APIResourceList{
					GroupVersion: gv,
					APIResources: []metav1.APIResource{
						{Name: "services", Kind: "Service", Namespaced: true},
						{Name: "services/status", Kind: "Service", Namespaced: true},
					},
				}, nil
			case "monitoring.coreos.com/v1":
				return &metav1.APIResourceList{
					GroupVersion: gv,
					APIResources: []metav1.APIResource{
						{Name: "servicemonitors", Kind: "ServiceMonitor", Namespaced: true},
					},
				}, nil
			default:
				return nil, errors.New("unexpected gv")
			}
		},
	}

	reader := NewCustomResourceStatusReader(fakeMapper, disc)
	gks := []schema.GroupKind{
		knativeGVK.GroupKind(),
		serviceMonitorGVK.GroupKind(),
		corev1.SchemeGroupVersion.WithKind("Pod").GroupKind(),
		rbacv1.SchemeGroupVersion.WithKind("ClusterRole").GroupKind(),
	}

	var wg sync.WaitGroup
	for range 50 {
		wg.Go(func() {
			for _, gk := range gks {
				_ = reader.Supports(gk)
			}
		})
	}
	wg.Wait()

	// Results stay stable and each custom GroupVersion is discovered once.
	assert.True(t, reader.Supports(knativeGVK.GroupKind()))
	assert.False(t, reader.Supports(serviceMonitorGVK.GroupKind()))
	assert.False(t, reader.Supports(gks[2]))
	assert.Equal(t, int32(2), disc.calls.Load())
}
