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
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
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

func TestReleaseTestingGetPodLogs_JobRetrievalError(t *testing.T) {
	config := actionConfigFixture(t)
	require.NoError(t, config.Init(cli.New().RESTClientGetter(), "", os.Getenv("HELM_DRIVER")))
	client := NewReleaseTesting(config)

	hooks := []*release.Hook{
		{
			Kind:   "Job",
			Name:   "job-1",
			Events: []release.HookEvent{release.HookTest},
		},
	}

	require.ErrorContains(t, client.GetPodLogs(&bytes.Buffer{}, &release.Release{Hooks: hooks}), "unable to get job")
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
	require.NoError(t, rt.getContainerLogs(&buf, client, "test-pod"))
	output := buf.String()
	assert.Contains(t, output, "POD LOGS: test-pod (main)")
	assert.Contains(t, output, "POD LOGS: test-pod (sidecar)")
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
	rt := &ReleaseTesting{Namespace: "default"}

	var buf bytes.Buffer
	require.NoError(t, rt.getContainerLogs(&buf, client, "multi-test"))
	output := buf.String()
	assert.Contains(t, output, "POD LOGS: multi-test (container-a)")
	assert.Contains(t, output, "POD LOGS: multi-test (container-b)")
}

func testJob(name string, selector *metav1.LabelSelector) *batchv1.Job {
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
		Spec: batchv1.JobSpec{
			Selector: selector,
		},
	}
}

func testJobPod(name string, labels map[string]string, created time.Time) *v1.Pod {
	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			Namespace:         "default",
			Labels:            labels,
			CreationTimestamp: metav1.NewTime(created),
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{Name: "test"},
			},
		},
	}
}

func TestGetJobLogs(t *testing.T) {
	jobLabels := map[string]string{"batch.kubernetes.io/controller-uid": "1234"}
	now := time.Now()

	tests := []struct {
		name         string
		objects      []runtime.Object
		expected     []string
		unexpected   []string
		errorMessage string
	}{
		{
			name: "logs from the job's pod",
			objects: []runtime.Object{
				testJob("test-job", &metav1.LabelSelector{MatchLabels: jobLabels}),
				testJobPod("test-job-abcde", jobLabels, now),
			},
			expected: []string{"POD LOGS: test-job-abcde (test)"},
		},
		{
			name: "logs from every attempt, oldest first",
			objects: []runtime.Object{
				testJob("test-job", &metav1.LabelSelector{MatchLabels: jobLabels}),
				testJobPod("test-job-retry", jobLabels, now.Add(time.Minute)),
				testJobPod("test-job-first", jobLabels, now),
			},
			expected: []string{
				"POD LOGS: test-job-first (test)",
				"POD LOGS: test-job-retry (test)",
			},
		},
		{
			name: "pods not created by the job are skipped",
			objects: []runtime.Object{
				testJob("test-job", &metav1.LabelSelector{MatchLabels: jobLabels}),
				testJobPod("test-job-abcde", jobLabels, now),
				testJobPod("unrelated", map[string]string{"app": "unrelated"}, now),
			},
			expected:   []string{"POD LOGS: test-job-abcde (test)"},
			unexpected: []string{"unrelated"},
		},
		{
			name: "job without pods writes nothing",
			objects: []runtime.Object{
				testJob("test-job", &metav1.LabelSelector{MatchLabels: jobLabels}),
			},
		},
		{
			name:         "job not found",
			errorMessage: "unable to get job test-job",
		},
		{
			name: "job without a selector",
			objects: []runtime.Object{
				testJob("test-job", nil),
				testJobPod("unrelated", map[string]string{"app": "unrelated"}, now),
			},
			unexpected:   []string{"unrelated"},
			errorMessage: "unable to get pods for job test-job: job has no selector",
		},
		{
			name: "job with an empty selector",
			objects: []runtime.Object{
				testJob("test-job", &metav1.LabelSelector{}),
				testJobPod("unrelated", map[string]string{"app": "unrelated"}, now),
			},
			unexpected:   []string{"unrelated"},
			errorMessage: "unable to get pods for job test-job: job has an empty selector",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := fakeclientset.NewClientset(tt.objects...)
			rt := &ReleaseTesting{Namespace: "default"}

			var buf bytes.Buffer
			err := rt.getJobLogs(&buf, client, "test-job")
			if tt.errorMessage != "" {
				require.ErrorContains(t, err, tt.errorMessage)
			} else {
				require.NoError(t, err)
			}

			output := buf.String()
			if len(tt.expected) == 0 {
				assert.Empty(t, output)
			}
			last := -1
			for _, want := range tt.expected {
				idx := strings.Index(output, want)
				require.GreaterOrEqual(t, idx, 0, "expected %q in output:\n%s", want, output)
				assert.Greater(t, idx, last, "expected %q to appear after the previous pod's logs", want)
				last = idx
			}
			for _, notWant := range tt.unexpected {
				assert.NotContains(t, output, notWant)
			}
		})
	}
}
