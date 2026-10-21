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

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/cli-runtime/pkg/resource"

	"helm.sh/helm/v4/pkg/kube"
)

func TestFilterResourcesToKeep(t *testing.T) {
	withPolicy := func(name, policy string) *resource.Info {
		return newDeploymentWithOwner(name, "ns-a", nil, map[string]string{kube.ResourcePolicyAnno: policy})
	}
	unstructuredKeep := &resource.Info{
		Name: "unstructured",
		Object: &unstructured.Unstructured{Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]any{
				"name":        "unstructured",
				"annotations": map[string]any{kube.ResourcePolicyAnno: kube.KeepPolicy},
			},
		}},
	}

	testCases := []struct {
		name          string
		resources     kube.ResourceList
		wantKeep      []string
		wantRemaining []string
	}{
		{
			name:      "keep",
			resources: kube.ResourceList{withPolicy("a", "keep")},
			wantKeep:  []string{"a"},
		},
		{
			name:      "keep with different case and spaces",
			resources: kube.ResourceList{withPolicy("a", " Keep ")},
			wantKeep:  []string{"a"},
		},
		{
			name:      "keep on an unstructured object",
			resources: kube.ResourceList{unstructuredKeep},
			wantKeep:  []string{"unstructured"},
		},
		{
			name:          "other policy value",
			resources:     kube.ResourceList{withPolicy("a", "delete")},
			wantRemaining: []string{"a"},
		},
		{
			name:          "no annotations",
			resources:     kube.ResourceList{newDeploymentWithOwner("a", "ns-a", nil, nil)},
			wantRemaining: []string{"a"},
		},
		{
			name:          "object without metadata",
			resources:     kube.ResourceList{{Name: "a"}},
			wantRemaining: []string{"a"},
		},
		{
			name: "order of remaining is preserved",
			resources: kube.ResourceList{
				withPolicy("a", ""),
				withPolicy("b", "keep"),
				withPolicy("c", "delete"),
				newDeploymentWithOwner("d", "ns-a", nil, nil),
			},
			wantKeep:      []string{"b"},
			wantRemaining: []string{"a", "c", "d"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			keep, remaining := filterResourcesToKeep(tc.resources)
			assert.Equal(t, tc.wantKeep, resourceNames(keep))
			assert.Equal(t, tc.wantRemaining, resourceNames(remaining))
		})
	}
}

func resourceNames(resources kube.ResourceList) []string {
	var names []string
	for _, info := range resources {
		names = append(names, info.Name)
	}
	return names
}
