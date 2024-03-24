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

	"helm.sh/helm/v3/pkg/kube"

	"github.com/stretchr/testify/assert"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest/fake"
)

func newDeploymentResource(name, namespace string) *resource.Info {
	return &resource.Info{
		Name: name,
		Mapping: &meta.RESTMapping{
			Resource:         schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployment"},
			GroupVersionKind: schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
		},
		Object: &appsv1.Deployment{
			ObjectMeta: v1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
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
		Client: fake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
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
	assert.NoError(t, err)
	assert.Len(t, found, 1)
	assert.Equal(t, found[0], existing)
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
	assert.NoError(t, err)
	assert.Len(t, found, 1)
	assert.Equal(t, found[0], existing)

	// Verify that an existing resource that lacks labels/annotations results in an error
	resources = append(resources, conflict)
	_, err = existingResourceConflict(resources, releaseName, releaseNamespace)
	assert.Error(t, err)
}

func TestCheckOwnership(t *testing.T) {
	deployFoo := newDeploymentResource("foo", "ns-a")

	// Verify that a resource that lacks labels/annotations is not owned
	err := checkOwnership(deployFoo.Object, "rel-a", "ns-a")
	assert.EqualError(t, err, `invalid ownership metadata; label validation error: missing key "app.kubernetes.io/managed-by": must be set to "Helm"; annotation validation error: missing key "meta.helm.sh/release-name": must be set to "rel-a"; annotation validation error: missing key "meta.helm.sh/release-namespace": must be set to "ns-a"`)

	// Set managed by label and verify annotation error message
	_ = accessor.SetLabels(deployFoo.Object, map[string]string{
		appManagedByLabel: appManagedByHelm,
	})
	err = checkOwnership(deployFoo.Object, "rel-a", "ns-a")
	assert.EqualError(t, err, `invalid ownership metadata; annotation validation error: missing key "meta.helm.sh/release-name": must be set to "rel-a"; annotation validation error: missing key "meta.helm.sh/release-namespace": must be set to "ns-a"`)

	// Set only the release name annotation and verify missing release namespace error message
	_ = accessor.SetAnnotations(deployFoo.Object, map[string]string{
		helmReleaseNameAnnotation: "rel-a",
	})
	err = checkOwnership(deployFoo.Object, "rel-a", "ns-a")
	assert.EqualError(t, err, `invalid ownership metadata; annotation validation error: missing key "meta.helm.sh/release-namespace": must be set to "ns-a"`)

	// Set both release name and namespace annotations and verify no ownership errors
	_ = accessor.SetAnnotations(deployFoo.Object, map[string]string{
		helmReleaseNameAnnotation:      "rel-a",
		helmReleaseNamespaceAnnotation: "ns-a",
	})
	err = checkOwnership(deployFoo.Object, "rel-a", "ns-a")
	assert.NoError(t, err)

	// Verify ownership error for wrong release name
	err = checkOwnership(deployFoo.Object, "rel-b", "ns-a")
	assert.EqualError(t, err, `invalid ownership metadata; annotation validation error: key "meta.helm.sh/release-name" must equal "rel-b": current value is "rel-a"`)

	// Verify ownership error for wrong release namespace
	err = checkOwnership(deployFoo.Object, "rel-a", "ns-b")
	assert.EqualError(t, err, `invalid ownership metadata; annotation validation error: key "meta.helm.sh/release-namespace" must equal "ns-b": current value is "ns-a"`)

	// Verify ownership error for wrong manager label
	_ = accessor.SetLabels(deployFoo.Object, map[string]string{
		appManagedByLabel: "helm",
	})
	err = checkOwnership(deployFoo.Object, "rel-a", "ns-a")
	assert.EqualError(t, err, `invalid ownership metadata; label validation error: key "app.kubernetes.io/managed-by" must equal "Helm": current value is "helm"`)
}

func TestSetMetadataVisitor(t *testing.T) {
	var (
		err       error
		deployFoo = newDeploymentResource("foo", "ns-a")
		deployBar = newDeploymentResource("bar", "ns-a-system")
		resources = kube.ResourceList{deployFoo, deployBar}
	)

	// Set release tracking metadata and verify no error
	err = resources.Visit(setMetadataVisitor("rel-a", "ns-a", true))
	assert.NoError(t, err)

	// Verify that release "b" cannot take ownership of "a"
	err = resources.Visit(setMetadataVisitor("rel-b", "ns-a", false))
	assert.Error(t, err)

	// Force release "b" to take ownership
	err = resources.Visit(setMetadataVisitor("rel-b", "ns-a", true))
	assert.NoError(t, err)

	// Check that there is now no ownership error when setting metadata without force
	err = resources.Visit(setMetadataVisitor("rel-b", "ns-a", false))
	assert.NoError(t, err)

	// Add a new resource that is missing ownership metadata and verify error
	resources.Append(newDeploymentResource("baz", "default"))
	err = resources.Visit(setMetadataVisitor("rel-b", "ns-a", false))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), `Deployment "baz" in namespace "" cannot be owned`)
}
