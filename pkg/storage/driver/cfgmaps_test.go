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

package driver

import (
	"encoding/base64"
	"encoding/json"
	"reflect"
	"testing"

	v1 "k8s.io/api/core/v1"

	rspb "helm.sh/helm/v3/pkg/release"
)

func TestConfigMapName(t *testing.T) {
	c := newTestFixtureCfgMaps(t)
	if c.Name() != ConfigMapsDriverName {
		t.Errorf("Expected name to be %q, got %q", ConfigMapsDriverName, c.Name())
	}
}

func TestConfigMapGet(t *testing.T) {
	vers := 1
	name := "smug-pigeon"
	namespace := "default"
	key := testKey(name, vers)
	rel := releaseStub(name, vers, namespace, rspb.StatusDeployed)

	cfgmaps := newTestFixtureCfgMaps(t, []*rspb.Release{rel}...)

	// get release with key
	got, err := cfgmaps.Get(key)
	if err != nil {
		t.Fatalf("Failed to get release: %s", err)
	}
	// compare fetched release with original
	if !reflect.DeepEqual(rel, got) {
		t.Errorf("Expected {%v}, got {%v}", rel, got)
	}
}

func TestUncompressedConfigMapGet(t *testing.T) {
	vers := 1
	name := "smug-pigeon"
	namespace := "default"
	key := testKey(name, vers)
	rel := releaseStub(name, vers, namespace, rspb.StatusDeployed)

	// Create a test fixture which contains an uncompressed release
	cfgmap, err := newConfigMapsObject(key, rel, nil)
	if err != nil {
		t.Fatalf("Failed to create configmap: %s", err)
	}
	b, err := json.Marshal(rel)
	if err != nil {
		t.Fatalf("Failed to marshal release: %s", err)
	}
	cfgmap.Data["release"] = base64.StdEncoding.EncodeToString(b)
	var mock MockConfigMapsInterface
	mock.objects = map[string]*v1.ConfigMap{key: cfgmap}
	cfgmaps := NewConfigMaps(&mock)

	// get release with key
	got, err := cfgmaps.Get(key)
	if err != nil {
		t.Fatalf("Failed to get release: %s", err)
	}
	// compare fetched release with original
	if !reflect.DeepEqual(rel, got) {
		t.Errorf("Expected {%v}, got {%v}", rel, got)
	}
}

func TestConfigMapList(t *testing.T) {
	cfgmaps := newTestFixtureCfgMaps(t, []*rspb.Release{
		releaseStub("key-1", 1, "default", rspb.StatusUninstalled),
		releaseStub("key-2", 1, "default", rspb.StatusUninstalled),
		releaseStub("key-3", 1, "default", rspb.StatusDeployed),
		releaseStub("key-4", 1, "default", rspb.StatusDeployed),
		releaseStub("key-5", 1, "default", rspb.StatusSuperseded),
		releaseStub("key-6", 1, "default", rspb.StatusSuperseded),
	}...)

	// list all deleted releases
	del, err := cfgmaps.List(func(rel *rspb.Release) bool {
		return rel.Info.Status == rspb.StatusUninstalled
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
		return rel.Info.Status == rspb.StatusDeployed
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
		return rel.Info.Status == rspb.StatusSuperseded
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
	cfgmaps := newTestFixtureCfgMaps(t)

	vers := 1
	name := "smug-pigeon"
	namespace := "default"
	key := testKey(name, vers)
	rel := releaseStub(name, vers, namespace, rspb.StatusDeployed)

	// store the release in a configmap
	if err := cfgmaps.Create(key, rel); err != nil {
		t.Fatalf("Failed to create release with key %q: %s", key, err)
	}

	// get the release back
	got, err := cfgmaps.Get(key)
	if err != nil {
		t.Fatalf("Failed to get release with key %q: %s", key, err)
	}

	// compare created release with original
	if !reflect.DeepEqual(rel, got) {
		t.Errorf("Expected {%v}, got {%v}", rel, got)
	}
}

func TestConfigMapUpdate(t *testing.T) {
	vers := 1
	name := "smug-pigeon"
	namespace := "default"
	key := testKey(name, vers)
	rel := releaseStub(name, vers, namespace, rspb.StatusDeployed)

	cfgmaps := newTestFixtureCfgMaps(t, []*rspb.Release{rel}...)

	// modify release status code
	rel.Info.Status = rspb.StatusSuperseded

	// perform the update
	if err := cfgmaps.Update(key, rel); err != nil {
		t.Fatalf("Failed to update release: %s", err)
	}

	// fetch the updated release
	got, err := cfgmaps.Get(key)
	if err != nil {
		t.Fatalf("Failed to get release with key %q: %s", key, err)
	}

	// check release has actually been updated by comparing modified fields
	if rel.Info.Status != got.Info.Status {
		t.Errorf("Expected status %s, got status %s", rel.Info.Status.String(), got.Info.Status.String())
	}
}
