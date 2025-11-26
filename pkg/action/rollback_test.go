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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"helm.sh/helm/v4/pkg/release/common"
)

func rollbackAction(t *testing.T) *Rollback {
	t.Helper()
	config := actionConfigFixture(t)
	rollAction := NewRollback(config)
	return rollAction
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

func TestNewRollback(t *testing.T) {
	is := assert.New(t)
	config := actionConfigFixture(t)

	rollback := NewRollback(config)

	is.NotNil(rollback)
	is.Equal(config, rollback.cfg)
	is.Equal(DryRunNone, rollback.DryRunStrategy)
	is.Empty(rollback.Description)
}
