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
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/cli-runtime/pkg/resource"

	releaseutil "helm.sh/helm/v4/internal/release/v2/manifest"
	"helm.sh/helm/v4/internal/release/v2/sequence"
	"helm.sh/helm/v4/pkg/kube"
	kubefake "helm.sh/helm/v4/pkg/kube/fake"
)

type recordingSequenceClient struct {
	kubefake.PrintingKubeClient
	operations          []string
	waitError           error
	waitErrorAt, waits  int
	onBuild, onMutation func()
}

func (c *recordingSequenceClient) Build(reader io.Reader, _ bool) (kube.ResourceList, error) {
	if c.onBuild != nil {
		c.onBuild()
	}
	data, _ := io.ReadAll(reader)
	var resources kube.ResourceList
	for document := range strings.SplitSeq(string(data), "---\n") {
		_, metadata, ok := strings.Cut(document, "  name: ")
		if !ok {
			continue
		}
		name, _, _ := strings.Cut(metadata, "\n")
		u := &unstructured.Unstructured{Object: map[string]any{"apiVersion": "v1", "kind": "ConfigMap", "metadata": map[string]any{"name": name}}}
		resources = append(resources, &resource.Info{Name: u.GetName(), Namespace: u.GetNamespace(), Object: u})
	}
	return resources, nil
}

func (c *recordingSequenceClient) Create(resources kube.ResourceList, _ ...kube.ClientCreateOption) (*kube.Result, error) {
	c.operations = append(c.operations, "create:"+resourceNames(resources))
	if c.onMutation != nil {
		c.onMutation()
	}
	return &kube.Result{Created: resources}, nil
}

func (c *recordingSequenceClient) Update(current, target kube.ResourceList, _ ...kube.ClientUpdateOption) (*kube.Result, error) {
	c.operations = append(c.operations, "update:"+resourceNames(target)+" from:"+resourceNames(current))
	if c.onMutation != nil {
		c.onMutation()
	}
	return &kube.Result{Updated: target}, nil
}

func (c *recordingSequenceClient) GetWaiter(kube.WaitStrategy) (kube.Waiter, error) {
	return &recordingSequenceWaiter{client: c}, nil
}

func (c *recordingSequenceClient) GetWaiterWithOptions(kube.WaitStrategy, ...kube.WaitOption) (kube.Waiter, error) {
	return &recordingSequenceWaiter{client: c}, nil
}

type recordingSequenceWaiter struct{ client *recordingSequenceClient }

func (w *recordingSequenceWaiter) Wait(resources kube.ResourceList, timeout time.Duration) error {
	if timeout <= 0 {
		return errors.New("nonpositive timeout")
	}
	w.client.waits++
	w.client.operations = append(w.client.operations, "wait:"+resourceNames(resources))
	if w.client.waits == w.client.waitErrorAt {
		return w.client.waitError
	}
	return nil
}

func (w *recordingSequenceWaiter) WaitWithJobs(r kube.ResourceList, d time.Duration) error {
	return w.Wait(r, d)
}

func (*recordingSequenceWaiter) WaitForDelete(kube.ResourceList, time.Duration) error { return nil }

func (*recordingSequenceWaiter) WatchUntilReady(kube.ResourceList, time.Duration) error { return nil }

func resourceNames(resources kube.ResourceList) string {
	names := make([]string, len(resources))
	for i := range resources {
		names[i] = resources[i].Name
	}
	return strings.Join(names, ",")
}

func sequenceManifest(path, name string) releaseutil.Manifest {
	return releaseutil.Manifest{Name: path, Content: "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: " + name + "\n"}
}

func threeLevelSequencePlan() *sequence.Plan {
	return &sequence.Plan{Batches: []sequence.Batch{
		{ChartPath: "parent/charts/child/charts/grand", Depth: 2, Manifests: []releaseutil.Manifest{sequenceManifest("parent/charts/child/charts/grand/templates/grand.yaml", "grand")}, Wait: true},
		{ChartPath: "parent/charts/child", Depth: 1, Manifests: []releaseutil.Manifest{sequenceManifest("parent/charts/child/templates/child.yaml", "child")}, Wait: true},
		{ChartPath: "parent", Manifests: []releaseutil.Manifest{sequenceManifest("parent/templates/parent.yaml", "parent")}, Wait: true},
	}}
}

