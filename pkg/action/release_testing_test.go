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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	kubefake "helm.sh/helm/v3/pkg/kube/fake"
	"helm.sh/helm/v3/pkg/release"
)

func testingActions(t *testing.T) *ReleaseTesting {
	config := actionConfigFixture(t)

	tester := config.KubeClient.(*kubefake.FailingKubeClient)
	tester.WatchDuration = 5 * time.Second
	config.KubeClient = tester

	relTesting := NewReleaseTesting(config)
	relTesting.Namespace = "spaced"

	return relTesting
}

func TestReleaseTesting_RaceWithAWaitInstall(t *testing.T) {
	is := assert.New(t)
	req := require.New(t)

	releaseTestingAction := testingActions(t)

	err := releaseTestingAction.cfg.Releases.Create(getTestRelease())
	req.NoError(err)

	rel, err := releaseTestingAction.cfg.Releases.Get(getTestRelease().Name, 2)
	req.NoError(err)
	req.Equal(rel.Info.Status, release.StatusPendingUpgrade)

	done := make(chan bool)
	go func() {
		_, err := releaseTestingAction.Run(rel.Name)
		req.NoError(err)
		done <- true
	}()

	time.Sleep(time.Second * 1)

	updageTestRelease := getTestRelease() //Recreate cause mem otherwise shares same release -- Data race and shared resource
	updageTestRelease.Info.Status = release.StatusDeployed
	err = releaseTestingAction.cfg.Releases.Update(updageTestRelease)
	req.NoError(err)

	finalRelease, err := releaseTestingAction.cfg.Releases.Get(updageTestRelease.Name, 2)
	req.NoError(err)
	is.Equal(release.StatusDeployed, finalRelease.Info.Status)

	<-done

	finalRelease, err = releaseTestingAction.cfg.Releases.Get(updageTestRelease.Name, 2)
	req.NoError(err)
	is.Equal(release.StatusDeployed, finalRelease.Info.Status)
}

func getTestRelease() *release.Release {
	testRelease := releaseStub()
	testRelease.Info.Status = release.StatusPendingUpgrade
	testRelease.Version = 2
	return testRelease
}
