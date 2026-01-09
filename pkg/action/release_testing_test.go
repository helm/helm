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
	"context"
	"errors"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"helm.sh/helm/v4/pkg/cli"
	"helm.sh/helm/v4/pkg/kube"
	kubefake "helm.sh/helm/v4/pkg/kube/fake"
	release "helm.sh/helm/v4/pkg/release/v1"
)

func TestNewReleaseTesting(t *testing.T) {
	config := actionConfigFixture(t)
	client := NewReleaseTesting(config)

	assert.NotNil(t, client)
	assert.Equal(t, config, client.cfg)
}

func TestReleaseTestingRun_UnreachableKubeClient(t *testing.T) {
	config := actionConfigFixture(t)
	failingKubeClient := kubefake.FailingKubeClient{PrintingKubeClient: kubefake.PrintingKubeClient{Out: io.Discard}, DummyResources: nil}
	failingKubeClient.ConnectionError = errors.New("connection refused")
	config.KubeClient = &failingKubeClient

	client := NewReleaseTesting(config)
	result, _, err := client.Run("")
	assert.Nil(t, result)
	assert.Error(t, err)
}

func TestReleaseTestingGetPodLogs_FilterEvents(t *testing.T) {
	config := actionConfigFixture(t)
	require.NoError(t, config.Init(cli.New().RESTClientGetter(), "", os.Getenv("HELM_DRIVER")))
	client := NewReleaseTesting(config)
	client.Filters[ExcludeNameFilter] = []string{"event-1"}
	client.Filters[IncludeNameFilter] = []string{"event-3"}

	hooks := []*release.Hook{
		{
			Name:   "event-1",
			Events: []release.HookEvent{release.HookTest},
		},
		{
			Name:   "event-2",
			Events: []release.HookEvent{release.HookTest},
		},
	}

	out := &bytes.Buffer{}
	require.NoError(t, client.GetPodLogs(out, &release.Release{Hooks: hooks}))

	assert.Empty(t, out.String())
}

func TestReleaseTestingGetPodLogs_PodRetrievalError(t *testing.T) {
	config := actionConfigFixture(t)
	require.NoError(t, config.Init(cli.New().RESTClientGetter(), "", os.Getenv("HELM_DRIVER")))
	client := NewReleaseTesting(config)

	hooks := []*release.Hook{
		{
			Name:   "event-1",
			Events: []release.HookEvent{release.HookTest},
		},
	}

	require.ErrorContains(t, client.GetPodLogs(&bytes.Buffer{}, &release.Release{Hooks: hooks}), "unable to get pod logs")
}

func TestReleaseTesting_WaitOptionsPassedDownstream(t *testing.T) {
	is := assert.New(t)
	config := actionConfigFixture(t)

	// Create a release with a test hook
	rel := releaseStub()
	rel.Name = "wait-options-test-release"
	rel.ApplyMethod = "csa"
	require.NoError(t, config.Releases.Create(rel))

	client := NewReleaseTesting(config)

	// Use WithWaitContext as a marker WaitOption that we can track
	ctx := context.Background()
	client.WaitOptions = []kube.WaitOption{kube.WithWaitContext(ctx)}

	// Access the underlying FailingKubeClient to check recorded options
	failer := config.KubeClient.(*kubefake.FailingKubeClient)

	_, _, err := client.Run(rel.Name)
	is.NoError(err)

	// Verify that WaitOptions were passed to GetWaiter
	is.NotEmpty(failer.RecordedWaitOptions, "WaitOptions should be passed to GetWaiter")
}
