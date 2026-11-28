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
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	fakeclientset "k8s.io/client-go/kubernetes/fake"

	"helm.sh/helm/v4/pkg/cli"
	"helm.sh/helm/v4/pkg/kube"
	kubefake "helm.sh/helm/v4/pkg/kube/fake"
	ri "helm.sh/helm/v4/pkg/release"
	release "helm.sh/helm/v4/pkg/release/v1"
	"helm.sh/helm/v4/pkg/storage"
	"helm.sh/helm/v4/pkg/storage/driver"
)

func TestNewReleaseTesting(t *testing.T) {
	config := actionConfigFixture(t)
	client := NewReleaseTesting(config)

	assert.NotNil(t, client)
	assert.Equal(t, config, client.cfg)
}

func TestReleaseTestingRun_UnreachableKubeClient(t *testing.T) {
	config := actionConfigFixture(t)
	failingKubeClient := kubefake.FailingKubeClient{Out: io.Discard, DummyResources: nil}
	failingKubeClient.ConnectionError = errors.New("connection refused")
	config.KubeClient = &failingKubeClient

	client := NewReleaseTesting(config)
	result, _, err := client.Run("")
	assert.Nil(t, result)
	assert.Error(t, err)
}

// Observe each persisted update, before Run can restore any filtered hooks.
type releaseTestingRecordingDriver struct {
	driver.Driver
	onUpdate func()
}

func (d *releaseTestingRecordingDriver) Update(key string, rel ri.Releaser) error {
	if err := d.Driver.Update(key, rel); err != nil {
		return err
	}
	d.onUpdate()
	return nil
}

