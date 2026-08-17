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

package action

import (
	"bytes"
	"io"
	"net/http"
	"testing"

	"helm.sh/helm/v4/pkg/kube"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest/fake"
)

func newDeploymentResource(name, namespace, generateName string) *resource.Info {
	return &resource.Info{
		Name: name,
		Mapping: &meta.RESTMapping{
			Resource:         schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployment"},
			GroupVersionKind: schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
		},
		Object: &appsv1.Deployment{
			ObjectMeta: v1.ObjectMeta{
				Name:         name,
				Namespace:    namespace,
				GenerateName: generateName,
			},
		},
	}
}

func newMissingDeployment(name, namespace string) *resource.Info {
	info := &resource.Info{
		Name:      name,
		Namespace: namespace,
		Mapping: &meta.RESTMapping{
			Resource:         schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployment"},
			GroupVersionKind: schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
			Scope:            meta.RESTScopeNamespace,
		},
		Object: &appsv1.Deployment{
			ObjectMeta: v1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
		},
		Client: fakeClientWith(http.StatusNotFound, appsV1GV, ""),
	}

	return info
}

func newDeploymentWithOwner(name, namespace string, labels map[string]string, annotations map[string]string) *resource.Info {
	obj := &appsv1.Deployment{
		ObjectMeta: v1.ObjectMeta{
			Name:        name,
			Namespace:   namespace,
			Labels:      labels,
			Annotations: annotations,
		},
	}
	return &resource.Info{
		Name:      name,
		Namespace: namespace,
		Mapping: &meta.RESTMapping{
			Resource:         schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployment"},
			GroupVersionKind: schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
			Scope:            meta.RESTScopeNamespace,
		},
		Object: obj,
		Client: fakeClientWith(http.StatusOK, appsV1GV, runtime.EncodeOrDie(appsv1Codec, obj)),
	}
}

var (
	appsV1GV    = schema.GroupVersion{Group: "apps", Version: "v1"}
	appsv1Codec = scheme.Codecs.CodecForVersions(scheme.Codecs.LegacyCodec(appsV1GV), scheme.Codecs.UniversalDecoder(appsV1GV), appsV1GV, appsV1GV)
)

func stringBody(body string) io.ReadCloser {
	return io.NopCloser(bytes.NewReader([]byte(body)))
}

func fakeClientWith(code int, gv schema.GroupVersion, body string) *fake.RESTClient {
	return &fake.RESTClient{
		GroupVersion:         gv,
		NegotiatedSerializer: scheme.Codecs.WithoutConversion(),
		Client: fake.CreateHTTPClient(func(_ *http.Request) (*http.Response, error) {
			header := http.Header{}
			header.Set("Content-Type", runtime.ContentTypeJSON)
			return &http.Response{
				StatusCode: code,
				Header:     header,
				Body:       stringBody(body),
			}, nil
		}),
	}
}

func TestRequireAdoption(t *testing.T) {
	var (
		missing   = newMissingDeployment("missing", "ns-a")
		existing  = newDeploymentWithOwner("existing", "ns-a", nil, nil)
		resources = kube.ResourceList{missing, existing}
	)

	// Verify that a resource that lacks labels/annotations can be adopted
	found, err := requireAdoption(resources)
	require.NoError(t, err)
	require.Len(t, found, 1)
	assert.Equal(t, found[0], existing)
	assert.NotSame(t, found[0], existing)
}

func TestExistingResourceConflict(t *testing.T) {
	var (
		releaseName      = "rel-name"
		releaseNamespace = "rel-namespace"
		labels           = map[string]string{
			appManagedByLabel: appManagedByHelm,
		}
		annotations = map[string]string{
			helmReleaseNameAnnotation:      releaseName,
			helmReleaseNamespaceAnnotation: releaseNamespace,
		}
		missing   = newMissingDeployment("missing", "ns-a")
		existing  = newDeploymentWithOwner("existing", "ns-a", labels, annotations)
		conflict  = newDeploymentWithOwner("conflict", "ns-a", nil, nil)
		resources = kube.ResourceList{missing, existing}
	)

	// Verify only existing resources are returned
	found, err := existingResourceConflict(resources, releaseName, releaseNamespace)
	require.NoError(t, err)
	require.Len(t, found, 1)
	assert.Equal(t, found[0], existing)
	assert.NotSame(t, found[0], existing)

	// Verify that an existing resource that lacks labels/annotations results in an error
	resources = append(resources, conflict)
	_, err = existingResourceConflict(resources, releaseName, releaseNamespace)
	assert.Error(t, err)
}

