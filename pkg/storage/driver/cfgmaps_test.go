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

func TestConfigMapName(t *testing.T) {
	c := newTestFixtureCfgMaps(t)
	assert.Equal(t, ConfigMapsDriverName, c.Name(), "Expected name to be %q, got %q", ConfigMapsDriverName, c.Name())
}

func TestConfigMapGet(t *testing.T) {
	vers := 1
	name := "smug-pigeon"
	namespace := "default"
	key := testKey(name, vers)
	rel := releaseStub(name, vers, namespace, common.StatusDeployed)

	cfgmaps := newTestFixtureCfgMaps(t, []*rspb.Release{rel}...)

	// get release with key
	got, err := cfgmaps.Get(key)
	require.NoError(t, err, "Failed to get release")
	// compare fetched release with original
	assert.Equalf(t, rel, got, "Expected {%v}, got {%v}", rel, got)
}

func TestUncompressedConfigMapGet(t *testing.T) {
	vers := 1
	name := "smug-pigeon"
	namespace := "default"
	key := testKey(name, vers)
	rel := releaseStub(name, vers, namespace, common.StatusDeployed)

	// Create a test fixture which contains an uncompressed release
	cfgmap, err := newConfigMapsObject(key, rel, nil)
	require.NoError(t, err, "Failed to create configmap")
	b, err := json.Marshal(rel)
	require.NoError(t, err, "Failed to marshal release")
	cfgmap.Data["release"] = base64.StdEncoding.EncodeToString(b)
	var mock MockConfigMapsInterface
	mock.objects = map[string]*v1.ConfigMap{key: cfgmap}
	cfgmaps := NewConfigMaps(&mock)

	// get release with key
	got, err := cfgmaps.Get(key)
	require.NoError(t, err, "Failed to get release")
	// compare fetched release with original
	assert.Equalf(t, rel, got, "Expected {%v}, got {%v}", rel, got)
}

func convertReleaserToV1(t *testing.T, rel release.Releaser) *rspb.Release {
	t.Helper()
	switch r := rel.(type) {
	case rspb.Release:
		return &r
	case *rspb.Release:
		return r
	case nil:
		return nil
	}

	t.Fatalf("Unsupported release type: %T", rel)
	return nil
}

func TestConfigMapList(t *testing.T) {
	cfgmaps := newTestFixtureCfgMaps(t, []*rspb.Release{
		releaseStub("key-1", 1, "default", common.StatusUninstalled),
		releaseStub("key-2", 1, "default", common.StatusUninstalled),
		releaseStub("key-3", 1, "default", common.StatusDeployed),
		releaseStub("key-4", 1, "default", common.StatusDeployed),
		releaseStub("key-5", 1, "default", common.StatusSuperseded),
		releaseStub("key-6", 1, "default", common.StatusSuperseded),
	}...)

	// list all deleted releases
	del, err := cfgmaps.List(func(rel release.Releaser) bool {
		rls := convertReleaserToV1(t, rel)
		return rls.Info.Status == common.StatusUninstalled
	})
	// check
	require.NoError(t, err, "Failed to list deleted")
	assert.Len(t, del, 2, "Expected 2 deleted")

	// list all deployed releases
	dpl, err := cfgmaps.List(func(rel release.Releaser) bool {
		rls := convertReleaserToV1(t, rel)
		return rls.Info.Status == common.StatusDeployed
	})
	// check
	require.NoError(t, err, "Failed to list deployed")
	assert.Len(t, dpl, 2, "Expected 2 deployed")

	// list all superseded releases
	ssd, err := cfgmaps.List(func(rel release.Releaser) bool {
		rls := convertReleaserToV1(t, rel)
		return rls.Info.Status == common.StatusSuperseded
	})
	// check
	require.NoError(t, err, "Failed to list superseded")
	assert.Len(t, ssd, 2, "Expected 2 superseded")
	// Check if release having both system and custom labels, this is needed to ensure that selector filtering would work.
	rls := convertReleaserToV1(t, ssd[0])
	require.Contains(t, rls.Labels, "name", "Expected 'name' label in results, actual %v", rls.Labels)
	require.Contains(t, rls.Labels, "key1", "Expected 'key1' label in results, actual %v", rls.Labels)
}

func TestConfigMapQuery(t *testing.T) {
	cfgmaps := newTestFixtureCfgMaps(t, []*rspb.Release{
		releaseStub("key-1", 1, "default", common.StatusUninstalled),
		releaseStub("key-2", 1, "default", common.StatusUninstalled),
		releaseStub("key-3", 1, "default", common.StatusDeployed),
		releaseStub("key-4", 1, "default", common.StatusDeployed),
		releaseStub("key-5", 1, "default", common.StatusSuperseded),
		releaseStub("key-6", 1, "default", common.StatusSuperseded),
	}...)

	rls, err := cfgmaps.Query(map[string]string{"status": "deployed"})
	require.NoError(t, err, "Failed to query")
	assert.Len(t, rls, 2, "Expected 2 results")

	_, err = cfgmaps.Query(map[string]string{"name": "notExist"})
	assert.ErrorIs(t, err, ErrReleaseNotFound)
}

func TestConfigMapCreate(t *testing.T) {
	cfgmaps := newTestFixtureCfgMaps(t)

	vers := 1
	name := "smug-pigeon"
	namespace := "default"
	key := testKey(name, vers)
	rel := releaseStub(name, vers, namespace, common.StatusDeployed)

	// store the release in a configmap
	require.NoErrorf(t, cfgmaps.Create(key, rel), "Failed to create release with key %q", key)

	// get the release back
	got, err := cfgmaps.Get(key)
	require.NoError(t, err, "Failed to get release with key %q", key)

	// compare created release with original
	assert.Equalf(t, rel, got, "Expected {%v}, got {%v}", rel, got)
}

func TestConfigMapUpdate(t *testing.T) {
	vers := 1
	name := "smug-pigeon"
	namespace := "default"
	key := testKey(name, vers)
	rel := releaseStub(name, vers, namespace, common.StatusDeployed)

	cfgmaps := newTestFixtureCfgMaps(t, []*rspb.Release{rel}...)

	// modify release status code
	rel.Info.Status = common.StatusSuperseded

	// perform the update
	require.NoErrorf(t, cfgmaps.Update(key, rel), "Failed to update release")

	// fetch the updated release
	goti, err := cfgmaps.Get(key)
	require.NoError(t, err, "Failed to get release with key %q", key)
	got := convertReleaserToV1(t, goti)

	// check release has actually been updated by comparing modified fields
	assert.Equal(t, got.Info.Status, rel.Info.Status, "Expected status %s, got status %s", rel.Info.Status.String(), got.Info.Status.String())
}

func TestConfigMapDelete(t *testing.T) {
	vers := 1
	name := "smug-pigeon"
	namespace := "default"
	key := testKey(name, vers)
	rel := releaseStub(name, vers, namespace, common.StatusDeployed)

	cfgmaps := newTestFixtureCfgMaps(t, []*rspb.Release{rel}...)

	// perform the delete on a non-existent release
	_, err := cfgmaps.Delete("nonexistent")
	require.ErrorIs(t, err, ErrReleaseNotFound)

	// perform the delete
	rls, err := cfgmaps.Delete(key)
	require.NoError(t, err, "Failed to delete release with key %q", key)
	assert.Equalf(t, rel, rls, "Expected {%v}, got {%v}", rel, rls)
	_, err = cfgmaps.Get(key)
	assert.ErrorIs(t, err, ErrReleaseNotFound)
}
