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
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	kubefake "helm.sh/helm/v4/pkg/kube/fake"
	release "helm.sh/helm/v4/pkg/release/v1"
)

func TestNewHistory(t *testing.T) {
	config := actionConfigFixture(t)
	client := NewHistory(config)

	assert.NotNil(t, client)
	assert.Equal(t, config, client.cfg)
}

func TestHistoryRun(t *testing.T) {
	releaseName := "test-release"
	simpleRelease := namedReleaseStub(releaseName, release.StatusPendingUpgrade)
	updatedRelease := namedReleaseStub(releaseName, release.StatusDeployed)
	updatedRelease.Chart.Metadata.Version = "0.1.1"
	updatedRelease.Version = 2

	config := actionConfigFixture(t)
	client := NewHistory(config)
	client.Max = 3
	client.cfg.Releases.MaxHistory = 3
	for _, rel := range []*release.Release{simpleRelease, updatedRelease} {
		if err := client.cfg.Releases.Create(rel); err != nil {
			t.Fatal(err, "Could not add releases to Config")
		}
	}

	releases, err := config.Releases.ListReleases()
	require.NoError(t, err)
	assert.Len(t, releases, 2, "expected 2 Releases in Config")

	result, err := client.Run(releaseName)
	require.NoError(t, err)

	assert.Len(t, result, 2, "expected 2 Releases in History result")
	assert.Equal(t, simpleRelease.Name, result[0].Name)
	assert.Equal(t, simpleRelease.Version, result[0].Version)
	assert.Equal(t, updatedRelease.Name, result[1].Name)
	assert.Equal(t, updatedRelease.Version, result[1].Version)
}

func TestHistoryRun_UnreachableKubeClient(t *testing.T) {
	config := actionConfigFixture(t)
	failingKubeClient := kubefake.FailingKubeClient{PrintingKubeClient: kubefake.PrintingKubeClient{Out: io.Discard}, DummyResources: nil}
	failingKubeClient.ConnectionError = errors.New("connection refused")
	config.KubeClient = &failingKubeClient

	client := NewHistory(config)
	result, err := client.Run("release-name")
	assert.Nil(t, result)
	assert.Error(t, err)
}

func TestHistoryRun_InvalidReleaseNames(t *testing.T) {
	config := actionConfigFixture(t)
	client := NewHistory(config)
	invalidReleaseNames := []string{
		"",
		"too-long-release-name-max-53-characters-abcdefghijklmnopqrstuvwxyz",
		"MyRelease",
		"release_name",
		"release@123",
		"-badstart",
		"badend-",
		".dotstart",
	}

	for _, name := range invalidReleaseNames {
		result, err := client.Run(name)
		assert.Nil(t, result)
		assert.ErrorContains(t, err, "release name is invalid")
	}
}
