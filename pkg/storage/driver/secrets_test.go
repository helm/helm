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
	"errors"
	"reflect"
	"testing"

	v1 "k8s.io/api/core/v1"

	"helm.sh/helm/v4/pkg/release"
	"helm.sh/helm/v4/pkg/release/common"
	rspb "helm.sh/helm/v4/pkg/release/v1"
)

func TestSecretName(t *testing.T) {
	c := newTestFixtureSecrets(t)
	if c.Name() != SecretsDriverName {
		t.Errorf("Expected name to be %q, got %q", SecretsDriverName, c.Name())
	}
}

func TestSecretGet(t *testing.T) {
	vers := 1
	name := "smug-pigeon"
	namespace := "default"
	key := testKey(name, vers)
	rel := releaseStub(name, vers, namespace, common.StatusDeployed)

	secrets := newTestFixtureSecrets(t, []*rspb.Release{rel}...)

	// get release with key
	got, err := secrets.Get(key)
	if err != nil {
		t.Fatalf("Failed to get release: %s", err)
	}
	// compare fetched release with original
	if !reflect.DeepEqual(rel, got) {
		t.Errorf("Expected {%v}, got {%v}", rel, got)
	}
}

func TestUNcompressedSecretGet(t *testing.T) {
	vers := 1
	name := "smug-pigeon"
	namespace := "default"
	key := testKey(name, vers)
	rel := releaseStub(name, vers, namespace, common.StatusDeployed)

	// Create a test fixture which contains an uncompressed release
	secret, err := newSecretsObject(key, rel, nil)
	if err != nil {
		t.Fatalf("Failed to create secret: %s", err)
	}
	b, err := json.Marshal(rel)
	if err != nil {
		t.Fatalf("Failed to marshal release: %s", err)
	}
	secret.Data["release"] = []byte(base64.StdEncoding.EncodeToString(b))
	var mock MockSecretsInterface
	mock.objects = map[string]*v1.Secret{key: secret}
	secrets := NewSecrets(&mock)

	// get release with key
	got, err := secrets.Get(key)
	if err != nil {
		t.Fatalf("Failed to get release: %s", err)
	}
	// compare fetched release with original
	if !reflect.DeepEqual(rel, got) {
		t.Errorf("Expected {%v}, got {%v}", rel, got)
	}
}

func TestSecretList(t *testing.T) {
	secrets := newTestFixtureSecrets(t, []*rspb.Release{
		releaseStub("key-1", 1, "default", common.StatusUninstalled),
		releaseStub("key-2", 1, "default", common.StatusUninstalled),
		releaseStub("key-3", 1, "default", common.StatusDeployed),
		releaseStub("key-4", 1, "default", common.StatusDeployed),
		releaseStub("key-5", 1, "default", common.StatusSuperseded),
		releaseStub("key-6", 1, "default", common.StatusSuperseded),
	}...)

	// list all deleted releases
	del, err := secrets.List(func(rel release.Releaser) bool {
		rls := convertReleaserToV1(t, rel)
		return rls.Info.Status == common.StatusUninstalled
	})
	// check
	if err != nil {
		t.Errorf("Failed to list deleted: %s", err)
	}
	if len(del) != 2 {
		t.Errorf("Expected 2 deleted, got %d:\n%v\n", len(del), del)
	}

	// list all deployed releases
	dpl, err := secrets.List(func(rel release.Releaser) bool {
		rls := convertReleaserToV1(t, rel)
		return rls.Info.Status == common.StatusDeployed
	})
	// check
	if err != nil {
		t.Errorf("Failed to list deployed: %s", err)
	}
	if len(dpl) != 2 {
		t.Errorf("Expected 2 deployed, got %d", len(dpl))
	}

	// list all superseded releases
	ssd, err := secrets.List(func(rel release.Releaser) bool {
		rls := convertReleaserToV1(t, rel)
		return rls.Info.Status == common.StatusSuperseded
	})
	// check
	if err != nil {
		t.Errorf("Failed to list superseded: %s", err)
	}
	if len(ssd) != 2 {
		t.Errorf("Expected 2 superseded, got %d", len(ssd))
	}
	// Check if release having both system and custom labels, this is needed to ensure that selector filtering would work.
	rls := convertReleaserToV1(t, ssd[0])
	_, ok := rls.Labels["name"]
	if !ok {
		t.Fatalf("Expected 'name' label in results, actual %v", rls.Labels)
	}
	_, ok = rls.Labels["key1"]
	if !ok {
		t.Fatalf("Expected 'key1' label in results, actual %v", rls.Labels)
	}
}

