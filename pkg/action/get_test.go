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

	"helm.sh/helm/v4/pkg/release/common"
)

func TestNewGet(t *testing.T) {
	config := actionConfigFixture(t)
	client := NewGet(config)

	assert.NotNil(t, client)
	assert.Equal(t, config, client.cfg)
	assert.Equal(t, 0, client.Version)
}

func TestGetRun(t *testing.T) {
	config := actionConfigFixture(t)
	client := NewGet(config)
	simpleRelease := namedReleaseStub("test-release", common.StatusPendingUpgrade)
	require.NoError(t, config.Releases.Create(simpleRelease))

	releaser, err := client.Run(simpleRelease.Name)
	require.NoError(t, err)

	result, err := releaserToV1Release(releaser)
	require.NoError(t, err)
	assert.Equal(t, simpleRelease.Name, result.Name)
	assert.Equal(t, simpleRelease.Version, result.Version)
}

func TestGetRun_UnreachableKubeClient(t *testing.T) {
	config := actionConfigFixture(t)
	failingKubeClient := kubefake.FailingKubeClient{PrintingKubeClient: kubefake.PrintingKubeClient{Out: io.Discard}, DummyResources: nil}
	failingKubeClient.ConnectionError = errors.New("connection refused")
	config.KubeClient = &failingKubeClient

	client := NewGet(config)
	simpleRelease := namedReleaseStub("test-release", common.StatusPendingUpgrade)
	require.NoError(t, config.Releases.Create(simpleRelease))

	result, err := client.Run(simpleRelease.Name)
	assert.Nil(t, result)
	assert.Error(t, err)
}
