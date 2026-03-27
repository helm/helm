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
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	applycorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest/fake"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"

	"helm.sh/helm/v4/pkg/kube"
)

const fakeErrorMessage = "fake helm error"

func TestLazyClient_Init(t *testing.T) {
	kc := kube.New(nil)
	lazyClient := &lazyClient{
		namespace: namespace,
		clientFn:  kc.Factory.KubernetesClientSet,
	}

	assert.NoError(t, lazyClient.init())
}

func TestLazyClient_Init_Error(t *testing.T) {
	lazyClient := &lazyClient{
		namespace: namespace,
		clientFn:  getErrorClient,
	}

	assert.EqualError(t, lazyClient.init(), fakeErrorMessage)
}

func TestLazySecretClient_Create(t *testing.T) {
	secretClient := newSecretClient(getLazyClientWithFakeHTTP(t, "POST"))
	secret := mockSecret("created")

	_, err := secretClient.Create(t.Context(), secret, metav1.CreateOptions{})
	assert.NoError(t, err)
}

func TestLazySecretClient_Update(t *testing.T) {
	secretClient := newSecretClient(getLazyClientWithFakeHTTP(t, "PUT"))
	secret := mockSecret("updated")

	_, err := secretClient.Update(t.Context(), secret, metav1.UpdateOptions{})
	assert.NoError(t, err)
}

func TestLazySecretClient_Delete(t *testing.T) {
	secretClient := newSecretClient(getLazyClientWithFakeHTTP(t, "DELETE"))

	err := secretClient.Delete(t.Context(), "secret", metav1.DeleteOptions{})
	assert.NoError(t, err)
}

func TestLazySecretClient_DeleteCollection(t *testing.T) {
	secretClient := newSecretClient(getLazyClientWithFakeHTTP(t, "DELETE"))

	err := secretClient.DeleteCollection(t.Context(), metav1.DeleteOptions{}, metav1.ListOptions{})
	assert.NoError(t, err)
}

func TestLazySecretClient_Get(t *testing.T) {
	secretClient := newSecretClient(getLazyClientWithFakeHTTP(t, "GET"))

	_, err := secretClient.Get(t.Context(), "test", metav1.GetOptions{})
	assert.NoError(t, err)
}

func TestLazySecretClient_List(t *testing.T) {
	secretClient := newSecretClient(getLazyClientWithFakeHTTP(t, "GET"))

	_, err := secretClient.List(t.Context(), metav1.ListOptions{})
	assert.NoError(t, err)
}

