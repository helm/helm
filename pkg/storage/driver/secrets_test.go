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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"

	"helm.sh/helm/v4/pkg/release"
	"helm.sh/helm/v4/pkg/release/common"
	rspb "helm.sh/helm/v4/pkg/release/v1"
)

func TestSecretName(t *testing.T) {
	c := newTestFixtureSecrets(t)
	assert.Equal(t, SecretsDriverName, c.Name(), "Expected name to be %q, got %q", SecretsDriverName, c.Name())
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
	require.NoError(t, err, "Failed to get release")
	// compare fetched release with original
	assert.Equalf(t, rel, got, "Expected {%v}, got {%v}", rel, got)
}

func TestUNcompressedSecretGet(t *testing.T) {
	vers := 1
	name := "smug-pigeon"
	namespace := "default"
	key := testKey(name, vers)
	rel := releaseStub(name, vers, namespace, common.StatusDeployed)

	// Create a test fixture which contains an uncompressed release
	secret, err := newSecretsObject(key, rel, nil)
	require.NoError(t, err, "Failed to create secret")
	b, err := json.Marshal(rel)
	require.NoError(t, err, "Failed to marshal release")
	secret.Data["release"] = []byte(base64.StdEncoding.EncodeToString(b))
	var mock MockSecretsInterface
	mock.objects = map[string]*v1.Secret{key: secret}
	secrets := NewSecrets(&mock)

	// get release with key
	got, err := secrets.Get(key)
	require.NoError(t, err, "Failed to get release")
	// compare fetched release with original
	assert.Equalf(t, rel, got, "Expected {%v}, got {%v}", rel, got)
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
	require.NoError(t, err, "Failed to list deleted")
	assert.Len(t, del, 2, "Expected 2 deleted")

	// list all deployed releases
	dpl, err := secrets.List(func(rel release.Releaser) bool {
		rls := convertReleaserToV1(t, rel)
		return rls.Info.Status == common.StatusDeployed
	})
	// check
	require.NoError(t, err, "Failed to list deployed")
	assert.Len(t, dpl, 2, "Expected 2 deployed")

	// list all superseded releases
	ssd, err := secrets.List(func(rel release.Releaser) bool {
		rls := convertReleaserToV1(t, rel)
		return rls.Info.Status == common.StatusSuperseded
	})
	// check
	require.NoError(t, err, "Failed to list superseded")
	require.Len(t, ssd, 2, "Expected 2 superseded")
	// Check if release having both system and custom labels, this is needed to ensure that selector filtering would work.
	rls := convertReleaserToV1(t, ssd[0])
	require.Contains(t, rls.Labels, "name", "Expected 'name' label in results, actual %v", rls.Labels)
	require.Contains(t, rls.Labels, "key1", "Expected 'key1' label in results, actual %v", rls.Labels)
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
	require.NoError(t, err, "Failed to query")
	require.Len(t, rls, 2, "Expected 2 results, actual %d", len(rls))

	_, err = secrets.Query(map[string]string{"name": "notExist"})
	assert.ErrorIs(t, err, ErrReleaseNotFound)
}

func TestSecretListAndQueryPaginate(t *testing.T) {
	setListPageSize(t, 2)

	secrets := newTestFixtureSecrets(t, []*rspb.Release{
		releaseStub("key-1", 1, "default", common.StatusUninstalled),
		releaseStub("key-2", 1, "default", common.StatusUninstalled),
		releaseStub("key-3", 1, "default", common.StatusDeployed),
		releaseStub("key-4", 1, "default", common.StatusDeployed),
		releaseStub("key-5", 1, "default", common.StatusSuperseded),
		releaseStub("key-6", 1, "default", common.StatusSuperseded),
		releaseStub("key-7", 1, "default", common.StatusDeployed),
	}...)
	mock := secrets.impl.(*MockSecretsInterface)

	tests := []struct {
		name      string
		run       func() ([]release.Releaser, error)
		wantLen   int
		wantPages int
	}{
		{
			name: "list all",
			run: func() ([]release.Releaser, error) {
				return secrets.List(func(release.Releaser) bool { return true })
			},
			wantLen:   7,
			wantPages: 4,
		},
		{
			name: "list with filter",
			run: func() ([]release.Releaser, error) {
				return secrets.List(func(rel release.Releaser) bool {
					return convertReleaserToV1(t, rel).Info.Status == common.StatusDeployed
				})
			},
			wantLen:   3,
			wantPages: 4,
		},
		{
			name: "query",
			run: func() ([]release.Releaser, error) {
				return secrets.Query(map[string]string{"status": "deployed"})
			},
			wantLen:   3,
			wantPages: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock.listCalls = nil

			rls, err := tt.run()
			require.NoError(t, err)
			assert.Len(t, rls, tt.wantLen)

			require.Len(t, mock.listCalls, tt.wantPages)
			for i, opts := range mock.listCalls {
				assert.Equal(t, int64(2), opts.Limit, "page %d should be limited", i)
				if i == 0 {
					assert.Empty(t, opts.Continue, "first page should not send a continue token")
				} else {
					assert.NotEmpty(t, opts.Continue, "page %d should send a continue token", i)
				}
			}
		})
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
	require.NoErrorf(t, secrets.Create(key, rel), "Failed to create release with key %q", key)

	// get the release back
	got, err := secrets.Get(key)
	require.NoError(t, err, "Failed to get release with key %q", key)

	// compare created release with original
	assert.Equalf(t, rel, got, "Expected {%v}, got {%v}", rel, got)
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
	require.NoErrorf(t, secrets.Update(key, rel), "Failed to update release")

	// fetch the updated release
	goti, err := secrets.Get(key)
	require.NoError(t, err, "Failed to get release with key %q", key)
	got := convertReleaserToV1(t, goti)

	// check release has actually been updated by comparing modified fields
	assert.Equal(t, got.Info.Status, rel.Info.Status, "Expected status %s, got status %s", rel.Info.Status.String(), got.Info.Status.String())
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
	require.ErrorIs(t, err, ErrReleaseNotFound, "Expected ErrReleaseNotFound")

	// perform the delete
	rls, err := secrets.Delete(key)
	require.NoError(t, err, "Failed to delete release with key %q", key)
	assert.Equalf(t, rel, rls, "Expected {%v}, got {%v}", rel, rls)
	_, err = secrets.Get(key)
	assert.ErrorIs(t, err, ErrReleaseNotFound)
}
