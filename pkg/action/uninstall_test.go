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
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	kubefake "helm.sh/helm/v3/pkg/kube/fake"
	"helm.sh/helm/v3/pkg/release"
	rspb "helm.sh/helm/v3/pkg/release"
)

func uninstallAction(t *testing.T) *Uninstall {
	config := actionConfigFixture(t)

	unAction := NewUninstall(config)
	return unAction
}

func getPreDeleteRelease(exitCode uint8) *rspb.Release {
	rel := releaseStub()
	rel.Name = "chart-with-pre-delete-hook"
	rel.Manifest = fmt.Sprintf(`{
		"apiVersion": "v1",
		"kind": "Pod",
		"metadata": {
			"name": "release-name-assert-job",
			"labels": {
				"app.kubernetes.io/managed-by": "service",
				"app.kubernetes.io/instance": "release-name",
				"app.kubernetes.io/version": "1.0.0",
				"helm.sh/chart": "pre-delete-hook-chart"
			},
			"annotations": {
				"helm.sh/hook": "pre-delete",
				"helm.sh/hook-weight": "5",
				"helm.sh/hook-delete-policy": "before-hook-creation,hook-succeeded"
			}
		},
		"spec": {
			"restartPolicy": "Never",
			"containers": [
				{
					"name": "assert-chart-removal",
					"image": "alpine:latest",
					"command": [
						"/bin/ash"
					],
					"args": [
						"-c",
						"#!/bin/ash\nexit %v\n"
					]
				}
			]
		}
	}`, exitCode)

	return rel
}

func uninstallWithPreDeleteHookSetup(t *testing.T, shouldError bool) ([]*rspb.Release, *assert.Assertions) {
	unAction := uninstallAction(t)
	kc, ok := unAction.cfg.KubeClient.(*kubefake.FailingKubeClient)
	if !ok {
		t.Fatalf("Failed casting KubeClient to FailingKubeClient")
	}

	if shouldError {
		kc.CreateError = fmt.Errorf("Could not delete Pod")
	}

	var errorCode uint8 = 0

	if shouldError {
		errorCode = 0
	}
	rel := getPreDeleteRelease(uint8(errorCode))
	unAction.cfg.Releases.Create(rel)
	unAction.Run(rel.Name)

	rels, _ := unAction.cfg.Releases.List(func(rl *rspb.Release) bool {
		return rl.Info.Status == release.StatusDeployed
	})

	is := assert.New(t)

	return rels, is
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

func TestUninstallWithPreDeleteHookFailure(t *testing.T) {
	releases, is := uninstallWithPreDeleteHookSetup(t, true)

	is.Len(releases, 1)
}

func TestUninstallWithPreDeleteHookSuccess(t *testing.T) {
	releases, is := uninstallWithPreDeleteHookSetup(t, false)

	is.Len(releases, 0)
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
	unAction.cfg.Releases.Create(rel)
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
	unAction.Wait = true

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
	failer := unAction.cfg.KubeClient.(*kubefake.FailingKubeClient)
	failer.WaitError = fmt.Errorf("U timed out")
	unAction.cfg.KubeClient = failer
	res, err := unAction.Run(rel.Name)
	is.Error(err)
	is.Contains(err.Error(), "U timed out")
	is.Equal(res.Release.Info.Status, release.StatusUninstalled)
}

func TestUninstallRelease_Cascade(t *testing.T) {
	is := assert.New(t)

	unAction := uninstallAction(t)
	unAction.DisableHooks = true
	unAction.DryRun = false
	unAction.Wait = false
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
	failer := unAction.cfg.KubeClient.(*kubefake.FailingKubeClient)
	failer.DeleteWithPropagationError = fmt.Errorf("Uninstall with cascade failed")
	failer.BuildDummy = true
	unAction.cfg.KubeClient = failer
	_, err := unAction.Run(rel.Name)
	is.Error(err)
	is.Contains(err.Error(), "failed to delete release: come-fail-away")
}
