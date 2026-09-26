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
	rcommon "helm.sh/helm/v4/pkg/release/common"
	release "helm.sh/helm/v4/pkg/release/v1"
)

func TestNewStatus(t *testing.T) {
	config := actionConfigFixture(t)
	client := NewStatus(config)

	assert.NotNil(t, client)
	assert.Equal(t, config, client.cfg)
	assert.Equal(t, 0, client.Version)
}

func TestStatusRun(t *testing.T) {
	config := actionConfigFixture(t)
	failingKubeClient := kubefake.FailingKubeClient{PrintingKubeClient: kubefake.PrintingKubeClient{Out: io.Discard}, DummyResources: nil}
	failingKubeClient.BuildDummy = true
	config.KubeClient = &failingKubeClient
	client := NewStatus(config)
	client.ShowResourcesTable = true

	releaseName := "test-release"
	require.NoError(t, configureReleaseContent(config, releaseName))
	releaser, err := client.Run(releaseName)
	require.NoError(t, err)

	result, err := releaserToV1Release(releaser)
	require.NoError(t, err)
	assert.Equal(t, releaseName, result.Name)
	assert.Equal(t, 1, result.Version)
}

func TestStatusRun_KubeClientNotReachable(t *testing.T) {
	config := actionConfigFixture(t)
	failingKubeClient := kubefake.FailingKubeClient{PrintingKubeClient: kubefake.PrintingKubeClient{Out: io.Discard}, DummyResources: nil}
	failingKubeClient.ConnectionError = errors.New("connection refused")
	config.KubeClient = &failingKubeClient

	client := NewStatus(config)

	result, err := client.Run("")
	assert.Nil(t, result)
	assert.Error(t, err)
}

func TestStatusRun_KubeClientBuildTableError(t *testing.T) {
	config := actionConfigFixture(t)
	failingKubeClient := kubefake.FailingKubeClient{PrintingKubeClient: kubefake.PrintingKubeClient{Out: io.Discard}, DummyResources: nil}
	failingKubeClient.BuildTableError = errors.New("build table error")
	config.KubeClient = &failingKubeClient

	releaseName := "test-release"
	require.NoError(t, configureReleaseContent(config, releaseName))

	client := NewStatus(config)
	client.ShowResourcesTable = true

	result, err := client.Run(releaseName)

	assert.Nil(t, result)
	assert.ErrorContains(t, err, "build table error")
}

func TestStatusRun_KubeClientBuildError(t *testing.T) {
	config := actionConfigFixture(t)
	failingKubeClient := kubefake.FailingKubeClient{PrintingKubeClient: kubefake.PrintingKubeClient{Out: io.Discard}, DummyResources: nil}
	failingKubeClient.BuildError = errors.New("build error")
	config.KubeClient = &failingKubeClient

	releaseName := "test-release"
	require.NoError(t, configureReleaseContent(config, releaseName))

	client := NewStatus(config)
	client.ShowResourcesTable = false

	result, err := client.Run(releaseName)
	assert.Nil(t, result)
	assert.ErrorContains(t, err, "build error")
}

func TestStatusRun_KubeClientGetError(t *testing.T) {
	config := actionConfigFixture(t)
	failingKubeClient := kubefake.FailingKubeClient{PrintingKubeClient: kubefake.PrintingKubeClient{Out: io.Discard}, DummyResources: nil}
	failingKubeClient.BuildError = errors.New("get error")
	config.KubeClient = &failingKubeClient

	releaseName := "test-release"
	require.NoError(t, configureReleaseContent(config, releaseName))
	client := NewStatus(config)

	result, err := client.Run(releaseName)
	assert.Nil(t, result)
	assert.ErrorContains(t, err, "get error")
}

func configureReleaseContent(cfg *Configuration, releaseName string) error {
	rel := &release.Release{
		Name: releaseName,
		Info: &release.Info{
			Status: rcommon.StatusDeployed,
		},
		Manifest:  testManifest,
		Version:   1,
		Namespace: "default",
	}

	return cfg.Releases.Create(rel)
}

const testManifest = `
apiVersion: v1
kind: Pod
metadata:
  namespace: default
  name: test-application
`
