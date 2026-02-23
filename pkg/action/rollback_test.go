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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"helm.sh/helm/v4/pkg/kube"
	kubefake "helm.sh/helm/v4/pkg/kube/fake"
	"helm.sh/helm/v4/pkg/release/common"
)

func rollbackAction(t *testing.T) *Rollback {
	t.Helper()
	config := actionConfigFixture(t)
	rollAction := NewRollback(config)
	return rollAction
}

func TestNewRollback(t *testing.T) {
	is := assert.New(t)
	config := actionConfigFixture(t)

	rollback := NewRollback(config)

	is.NotNil(rollback)
	is.Equal(config, rollback.cfg)
	is.Equal(DryRunNone, rollback.DryRunStrategy)
	is.Empty(rollback.Description)
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

func TestRollback_WithDescription(t *testing.T) {
	is := assert.New(t)
	req := require.New(t)

	rollAction := rollbackAction(t)

	// Create two releases - version 1 (superseded) and version 2 (deployed)
	rel1 := releaseStub()
	rel1.Name = "test-release"
	rel1.Version = 1
	rel1.Info.Status = common.StatusSuperseded
	rel1.ApplyMethod = "csa" // client-side apply
	req.NoError(rollAction.cfg.Releases.Create(rel1))

	rel2 := releaseStub()
	rel2.Name = "test-release"
	rel2.Version = 2
	rel2.Info.Status = common.StatusDeployed
	rel2.ApplyMethod = "csa" // client-side apply
	req.NoError(rollAction.cfg.Releases.Create(rel2))

	// Set custom description
	customDescription := "Rollback due to critical bug in version 2"
	rollAction.Description = customDescription
	rollAction.Version = 1
	rollAction.ServerSideApply = "false" // Disable server-side apply for testing

	err := rollAction.Run("test-release")
	req.NoError(err)

	// Get the new release (version 3)
	newReleasei, err := rollAction.cfg.Releases.Get("test-release", 3)
	req.NoError(err)
	newRelease, err := releaserToV1Release(newReleasei)
	req.NoError(err)

	// Verify the custom description was set
	is.Equal(customDescription, newRelease.Info.Description)
}

func TestRollback_DefaultDescription(t *testing.T) {
	is := assert.New(t)
	req := require.New(t)

	rollAction := rollbackAction(t)

	// Create two releases - version 1 (superseded) and version 2 (deployed)
	rel1 := releaseStub()
	rel1.Name = "test-release-default"
	rel1.Version = 1
	rel1.Info.Status = common.StatusSuperseded
	rel1.ApplyMethod = "csa" // client-side apply
	req.NoError(rollAction.cfg.Releases.Create(rel1))

	rel2 := releaseStub()
	rel2.Name = "test-release-default"
	rel2.Version = 2
	rel2.Info.Status = common.StatusDeployed
	rel2.ApplyMethod = "csa" // client-side apply
	req.NoError(rollAction.cfg.Releases.Create(rel2))

	// Don't set a description, rely on default
	rollAction.Version = 1
	rollAction.ServerSideApply = "false" // Disable server-side apply for testing

	err := rollAction.Run("test-release-default")
	req.NoError(err)

	// Get the new release (version 3)
	newReleasei, err := rollAction.cfg.Releases.Get("test-release-default", 3)
	req.NoError(err)
	newRelease, err := releaserToV1Release(newReleasei)
	req.NoError(err)

	// Verify the default description was set
	is.Equal("Rollback to 1", newRelease.Info.Description)
}

func TestRollback_EmptyDescription(t *testing.T) {
	is := assert.New(t)
	req := require.New(t)

	rollAction := rollbackAction(t)

	// Create two releases - version 1 (superseded) and version 2 (deployed)
	rel1 := releaseStub()
	rel1.Name = "test-release-empty"
	rel1.Version = 1
	rel1.Info.Status = common.StatusSuperseded
	rel1.ApplyMethod = "csa" // client-side apply
	req.NoError(rollAction.cfg.Releases.Create(rel1))

	rel2 := releaseStub()
	rel2.Name = "test-release-empty"
	rel2.Version = 2
	rel2.Info.Status = common.StatusDeployed
	rel2.ApplyMethod = "csa" // client-side apply
	req.NoError(rollAction.cfg.Releases.Create(rel2))

	// Set empty description (should use default)
	rollAction.Description = ""
	rollAction.Version = 1
	rollAction.ServerSideApply = "false" // Disable server-side apply for testing

	err := rollAction.Run("test-release-empty")
	req.NoError(err)

	// Get the new release (version 3)
	newReleasei, err := rollAction.cfg.Releases.Get("test-release-empty", 3)
	req.NoError(err)
	newRelease, err := releaserToV1Release(newReleasei)
	req.NoError(err)

	// Verify the default description was used for empty string
	is.Equal("Rollback to 1", newRelease.Info.Description)
}

func TestRollback_DescriptionTooLong(t *testing.T) {
	req := require.New(t)

	rollAction := rollbackAction(t)

	rel1 := releaseStub()
	rel1.Name = "test-release-desc-long"
	rel1.Version = 1
	rel1.Info.Status = common.StatusSuperseded
	rel1.ApplyMethod = "csa"
	req.NoError(rollAction.cfg.Releases.Create(rel1))

	rel2 := releaseStub()
	rel2.Name = "test-release-desc-long"
	rel2.Version = 2
	rel2.Info.Status = common.StatusDeployed
	rel2.ApplyMethod = "csa"
	req.NoError(rollAction.cfg.Releases.Create(rel2))

	rollAction.Description = strings.Repeat("a", MaxDescriptionLength+1)
	rollAction.Version = 1
	rollAction.ServerSideApply = "false"

	err := rollAction.Run("test-release-desc-long")
	req.Error(err)
	req.Contains(err.Error(), "description must be")
}

func TestRollback_DescriptionAtMaxLength(t *testing.T) {
	is := assert.New(t)
	req := require.New(t)

	rollAction := rollbackAction(t)

	rel1 := releaseStub()
	rel1.Name = "test-release-desc-max"
	rel1.Version = 1
	rel1.Info.Status = common.StatusSuperseded
	rel1.ApplyMethod = "csa"
	req.NoError(rollAction.cfg.Releases.Create(rel1))

	rel2 := releaseStub()
	rel2.Name = "test-release-desc-max"
	rel2.Version = 2
	rel2.Info.Status = common.StatusDeployed
	rel2.ApplyMethod = "csa"
	req.NoError(rollAction.cfg.Releases.Create(rel2))

	rollAction.Description = strings.Repeat("a", MaxDescriptionLength)
	rollAction.Version = 1
	rollAction.ServerSideApply = "false"

	err := rollAction.Run("test-release-desc-max")
	req.NoError(err)

	newReleasei, err := rollAction.cfg.Releases.Get("test-release-desc-max", 3)
	req.NoError(err)
	newRelease, err := releaserToV1Release(newReleasei)
	req.NoError(err)

	is.Equal(strings.Repeat("a", MaxDescriptionLength), newRelease.Info.Description)
}
