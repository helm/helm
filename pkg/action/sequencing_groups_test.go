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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	chart "helm.sh/helm/v4/internal/chart/v3"
	releaseutil "helm.sh/helm/v4/internal/release/v2/manifest"
	"helm.sh/helm/v4/internal/release/v2/resourcegroup"
	"helm.sh/helm/v4/internal/release/v2/sequence"
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
