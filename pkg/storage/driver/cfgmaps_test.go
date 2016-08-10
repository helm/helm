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

package driver

import (
	"reflect"
	"testing"

	rspb "k8s.io/helm/pkg/proto/hapi/release"

	"k8s.io/kubernetes/pkg/api"
	kberrs "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/client/unversioned"
)

func TestConfigMapGet(t *testing.T) {
	key := "key-1"
	rel := newTestRelease(key, 1, rspb.Status_DEPLOYED)

	cfgmaps := newTestFixture(t, []*rspb.Release{rel}...)

	// get release with key
	got, err := cfgmaps.Get(key)
	if err != nil {
		t.Fatalf("Failed to get release: %s", err)
	}
	// compare fetched release with original
	if !reflect.DeepEqual(rel, got) {
		t.Errorf("Expected {%q}, got {%q}", rel, got)
	}
}

func TestConfigMapList(t *testing.T) {
	cfgmaps := newTestFixture(t, []*rspb.Release{
		newTestRelease("key-1", 1, rspb.Status_DELETED),
		newTestRelease("key-2", 1, rspb.Status_DELETED),
		newTestRelease("key-3", 1, rspb.Status_DEPLOYED),
		newTestRelease("key-4", 1, rspb.Status_DEPLOYED),
		newTestRelease("key-5", 1, rspb.Status_SUPERSEDED),
		newTestRelease("key-6", 1, rspb.Status_SUPERSEDED),
	}...)

	// list all deleted releases
	del, err := cfgmaps.List(func(rel *rspb.Release) bool {
		return rel.Info.Status.Code == rspb.Status_DELETED
	})
	// check
	if err != nil {
		t.Errorf("Failed to list deleted: %s", err)
	}
	if len(del) != 2 {
		t.Errorf("Expected 2 deleted, got %d:\n%v\n", len(del), del)
	}

	// list all deployed releases
	dpl, err := cfgmaps.List(func(rel *rspb.Release) bool {
		return rel.Info.Status.Code == rspb.Status_DEPLOYED
	})
	// check
	if err != nil {
		t.Errorf("Failed to list deployed: %s", err)
	}
	if len(dpl) != 2 {
		t.Errorf("Expected 2 deployed, got %d", len(dpl))
	}

	// list all superseded releases
	ssd, err := cfgmaps.List(func(rel *rspb.Release) bool {
		return rel.Info.Status.Code == rspb.Status_SUPERSEDED
	})
	// check
	if err != nil {
		t.Errorf("Failed to list superseded: %s", err)
	}
	if len(ssd) != 2 {
		t.Errorf("Expected 2 superseded, got %d", len(ssd))
	}
}

func TestConfigMapCreate(t *testing.T) {
	cfgmaps := newTestFixture(t)

	key := "key-1"
	rel := newTestRelease(key, 1, rspb.Status_DEPLOYED)

	// store the release in a configmap
	if err := cfgmaps.Create(rel); err != nil {
		t.Fatalf("Failed to create release with key %q: %s", key, err)
	}

	// get the release back
	got, err := cfgmaps.Get(key)
	if err != nil {
		t.Fatalf("Failed to get release with key %q: %s", key, err)
	}

	// compare created release with original
	if !reflect.DeepEqual(rel, got) {
		t.Errorf("Expected {%q}, got {%q}", rel, got)
	}
}

func TestConfigMapUpdate(t *testing.T) {
	key := "key-1"
	rel := newTestRelease(key, 1, rspb.Status_DEPLOYED)

	cfgmaps := newTestFixture(t, []*rspb.Release{rel}...)

	// modify release status code & version
	rel = newTestRelease(key, 2, rspb.Status_SUPERSEDED)

	// perform the update
	if err := cfgmaps.Update(rel); err != nil {
		t.Fatalf("Failed to update release: %s", err)
	}

	// fetch the updated release
	got, err := cfgmaps.Get(key)
	if err != nil {
		t.Fatalf("Failed to get release with key %q: %s", key, err)
	}

	// check release has actually been updated by comparing modified fields
	switch {
	case rel.Info.Status.Code != got.Info.Status.Code:
		t.Errorf("Expected status %s, got status %s", rel.Info.Status.Code, got.Info.Status.Code)
	case rel.Version != got.Version:
		t.Errorf("Expected version %d, got version %d", rel.Version, got.Version)
	}
}

// newTestFixture initializes a MockConfigMapsInterface.
// ConfigMaps are created for each release provided.
func newTestFixture(t *testing.T, releases ...*rspb.Release) *ConfigMaps {
	var mock MockConfigMapsInterface
	mock.Init(t, releases...)

	return NewConfigMaps(&mock)
}

// newTestRelease creates a release object for testing.
func newTestRelease(key string, version int32, status rspb.Status_Code) *rspb.Release {
	return &rspb.Release{Name: key, Info: &rspb.Info{Status: &rspb.Status{Code: status}}, Version: version}
}

// MockConfigMapsInterface mocks a kubernetes ConfigMapsInterface
type MockConfigMapsInterface struct {
	unversioned.ConfigMapsInterface

	objects map[string]*api.ConfigMap
}

func (mock *MockConfigMapsInterface) Init(t *testing.T, releases ...*rspb.Release) {
	mock.objects = map[string]*api.ConfigMap{}

	for _, rls := range releases {
		cfgmap, err := newConfigMapsObject(rls, nil)
		if err != nil {
			t.Fatalf("Failed to create configmap: %s", err)
		}
		mock.objects[rls.Name] = cfgmap
	}
}

func (mock *MockConfigMapsInterface) Get(name string) (*api.ConfigMap, error) {
	object, ok := mock.objects[name]
	if !ok {
		return nil, kberrs.NewNotFound(api.Resource("tests"), name)
	}
	return object, nil
}

func (mock *MockConfigMapsInterface) List(opts api.ListOptions) (*api.ConfigMapList, error) {
	var list api.ConfigMapList
	for _, cfgmap := range mock.objects {
		list.Items = append(list.Items, *cfgmap)
	}
	return &list, nil
}

func (mock *MockConfigMapsInterface) Create(cfgmap *api.ConfigMap) (*api.ConfigMap, error) {
	name := cfgmap.ObjectMeta.Name
	if object, ok := mock.objects[name]; ok {
		return object, kberrs.NewAlreadyExists(api.Resource("tests"), name)
	}
	mock.objects[name] = cfgmap
	return cfgmap, nil
}

func (mock *MockConfigMapsInterface) Update(cfgmap *api.ConfigMap) (*api.ConfigMap, error) {
	name := cfgmap.ObjectMeta.Name
	if _, ok := mock.objects[name]; !ok {
		return nil, kberrs.NewNotFound(api.Resource("tests"), name)
	}
	mock.objects[name] = cfgmap
	return cfgmap, nil
}

func (mock *MockConfigMapsInterface) Delete(name string) error {
	if _, ok := mock.objects[name]; !ok {
		return kberrs.NewNotFound(api.Resource("tests"), name)
	}
	delete(mock.objects, name)
	return nil
}
