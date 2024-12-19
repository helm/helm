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

//nolint:dupl
package driver

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"testing"

	v1 "k8s.io/api/core/v1"

	rspb "helm.sh/helm/v3/pkg/release"
)

func TestMultiSecretsName(t *testing.T) {
	c := newTestFixtureMultiSecrets(t)
	if c.Name() != MultiSecretsDriverName {
		t.Errorf("Expected name to be %q, got %q", MultiSecretsDriverName, c.Name())
	}
}

func TestMultiSecretsGet(t *testing.T) {
	vers := 1
	name := "smug-pigeon"
	namespace := "default"
	key := testKey(name, vers)
	rel := multiReleaseStub(name, vers, namespace, rspb.StatusDeployed)

	multisecrets := newTestFixtureMultiSecrets(t, []*rspb.Release{rel}...)

	// get release with key
	got, err := multisecrets.Get(key)
	if err != nil {
		t.Fatalf("Failed to get release: %s", err)
	}
	// compare fetched release with original
	if !reflect.DeepEqual(rel, got) {
		t.Errorf("Expected {%v}, got {%v}", rel, got)
	}
}

func TestUNcompressedMultiSecretsGet(t *testing.T) {
	vers := 1
	name := "smug-pigeon"
	namespace := "default"
	key := testKey(name, vers)
	rel := multiReleaseStub(name, vers, namespace, rspb.StatusDeployed)

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
	var mock MockMultiSecretsInterface
	mock.objects = map[string]*v1.Secret{key: secret}
	multisecrets := NewMultiSecrets(&mock)

	// get release with key
	got, err := multisecrets.Get(key)
	if err != nil {
		t.Fatalf("Failed to get release: %s", err)
	}
	// compare fetched release with original
	if !reflect.DeepEqual(rel, got) {
		t.Errorf("Expected {%v}, got {%v}", rel, got)
	}
}

