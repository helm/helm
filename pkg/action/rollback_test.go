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
	"context"
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"helm.sh/helm/v4/pkg/kube"
	kubefake "helm.sh/helm/v4/pkg/kube/fake"
)

func TestNewRollback(t *testing.T) {
	config := actionConfigFixture(t)
	client := NewRollback(config)

	assert.NotNil(t, client)
	assert.Equal(t, config, client.cfg)
}

func TestRollbackRun_UnreachableKubeClient(t *testing.T) {
	config := actionConfigFixture(t)
	failingKubeClient := kubefake.FailingKubeClient{PrintingKubeClient: kubefake.PrintingKubeClient{Out: io.Discard}, DummyResources: nil}
	failingKubeClient.ConnectionError = errors.New("connection refused")
	config.KubeClient = &failingKubeClient

	client := NewRollback(config)
	assert.Error(t, client.Run(""))
}

func TestRollback_WaitOptionsPassedDownstream(t *testing.T) {
	is := assert.New(t)
	config := actionConfigFixture(t)

	// Create a deployed release and a second version to roll back to
	rel := releaseStub()
	rel.Name = "wait-options-rollback"
	rel.Info.Status = "deployed"
	rel.ApplyMethod = "csa"
	require.NoError(t, config.Releases.Create(rel))

	rel2 := releaseStub()
	rel2.Name = "wait-options-rollback"
	rel2.Version = 2
	rel2.Info.Status = "deployed"
	rel2.ApplyMethod = "csa"
	require.NoError(t, config.Releases.Create(rel2))

	client := NewRollback(config)
	client.Version = 1
	client.WaitStrategy = kube.StatusWatcherStrategy
	client.ServerSideApply = "auto"

	// Use WithWaitContext as a marker WaitOption that we can track
	ctx := context.Background()
	client.WaitOptions = []kube.WaitOption{kube.WithWaitContext(ctx)}

	// Access the underlying FailingKubeClient to check recorded options
	failer := config.KubeClient.(*kubefake.FailingKubeClient)

	err := client.Run(rel.Name)
	is.NoError(err)

	// Verify that WaitOptions were passed to GetWaiter
	is.NotEmpty(failer.RecordedWaitOptions, "WaitOptions should be passed to GetWaiter")
}
