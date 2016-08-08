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
	"testing"
	"reflect"

	"k8s.io/kubernetes/pkg/runtime"
	rspb "k8s.io/helm/pkg/proto/hapi/release"
	"k8s.io/kubernetes/pkg/client/unversioned/testclient"
)

func TestConfigMapGet(t *testing.T) {
	// test release
	key := "key-1"
	rls := &rspb.Release{Name: key, Version: 1}
	
	// create test fixture
	cfgmaps := newTestFixture(t, rls)

	// get the release from configmaps
	got, err := cfgmaps.Get(key)
	if err != nil {
		t.Fatalf("failed to get release with key %q. %s", key, err)
	}

	// compare fetched release with original
	if !reflect.DeepEqual(rls, got) {
		t.Errorf("expected {%q}, got {%q}", rls, got)
	}
}

func TestConfigMapList(t *testing.T) {
	t.Skip("ConfigMapList")
}

func TestConfigMapCreate(t *testing.T) {
	// setup
	key := "key-1"
	rls := &rspb.Release{Name: "key-1", Version: 1}

	// create test fixture
	cfgmaps := newTestFixture(t, rls)

	// store the release in a configmap
	if err := cfgmaps.Create(rls); err != nil {
		t.Fatalf("failed to create release: %s", key, err)
	}

	if err := cfgmaps.Create(rls); err != nil {
		t.Fatalf("failed to create release: %s", key, err)
	}
	
	// get the release back
	got, err := cfgmaps.Get(key)
	if err != nil {
		t.Fatalf("failed to get release with key %q: %s", key, err)
	}

	// compare created release with original
	if !reflect.DeepEqual(rls, got) {
		t.Errorf("expected {%q}, got {%q}", rls, got)
	}
}

func TestConfigMapDelete(t *testing.T) {
	// setup
	key := "key-1"
	rls := &rspb.Release{Name: "key-1", Version: 1}

	// create test fixture
	cfgmaps := newTestFixture(t, rls)

	// delete the release
	got, err := cfgmaps.Delete(key)
	if err != nil {
		t.Fatalf("failed to delete release with key %q: %s", key, err)
	}

	// compare deleted release with original
	if !reflect.DeepEqual(rls, got) {
		t.Errorf("expected {%q}, got {%q}", rls, got)
	}
}

func TestConfigMapUpdate(t *testing.T) {
	// setup
	key := "key-1"
	rls := &rspb.Release{Name: "key-1", Version: 1}

	// create test fixture
	cfgmaps := newTestFixture(t, rls)

	// update release version
	rls.Version = 2

	// update the release
	if err := cfgmaps.Update(rls); err != nil {
		t.Fatalf("failed to update release with key %q: %s", key, err)
	}

	// fetch the updated release
	got, err := cfgmaps.Get(key)
	if err != nil {
		t.Fatalf("failed to get release with key %q: %s", key, err)
	}

	// validate the version was update correctly
	if rls.Version != got.Version {
		t.Fatalf("expected version %d, got version %d", rls.Version, got.Version)
	}
}

// newTestFixture prepopulates a mock implementation of a kubernetes
// ConfigMapsInterface returning an initialized driver.ConfigMaps.
func newTestFixture(t *testing.T, list ...*rspb.Release) *ConfigMaps {
	var objs []runtime.Object

	for i := range list {
		obj, err := newConfigMapsObject(list[i])
		if err != nil {
			t.Fatalf("failed to create object: %s", err)
		}
		objs = append(objs, obj)
	}

	return NewConfigMaps(&testclient.FakeConfigMaps{
		Fake: testclient.NewSimpleFake(objs...),
	})
}