func newSequenceDeployment(client *recordingSequenceClient) *sequencedDeployment {
	return &sequencedDeployment{kubeClient: client, releaseName: "demo", releaseNamespace: "spaced", waitStrategy: kube.OrderedWaitStrategy}
}

func TestSequencedApply_DependenciesBeforeDependents(t *testing.T) {
	client := &recordingSequenceClient{}
	require.NoError(t, newSequenceDeployment(client).apply(context.Background(), threeLevelSequencePlan()))
	assert.Equal(t, []string{"create:grand", "wait:grand", "create:child", "wait:child", "create:parent", "wait:parent"}, client.operations)
}

func TestSequencedApply_BatchFailureStopsLaterBatches(t *testing.T) {
	client := &recordingSequenceClient{waitError: assert.AnError, waitErrorAt: 2}
	err := newSequenceDeployment(client).apply(context.Background(), threeLevelSequencePlan())
	require.ErrorIs(t, err, assert.AnError)
	assert.Equal(t, []string{"create:grand", "wait:grand", "create:child", "wait:child"}, client.operations)
}

func TestSequencedApply_AdoptionUsesUpdate(t *testing.T) {
	client := &recordingSequenceClient{}
	deployment := newSequenceDeployment(client)
	current, err := client.Build(strings.NewReader(sequenceManifest("", "adopt").Content), false)
	require.NoError(t, err)
	deployment.currentResources = current
	plan := &sequence.Plan{Batches: []sequence.Batch{
		{Manifests: []releaseutil.Manifest{sequenceManifest("", "adopt")}, Wait: true},
		{Manifests: []releaseutil.Manifest{sequenceManifest("", "new")}, Wait: true},
	}}
	require.NoError(t, deployment.apply(context.Background(), plan))
	assert.Equal(t, []string{"update:adopt from:adopt", "wait:adopt", "create:new", "wait:new"}, client.operations)
}

func TestSequencedApply_ContextCancellation_Create(t *testing.T) {
	testSequenceCancellation(t, false)
}

func TestSequencedApply_ContextCancellation_Update(t *testing.T) {
	testSequenceCancellation(t, true)
}

func testSequenceCancellation(t *testing.T, update bool) {
	t.Helper()
	for _, stage := range []string{"before build", "after build", "after mutation"} {
		t.Run(stage, func(t *testing.T) {
			client := &recordingSequenceClient{}
			deployment := newSequenceDeployment(client)
			deployment.upgradeMode = update
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			switch stage {
			case "before build":
				cancel()
			case "after build":
				client.onBuild = cancel
			case "after mutation":
				client.onMutation = cancel
			}
			err := deployment.applyBatch(ctx, sequence.Batch{Manifests: []releaseutil.Manifest{sequenceManifest("", "cm")}, Wait: true})
			require.ErrorIs(t, err, context.Canceled)
			assert.NotContains(t, client.operations, "wait:cm")
		})
	}
}

func TestSequencing_BuildManifestYAML(t *testing.T) {
	assert.Equal(t, "one\n---\ntwo\n", buildManifestYAML([]releaseutil.Manifest{{Content: "one"}, {Content: "two"}}))
}

func TestBatchWaitTimeout(t *testing.T) {
	t.Run("unset defaults to one minute", func(t *testing.T) {
		got, err := (&sequencedDeployment{}).batchWaitTimeout()
		require.NoError(t, err)
		assert.Equal(t, time.Minute, got)
	})
	t.Run("remaining deadline caps readiness timeout", func(t *testing.T) {
		got, err := (&sequencedDeployment{readinessTimeout: time.Minute, deadline: time.Now().Add(20 * time.Second)}).batchWaitTimeout()
		require.NoError(t, err)
		assert.InDelta(t, 20*time.Second, got, float64(time.Second))
	})
	t.Run("expired deadline errors", func(t *testing.T) {
		_, err := (&sequencedDeployment{readinessTimeout: time.Minute, deadline: time.Now().Add(-time.Second)}).batchWaitTimeout()
		require.Error(t, err)
	})
}
