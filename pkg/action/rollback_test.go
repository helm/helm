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

	kubefake "helm.sh/helm/v3/pkg/kube/fake"
	"helm.sh/helm/v3/pkg/release"
)

func rollbackAction(t *testing.T) *Rollback {
	config := actionConfigFixture(t)
	rbAction := NewRollback(config)

	return rbAction
}

func TestRollbackRelease_RecreateResources(t *testing.T) {
	is := assert.New(t)

	rbAction := rollbackAction(t)
	rbAction.Timeout = 0
	rbAction.RecreateResources = true

	cfg := rbAction.cfg
	kubeClient := cfg.KubeClientV2

	prevRelease := releaseStub()
	prevRelease.Info.Status = release.StatusSuperseded
	prevRelease.Name = "my-release"
	prevRelease.Version = 0
	err := cfg.Releases.Create(prevRelease)
	is.NoError(err)

	currRelease := releaseStub()
	currRelease.Info.Status = release.StatusDeployed
	currRelease.Name = "my-release"
	currRelease.Version = 1
	err = cfg.Releases.Create(currRelease)
	is.NoError(err)

	t.Run("recreate should work when kubeClient and kubeClientV2 is set", func(t *testing.T) {
		verifiableKubeClient := kubefake.NewKubeClientSpy(kubeClient)
		cfg.KubeClient = verifiableKubeClient
		cfg.KubeClientV2 = verifiableKubeClient

		err := rbAction.Run(currRelease.Name)
		is.NoError(err)
		is.Equal(verifiableKubeClient.Calls["Update"], 0)
		is.Equal(verifiableKubeClient.Calls["UpdateRecreate"], 1)
	})

	t.Run("recreate should fallback to Update when only kubeClient is set", func(t *testing.T) {
		kubeClientSpy := kubefake.NewKubeClientSpy(kubeClient)
		cfg.KubeClient = kubeClientSpy
		cfg.KubeClientV2 = nil

		err := rbAction.Run(currRelease.Name)
		is.NoError(err)
		is.Equal(kubeClientSpy.Calls["Update"], 1)
		is.Equal(kubeClientSpy.Calls["UpdateRecreate"], 0)
	})
}