func TestReleaseTestingRun_PreservesStoredHooks(t *testing.T) {
	filterCases := []struct {
		name    string
		filters map[string][]string
		wantRun []string
	}{
		{name: "unfiltered", wantRun: []string{"gamma", "alpha", "beta"}},
		{name: "include", filters: map[string][]string{IncludeNameFilter: {"beta", "gamma"}}, wantRun: []string{"gamma", "beta"}},
		{name: "exclude", filters: map[string][]string{ExcludeNameFilter: {"beta"}}, wantRun: []string{"gamma", "alpha"}},
		{name: "include and exclude", filters: map[string][]string{IncludeNameFilter: {"beta", "gamma"}, ExcludeNameFilter: {"beta"}}, wantRun: []string{"gamma"}},
		{name: "no matching name", filters: map[string][]string{IncludeNameFilter: {"missing"}}},
		{name: "exclude all tests", filters: map[string][]string{ExcludeNameFilter: {"alpha", "beta", "gamma"}}},
		{name: "non-test hook", filters: map[string][]string{IncludeNameFilter: {"install"}}},
	}
	failureCases := []struct {
		name        string
		createError error
		waitError   error
	}{
		{name: "success"},
		{name: "create failure", createError: errors.New("cannot create test pod")},
		{name: "canceled wait", waitError: context.Canceled},
	}

	for _, filterCase := range filterCases {
		for _, failureCase := range failureCases {
			t.Run(filterCase.name+"/"+failureCase.name, func(t *testing.T) {
				config := actionConfigFixture(t)
				clientset := fakeclientset.NewClientset()
				// Use serialized storage so snapshots cannot change through shared pointers.
				recorder := &releaseTestingRecordingDriver{Driver: driver.NewSecrets(clientset.CoreV1().Secrets("default"))}
				config.Releases = storage.Init(recorder)
				failer := config.KubeClient.(*kubefake.FailingKubeClient)
				failer.CreateError = failureCase.createError
				failer.WatchUntilReadyError = failureCase.waitError

				rel := releaseStub()
				rel.Namespace = "default"
				rel.Hooks = nil
				wantNames := []string{"gamma", "install", "beta", "alpha"}
				previousRun := release.HookExecution{
					StartedAt:   time.Date(2020, time.January, 1, 0, 0, 0, 0, time.UTC),
					CompletedAt: time.Date(2020, time.January, 1, 0, 1, 0, 0, time.UTC),
					Phase:       release.HookPhaseSucceeded,
				}
				for _, name := range wantNames {
					event := release.HookTest
					if name == "install" {
						event = release.HookPreInstall
					}
					h := &release.Hook{
						Name: name, Kind: "Pod", Events: []release.HookEvent{event},
						LastRun: previousRun,
					}
					if name == "gamma" {
						h.Weight = -1
					}
					rel.Hooks = append(rel.Hooks, h)
				}
				require.NoError(t, config.Releases.Create(rel))

				updates := 0
				runningUpdates := 0
				recorder.onUpdate = func() {
					stored, err := config.Releases.Get(rel.Name, rel.Version)
					require.NoError(t, err)
					snapshot, err := releaserToV1Release(stored)
					require.NoError(t, err)
					var names []string
					for _, h := range snapshot.Hooks {
						names = append(names, h.Name)
						if h.LastRun.Phase == release.HookPhaseRunning {
							runningUpdates++
						}
					}
					assert.Equal(t, wantNames, names, "every persisted release must retain all hooks in their original order")
					updates++
				}

				client := NewReleaseTesting(config)
				client.Filters = filterCase.filters
				result, shutdown, err := client.Run(rel.Name)
				wantRun := filterCase.wantRun
				wantErr := failureCase.createError
				if wantErr == nil {
					wantErr = failureCase.waitError
				}
				if len(wantRun) == 0 || wantErr == nil {
					require.NoError(t, err)
				} else {
					require.ErrorIs(t, err, wantErr)
					wantRun = wantRun[:1]
				}
				require.NotNil(t, shutdown)
				shutdownErr := shutdown()
				if len(wantRun) > 0 && failureCase.waitError != nil {
					require.ErrorIs(t, shutdownErr, failureCase.waitError)
				} else {
					require.NoError(t, shutdownErr)
				}
				assert.Equal(t, len(wantRun)+1, updates)
				assert.Equal(t, len(wantRun), runningUpdates)

				returned, err := releaserToV1Release(result)
				require.NoError(t, err)
				stored, err := config.Releases.Get(rel.Name, rel.Version)
				require.NoError(t, err)
				persisted, err := releaserToV1Release(stored)
				require.NoError(t, err)
				for _, checked := range []*release.Release{returned, persisted} {
					var names []string
					for _, h := range checked.Hooks {
						names = append(names, h.Name)
						if !slices.Contains(wantRun, h.Name) {
							assert.Equal(t, previousRun, h.LastRun, "skipped hook %s must be untouched", h.Name)
							assert.Empty(t, h.DeletePolicies, "skipped hook %s must not be prepared", h.Name)
							continue
						}
						wantPhase := release.HookPhaseSucceeded
						if wantErr != nil {
							wantPhase = release.HookPhaseFailed
						}
						assert.Equal(t, wantPhase, h.LastRun.Phase)
						assert.True(t, h.LastRun.StartedAt.After(previousRun.CompletedAt))
						assert.False(t, h.LastRun.CompletedAt.IsZero())
					}
					assert.Equal(t, wantNames, names)
				}
			})
		}
	}
}

func TestReleaseTestingGetPodLogs_FilterEvents(t *testing.T) {
	config := actionConfigFixture(t)
	require.NoError(t, config.Init(cli.New().RESTClientGetter(), "", os.Getenv("HELM_DRIVER")))
	client := NewReleaseTesting(config)
	client.Filters[ExcludeNameFilter] = []string{"event-1"}
	client.Filters[IncludeNameFilter] = []string{"event-3"}

	hooks := []*release.Hook{
		{
			Kind:   "Pod",
			Name:   "event-1",
			Events: []release.HookEvent{release.HookTest},
		},
		{
			Kind:   "Pod",
			Name:   "event-2",
			Events: []release.HookEvent{release.HookTest},
		},
		{
			Kind:   "ConfigMap",
			Name:   "event-3",
			Events: []release.HookEvent{release.HookTest},
		},
	}

	out := &bytes.Buffer{}
	require.NoError(t, client.GetPodLogs(out, &release.Release{Hooks: hooks}))

	assert.Empty(t, out.String())
}

