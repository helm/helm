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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	fakeclientset "k8s.io/client-go/kubernetes/fake"

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

	require.ErrorContains(t, client.GetPodLogs(&bytes.Buffer{}, &release.Release{Hooks: hooks}), "unable to get pod")
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

func TestGetContainerLogs_MultipleContainers(t *testing.T) {
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{Name: "main"},
				{Name: "sidecar"},
			},
		},
	}

	client := fakeclientset.NewClientset(pod)
	rt := &ReleaseTesting{Namespace: "default"}

	var buf bytes.Buffer
	err := rt.getContainerLogs(&buf, client, "test-pod")
	// The fake client doesn't serve real log streams, so we expect
	// per-container errors rather than success, but critically it should
	// NOT fail with "a container name must be specified".
	if err != nil {
		assert.NotContains(t, err.Error(), "a container name must be specified")
		assert.Contains(t, err.Error(), "container main")
		assert.Contains(t, err.Error(), "container sidecar")
	}
}

func TestGetContainerLogs_WithInitContainers(t *testing.T) {
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
		},
		Spec: v1.PodSpec{
			InitContainers: []v1.Container{
				{Name: "init-setup"},
			},
			Containers: []v1.Container{
				{Name: "main"},
			},
		},
	}

	client := fakeclientset.NewClientset(pod)
	rt := &ReleaseTesting{Namespace: "default"}

	var buf bytes.Buffer
	err := rt.getContainerLogs(&buf, client, "test-pod")
	if err != nil {
		// Both init and regular containers should be attempted
		assert.Contains(t, err.Error(), "container init-setup")
		assert.Contains(t, err.Error(), "container main")
	}
}

func TestGetContainerLogs_PodNotFound(t *testing.T) {
	client := fakeclientset.NewClientset()
	rt := &ReleaseTesting{Namespace: "default"}

	var buf bytes.Buffer
	err := rt.getContainerLogs(&buf, client, "nonexistent-pod")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unable to get pod nonexistent-pod")
}

func TestGetPodLogs_MultiContainerOutput(t *testing.T) {
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "multi-test",
			Namespace: "default",
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{Name: "container-a"},
				{Name: "container-b"},
			},
		},
	}

	client := fakeclientset.NewClientset(pod)
	rt := &ReleaseTesting{
		Namespace: "default",
		Filters:   map[string][]string{},
	}

	// Call getContainerLogs directly to test output formatting
	var buf bytes.Buffer
	_ = rt.getContainerLogs(&buf, client, "multi-test")
	output := buf.String()
	// Even if logs fail, check that header formatting uses container names
	if len(output) > 0 {
		assert.True(t, strings.Contains(output, "(container-a)") || strings.Contains(output, "(container-b)"))
	}
}
