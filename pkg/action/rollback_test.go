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
	rcommon "helm.sh/helm/v4/pkg/release/common"
	release "helm.sh/helm/v4/pkg/release/v1"
	releaseutil "helm.sh/helm/v4/pkg/release/v1/util"
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

	err := client.Run(rel.Name)
	req.NoError(err)

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
	rev1.Info.Status = rcommon.StatusSuperseded
	rev1.Manifest = plainManifest
	rev1.SequencingInfo = nil // plain install

	rev2 := releaseStub()
	rev2.Name = "cap21"
	rev2.Version = 2
	rev2.Info.Status = rcommon.StatusDeployed
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