func TestCheckOwnership(t *testing.T) {
	deployFoo := newDeploymentResource("foo", "ns-a", "")

	// Verify that a resource that lacks labels/annotations is not owned
	require.EqualError(t, checkOwnership(deployFoo.Object, "rel-a", "ns-a"), `invalid ownership metadata; label validation error: missing key "app.kubernetes.io/managed-by": must be set to "Helm"; annotation validation error: missing key "meta.helm.sh/release-name": must be set to "rel-a"; annotation validation error: missing key "meta.helm.sh/release-namespace": must be set to "ns-a"`)

	// Set managed by label and verify annotation error message
	_ = accessor.SetLabels(deployFoo.Object, map[string]string{
		appManagedByLabel: appManagedByHelm,
	})
	require.EqualError(t, checkOwnership(deployFoo.Object, "rel-a", "ns-a"), `invalid ownership metadata; annotation validation error: missing key "meta.helm.sh/release-name": must be set to "rel-a"; annotation validation error: missing key "meta.helm.sh/release-namespace": must be set to "ns-a"`)

	// Set only the release name annotation and verify missing release namespace error message
	_ = accessor.SetAnnotations(deployFoo.Object, map[string]string{
		helmReleaseNameAnnotation: "rel-a",
	})
	require.EqualError(t, checkOwnership(deployFoo.Object, "rel-a", "ns-a"), `invalid ownership metadata; annotation validation error: missing key "meta.helm.sh/release-namespace": must be set to "ns-a"`)

	// Set both release name and namespace annotations and verify no ownership errors
	_ = accessor.SetAnnotations(deployFoo.Object, map[string]string{
		helmReleaseNameAnnotation:      "rel-a",
		helmReleaseNamespaceAnnotation: "ns-a",
	})
	require.NoError(t, checkOwnership(deployFoo.Object, "rel-a", "ns-a"))

	// Verify ownership error for wrong release name
	require.EqualError(t, checkOwnership(deployFoo.Object, "rel-b", "ns-a"), `invalid ownership metadata; annotation validation error: key "meta.helm.sh/release-name" must equal "rel-b": current value is "rel-a"`)

	// Verify ownership error for wrong release namespace
	require.EqualError(t, checkOwnership(deployFoo.Object, "rel-a", "ns-b"), `invalid ownership metadata; annotation validation error: key "meta.helm.sh/release-namespace" must equal "ns-b": current value is "ns-a"`)

	// Verify ownership error for wrong manager label
	_ = accessor.SetLabels(deployFoo.Object, map[string]string{
		appManagedByLabel: "helm",
	})
	assert.EqualError(t, checkOwnership(deployFoo.Object, "rel-a", "ns-a"), `invalid ownership metadata; label validation error: key "app.kubernetes.io/managed-by" must equal "Helm": current value is "helm"`)
}

