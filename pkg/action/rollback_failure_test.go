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
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
	fakeclientset "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

	"helm.sh/helm/v4/pkg/kube"
	kubefake "helm.sh/helm/v4/pkg/kube/fake"
	"helm.sh/helm/v4/pkg/release/common"
	release "helm.sh/helm/v4/pkg/release/v1"
	"helm.sh/helm/v4/pkg/storage"
	"helm.sh/helm/v4/pkg/storage/driver"
)

func TestRollbackRunFailure(t *testing.T) {
	tests := []struct {
		name                                             string
		hook                                             release.HookEvent
		build                                            int
		create, watch, waiter, update, wait, jobs, query bool
	}{
		{name: "current manifest", build: 1},
		{name: "target manifest", build: 2},
		{name: "pre-hook creation", hook: release.HookPreRollback, create: true},
		{name: "pre-hook watching", hook: release.HookPreRollback, watch: true},
		{name: "waiter acquisition", waiter: true},
		{name: "post-hook creation", hook: release.HookPostRollback, create: true},
		{name: "post-hook watching", hook: release.HookPostRollback, watch: true},
		{name: "deployed query", query: true},
		{name: "update", update: true},
		{name: "wait", wait: true},
		{name: "wait with jobs", wait: true, jobs: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := actionConfigFixture(t)
			client := fakeclientset.NewClientset()
			// Serialization keeps mutations of the in-flight release separate from stored state.
			cfg.Releases = storage.Init(driver.NewSecrets(client.CoreV1().Secrets("default")))
			executionErr := errors.New("execution failed")
			kc := &rollbackFailingKubeClient{
				FailingKubeClient: cfg.KubeClient.(*kubefake.FailingKubeClient),
				failBuild:         tt.build,
				err:               executionErr,
				failWaiter:        tt.waiter,
			}
			if tt.create {
				kc.CreateError = executionErr
			}
			if tt.watch {
				kc.WatchUntilReadyError = executionErr
			}
			if tt.update {
				kc.UpdateError = executionErr
			}
			if tt.wait {
				kc.WaitError = executionErr
			}
			cfg.KubeClient = kc
			failQuery := tt.query
			if tt.query {
				client.PrependReactor("list", "secrets", func(action k8stesting.Action) (bool, runtime.Object, error) {
					status, _ := action.(k8stesting.ListAction).GetListRestrictions().Labels.RequiresExactMatch("status")
					if failQuery && status == common.StatusDeployed.String() {
						return true, nil, executionErr
					}
					return false, nil, nil
				})
			}
			previous := releaseStub()
			previous.Namespace = "default"
			previous.Info.Status = common.StatusSuperseded
			previous.ApplyMethod = "csa"
			previous.Hooks = nil
			if tt.hook != "" {
				previous.Hooks = []*release.Hook{{
					Name: "rollback-hook", Kind: "Job", Path: "rollback-hook",
					Manifest:       "apiVersion: batch/v1\nkind: Job\nmetadata:\n  name: rollback-hook\n",
					Events:         []release.HookEvent{tt.hook},
					DeletePolicies: []release.HookDeletePolicy{release.HookFailed},
				}}
			}
			require.NoError(t, cfg.Releases.Create(previous))
			current := releaseStub()
			current.Namespace = "default"
			current.Version = 2
			current.ApplyMethod = "csa"
			require.NoError(t, cfg.Releases.Create(current))

			rollback := NewRollback(cfg)
			rollback.Version = 1
			rollback.WaitForJobs = tt.jobs
			err := rollback.Run(previous.Name)
			require.ErrorIs(t, err, executionErr, "rollback must return its execution error")
			stored, err := cfg.Releases.Get(previous.Name, 3)
			require.NoError(t, err)
			failed, err := releaserToV1Release(stored)
			require.NoError(t, err)
			assert.Equal(t, common.StatusFailed, failed.Info.Status, "rollback revision must be terminal after Run returns")
			assert.Contains(t, failed.Info.Description, executionErr.Error(), "failed rollback must describe its error")
			if tt.hook != "" {
				assert.Equal(t, release.HookPhaseFailed, failed.Hooks[0].LastRun.Phase, "failed hook execution must be recorded")
			}
			stored, err = cfg.Releases.Get(previous.Name, 2)
			require.NoError(t, err)
			prior, err := releaserToV1Release(stored)
			require.NoError(t, err)
			wantStatus := common.StatusDeployed
			if tt.update {
				wantStatus = common.StatusSuperseded
			}
			assert.Equal(t, wantStatus, prior.Info.Status, "failed rollback must preserve previous revision status semantics")

			// Remove fault injection before exercising the successor through the public API.
			cfg.KubeClient = &kubefake.FailingKubeClient{Out: io.Discard}
			failQuery = false
			upgrade := NewUpgrade(cfg)
			upgrade.DryRunStrategy = DryRunClient
			_, err = upgrade.Run(previous.Name, buildChart(), nil)
			assert.NoError(t, err, "upgrade after failed rollback must not be locked out")
		})
	}
}

type rollbackFailingKubeClient struct {
	*kubefake.FailingKubeClient
	failBuild, builds int
	failWaiter        bool
	err               error
}

func (c *rollbackFailingKubeClient) Build(r io.Reader, validate bool) (kube.ResourceList, error) {
	c.builds++
	if c.builds == c.failBuild {
		return nil, c.err
	}
	return c.FailingKubeClient.Build(r, validate)
}

func (c *rollbackFailingKubeClient) GetWaiterWithOptions(strategy kube.WaitStrategy, options ...kube.WaitOption) (kube.Waiter, error) {
	if c.failWaiter {
		return nil, c.err
	}
	return c.FailingKubeClient.GetWaiterWithOptions(strategy, options...)
}
