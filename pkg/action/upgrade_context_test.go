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
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/resource"

	"helm.sh/helm/v4/pkg/kube"
	kubefake "helm.sh/helm/v4/pkg/kube/fake"
	"helm.sh/helm/v4/pkg/release/common"
)

type observedUpgradeCancellation struct {
	context.Context
	observed chan struct{}
}

func (c observedUpgradeCancellation) Err() error {
	close(c.observed)
	return c.Context.Err()
}

func withObservedUpgradeCancellation(ctx context.Context, observed chan struct{}) context.Context {
	return observedUpgradeCancellation{ctx, observed}
}

func TestUpgradeContextCancellationAfterCompletion(t *testing.T) {
	for _, status := range []common.Status{common.StatusDeployed, common.StatusFailed} {
		t.Run(string(status), func(t *testing.T) {
			u := upgradeAction(t)
			rel := releaseStub()
			rel.Info.Status = status
			description := "original result: " + string(status)
			rel.Info.Description = description
			require.NoError(t, u.cfg.Releases.Create(rel))

			ctx, cancel := context.WithCancel(t.Context())
			cancel()
			observed := make(chan struct{})
			done := make(chan any)
			results := make(chan resultMessage, 1)
			returned := make(chan struct{})

			// Queue cancellation behind a completed upgrade's result reporting. The
			// context handler must recheck completion after acquiring the result lock.
			u.Lock.Lock()
			locked := true
			defer func() {
				if locked {
					u.Lock.Unlock()
				}
			}()
			go func() {
				u.handleContext(withObservedUpgradeCancellation(ctx, observed), done, results, rel)
				close(returned)
			}()
			select {
			case <-observed:
			case <-time.After(5 * time.Second):
				t.Fatal("context handler did not observe cancellation")
			}
			close(done)
			u.Lock.Unlock()
			locked = false

			select {
			case <-returned:
			case <-time.After(5 * time.Second):
				t.Fatal("context handler did not return after upgrade completion")
			}
			require.Equal(t, status, rel.Info.Status)
			require.Equal(t, description, rel.Info.Description)
			require.Empty(t, results, "completed upgrades must not report another result")
			stored, err := u.cfg.Releases.Last(rel.Name)
			require.NoError(t, err)
			storedRelease, err := releaserToV1Release(stored)
			require.NoError(t, err)
			require.Equal(t, status, storedRelease.Info.Status)
			require.Equal(t, description, storedRelease.Info.Description)
		})
	}
}

type upgradeCleanupAfterCancelClient struct {
	*kubefake.FailingKubeClient
	updateStarted chan struct{}
	finishUpdate  chan struct{}
	deleted       chan kube.ResourceList
	created       kube.ResourceList
}

func (c *upgradeCleanupAfterCancelClient) Update(_, _ kube.ResourceList, _ ...kube.ClientUpdateOption) (*kube.Result, error) {
	close(c.updateStarted)
	<-c.finishUpdate
	return &kube.Result{Created: c.created}, errors.New("update failed after cancellation")
}

func (c *upgradeCleanupAfterCancelClient) Delete(resources kube.ResourceList, _ metav1.DeletionPropagation) (*kube.Result, []error) {
	c.deleted <- resources
	return &kube.Result{Deleted: resources}, nil
}

func TestUpgradeCleanupAfterCancellation(t *testing.T) {
	u := upgradeAction(t)
	u.CleanupOnFail = true
	rel := releaseStub()
	rel.Info.Status = common.StatusDeployed
	require.NoError(t, u.cfg.Releases.Create(rel))
	client := &upgradeCleanupAfterCancelClient{
		FailingKubeClient: u.cfg.KubeClient.(*kubefake.FailingKubeClient),
		updateStarted:     make(chan struct{}),
		finishUpdate:      make(chan struct{}),
		deleted:           make(chan kube.ResourceList, 1),
		created:           kube.ResourceList{&resource.Info{Name: "created-after-cancellation"}},
	}
	u.cfg.KubeClient = client
	var finish sync.Once
	finishUpdate := func() { finish.Do(func() { close(client.finishUpdate) }) }
	defer finishUpdate()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	result := make(chan error, 1)
	go func() {
		_, err := u.RunWithContext(ctx, rel.Name, buildChart(), nil)
		result <- err
	}()
	select {
	case <-client.updateStarted:
	case <-time.After(5 * time.Second):
		t.Fatal("upgrade did not start updating resources")
	}
	cancel()
	select {
	case err := <-result:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(5 * time.Second):
		t.Fatal("upgrade did not return after cancellation")
	}

	// The Kubernetes operation can still fail after the caller has returned.
	// Preserve cleanup of resources it created despite that cancellation.
	finishUpdate()
	select {
	case deleted := <-client.deleted:
		require.Equal(t, client.created, deleted)
	case <-time.After(5 * time.Second):
		t.Fatal("resources created after cancellation were not cleaned up")
	}
}
