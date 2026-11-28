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
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/resource"
	"sigs.k8s.io/yaml"

	chart "helm.sh/helm/v4/internal/chart/v3"
	releaseutil "helm.sh/helm/v4/internal/release/v2/manifest"
	"helm.sh/helm/v4/internal/release/v2/resourcegroup"
	"helm.sh/helm/v4/internal/release/v2/sequence"
	"helm.sh/helm/v4/pkg/kube"
)

func TestGroupSequencedApply_OrderAndFailureBoundaries(t *testing.T) {
	t.Run("group-only chart applies groups before unsequenced resources", func(t *testing.T) {
		chrt, manifests := groupActionFixture()
		assert.Empty(t, chrt.Metadata.Dependencies)

		plan, err := sequence.Build(chrt, manifests)
		require.NoError(t, err)
		require.Len(t, plan.Batches, 3)
		assert.Equal(t, []sequence.BatchKind{
			sequence.BatchKindGroups,
			sequence.BatchKindGroups,
			sequence.BatchKindUnsequenced,
		}, groupActionBatchKinds(plan))

		client := &recordingSequenceClient{}
		require.NoError(t, newSequenceDeployment(client).apply(context.Background(), plan))
		assert.Equal(t, []string{
			"create:database", "wait:database",
			"create:app", "wait:app",
			"create:plain", "wait:plain",
		}, client.operations)
	})

	t.Run("subchart group batches apply before parent group batches", func(t *testing.T) {
		chrt, manifests := combinedGroupActionFixture()
		plan, err := sequence.Build(chrt, manifests)
		require.NoError(t, err)

		client := &recordingSequenceClient{}
		require.NoError(t, newSequenceDeployment(client).apply(context.Background(), plan))
		assert.Equal(t, []string{
			"create:child-database", "wait:child-database",
			"create:child-app", "wait:child-app",
			"create:parent-database", "wait:parent-database",
			"create:parent-app", "wait:parent-app",
		}, client.operations)
	})

	for _, upgrade := range []bool{false, true} {
		mode := "create"
		mutation := "create:database"
		if upgrade {
			mode = "update"
			mutation = "update:database from:"
		}
		t.Run(mode+" failure prevents the next batch mutation", func(t *testing.T) {
			chrt, manifests := groupActionFixture()
			plan, err := sequence.Build(chrt, manifests)
			require.NoError(t, err)

			client := &recordingSequenceClient{waitError: assert.AnError, waitErrorAt: 1}
			deployment := newSequenceDeployment(client)
			deployment.upgradeMode = upgrade
			err = deployment.apply(context.Background(), plan)
			require.ErrorIs(t, err, assert.AnError)
			assert.Equal(t, []string{mutation, "wait:database"}, client.operations)
		})
	}
}

func TestGroupSequencedDelete_ExactReverseOrder(t *testing.T) {
	chrt, manifests := groupActionFixture()
	plan, err := sequence.Build(chrt, manifests)
	require.NoError(t, err)

	client := &recordingSequenceClient{}
	deleted, report, err := newSequenceDeployment(client).deleteSequencedBatches(
		plan.Reverse(),
		metav1.DeletePropagationBackground,
		&recordingSequenceWaiter{client: client},
		nil,
	)
	require.NoError(t, err)
	assert.Equal(t, "plain,app,database", resourceNames(deleted))
	assert.Empty(t, report)
	assert.Equal(t, []string{
		"delete:plain", "wait-delete:plain",
		"delete:app", "wait-delete:app",
		"delete:database", "wait-delete:database",
	}, client.operations)
}

func groupActionFixture() (*chart.Chart, []releaseutil.Manifest) {
	return groupActionChart("parent"), []releaseutil.Manifest{
		groupActionManifest("parent/templates/database.yaml", "database", "database"),
		groupActionManifest("parent/templates/app.yaml", "app", "app", "database"),
		sequenceManifest("parent/templates/plain.yaml", "plain"),
	}
}

func combinedGroupActionFixture() (*chart.Chart, []releaseutil.Manifest) {
	child := groupActionChart("child")
	parent := groupActionChart("parent")
	parent.Metadata.Dependencies = []*chart.Dependency{{Name: "child", Enabled: true}}
	parent.SetDependencies(child)

	return parent, []releaseutil.Manifest{
		groupActionManifest("parent/charts/child/templates/database.yaml", "child-database", "database"),
		groupActionManifest("parent/charts/child/templates/app.yaml", "child-app", "app", "database"),
		groupActionManifest("parent/templates/database.yaml", "parent-database", "database"),
		groupActionManifest("parent/templates/app.yaml", "parent-app", "app", "database"),
	}
}

func groupActionChart(name string) *chart.Chart {
	return &chart.Chart{Metadata: &chart.Metadata{
		APIVersion: chart.APIVersionV3,
		Name:       name,
		Version:    "0.1.0",
	}}
}

func groupActionManifest(path, name, group string, dependencies ...string) releaseutil.Manifest {
	annotations := map[string]string{resourcegroup.AnnotationResourceGroup: group}
	content := fmt.Sprintf("apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: %s\n  annotations:\n    %s: %s\n", name, resourcegroup.AnnotationResourceGroup, group)
	if len(dependencies) > 0 {
		encoded, err := json.Marshal(dependencies)
		if err != nil {
			panic(err)
		}
		annotations[resourcegroup.AnnotationDependsOnResourceGroups] = string(encoded)
		content += fmt.Sprintf("    %s: '%s'\n", resourcegroup.AnnotationDependsOnResourceGroups, encoded)
	}

	return releaseutil.Manifest{
		Name:    path,
		Content: content,
		Head: &releaseutil.SimpleHead{
			Version: "v1",
			Kind:    "ConfigMap",
			Metadata: &struct {
				Name        string            `json:"name"`
				Namespace   string            `json:"namespace,omitempty"`
				Annotations map[string]string `json:"annotations"`
			}{Name: name, Annotations: annotations},
		},
	}
}