func TestLazySecretClient_Watch(t *testing.T) {
	secretClient := newSecretClient(getLazyClientWithFakeHTTP(t, "GET"))

	result, err := secretClient.Watch(t.Context(), metav1.ListOptions{})
	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestLazySecretClient_Patch(t *testing.T) {
	secretClient := newSecretClient(getLazyClientWithFakeHTTP(t, "PATCH"))

	result, err := secretClient.Patch(t.Context(), "secret", types.MergePatchType, []byte("data"), metav1.PatchOptions{})
	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestLazySecretClient_InitErrorHandling(t *testing.T) {
	lazyClient := &lazyClient{
		namespace: namespace,
		clientFn:  getErrorClient,
	}
	secretClient := newSecretClient(lazyClient)

	_, err := secretClient.Create(t.Context(), mockSecret("created"), metav1.CreateOptions{})
	assert.EqualError(t, err, fakeErrorMessage)

	_, err = secretClient.Update(t.Context(), mockSecret("updated"), metav1.UpdateOptions{})
	assert.EqualError(t, err, fakeErrorMessage)

	assert.EqualError(t, secretClient.Delete(t.Context(), "secret", metav1.DeleteOptions{}), fakeErrorMessage)
	assert.EqualError(t, secretClient.DeleteCollection(t.Context(), metav1.DeleteOptions{}, metav1.ListOptions{}), fakeErrorMessage)

	_, err = secretClient.Get(t.Context(), "test", metav1.GetOptions{})
	assert.EqualError(t, err, fakeErrorMessage)

	_, err = secretClient.List(t.Context(), metav1.ListOptions{})
	assert.EqualError(t, err, fakeErrorMessage)

	_, err = secretClient.Watch(t.Context(), metav1.ListOptions{})
	assert.EqualError(t, err, fakeErrorMessage)

	_, err = secretClient.Patch(t.Context(), "secret", types.MergePatchType, []byte("data"), metav1.PatchOptions{})
	assert.EqualError(t, err, fakeErrorMessage)

	_, err = secretClient.Apply(t.Context(), &applycorev1.SecretApplyConfiguration{}, metav1.ApplyOptions{})
	assert.EqualError(t, err, fakeErrorMessage)
}

func TestLazyConfigMapClient_Create(t *testing.T) {
	configMapClient := newConfigMapClient(getLazyClientWithFakeHTTP(t, "POST"))
	configMap := mockConfigMap("created")

	_, err := configMapClient.Create(t.Context(), configMap, metav1.CreateOptions{})
	assert.NoError(t, err)
}

func TestLazyConfigMapClient_Update(t *testing.T) {
	configMapClient := newConfigMapClient(getLazyClientWithFakeHTTP(t, "PUT"))
	configMap := mockConfigMap("updated")

	_, err := configMapClient.Update(t.Context(), configMap, metav1.UpdateOptions{})
	assert.NoError(t, err)
}

func TestLazyConfigMapClient_Delete(t *testing.T) {
	configMapClient := newConfigMapClient(getLazyClientWithFakeHTTP(t, "DELETE"))

	err := configMapClient.Delete(t.Context(), "configMap", metav1.DeleteOptions{})
	assert.NoError(t, err)
}

func TestLazyConfigMapClient_DeleteCollection(t *testing.T) {
	configMapClient := newConfigMapClient(getLazyClientWithFakeHTTP(t, "DELETE"))

	err := configMapClient.DeleteCollection(t.Context(), metav1.DeleteOptions{}, metav1.ListOptions{})
	assert.NoError(t, err)
}

func TestLazyConfigMapClient_Get(t *testing.T) {
	configMapClient := newConfigMapClient(getLazyClientWithFakeHTTP(t, "GET"))

	_, err := configMapClient.Get(t.Context(), "test", metav1.GetOptions{})
	assert.NoError(t, err)
}

func TestLazyConfigMapClient_List(t *testing.T) {
	configMapClient := newConfigMapClient(getLazyClientWithFakeHTTP(t, "GET"))

	_, err := configMapClient.List(t.Context(), metav1.ListOptions{})
	assert.NoError(t, err)
}

func TestLazyConfigMapClient_Watch(t *testing.T) {
	configMapClient := newConfigMapClient(getLazyClientWithFakeHTTP(t, "GET"))

	result, err := configMapClient.Watch(t.Context(), metav1.ListOptions{})
	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestLazyConfigMapClient_Patch(t *testing.T) {
	configMapClient := newConfigMapClient(getLazyClientWithFakeHTTP(t, "PATCH"))

	result, err := configMapClient.Patch(t.Context(), "configMap", types.MergePatchType, []byte("data"), metav1.PatchOptions{})
	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestLazyConfigMapClient_InitErrorHandling(t *testing.T) {
	lazyClient := &lazyClient{
		namespace: namespace,
		clientFn:  getErrorClient,
	}
	configMapClient := newConfigMapClient(lazyClient)

	_, err := configMapClient.Create(t.Context(), mockConfigMap("created"), metav1.CreateOptions{})
	assert.EqualError(t, err, fakeErrorMessage)

	_, err = configMapClient.Update(t.Context(), mockConfigMap("updated"), metav1.UpdateOptions{})
	assert.EqualError(t, err, fakeErrorMessage)

	assert.EqualError(t, configMapClient.Delete(t.Context(), "configMap", metav1.DeleteOptions{}), fakeErrorMessage)
	assert.EqualError(t, configMapClient.DeleteCollection(t.Context(), metav1.DeleteOptions{}, metav1.ListOptions{}), fakeErrorMessage)

	_, err = configMapClient.Get(t.Context(), "test", metav1.GetOptions{})
	assert.EqualError(t, err, fakeErrorMessage)

	_, err = configMapClient.List(t.Context(), metav1.ListOptions{})
	assert.EqualError(t, err, fakeErrorMessage)

	_, err = configMapClient.Watch(t.Context(), metav1.ListOptions{})
	assert.EqualError(t, err, fakeErrorMessage)

	_, err = configMapClient.Patch(t.Context(), "configMap", types.MergePatchType, []byte("data"), metav1.PatchOptions{})
	assert.EqualError(t, err, fakeErrorMessage)

	_, err = configMapClient.Apply(t.Context(), &applycorev1.ConfigMapApplyConfiguration{}, metav1.ApplyOptions{})
	assert.EqualError(t, err, fakeErrorMessage)
}

// HELPER FUNCTIONS
func getErrorClient() (*kubernetes.Clientset, error) {
	return nil, errors.New(fakeErrorMessage)
}

func getLazyClientWithFakeHTTP(t *testing.T, httpMethod string) *lazyClient {
	t.Helper()
	testingFactory := cmdtesting.NewTestFactory()
	testingFactory.WithNamespace(namespace)

	testingFactory.Client = &fake.RESTClient{
		Client: fake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
			p, m := req.URL.Path, req.Method
			t.Logf("got request %s %s", p, m)
			assert.Equal(t, httpMethod, m, "expected request method %s, got %s", httpMethod, m)
			header := http.Header{}
			header.Set("Content-Type", runtime.ContentTypeJSON)
			return &http.Response{StatusCode: http.StatusOK, Header: header, Body: getMockBodyByPath(p)}, nil
		}),
	}

	return &lazyClient{
		namespace: "",
		clientFn:  testingFactory.KubernetesClientSet,
	}

}

var (
	codec = scheme.Codecs.LegacyCodec(scheme.Scheme.PrioritizedVersionsAllGroups()...)
)

func getMockBodyByPath(path string) io.ReadCloser {
	if strings.Contains(path, "secret") {
		mock := mockSecret("secret")
		return io.NopCloser(bytes.NewReader([]byte(runtime.EncodeOrDie(codec, mock))))
	}
	mock := mockConfigMap("configMap")
	return io.NopCloser(bytes.NewReader([]byte(runtime.EncodeOrDie(codec, mock))))
}

func mockSecret(name string) *v1.Secret {
	return &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: nil,
		},
	}
}

func mockConfigMap(name string) *v1.ConfigMap {
	return &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: nil,
		},
	}
}