func TestVerifyOwnershipBeforeDelete(t *testing.T) {
	var (
		releaseName      = "rel-a"
		releaseNamespace = "ns-a"
		labels           = map[string]string{
			appManagedByLabel: appManagedByHelm,
		}
		annotations = map[string]string{
			helmReleaseNameAnnotation:      releaseName,
			helmReleaseNamespaceAnnotation: releaseNamespace,
		}
		wrongAnnotations = map[string]string{
			helmReleaseNameAnnotation:      "rel-b",
			helmReleaseNamespaceAnnotation: releaseNamespace,
		}
	)

	// Test all resources properly owned
	t.Run("all resources owned", func(t *testing.T) {
		owned1 := newDeploymentWithOwner("owned1", "ns-a", labels, annotations)
		owned2 := newDeploymentWithOwner("owned2", "ns-a", labels, annotations)
		resources := kube.ResourceList{owned1, owned2}

		ownedList, unownedList, _, err := verifyOwnershipBeforeDelete(resources, releaseName, releaseNamespace)
		require.NoError(t, err)
		assert.Len(t, ownedList, 2)
		assert.Empty(t, unownedList)
	})

	// Test mix of owned and unowned resources
	t.Run("mixed ownership", func(t *testing.T) {
		owned := newDeploymentWithOwner("owned", "ns-a", labels, annotations)
		unowned := newDeploymentWithOwner("unowned", "ns-a", labels, wrongAnnotations)
		resources := kube.ResourceList{owned, unowned}

		ownedList, unownedList, _, err := verifyOwnershipBeforeDelete(resources, releaseName, releaseNamespace)
		require.NoError(t, err)
		require.Len(t, ownedList, 1)
		require.Len(t, unownedList, 1)
		assert.Equal(t, "owned", ownedList[0].Name)
		assert.Equal(t, "unowned", unownedList[0].Name)
	})

	// Test resource not found (should be skipped - not in either list)
	t.Run("resource not found", func(t *testing.T) {
		missing := newMissingDeployment("missing", "ns-a")
		resources := kube.ResourceList{missing}

		ownedList, unownedList, _, err := verifyOwnershipBeforeDelete(resources, releaseName, releaseNamespace)
		require.NoError(t, err)
		assert.Empty(t, ownedList)
		assert.Empty(t, unownedList)
	})

	// Test resource with no ownership metadata
	t.Run("no ownership metadata", func(t *testing.T) {
		noMeta := newDeploymentWithOwner("no-meta", "ns-a", nil, nil)
		resources := kube.ResourceList{noMeta}

		ownedList, unownedList, _, err := verifyOwnershipBeforeDelete(resources, releaseName, releaseNamespace)
		require.NoError(t, err)
		assert.Empty(t, ownedList)
		assert.Len(t, unownedList, 1)
	})

	// Test resource owned by different release
	t.Run("owned by different release", func(t *testing.T) {
		otherRelease := newDeploymentWithOwner("other", "ns-a", labels, wrongAnnotations)
		resources := kube.ResourceList{otherRelease}

		ownedList, unownedList, _, err := verifyOwnershipBeforeDelete(resources, releaseName, releaseNamespace)
		require.NoError(t, err)
		assert.Empty(t, ownedList)
		assert.Len(t, unownedList, 1)
	})

	// Test mixed scenario: owned, unowned, and missing resources
	t.Run("mixed with missing resources", func(t *testing.T) {
		owned := newDeploymentWithOwner("owned", "ns-a", labels, annotations)
		unowned := newDeploymentWithOwner("unowned", "ns-a", labels, wrongAnnotations)
		missing := newMissingDeployment("missing", "ns-a")
		resources := kube.ResourceList{owned, unowned, missing}

		ownedList, unownedList, _, err := verifyOwnershipBeforeDelete(resources, releaseName, releaseNamespace)
		require.NoError(t, err)
		require.Len(t, ownedList, 1)
		require.Len(t, unownedList, 1)
		assert.Equal(t, "owned", ownedList[0].Name)
		assert.Equal(t, "unowned", unownedList[0].Name)
	})
}

func TestSetMetadataVisitor(t *testing.T) {
	var (
		deployFoo = newDeploymentResource("foo", "ns-a", "")
		deployBar = newDeploymentResource("bar", "ns-a-system", "")
		resources = kube.ResourceList{deployFoo, deployBar}
	)

	// Set release tracking metadata and verify no error
	require.NoError(t, resources.Visit(setMetadataVisitor("rel-a", "ns-a", true)))

	// Verify that release "b" cannot take ownership of "a"
	require.Error(t, resources.Visit(setMetadataVisitor("rel-b", "ns-a", false)))

	// Force release "b" to take ownership
	require.NoError(t, resources.Visit(setMetadataVisitor("rel-b", "ns-a", true)))

	// Check that there is now no ownership error when setting metadata without force
	require.NoError(t, resources.Visit(setMetadataVisitor("rel-b", "ns-a", false)))

	// Add a new resource that is missing ownership metadata and verify error
	resources.Append(newDeploymentResource("baz", "default", ""))
	assert.ErrorContains(t, resources.Visit(setMetadataVisitor("rel-b", "ns-a", false)), `Deployment "baz" in namespace "" cannot be owned`)
}

func TestValidateNameAndGenerateName(t *testing.T) {
	tests := []struct {
		name        string
		info        *resource.Info
		wantSkip    bool
		wantErr     bool
		errContains string
	}{
		{
			name:        "both name and generateName present",
			info:        newDeploymentResource("job-a", "foo", "job-a-"),
			wantSkip:    true,
			wantErr:     true,
			errContains: "metadata.name and metadata.generateName cannot both be set",
		},
		{
			name:     "only generateName present",
			info:     newDeploymentResource("", "foo", "job-a-"),
			wantSkip: true,
			wantErr:  false,
		},
		{
			name:     "only name present",
			info:     newDeploymentResource("job-a", "foo", ""),
			wantSkip: false,
			wantErr:  false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			skip, err := validateNameAndGenerateName(tc.info)

			if tc.wantErr {
				require.ErrorContains(t, err, tc.errContains)
			} else {
				require.NoError(t, err)
			}

			assert.Equal(t, tc.wantSkip, skip)
		})
	}
}
