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
	"context"
	"sync"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	applycorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	"k8s.io/client-go/kubernetes"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

// lazyClient is a workaround to deal with Kubernetes having an unstable client API.
// In Kubernetes v1.18 the defaults where removed which broke creating a
// client without an explicit configuration. ಠ_ಠ
type lazyClient struct {
	// client caches an initialized kubernetes client
	initClient sync.Once
	client     kubernetes.Interface
	clientErr  error

	// clientFn loads a kubernetes client
	clientFn func() (*kubernetes.Clientset, error)

	// namespace passed to each client request
	namespace string
}

func (s *lazyClient) init() error {
	s.initClient.Do(func() {
		s.client, s.clientErr = s.clientFn()
	})
	return s.clientErr
}

// secretClient implements a corev1.SecretsInterface
type secretClient struct{ *lazyClient }

var _ corev1.SecretInterface = (*secretClient)(nil)

func newSecretClient(lc *lazyClient) *secretClient {
	return &secretClient{lazyClient: lc}
}

func (s *secretClient) Create(ctx context.Context, secret *v1.Secret, opts metav1.CreateOptions) (result *v1.Secret, err error) {
	if err := s.init(); err != nil {
		return nil, err
	}
	return s.client.CoreV1().Secrets(s.namespace).Create(ctx, secret, opts)
}

func (s *secretClient) Update(ctx context.Context, secret *v1.Secret, opts metav1.UpdateOptions) (*v1.Secret, error) {
	if err := s.init(); err != nil {
		return nil, err
	}
	return s.client.CoreV1().Secrets(s.namespace).Update(ctx, secret, opts)
}

func (s *secretClient) Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error {
	if err := s.init(); err != nil {
		return err
	}
	return s.client.CoreV1().Secrets(s.namespace).Delete(ctx, name, opts)
}

func (s *secretClient) DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	if err := s.init(); err != nil {
		return err
	}
	return s.client.CoreV1().Secrets(s.namespace).DeleteCollection(ctx, opts, listOpts)
}

func (s *secretClient) Get(ctx context.Context, name string, opts metav1.GetOptions) (*v1.Secret, error) {
	if err := s.init(); err != nil {
		return nil, err
	}
	return s.client.CoreV1().Secrets(s.namespace).Get(ctx, name, opts)
}

func (s *secretClient) List(ctx context.Context, opts metav1.ListOptions) (*v1.SecretList, error) {
	if err := s.init(); err != nil {
		return nil, err
	}
	return s.client.CoreV1().Secrets(s.namespace).List(ctx, opts)
}

func (s *secretClient) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	if err := s.init(); err != nil {
		return nil, err
	}
	return s.client.CoreV1().Secrets(s.namespace).Watch(ctx, opts)
}

func (s *secretClient) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (*v1.Secret, error) {
	if err := s.init(); err != nil {
		return nil, err
	}
	return s.client.CoreV1().Secrets(s.namespace).Patch(ctx, name, pt, data, opts, subresources...)
}

func (s *secretClient) Apply(ctx context.Context, secretConfiguration *applycorev1.SecretApplyConfiguration, opts metav1.ApplyOptions) (*v1.Secret, error) {
	if err := s.init(); err != nil {
		return nil, err
	}
	return s.client.CoreV1().Secrets(s.namespace).Apply(ctx, secretConfiguration, opts)
}

// configMapClient implements a corev1.ConfigMapInterface
type configMapClient struct{ *lazyClient }

var _ corev1.ConfigMapInterface = (*configMapClient)(nil)

func newConfigMapClient(lc *lazyClient) *configMapClient {
	return &configMapClient{lazyClient: lc}
}

func (c *configMapClient) Create(ctx context.Context, configMap *v1.ConfigMap, opts metav1.CreateOptions) (*v1.ConfigMap, error) {
	if err := c.init(); err != nil {
		return nil, err
	}
	return c.client.CoreV1().ConfigMaps(c.namespace).Create(ctx, configMap, opts)
}

func (c *configMapClient) Update(ctx context.Context, configMap *v1.ConfigMap, opts metav1.UpdateOptions) (*v1.ConfigMap, error) {
	if err := c.init(); err != nil {
		return nil, err
	}
	return c.client.CoreV1().ConfigMaps(c.namespace).Update(ctx, configMap, opts)
}

func (c *configMapClient) Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error {
	if err := c.init(); err != nil {
		return err
	}
	return c.client.CoreV1().ConfigMaps(c.namespace).Delete(ctx, name, opts)
}

func (c *configMapClient) DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	if err := c.init(); err != nil {
		return err
	}
	return c.client.CoreV1().ConfigMaps(c.namespace).DeleteCollection(ctx, opts, listOpts)
}

func (c *configMapClient) Get(ctx context.Context, name string, opts metav1.GetOptions) (*v1.ConfigMap, error) {
	if err := c.init(); err != nil {
		return nil, err
	}
	return c.client.CoreV1().ConfigMaps(c.namespace).Get(ctx, name, opts)
}

func (c *configMapClient) List(ctx context.Context, opts metav1.ListOptions) (*v1.ConfigMapList, error) {
	if err := c.init(); err != nil {
		return nil, err
	}
	return c.client.CoreV1().ConfigMaps(c.namespace).List(ctx, opts)
}

func (c *configMapClient) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	if err := c.init(); err != nil {
		return nil, err
	}
	return c.client.CoreV1().ConfigMaps(c.namespace).Watch(ctx, opts)
}

func (c *configMapClient) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (*v1.ConfigMap, error) {
	if err := c.init(); err != nil {
		return nil, err
	}
	return c.client.CoreV1().ConfigMaps(c.namespace).Patch(ctx, name, pt, data, opts, subresources...)
}

func (c *configMapClient) Apply(ctx context.Context, configMap *applycorev1.ConfigMapApplyConfiguration, opts metav1.ApplyOptions) (*v1.ConfigMap, error) {
	if err := c.init(); err != nil {
		return nil, err
	}
	return c.client.CoreV1().ConfigMaps(c.namespace).Apply(ctx, configMap, opts)
}
