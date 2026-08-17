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
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kuberuntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest/fake"

	"helm.sh/helm/v4/pkg/kube"
	kubefake "helm.sh/helm/v4/pkg/kube/fake"
	rcommon "helm.sh/helm/v4/pkg/release/common"
)

// existingNamespaceResourceList returns a ResourceList for a namespace that
// already exists in the cluster: its REST client answers GETs with the
// namespace object.
func existingNamespaceResourceList(name string) kube.ResourceList {
	obj := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}

	corev1GV := schema.GroupVersion{Version: "v1"}
	corev1Codec := scheme.Codecs.CodecForVersions(scheme.Codecs.LegacyCodec(corev1GV), scheme.Codecs.UniversalDecoder(corev1GV), corev1GV, corev1GV)
	body := []byte(kuberuntime.EncodeOrDie(corev1Codec, obj))

	resInfo := resource.Info{
		Name: name,
		Mapping: &meta.RESTMapping{
			Resource:         schema.GroupVersionResource{Version: "v1", Resource: "namespaces"},
			GroupVersionKind: schema.GroupVersionKind{Version: "v1", Kind: "Namespace"},
			Scope:            meta.RESTScopeRoot,
		},
		Object: obj,
	}
	resInfo.Client = &fake.RESTClient{
		GroupVersion:         corev1GV,
		NegotiatedSerializer: scheme.Codecs.WithoutConversion(),
		Client: fake.CreateHTTPClient(func(_ *http.Request) (*http.Response, error) {
			header := http.Header{}
			header.Set("Content-Type", kuberuntime.ContentTypeJSON)
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     header,
				Body:       io.NopCloser(bytes.NewReader(body)),
			}, nil
		}),
	}
	var resourceList kube.ResourceList
	resourceList.Append(&resInfo)
	return resourceList
}

// namespaceCreateForbiddenKubeClient simulates a user whose RBAC role allows
// reading namespaces but not creating them: any attempt to create a Namespace
// is rejected with a Forbidden error, while the namespace itself already
// exists in the cluster.
type namespaceCreateForbiddenKubeClient struct {
	kubefake.FailingKubeClient
	nsResources kube.ResourceList
}

func (f *namespaceCreateForbiddenKubeClient) Build(r io.Reader, validate bool) (kube.ResourceList, error) {
	buf, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	if strings.Contains(string(buf), "kind: Namespace") {
		return f.nsResources, nil
	}
	return f.FailingKubeClient.Build(bytes.NewReader(buf), validate)
}

func (f *namespaceCreateForbiddenKubeClient) Create(resources kube.ResourceList, options ...kube.ClientCreateOption) (*kube.Result, error) {
	for _, info := range resources {
		if info.Mapping != nil && info.Mapping.GroupVersionKind.Kind == "Namespace" {
			return nil, apierrors.NewForbidden(
				schema.GroupResource{Resource: "namespaces"},
				info.Name,
				errors.New(`User "system:serviceaccount:mcp:mcp" cannot create resource "namespaces" in API group "" at the cluster scope`),
			)
		}
	}
	return f.FailingKubeClient.Create(resources, options...)
}

func TestInstallReleaseCreateNamespace_NamespaceExistsWithoutCreatePermission(t *testing.T) {
	is := assert.New(t)
	req := require.New(t)

	config := actionConfigFixture(t)
	config.KubeClient = &namespaceCreateForbiddenKubeClient{
		FailingKubeClient: kubefake.FailingKubeClient{PrintingKubeClient: kubefake.PrintingKubeClient{Out: io.Discard}},
		nsResources:       existingNamespaceResourceList("spaced"),
	}
	instAction := installActionWithConfig(config)
	instAction.CreateNamespace = true

	resi, err := instAction.Run(buildChart(), map[string]any{})
	req.NoError(err, "--create-namespace must not fail when the namespace already exists and the user may not create namespaces")
	res, err := releaserToV1Release(resi)
	req.NoError(err)

	r, err := instAction.cfg.Releases.Get(res.Name, res.Version)
	req.NoError(err)
	rel, err := releaserToV1Release(r)
	req.NoError(err)

	is.Equal(rcommon.StatusDeployed, rel.Info.Status)
	is.Equal("Install complete", rel.Info.Description)
}
