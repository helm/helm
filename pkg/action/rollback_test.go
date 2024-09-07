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
	"time"

	kubefake "helm.sh/helm/v3/pkg/kube/fake"
	"helm.sh/helm/v3/pkg/release"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func rollbackAction(t *testing.T) *Rollback {
	config := actionConfigFixture(t)
	rollAction := NewRollback(config)
	rollAction.Timeout = 30 * time.Second
	rollAction.RollbackTimeout = 60 * time.Second

	return rollAction
}

func TestRollback_Base(t *testing.T) {
	is := assert.New(t)
	req := require.New(t)

	rollAction := rollbackAction(t)
	currentReleaseName := "rollback-base"

	// Create v1 release
	rel1 := releaseStub()
	rel1.Name = currentReleaseName
	rel1.Version = 1
	rel1.Info.Status = release.StatusSuperseded

	// Create v2 release
	rel2 := releaseStub()
	rel2.Name = currentReleaseName
	rel2.Version = 2
	rel2.Info.Status = release.StatusDeployed
	rollAction.cfg.Releases.Create(rel1)
	rollAction.cfg.Releases.Create(rel2)

	fakeClient := rollAction.cfg.KubeClient.(*kubefake.FailingKubeClient)
	rollAction.cfg.KubeClient = fakeClient

	rollAction.Run(currentReleaseName)
	curr, err := rollAction.cfg.Releases.Last(currentReleaseName)
	req.NoError(err)
	is.Equal(3, curr.Version)
	is.Equal(release.StatusDeployed, curr.Info.Status)
	is.Equal("Rollback to 1", curr.Info.Description)

	v2, err := rollAction.cfg.Releases.Get(currentReleaseName, 2)
	req.NoError(err)
	is.Equal(2, v2.Version)
	is.Equal(release.StatusSuperseded, v2.Info.Status)

}

func TestRollback_RollbackTimeout(t *testing.T) {
	is := assert.New(t)
	req := require.New(t)

	rollAction := rollbackAction(t)
	rollAction.RollbackTimeout = 1 * time.Second
	currentReleaseName := "rollback-rollback-timeout"

	// Create v1 release
	rel1 := releaseStub()
	rel1.Name = currentReleaseName
	rel1.Version = 1
	rel1.Info.Status = release.StatusSuperseded

	// Create v2 release
	rel2 := releaseStub()
	rel2.Name = currentReleaseName
	rel2.Version = 2
	rel2.Info.Status = release.StatusDeployed
	rollAction.cfg.Releases.Create(rel1)
	rollAction.cfg.Releases.Create(rel2)

	fakeClient := rollAction.cfg.KubeClient.(*kubefake.FailingKubeClient)
	rollAction.cfg.KubeClient = fakeClient

	rollAction.RunTest(currentReleaseName, 3*time.Second)
	curr, err := rollAction.cfg.Releases.Last(currentReleaseName)
	req.NoError(err)
	is.Equal(2, curr.Version)
	is.Equal(release.StatusDeployed, curr.Info.Status)
	is.Equal("Named Release Stub", curr.Info.Description)

}