func TestSecretQuery(t *testing.T) {
	secrets := newTestFixtureSecrets(t, []*rspb.Release{
		releaseStub("key-1", 1, "default", common.StatusUninstalled),
		releaseStub("key-2", 1, "default", common.StatusUninstalled),
		releaseStub("key-3", 1, "default", common.StatusDeployed),
		releaseStub("key-4", 1, "default", common.StatusDeployed),
		releaseStub("key-5", 1, "default", common.StatusSuperseded),
		releaseStub("key-6", 1, "default", common.StatusSuperseded),
	}...)

	rls, err := secrets.Query(map[string]string{"status": "deployed"})
	if err != nil {
		t.Fatalf("Failed to query: %s", err)
	}
	if len(rls) != 2 {
		t.Fatalf("Expected 2 results, actual %d", len(rls))
	}

	_, err = secrets.Query(map[string]string{"name": "notExist"})
	if err != ErrReleaseNotFound {
		t.Errorf("Expected {%v}, got {%v}", ErrReleaseNotFound, err)
	}
}

func TestSecretCreate(t *testing.T) {
	secrets := newTestFixtureSecrets(t)

	vers := 1
	name := "smug-pigeon"
	namespace := "default"
	key := testKey(name, vers)
	rel := releaseStub(name, vers, namespace, common.StatusDeployed)

	// store the release in a secret
	if err := secrets.Create(key, rel); err != nil {
		t.Fatalf("Failed to create release with key %q: %s", key, err)
	}

	// get the release back
	got, err := secrets.Get(key)
	if err != nil {
		t.Fatalf("Failed to get release with key %q: %s", key, err)
	}

	// compare created release with original
	if !reflect.DeepEqual(rel, got) {
		t.Errorf("Expected {%v}, got {%v}", rel, got)
	}
}

func TestSecretUpdate(t *testing.T) {
	vers := 1
	name := "smug-pigeon"
	namespace := "default"
	key := testKey(name, vers)
	rel := releaseStub(name, vers, namespace, common.StatusDeployed)

	secrets := newTestFixtureSecrets(t, []*rspb.Release{rel}...)

	// modify release status code
	rel.Info.Status = common.StatusSuperseded

	// perform the update
	if err := secrets.Update(key, rel); err != nil {
		t.Fatalf("Failed to update release: %s", err)
	}

	// fetch the updated release
	goti, err := secrets.Get(key)
	if err != nil {
		t.Fatalf("Failed to get release with key %q: %s", key, err)
	}
	got := convertReleaserToV1(t, goti)

	// check release has actually been updated by comparing modified fields
	if rel.Info.Status != got.Info.Status {
		t.Errorf("Expected status %s, got status %s", rel.Info.Status.String(), got.Info.Status.String())
	}
}

func TestSecretDelete(t *testing.T) {
	vers := 1
	name := "smug-pigeon"
	namespace := "default"
	key := testKey(name, vers)
	rel := releaseStub(name, vers, namespace, common.StatusDeployed)

	secrets := newTestFixtureSecrets(t, []*rspb.Release{rel}...)

	// perform the delete on a non-existing release
	_, err := secrets.Delete("nonexistent")
	if err != ErrReleaseNotFound {
		t.Fatalf("Expected ErrReleaseNotFound, got: {%v}", err)
	}

	// perform the delete
	rls, err := secrets.Delete(key)
	if err != nil {
		t.Fatalf("Failed to delete release with key %q: %s", key, err)
	}
	if !reflect.DeepEqual(rel, rls) {
		t.Errorf("Expected {%v}, got {%v}", rel, rls)
	}
	_, err = secrets.Get(key)
	if !errors.Is(err, ErrReleaseNotFound) {
		t.Errorf("Expected {%v}, got {%v}", ErrReleaseNotFound, err)
	}
}