func TestReleaseTestingGetPodLogs_ExcludeFilter_SkipsPodHook(t *testing.T) {
	config := actionConfigFixture(t)
	require.NoError(t, config.Init(cli.New().RESTClientGetter(), "", os.Getenv("HELM_DRIVER")))
	client := NewReleaseTesting(config)
	client.Filters[ExcludeNameFilter] = []string{"excluded-pod"}

	hooks := []*release.Hook{
		{
			Kind:   "Pod",
			Name:   "excluded-pod",
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
			Kind:   "Pod",
			Name:   "event-1",
			Events: []release.HookEvent{release.HookTest},
		},
	}

	require.ErrorContains(t, client.GetPodLogs(&bytes.Buffer{}, &release.Release{Hooks: hooks}), "unable to get pod")
}

func TestReleaseTestingGetPodLogs_SkipNonPodHooks(t *testing.T) {
	config := actionConfigFixture(t)
	require.NoError(t, config.Init(cli.New().RESTClientGetter(), "", os.Getenv("HELM_DRIVER")))
	client := NewReleaseTesting(config)

	hooks := []*release.Hook{
		{
			Name:   "cm-hook",
			Kind:   "ConfigMap",
			Events: []release.HookEvent{release.HookTest},
		},
		{
			Name:   "secret-hook",
			Kind:   "Secret",
			Events: []release.HookEvent{release.HookTest},
		},
	}

	out := &bytes.Buffer{}
	require.NoError(t, client.GetPodLogs(out, &release.Release{Hooks: hooks}))
	assert.Empty(t, out.String())
}

func TestReleaseTesting_WaitOptionsPassedDownstream(t *testing.T) {
	is := assert.New(t)
	req := require.New(t)
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
	req.NoError(err)

	// Verify that WaitOptions were passed to GetWaiter
	is.NotEmpty(failer.RecordedWaitOptions, "WaitOptions should be passed to GetWaiter")
}

func TestGetContainerLogs_MultipleContainers(t *testing.T) {
	pod := &v1.Pod{
		Name:      "test-pod",
		Namespace: "default",
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
	require.NoError(t, rt.getContainerLogs(&buf, client, "test-pod"))
	output := buf.String()
	assert.Contains(t, output, "POD LOGS: test-pod (main)")
	assert.Contains(t, output, "POD LOGS: test-pod (sidecar)")
}

func TestGetContainerLogs_WithInitContainers(t *testing.T) {
	pod := &v1.Pod{
		Name:      "test-pod",
		Namespace: "default",
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
	require.NoError(t, rt.getContainerLogs(&buf, client, "test-pod"))
	output := buf.String()
	// Init containers should appear before regular containers
	assert.Contains(t, output, "POD LOGS: test-pod (init-setup)")
	assert.Contains(t, output, "POD LOGS: test-pod (main)")
}

func TestGetContainerLogs_PodNotFound(t *testing.T) {
	client := fakeclientset.NewClientset()
	rt := &ReleaseTesting{Namespace: "default"}

	var buf bytes.Buffer
	assert.ErrorContains(t, rt.getContainerLogs(&buf, client, "nonexistent-pod"), "unable to get pod nonexistent-pod")
}

func TestGetContainerLogs_OutputHeaderFormat(t *testing.T) {
	pod := &v1.Pod{
		Name:      "multi-test",
		Namespace: "default",
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{Name: "container-a"},
				{Name: "container-b"},
			},
		},
	}

	client := fakeclientset.NewClientset(pod)
	rt := &ReleaseTesting{Namespace: "default"}

	var buf bytes.Buffer
	require.NoError(t, rt.getContainerLogs(&buf, client, "multi-test"))
	output := buf.String()
	assert.Contains(t, output, "POD LOGS: multi-test (container-a)")
	assert.Contains(t, output, "POD LOGS: multi-test (container-b)")
}
