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
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"helm.sh/helm/v4/pkg/kube"
	kubefake "helm.sh/helm/v4/pkg/kube/fake"
	"helm.sh/helm/v4/pkg/release/common"
)

func uninstallAction(t *testing.T) *Uninstall {
	t.Helper()
	config := actionConfigFixture(t)
	unAction := NewUninstall(config)
	return unAction
}

func TestUninstallRelease_dryRun_ignoreNotFound(t *testing.T) {
	unAction := uninstallAction(t)
	unAction.DryRun = true
	unAction.IgnoreNotFound = true

	is := assert.New(t)
	res, err := unAction.Run("release-non-exist")
	is.Nil(res)
	is.NoError(err)
}

func TestUninstallRelease_ignoreNotFound(t *testing.T) {
	unAction := uninstallAction(t)
	unAction.DryRun = false
	unAction.IgnoreNotFound = true

	is := assert.New(t)
	res, err := unAction.Run("release-non-exist")
	is.Nil(res)
	is.NoError(err)
}
func TestUninstallRelease_deleteRelease(t *testing.T) {
	is := assert.New(t)

	unAction := uninstallAction(t)
	unAction.DisableHooks = true
	unAction.DryRun = false
	unAction.KeepHistory = true

	rel := releaseStub()
	rel.Name = "keep-secret"
	rel.Manifest = `{
		"apiVersion": "v1",
		"kind": "Secret",
		"metadata": {
		  "name": "secret",
		  "annotations": {
			"helm.sh/resource-policy": "keep"
		  }
		},
		"type": "Opaque",
		"data": {
		  "password": "password"
		}
	}`
	require.NoError(t, unAction.cfg.Releases.Create(rel))
	res, err := unAction.Run(rel.Name)
	is.NoError(err)
	expected := `These resources were kept due to the resource policy:
[Secret] secret
`
	is.Contains(res.Info, expected)
}

func TestUninstallRelease_Wait(t *testing.T) {
	is := assert.New(t)

	unAction := uninstallAction(t)
	unAction.DisableHooks = true
	unAction.DryRun = false
	unAction.WaitStrategy = kube.StatusWatcherStrategy

	rel := releaseStub()
	rel.Name = "come-fail-away"
	rel.Manifest = `{
		"apiVersion": "v1",
		"kind": "Secret",
		"metadata": {
		  "name": "secret"
		},
		"type": "Opaque",
		"data": {
		  "password": "password"
		}
	}`
	require.NoError(t, unAction.cfg.Releases.Create(rel))
	failer := unAction.cfg.KubeClient.(*kubefake.FailingKubeClient)
	failer.WaitForDeleteError = fmt.Errorf("U timed out")
	unAction.cfg.KubeClient = failer
	resi, err := unAction.Run(rel.Name)
	is.Error(err)
	is.Contains(err.Error(), "U timed out")
	res, err := releaserToV1Release(resi.Release)
	is.NoError(err)
	is.Equal(res.Info.Status, common.StatusUninstalled)
}

func TestUninstallRelease_Cascade(t *testing.T) {
	is := assert.New(t)

	unAction := uninstallAction(t)
	unAction.DisableHooks = true
	unAction.DryRun = false
	unAction.WaitStrategy = kube.HookOnlyStrategy
	unAction.DeletionPropagation = "foreground"

	rel := releaseStub()
	rel.Name = "come-fail-away"
	rel.Manifest = `{
		"apiVersion": "v1",
		"kind": "Secret",
		"metadata": {
		  "name": "secret"
		},
		"type": "Opaque",
		"data": {
		  "password": "password"
		}
	}`
	require.NoError(t, unAction.cfg.Releases.Create(rel))
	failer := unAction.cfg.KubeClient.(*kubefake.FailingKubeClient)
	failer.DeleteError = fmt.Errorf("Uninstall with cascade failed")
	failer.BuildDummy = true
	unAction.cfg.KubeClient = failer
	_, err := unAction.Run(rel.Name)
	require.Error(t, err)
	is.Contains(err.Error(), "failed to delete release: come-fail-away")
}

func TestUninstallRun_UnreachableKubeClient(t *testing.T) {
	t.Helper()
	config := actionConfigFixture(t)
	failingKubeClient := kubefake.FailingKubeClient{PrintingKubeClient: kubefake.PrintingKubeClient{Out: io.Discard}, DummyResources: nil}
	failingKubeClient.ConnectionError = errors.New("connection refused")
	config.KubeClient = &failingKubeClient

	client := NewUninstall(config)
	result, err := client.Run("")

	assert.Nil(t, result)
	assert.ErrorContains(t, err, "connection refused")
}

func TestUninstall_WaitOptionsPassedDownstream(t *testing.T) {
	is := assert.New(t)

	unAction := uninstallAction(t)
	unAction.DisableHooks = true
	unAction.DryRun = false
	unAction.WaitStrategy = kube.StatusWatcherStrategy

	// Use WithWaitContext as a marker WaitOption that we can track
	ctx := context.Background()
	unAction.WaitOptions = []kube.WaitOption{kube.WithWaitContext(ctx)}

	rel := releaseStub()
	rel.Name = "wait-options-uninstall"
	rel.Manifest = `{
		"apiVersion": "v1",
		"kind": "Secret",
		"metadata": {
		  "name": "secret"
		},
		"type": "Opaque",
		"data": {
		  "password": "password"
		}
	}`
	require.NoError(t, unAction.cfg.Releases.Create(rel))

	// Access the underlying FailingKubeClient to check recorded options
	failer := unAction.cfg.KubeClient.(*kubefake.FailingKubeClient)

	_, err := unAction.Run(rel.Name)
	is.NoError(err)

	// Verify that WaitOptions were passed to GetWaiter
	is.NotEmpty(failer.RecordedWaitOptions, "WaitOptions should be passed to GetWaiter")
}
