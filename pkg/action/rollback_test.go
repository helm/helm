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
	"github.com/stretchr/testify/require"

	kubefake "helm.sh/helm/v3/pkg/kube/fake"
	"helm.sh/helm/v3/pkg/release"
)

func rollbackAction(t *testing.T) *Rollback {
	config := actionConfigFixture(t)
	rollAction := NewRollback(config)

	return rollAction
}

func TestRollbackRelease_CleanupOnFail(t *testing.T) {
	is := assert.New(t)
	req := require.New(t)

	rbAction := rollbackAction(t)
	rbAction.Version = 1
	curRel := releaseStub()
	curRel.Name = "rollback-release"
	curRel.Version = 2
	curRel.Namespace = "spaced"
	curRel.Info.Status = release.StatusDeployed
	rbAction.cfg.Releases.Create(curRel)

	tarRel := releaseStub()
	tarRel.Name = "rollback-release"
	tarRel.Version = 1
	tarRel.Namespace = "spaced"
	tarRel.Info.Status = release.StatusSuperseded
	rbAction.cfg.Releases.Create(tarRel)

	failer := rbAction.cfg.KubeClient.(*kubefake.FailingKubeClient)
	failer.UpdateError = fmt.Errorf("I timed out")
	failer.DeleteError = fmt.Errorf("I tried to delete nil")
	rbAction.cfg.KubeClient = failer
	rbAction.CleanupOnFail = true

	err := rbAction.Run(curRel.Name)
	req.Error(err)
	is.NotContains(err.Error(), "unable to cleanup resources")
	is.Contains(err.Error(), "I timed out")
}