func TestMultiSecretsList(t *testing.T) {
	multisecrets := fakeMultiSecretsWithRels(t)

	// list all deleted releases
	del, err := multisecrets.List(func(rel *rspb.Release) bool {
		return rel.Info.Status == rspb.StatusUninstalled
	})
	// check
	if err != nil {
		t.Errorf("Failed to list deleted: %s", err)
	}
	if len(del) != 2 {
		t.Errorf("Expected 2 deleted, got %d:\n%v\n", len(del), del)
	}

	multisecrets = fakeMultiSecretsWithRels(t)
	// list all deployed releases
	dpl, err := multisecrets.List(func(rel *rspb.Release) bool {
		return rel.Info.Status == rspb.StatusDeployed
	})
	// check
	if err != nil {
		t.Errorf("Failed to list deployed: %s", err)
	}
	if len(dpl) != 2 {
		t.Errorf("Expected 2 deployed, got %d", len(dpl))
	}

	multisecrets = fakeMultiSecretsWithRels(t)
	// list all superseded releases
	ssd, err := multisecrets.List(func(rel *rspb.Release) bool {
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

func TestMultiSecretsQuery(t *testing.T) {
	multisecrets := fakeMultiSecretsWithRels(t)

	rls, err := multisecrets.Query(map[string]string{"status": "deployed"})
	if err != nil {
		t.Fatalf("Failed to query: %s", err)
	}
	if len(rls) != 2 {
		t.Fatalf("Expected 2 results, actual %d", len(rls))
	}

	_, err = multisecrets.Query(map[string]string{"name": "notExist"})
	if err != ErrReleaseNotFound {
		t.Errorf("Expected {%v}, got {%v}", ErrReleaseNotFound, err)
	}
}

func TestMultiSecretsCreate(t *testing.T) {
	multisecrets := newTestFixtureMultiSecrets(t)

	vers := 1
	name := "smug-pigeon"
	namespace := "default"
	key := testKey(name, vers)
	rel := multiReleaseStub(name, vers, namespace, rspb.StatusDeployed)

	// store the release in a secret
	if err := multisecrets.Create(key, rel); err != nil {
		t.Fatalf("Failed to create release with key %q: %s", key, err)
	}
	// get the release back
	got, err := multisecrets.Get(key)
	if err != nil {
		t.Fatalf("Failed to get release with key %q: %s", key, err)
	}

	// compare created release with original
	if !reflect.DeepEqual(rel, got) {
		t.Errorf("Expected {%v}, got {%v}", rel, got)
	}
}

func TestMultiSecretsUpdate(t *testing.T) {
	vers := 1
	name := "smug-pigeon"
	namespace := "default"
	key := testKey(name, vers)
	rel := multiReleaseStub(name, vers, namespace, rspb.StatusDeployed)

	multisecrets := newTestFixtureMultiSecrets(t, []*rspb.Release{rel}...)

	// modify release status code
	rel.Info.Status = rspb.StatusSuperseded

	// perform the update
	if err := multisecrets.Update(key, rel); err != nil {
		t.Fatalf("Failed to update release: %s", err)
	}

	// fetch the updated release
	got, err := multisecrets.Get(key)
	if err != nil {
		t.Fatalf("Failed to get release with key %q: %s", key, err)
	}

	// check release has actually been updated by comparing modified fields
	if rel.Info.Status != got.Info.Status {
		t.Errorf("Expected status %s, got status %s", rel.Info.Status.String(), got.Info.Status.String())
	}
}

func TestMultiSecretsDelete(t *testing.T) {
	vers := 1
	name := "smug-pigeon"
	namespace := "default"
	key := testKey(name, vers)
	rel := multiReleaseStub(name, vers, namespace, rspb.StatusDeployed)

	multisecrets := newTestFixtureMultiSecrets(t, []*rspb.Release{rel}...)

	// perform the delete on a non-existing release
	_, err := multisecrets.Delete("nonexistent")
	if err != ErrReleaseNotFound {
		t.Fatalf("Expected ErrReleaseNotFound, got: {%v}", err)
	}

	// perform the delete
	rls, err := multisecrets.Delete(key)
	if err != nil {
		t.Fatalf("Failed to delete release with key %q: %s", key, err)
	}
	if !reflect.DeepEqual(rel, rls) {
		t.Errorf("Expected {%v}, got {%v}", rel, rls)
	}

	// fetch the deleted release
	_, err = multisecrets.Get(key)
	if !reflect.DeepEqual(ErrReleaseNotFound, err) {
		t.Errorf("Expected {%v}, got {%v}", ErrReleaseNotFound, err)
	}
}

func TestMultiSecretsSplitChunks(t *testing.T) {
	vers := 1
	name := "smug-pigeon"
	namespace := "default"
	key := testKey(name, vers)
	tests := []struct {
		rls    *rspb.Release
		size   int
		chunks int
	}{
		{
			multiReleaseStub(name, vers, namespace, rspb.StatusDeployed),
			10240,
			0,
		},
		{
			multiReleaseStub(name, vers, namespace, rspb.StatusDeployed),
			4096,
			2,
		},
		{
			multiReleaseStub(name, vers, namespace, rspb.StatusDeployed),
			2048,
			3,
		},
		{
			multiReleaseStub(name, vers, namespace, rspb.StatusDeployed),
			1024,
			5,
		},
		{
			multiReleaseStub(name, vers, namespace, rspb.StatusDeployed),
			512,
			10,
		},
	}

	for _, item := range tests {
		secrets, err := newMultiSecretsObject(key, item.rls, nil, item.size)
		if err != nil {
			t.Fatalf("Failed to create secrets: %s", err)
		}
		for k, se := range *secrets {
			if k == 0 && se.Name != "smug-pigeon.v1" {
				t.Errorf("Expected {%s}, got {%s}", "smug-pigeon.v1", se.Name)
			}
			if k > 0 && se.Name != fmt.Sprintf("smug-pigeon.v1.%d", k+1) {
				t.Errorf("Expected {%s}, got {%s}", "smug-pigeon.v1", se.Name)
			}
			if item.chunks == 0 {
				if _, ok := se.Data["chunks"]; ok {
					t.Error("Expected false, got true")
				}
				if _, ok := se.Data["chunk"]; ok {
					t.Error("Expected false, got true")
				}
			} else {
				chunks, _ := strconv.Atoi(string(se.Data["chunks"]))
				if !reflect.DeepEqual(item.chunks, chunks) {
					t.Errorf("Expected {%v}, got {%v}", item.chunks, chunks)
				}
				chunk, _ := strconv.Atoi(string(se.Data["chunk"]))
				if !reflect.DeepEqual(k+1, chunk) {
					t.Errorf("Expected {%v}, got {%v}", k+1, chunk)
				}
			}
		}
	}
}

func fakeMultiSecretsWithRels(t *testing.T) *MultiSecrets {
	t.Helper()
	return newTestFixtureMultiSecrets(t, []*rspb.Release{
		multiReleaseStub("multi-secrets-key-1", 1, "default", rspb.StatusUninstalled),
		multiReleaseStub("multi-secrets-key-2", 1, "default", rspb.StatusUninstalled),
		multiReleaseStub("multi-secrets-key-3", 1, "default", rspb.StatusDeployed),
		multiReleaseStub("multi-secrets-key-4", 1, "default", rspb.StatusDeployed),
		multiReleaseStub("multi-secrets-key-5", 1, "default", rspb.StatusSuperseded),
		multiReleaseStub("multi-secrets-key-6", 1, "default", rspb.StatusSuperseded),
	}...)
}
