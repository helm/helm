/*
Copyright 2016 The Kubernetes Authors All rights reserved.

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

package driver // import "k8s.io/helm/pkg/storage/driver"

import (
	"fmt"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/core/internalversion"

	rspb "k8s.io/helm/pkg/proto/hapi/release"
)

func releaseStub(name string, vers int32, namespace string, code rspb.Status_Code) *rspb.Release {
	return &rspb.Release{
		Name:      name,
		Version:   vers,
		Namespace: namespace,
		Info:      &rspb.Info{Status: &rspb.Status{Code: code}},
	}
}

func testKey(name string, vers int32) string {
	return fmt.Sprintf("%s.v%d", name, vers)
}

func tsFixtureMemory(t *testing.T) *Memory {
	hs := []*rspb.Release{
		// rls-a
		releaseStub("rls-a", 4, "default", rspb.Status_DEPLOYED),
		releaseStub("rls-a", 1, "default", rspb.Status_SUPERSEDED),
		releaseStub("rls-a", 3, "default", rspb.Status_SUPERSEDED),
		releaseStub("rls-a", 2, "default", rspb.Status_SUPERSEDED),
		// rls-b
		releaseStub("rls-b", 4, "default", rspb.Status_DEPLOYED),
		releaseStub("rls-b", 1, "default", rspb.Status_SUPERSEDED),
		releaseStub("rls-b", 3, "default", rspb.Status_SUPERSEDED),
		releaseStub("rls-b", 2, "default", rspb.Status_SUPERSEDED),
	}

	mem := NewMemory()
	for _, tt := range hs {
		err := mem.Create(testKey(tt.Name, tt.Version), tt)
		if err != nil {
			t.Fatalf("Test setup failed to create: %s\n", err)
		}
	}
	return mem
}

// newTestFixture initializes a MockConfigMapsInterface.
// ConfigMaps are created for each release provided.
func newTestFixtureCfgMaps(t *testing.T, releases ...*rspb.Release) *ConfigMaps {
	var mock MockConfigMapsInterface
	mock.Init(t, releases...)

	return NewConfigMaps(&mock)
}

// MockConfigMapsInterface mocks a kubernetes ConfigMapsInterface
type MockConfigMapsInterface struct {
	internalversion.ConfigMapInterface

	objects map[string]*api.ConfigMap
}

// Init initializes the MockConfigMapsInterface with the set of releases.
func (mock *MockConfigMapsInterface) Init(t *testing.T, releases ...*rspb.Release) {
	mock.objects = map[string]*api.ConfigMap{}

	for _, rls := range releases {
		objkey := testKey(rls.Name, rls.Version)

		cfgmap, err := newConfigMapsObject(objkey, rls, nil)
		if err != nil {
			t.Fatalf("Failed to create configmap: %s", err)
		}
		mock.objects[objkey] = cfgmap
	}
}

// Get returns the ConfigMap by name.
func (mock *MockConfigMapsInterface) Get(name string, options metav1.GetOptions) (*api.ConfigMap, error) {
	object, ok := mock.objects[name]
	if !ok {
		return nil, apierrors.NewNotFound(api.Resource("tests"), name)
	}
	return object, nil
}

// List returns the a of ConfigMaps.
func (mock *MockConfigMapsInterface) List(opts metav1.ListOptions) (*api.ConfigMapList, error) {
	var list api.ConfigMapList
	for _, cfgmap := range mock.objects {
		list.Items = append(list.Items, *cfgmap)
	}
	return &list, nil
}

// Create creates a new ConfigMap.
func (mock *MockConfigMapsInterface) Create(cfgmap *api.ConfigMap) (*api.ConfigMap, error) {
	name := cfgmap.ObjectMeta.Name
	if object, ok := mock.objects[name]; ok {
		return object, apierrors.NewAlreadyExists(api.Resource("tests"), name)
	}
	mock.objects[name] = cfgmap
	return cfgmap, nil
}

// Update updates a ConfigMap.
func (mock *MockConfigMapsInterface) Update(cfgmap *api.ConfigMap) (*api.ConfigMap, error) {
	name := cfgmap.ObjectMeta.Name
	if _, ok := mock.objects[name]; !ok {
		return nil, apierrors.NewNotFound(api.Resource("tests"), name)
	}
	mock.objects[name] = cfgmap
	return cfgmap, nil
}

// Delete deletes a ConfigMap by name.
func (mock *MockConfigMapsInterface) Delete(name string, opts *metav1.DeleteOptions) error {
	if _, ok := mock.objects[name]; !ok {
		return apierrors.NewNotFound(api.Resource("tests"), name)
	}
	delete(mock.objects, name)
	return nil
}

// newTestFixture initializes a MockSecretsInterface.
// Secrets are created for each release provided.
func newTestFixtureSecrets(t *testing.T, releases ...*rspb.Release) *Secrets {
	var mock MockSecretsInterface
	mock.Init(t, releases...)

	return NewSecrets(&mock)
}

// MockSecretsInterface mocks a kubernetes SecretsInterface
type MockSecretsInterface struct {
	internalversion.SecretInterface

	objects map[string]*api.Secret
}

// Init initializes the MockSecretsInterface with the set of releases.
func (mock *MockSecretsInterface) Init(t *testing.T, releases ...*rspb.Release) {
	mock.objects = map[string]*api.Secret{}

	for _, rls := range releases {
		objkey := testKey(rls.Name, rls.Version)

		secret, err := newSecretsObject(objkey, rls, nil)
		if err != nil {
			t.Fatalf("Failed to create secret: %s", err)
		}
		mock.objects[objkey] = secret
	}
}

// Get returns the Secret by name.
func (mock *MockSecretsInterface) Get(name string, options metav1.GetOptions) (*api.Secret, error) {
	object, ok := mock.objects[name]
	if !ok {
		return nil, apierrors.NewNotFound(api.Resource("tests"), name)
	}
	return object, nil
}

// List returns the a of Secret.
func (mock *MockSecretsInterface) List(opts metav1.ListOptions) (*api.SecretList, error) {
	var list api.SecretList
	for _, secret := range mock.objects {
		list.Items = append(list.Items, *secret)
	}
	return &list, nil
}

// Create creates a new Secret.
func (mock *MockSecretsInterface) Create(secret *api.Secret) (*api.Secret, error) {
	name := secret.ObjectMeta.Name
	if object, ok := mock.objects[name]; ok {
		return object, apierrors.NewAlreadyExists(api.Resource("tests"), name)
	}
	mock.objects[name] = secret
	return secret, nil
}

// Update updates a Secret.
func (mock *MockSecretsInterface) Update(secret *api.Secret) (*api.Secret, error) {
	name := secret.ObjectMeta.Name
	if _, ok := mock.objects[name]; !ok {
		return nil, apierrors.NewNotFound(api.Resource("tests"), name)
	}
	mock.objects[name] = secret
	return secret, nil
}

// Delete deletes a Secret by name.
func (mock *MockSecretsInterface) Delete(name string, opts *metav1.DeleteOptions) error {
	if _, ok := mock.objects[name]; !ok {
		return apierrors.NewNotFound(api.Resource("tests"), name)
	}
	delete(mock.objects, name)
	return nil
}
