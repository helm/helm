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
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"helm.sh/helm/v4/pkg/kube"
	kubefake "helm.sh/helm/v4/pkg/kube/fake"
	release "helm.sh/helm/v4/pkg/release/v1"
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

// TestRollback_SequencingInfoPropagated verifies that a previous release's SequencingInfo
// is carried forward into the new rollback release record.
func TestRollback_SequencingInfoPropagated(t *testing.T) {
	config := actionConfigFixture(t)

	// rel1 was originally deployed with --wait=ordered
	rel1 := releaseStub()
	rel1.Name = "seq-rollback-test"
	rel1.Version = 1
	rel1.Info.Status = "deployed"
	rel1.ApplyMethod = "csa"
	rel1.SequencingInfo = &release.SequencingInfo{Enabled: true, Strategy: "ordered"}
	require.NoError(t, config.Releases.Create(rel1))

	// rel2 is the current (last) deployed version
	rel2 := releaseStub()
	rel2.Name = "seq-rollback-test"
	rel2.Version = 2
	rel2.Info.Status = "deployed"
	rel2.ApplyMethod = "csa"
	require.NoError(t, config.Releases.Create(rel2))

	client := NewRollback(config)
	client.Version = 1
	client.WaitStrategy = kube.OrderedWaitStrategy
	client.ServerSideApply = "auto"
	client.Timeout = 5 * time.Minute

	err := client.Run(rel1.Name)
	require.NoError(t, err)

	// The new rollback release (version 3) should have SequencingInfo from rel1
	newReli, err := config.Releases.Last(rel1.Name)
	require.NoError(t, err)
	newRel, err := releaserToV1Release(newReli)
	require.NoError(t, err)
	assert.Equal(t, 3, newRel.Version)
	require.NotNil(t, newRel.SequencingInfo, "rollback release should carry SequencingInfo from previous version")
	assert.True(t, newRel.SequencingInfo.Enabled)
}
