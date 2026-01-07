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
	"bytes"
	"errors"
	"fmt"
	"io"
	"log/slog"
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
	unAction.cfg.Releases.Create(rel)

	// Create dummy resources with Mapping but no Client - this skips ownership verification
	// (nil Client is treated as owned) and goes directly to delete
	dummyResources := kube.ResourceList{
		newDeploymentResource("secret", ""),
	}

	failer := unAction.cfg.KubeClient.(*kubefake.FailingKubeClient)
	failer.DeleteError = fmt.Errorf("Uninstall with cascade failed")
	failer.DummyResources = dummyResources
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

func TestUninstallRelease_OwnershipVerification(t *testing.T) {
	is := assert.New(t)

	// Create a buffer to capture log output
	logBuffer := &bytes.Buffer{}
	handler := slog.NewTextHandler(logBuffer, &slog.HandlerOptions{Level: slog.LevelDebug})

	config := actionConfigFixture(t)
	config.SetLogger(handler)

	unAction := NewUninstall(config)
	unAction.DisableHooks = true
	unAction.DryRun = false
	unAction.KeepHistory = true

	rel := releaseStub()
	rel.Name = "ownership-test"
	rel.Namespace = "default"
	rel.Manifest = `apiVersion: v1
kind: ConfigMap
metadata:
  name: test-configmap
  labels:
    app.kubernetes.io/managed-by: Helm
  annotations:
    meta.helm.sh/release-name: ownership-test
    meta.helm.sh/release-namespace: default
data:
  key: value`
	config.Releases.Create(rel)

	// Create dummy resources with proper ownership metadata
	labels := map[string]string{
		"app.kubernetes.io/managed-by": "Helm",
	}
	annotations := map[string]string{
		"meta.helm.sh/release-name":      "ownership-test",
		"meta.helm.sh/release-namespace": "default",
	}
	dummyResources := kube.ResourceList{
		newDeploymentWithOwner("owned-deploy", "default", labels, annotations),
	}
	failer := config.KubeClient.(*kubefake.FailingKubeClient)
	failer.DummyResources = dummyResources

	resi, err := unAction.Run(rel.Name)
	is.NoError(err)
	is.NotNil(resi)
	res, err := releaserToV1Release(resi.Release)
	is.NoError(err)
	is.Equal(common.StatusUninstalled, res.Info.Status)

	// Verify log contains debug message about deleting owned resource
	logOutput := logBuffer.String()
	is.Contains(logOutput, "deleting resource owned by this release")
	is.Contains(logOutput, "owned-deploy")
	is.Contains(logOutput, "Deployment")
}

func TestUninstallRelease_OwnershipVerification_WithKeepPolicy(t *testing.T) {
	is := assert.New(t)

	// Create a buffer to capture log output
	logBuffer := &bytes.Buffer{}
	handler := slog.NewTextHandler(logBuffer, &slog.HandlerOptions{Level: slog.LevelWarn})

	config := actionConfigFixture(t)
	config.SetLogger(handler)

	unAction := NewUninstall(config)
	unAction.DisableHooks = true
	unAction.DryRun = false
	unAction.KeepHistory = true

	rel := releaseStub()
	rel.Name = "keep-and-ownership"
	rel.Namespace = "default"
	rel.Manifest = `apiVersion: v1
kind: Secret
metadata:
  name: kept-secret
  annotations:
    helm.sh/resource-policy: keep
    meta.helm.sh/release-name: keep-and-ownership
    meta.helm.sh/release-namespace: default
  labels:
    app.kubernetes.io/managed-by: Helm
type: Opaque
data:
  password: cGFzc3dvcmQ=
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: deleted-configmap
  labels:
    app.kubernetes.io/managed-by: Helm
  annotations:
    meta.helm.sh/release-name: keep-and-ownership
    meta.helm.sh/release-namespace: default
data:
  key: value`
	config.Releases.Create(rel)

	// Create dummy resources - one unowned to test logging
	dummyResources := kube.ResourceList{
		newDeploymentWithOwner("unowned-deploy", "default", nil, nil),
	}
	failer := config.KubeClient.(*kubefake.FailingKubeClient)
	failer.DummyResources = dummyResources

	res, err := unAction.Run(rel.Name)
	is.NoError(err)
	is.NotNil(res)
	// Should contain info about kept resources
	is.Contains(res.Info, "kept due to the resource policy")

	// Verify log contains warning about skipped unowned resource
	logOutput := logBuffer.String()
	is.Contains(logOutput, "skipping delete of resource not owned by this release")
	is.Contains(logOutput, "unowned-deploy")
}

func TestUninstallRelease_DryRun_OwnershipVerification(t *testing.T) {
	is := assert.New(t)

	// Create a buffer to capture log output
	logBuffer := &bytes.Buffer{}
	handler := slog.NewTextHandler(logBuffer, &slog.HandlerOptions{Level: slog.LevelWarn})

	config := actionConfigFixture(t)
	config.SetLogger(handler)

	unAction := NewUninstall(config)
	unAction.DisableHooks = true
	unAction.DryRun = true

	rel := releaseStub()
	rel.Name = "dryrun-ownership"
	rel.Namespace = "default"
	rel.Manifest = `apiVersion: v1
kind: ConfigMap
metadata:
  name: test-configmap
  labels:
    app.kubernetes.io/managed-by: Helm
  annotations:
    meta.helm.sh/release-name: dryrun-ownership
    meta.helm.sh/release-namespace: default
data:
  key: value`
	config.Releases.Create(rel)

	// Create dummy resources - one unowned to test dry-run logging
	dummyResources := kube.ResourceList{
		newDeploymentWithOwner("dryrun-unowned-deploy", "default", nil, nil),
	}
	failer := config.KubeClient.(*kubefake.FailingKubeClient)
	failer.DummyResources = dummyResources

	resi, err := unAction.Run(rel.Name)
	is.NoError(err)
	is.NotNil(resi)
	is.NotNil(resi.Release)
	res, err := releaserToV1Release(resi.Release)
	is.NoError(err)
	is.Equal("dryrun-ownership", res.Name)

	// Verify log contains dry-run warning about resources that would be skipped
	logOutput := logBuffer.String()
	is.Contains(logOutput, "dry-run: would skip resource")
	is.Contains(logOutput, "dryrun-unowned-deploy")
	is.Contains(logOutput, "Deployment")
}