func groupActionBatchKinds(plan *sequence.Plan) []sequence.BatchKind {
	kinds := make([]sequence.BatchKind, len(plan.Batches))
	for i, batch := range plan.Batches {
		kinds[i] = batch.Kind
	}
	return kinds
}

func TestStripSequencingAnnotations(t *testing.T) {
	tests := []struct {
		name        string
		annotations map[string]string
		want        map[string]string
	}{
		{
			name: "removes only the dependency key",
			annotations: map[string]string{
				resourcegroup.AnnotationResourceGroup:           "app",
				resourcegroup.AnnotationDependsOnResourceGroups: `["database"]`,
				"example.com/other":                             "kept",
			},
			want: map[string]string{
				resourcegroup.AnnotationResourceGroup: "app",
				"example.com/other":                   "kept",
			},
		},
		{
			name:        "dependency key alone",
			annotations: map[string]string{resourcegroup.AnnotationDependsOnResourceGroups: `["database"]`},
			want:        map[string]string{},
		},
		{
			name:        "no annotations",
			annotations: nil,
			want:        nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := &unstructured.Unstructured{Object: map[string]any{"apiVersion": "v1", "kind": "ConfigMap"}}
			u.SetName("cm")
			u.SetAnnotations(tt.annotations)
			require.NoError(t, stripSequencingAnnotations(kube.ResourceList{{Name: "cm", Object: u}}))
			assert.Equal(t, tt.want, u.GetAnnotations())
		})
	}
}

// annotationRecordingClient decodes manifests and records the annotations of
// every object sent to Create or Update.
type annotationRecordingClient struct {
	recordingSequenceClient
	sent []map[string]string
}

func (c *annotationRecordingClient) Build(reader io.Reader, _ bool) (kube.ResourceList, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	var resources kube.ResourceList
	for document := range strings.SplitSeq(string(data), "---\n") {
		if strings.TrimSpace(document) == "" {
			continue
		}
		object := map[string]any{}
		if err := yaml.Unmarshal([]byte(document), &object); err != nil {
			return nil, err
		}
		u := &unstructured.Unstructured{Object: object}
		u.SetNamespace("spaced")
		resources = append(resources, &resource.Info{
			Name: u.GetName(), Namespace: u.GetNamespace(), Object: u,
			Mapping: &meta.RESTMapping{
				Resource:         schema.GroupVersionResource{Version: "v1", Resource: "configmaps"},
				GroupVersionKind: schema.GroupVersionKind{Version: "v1", Kind: "ConfigMap"},
				Scope:            meta.RESTScopeNamespace,
			},
		})
	}
	return resources, nil
}

func (c *annotationRecordingClient) record(resources kube.ResourceList) {
	for _, info := range resources {
		accessor, err := meta.Accessor(info.Object)
		if err == nil {
			c.sent = append(c.sent, accessor.GetAnnotations())
		}
	}
}

func (c *annotationRecordingClient) Create(resources kube.ResourceList, _ ...kube.ClientCreateOption) (*kube.Result, error) {
	c.record(resources)
	return &kube.Result{Created: resources}, nil
}

func (c *annotationRecordingClient) Update(current, target kube.ResourceList, _ ...kube.ClientUpdateOption) (*kube.Result, error) {
	c.record(current)
	c.record(target)
	return &kube.Result{Updated: target}, nil
}

func TestSequencedApply_DependsOnAnnotationNeverSent(t *testing.T) {
	manifest := func(name string) releaseutil.Manifest {
		return releaseutil.Manifest{
			Name: "parent/templates/" + name + ".yaml",
			Content: "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: " + name + "\n  annotations:\n" +
				"    " + resourcegroup.AnnotationResourceGroup + ": app\n" +
				"    " + resourcegroup.AnnotationDependsOnResourceGroups + ": '[\"database\"]'\n",
		}
	}
	plan := &sequence.Plan{Batches: []sequence.Batch{{ChartPath: "parent", Manifests: []releaseutil.Manifest{manifest("app")}, Wait: true}}}

	for _, upgrade := range []bool{false, true} {
		t.Run(fmt.Sprintf("upgrade=%t", upgrade), func(t *testing.T) {
			client := &annotationRecordingClient{}
			deployment := newSequenceDeployment(&client.recordingSequenceClient)
			deployment.kubeClient = client
			deployment.upgradeMode = upgrade
			if upgrade {
				current, err := client.Build(strings.NewReader(manifest("app").Content), false)
				require.NoError(t, err)
				deployment.currentResources = current
			}

			require.NoError(t, deployment.apply(context.Background(), plan))
			require.NotEmpty(t, client.sent)
			for _, annotations := range client.sent {
				assert.NotContains(t, annotations, resourcegroup.AnnotationDependsOnResourceGroups)
				assert.Equal(t, "app", annotations[resourcegroup.AnnotationResourceGroup])
			}
		})
	}
}
