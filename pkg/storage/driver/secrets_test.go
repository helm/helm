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

func labeledReleaseStub(name string, vers int, namespace string, status rspb.Status, labels map[string]string) *rspb.Release {
	rls := releaseStub(name, vers, namespace, status)
	rls.Labels = labels
	return rls
}

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
	rel := releaseStub(name, vers, namespace, rspb.StatusDeployed)

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
	rel := releaseStub(name, vers, namespace, rspb.StatusDeployed)

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
		labeledReleaseStub("key-1", 1, "default", rspb.StatusUninstalled, map[string]string{"releaseName": "key-1"}),
		labeledReleaseStub("key-2", 1, "default", rspb.StatusUninstalled, map[string]string{"releaseName": "key-2"}),
		labeledReleaseStub("key-3", 1, "default", rspb.StatusDeployed, map[string]string{"releaseName": "key-3"}),
		labeledReleaseStub("key-4", 1, "default", rspb.StatusDeployed, map[string]string{"releaseName": "key-4"}),
		labeledReleaseStub("key-5", 1, "default", rspb.StatusSuperseded, map[string]string{"releaseName": "key-5"}),
		labeledReleaseStub("key-6", 1, "default", rspb.StatusSuperseded, map[string]string{"releaseName": "key-6"}),
	}...)

	// list all deleted releases
	del, err := secrets.List(func(rel *rspb.Release) bool {
		return rel.Info.Status == rspb.StatusUninstalled
	})
	// check
	if err != nil {
		t.Errorf("Failed to list deleted: %s", err)
	}
	if len(del) != 2 {
		t.Errorf("Expected 2 deleted, got %d:\n%v\n", len(del), del)
	}
	for _, delRls := range del {
		if rlsName, ok := delRls.Labels["releaseName"]; !ok || !(rlsName == "key-1" || rlsName == "key-2") {
			t.Errorf("Unexpected labels, expected 'key-1' or 'key-2', got '%s'", rlsName)
		}
	}

	// list all deployed releases
	dpl, err := secrets.List(func(rel *rspb.Release) bool {
		return rel.Info.Status == rspb.StatusDeployed
	})
	// check
	if err != nil {
		t.Errorf("Failed to list deployed: %s", err)
	}
	if len(dpl) != 2 {
		t.Errorf("Expected 2 deployed, got %d", len(dpl))
	}
	for _, dplRls := range dpl {
		if rlsName, ok := dplRls.Labels["releaseName"]; !ok || !(rlsName == "key-3" || rlsName == "key-4") {
			t.Errorf("Unexpected labels, expected 'key-3' or 'key-4', got '%s'", rlsName)
		}
	}

	// list all superseded releases
	ssd, err := secrets.List(func(rel *rspb.Release) bool {
		return rel.Info.Status == rspb.StatusSuperseded
	})
	// check
	if err != nil {
		t.Errorf("Failed to list superseded: %s", err)
	}
	if len(ssd) != 2 {
		t.Errorf("Expected 2 superseded, got %d", len(ssd))
	}
	for _, ssdRls := range ssd {
		if rlsName, ok := ssdRls.Labels["releaseName"]; !ok || !(rlsName == "key-5" || rlsName == "key-6") {
			t.Errorf("Unexpected labels, expected 'key-5' or 'key-6', got '%s'", rlsName)
		}
	}
}

func TestSecretQuery(t *testing.T) {
	secrets := newTestFixtureSecrets(t, []*rspb.Release{
		releaseStub("key-1", 1, "default", rspb.StatusUninstalled),
		releaseStub("key-2", 1, "default", rspb.StatusUninstalled),
		releaseStub("key-3", 1, "default", rspb.StatusDeployed),
		releaseStub("key-4", 1, "default", rspb.StatusDeployed),
		releaseStub("key-5", 1, "default", rspb.StatusSuperseded),
		releaseStub("key-6", 1, "default", rspb.StatusSuperseded),
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
	rel := releaseStub(name, vers, namespace, rspb.StatusDeployed)

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
	rel := releaseStub(name, vers, namespace, rspb.StatusDeployed)

	secrets := newTestFixtureSecrets(t, []*rspb.Release{rel}...)

	// modify release status code
	rel.Info.Status = rspb.StatusSuperseded

	// perform the update
	if err := secrets.Update(key, rel); err != nil {
		t.Fatalf("Failed to update release: %s", err)
	}

	// fetch the updated release
	got, err := secrets.Get(key)
	if err != nil {
		t.Fatalf("Failed to get release with key %q: %s", key, err)
	}

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
	rel := releaseStub(name, vers, namespace, rspb.StatusDeployed)

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

	// fetch the deleted release
	_, err = secrets.Get(key)
	if !reflect.DeepEqual(ErrReleaseNotFound, err) {
		t.Errorf("Expected {%v}, got {%v}", ErrReleaseNotFound, err)
	}
}
