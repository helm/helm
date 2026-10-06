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
	release "helm.sh/helm/v4/pkg/release/v1"
	releaseutil "helm.sh/helm/v4/pkg/release/v1/util"
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
	req := require.New(t)
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

	req.NoError(client.Run(rel.Name))

	// Verify that WaitOptions were passed to GetWaiter
	is.NotEmpty(failer.RecordedWaitOptions, "WaitOptions should be passed to GetWaiter")
}

func TestRollbackSetsRollbackRevision(t *testing.T) {
	config := actionConfigFixture(t)

	rel1 := releaseStub()
	rel1.Name = "rollback-rev-test"
	rel1.Version = 1
	rel1.Info.Status = "superseded"
	rel1.ApplyMethod = "csa"
	require.NoError(t, config.Releases.Create(rel1))

	rel2 := releaseStub()
	rel2.Name = "rollback-rev-test"
	rel2.Version = 2
	rel2.Info.Status = "deployed"
	rel2.ApplyMethod = "csa"
	require.NoError(t, config.Releases.Create(rel2))

	client := NewRollback(config)
	client.Version = 1
	client.ServerSideApply = "auto"

	require.NoError(t, client.Run("rollback-rev-test"))

	reli, err := config.Releases.Get("rollback-rev-test", 3)
	require.NoError(t, err)
	rel, err := releaserToV1Release(reli)
	require.NoError(t, err)

	assert.Equal(t, 1, rel.Info.RollbackRevision)
	assert.Equal(t, "Rollback to 1", rel.Info.Description)
}

func TestRollbackRevisionZeroForNonRollback(t *testing.T) {
	config := actionConfigFixture(t)

	rel := releaseStub()
	rel.Name = "non-rollback"
	rel.Info.Status = "deployed"
	require.NoError(t, config.Releases.Create(rel))

	reli, err := config.Releases.Get("non-rollback", 1)
	require.NoError(t, err)
	r, err := releaserToV1Release(reli)
	require.NoError(t, err)

	assert.Equal(t, 0, r.Info.RollbackRevision)
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

func TestRollback_DescriptionMultiByteCharacters(t *testing.T) {
	is := assert.New(t)
	req := require.New(t)

	rollAction := rollbackAction(t)

	rel1 := releaseStub()
	rel1.Name = "test-release-desc-utf8"
	rel1.Version = 1
	rel1.Info.Status = common.StatusSuperseded
	rel1.ApplyMethod = "csa"
	req.NoError(rollAction.cfg.Releases.Create(rel1))

	rel2 := releaseStub()
	rel2.Name = "test-release-desc-utf8"
	rel2.Version = 2
	rel2.Info.Status = common.StatusDeployed
	rel2.ApplyMethod = "csa"
	req.NoError(rollAction.cfg.Releases.Create(rel2))

	// "é" is 2 bytes in UTF-8 but 1 rune
	rollAction.Description = strings.Repeat("é", MaxDescriptionLength)
	rollAction.Version = 1
	rollAction.ServerSideApply = "false"

	err := rollAction.Run("test-release-desc-utf8")
	req.NoError(err)

	newReleasei, err := rollAction.cfg.Releases.Get("test-release-desc-utf8", 3)
	req.NoError(err)
	newRelease, err := releaserToV1Release(newReleasei)
	req.NoError(err)

	is.Equal(strings.Repeat("é", MaxDescriptionLength), newRelease.Info.Description)
}

// TestRollback_StripsSequencingAnnotationsOnPlainPath locks the fix for cap-21
// (hip-0025-r5y). Scenario: a release was installed plain at rev-1, upgraded
// with sequencing at rev-2 (manifest stored in the secret retains the raw
// helm.sh/depends-on/resource-groups annotation), then rolled back to rev-1.
// Because targetRelease.SequencingInfo is unset, performRollback falls through
// to the plain (non-sequenced) UPDATE path. Without stripping, SSA on the
// rollback rejects the multi-slash annotation key as invalid. This test
// verifies that BOTH current (rev-2 manifest) and target (rev-1 manifest) are
// passed through stripSequencingAnnotations before KubeClient.Update is called.
func TestRollback_StripsSequencingAnnotationsOnPlainPath(t *testing.T) {
	const annotation = releaseutil.AnnotationDependsOnResourceGroups

	// rev-2's manifest carries the helm-internal annotation because the
	// rendered template kept it; the live K8s objects had it stripped by the
	// sequenced upgrade path, but the secret-stored Manifest still has it.
	sequencedManifest := `apiVersion: v1
kind: ConfigMap
metadata:
  name: cap21-cm
  namespace: spaced
  annotations:
    "` + annotation + `": "[other-group]"
data:
  k: v
`

	// rev-1's manifest is plain.
	plainManifest := `apiVersion: v1
kind: ConfigMap
metadata:
  name: cap21-cm
  namespace: spaced
data:
  k: v
`

	rev1 := releaseStub()
	rev1.Name = "cap21"
	rev1.Version = 1
	rev1.Info.Status = common.StatusSuperseded
	rev1.Manifest = plainManifest
	rev1.SequencingInfo = nil // plain install

	rev2 := releaseStub()
	rev2.Name = "cap21"
	rev2.Version = 2
	rev2.Info.Status = common.StatusDeployed
	rev2.Manifest = sequencedManifest
	rev2.SequencingInfo = &release.SequencingInfo{Enabled: true, Strategy: "ordered"}

	cfg := actionConfigFixture(t)
	require.NoError(t, cfg.Releases.Create(rev1))
	require.NoError(t, cfg.Releases.Create(rev2))

	recorder := newRecordingKubeClient()
	cfg.KubeClient = recorder

	client := NewRollback(cfg)
	client.Version = 1
	client.DisableHooks = true
	require.NoError(t, client.Run("cap21"))

	require.Len(t, recorder.updateCalls, 1, "exactly one KubeClient.Update call expected on plain rollback path")
	call := recorder.updateCalls[0]

	assertNoSequencingAnnotation(t, "current", call.currentResources, annotation)
	assertNoSequencingAnnotation(t, "target", call.targetResources, annotation)
}

func assertNoSequencingAnnotation(t *testing.T, label string, resources kube.ResourceList, key string) {
	t.Helper()
	for _, info := range resources {
		acc := info.Object.(interface {
			GetAnnotations() map[string]string
		})
		anns := acc.GetAnnotations()
		if _, present := anns[key]; present {
			t.Fatalf("%s resource %q still carries stripped annotation %q after rollback", label, info.Name, key)
		}
	}
}
