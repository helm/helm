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

package tiller

import (
	"bytes"
	"testing"

	"k8s.io/helm/pkg/manifest"
)

func TestKindSorter(t *testing.T) {
	manifests := []Manifest{
		{
			Name: "i",
			Head: &manifest.SimpleHead{Kind: "ClusterRole"},
		},
		{
			Name: "I",
			Head: &manifest.SimpleHead{Kind: "ClusterRoleList"},
		},
		{
			Name: "j",
			Head: &manifest.SimpleHead{Kind: "ClusterRoleBinding"},
		},
		{
			Name: "J",
			Head: &manifest.SimpleHead{Kind: "ClusterRoleBindingList"},
		},
		{
			Name: "e",
			Head: &manifest.SimpleHead{Kind: "ConfigMap"},
		},
		{
			Name: "u",
			Head: &manifest.SimpleHead{Kind: "CronJob"},
		},
		{
			Name: "2",
			Head: &manifest.SimpleHead{Kind: "CustomResourceDefinition"},
		},
		{
			Name: "n",
			Head: &manifest.SimpleHead{Kind: "DaemonSet"},
		},
		{
			Name: "r",
			Head: &manifest.SimpleHead{Kind: "Deployment"},
		},
		{
			Name: "!",
			Head: &manifest.SimpleHead{Kind: "HonkyTonkSet"},
		},
		{
			Name: "v",
			Head: &manifest.SimpleHead{Kind: "Ingress"},
		},
		{
			Name: "t",
			Head: &manifest.SimpleHead{Kind: "Job"},
		},
		{
			Name: "c",
			Head: &manifest.SimpleHead{Kind: "LimitRange"},
		},
		{
			Name: "a",
			Head: &manifest.SimpleHead{Kind: "Namespace"},
		},
		{
			Name: "f",
			Head: &manifest.SimpleHead{Kind: "PersistentVolume"},
		},
		{
			Name: "g",
			Head: &manifest.SimpleHead{Kind: "PersistentVolumeClaim"},
		},
		{
			Name: "o",
			Head: &manifest.SimpleHead{Kind: "Pod"},
		},
		{
			Name: "3",
			Head: &manifest.SimpleHead{Kind: "PodSecurityPolicy"},
		},
		{
			Name: "q",
			Head: &manifest.SimpleHead{Kind: "ReplicaSet"},
		},
		{
			Name: "p",
			Head: &manifest.SimpleHead{Kind: "ReplicationController"},
		},
		{
			Name: "b",
			Head: &manifest.SimpleHead{Kind: "ResourceQuota"},
		},
		{
			Name: "k",
			Head: &manifest.SimpleHead{Kind: "Role"},
		},
		{
			Name: "K",
			Head: &manifest.SimpleHead{Kind: "RoleList"},
		},
		{
			Name: "l",
			Head: &manifest.SimpleHead{Kind: "RoleBinding"},
		},
		{
			Name: "L",
			Head: &manifest.SimpleHead{Kind: "RoleBindingList"},
		},
		{
			Name: "d",
			Head: &manifest.SimpleHead{Kind: "Secret"},
		},
		{
			Name: "m",
			Head: &manifest.SimpleHead{Kind: "Service"},
		},
		{
			Name: "h",
			Head: &manifest.SimpleHead{Kind: "ServiceAccount"},
		},
		{
			Name: "s",
			Head: &manifest.SimpleHead{Kind: "StatefulSet"},
		},
		{
			Name: "1",
			Head: &manifest.SimpleHead{Kind: "StorageClass"},
		},
		{
			Name: "w",
			Head: &manifest.SimpleHead{Kind: "APIService"},
		},
		{
			Name: "z",
			Head: &manifest.SimpleHead{Kind: "PodDisruptionBudget"},
		},
		{
			Name: "x",
			Head: &manifest.SimpleHead{Kind: "HorizontalPodAutoscaler"},
		},
	}

	for _, test := range []struct {
		description string
		order       SortType
		expected    string
	}{
		{"install", SortInstall, "abc3zde1fgh2iIjJkKlLmnopqrxstuvw!"},
		{"uninstall", SortUninstall, "wvmutsxrqponLlKkJjIi2hgf1edz3cba!"},
	} {
		var buf bytes.Buffer
		t.Run(test.description, func(t *testing.T) {
			if got, want := len(test.expected), len(manifests); got != want {
				t.Fatalf("Expected %d names in order, got %d", want, got)
			}
			defer buf.Reset()
			for _, r := range sortByKind(manifests, test.order) {
				buf.WriteString(r.Name)
			}
			if got := buf.String(); got != test.expected {
				t.Errorf("Expected %q, got %q", test.expected, got)
			}
		})
	}
}

// TestKindSorterSubSort verifies manifests of same kind are also sorted alphanumeric
func TestKindSorterSubSort(t *testing.T) {
	manifests := []Manifest{
		{
			Name: "a",
			Head: &manifest.SimpleHead{Kind: "ClusterRole"},
		},
		{
			Name: "A",
			Head: &manifest.SimpleHead{Kind: "ClusterRole"},
		},
		{
			Name: "0",
			Head: &manifest.SimpleHead{Kind: "ConfigMap"},
		},
		{
			Name: "1",
			Head: &manifest.SimpleHead{Kind: "ConfigMap"},
		},
		{
			Name: "z",
			Head: &manifest.SimpleHead{Kind: "ClusterRoleBinding"},
		},
		{
			Name: "!",
			Head: &manifest.SimpleHead{Kind: "ClusterRoleBinding"},
		},
		{
			Name: "u2",
			Head: &manifest.SimpleHead{Kind: "Unknown"},
		},
		{
			Name: "u1",
			Head: &manifest.SimpleHead{Kind: "Unknown"},
		},
		{
			Name: "t3",
			Head: &manifest.SimpleHead{Kind: "Unknown2"},
		},
	}
	for _, test := range []struct {
		description string
		order       SortType
		expected    string
	}{
		// expectation is sorted by kind (unknown is last) and then sub sorted alphabetically within each group
		{"cm,clusterRole,clusterRoleBinding,Unknown,Unknown2", SortInstall, "01Aa!zu1u2t3"},
	} {
		var buf bytes.Buffer
		t.Run(test.description, func(t *testing.T) {
			defer buf.Reset()
			for _, r := range sortByKind(manifests, test.order) {
				buf.WriteString(r.Name)
			}
			if got := buf.String(); got != test.expected {
				t.Errorf("Expected %q, got %q", test.expected, got)
			}
		})
	}
}

func TestKindSorterNamespaceAgainstUnknown(t *testing.T) {
	unknown := Manifest{
		Name: "a",
		Head: &manifest.SimpleHead{Kind: "Unknown"},
	}
	namespace := Manifest{
		Name: "b",
		Head: &manifest.SimpleHead{Kind: "Namespace"},
	}

	manifests := []Manifest{unknown, namespace}
	sortByKind(manifests, SortInstall)

	expectedOrder := []Manifest{namespace, unknown}
	for i, manifest := range manifests {
		if expectedOrder[i].Name != manifest.Name {
			t.Errorf("Expected %s, got %s", expectedOrder[i].Name, manifest.Name)
		}
	}
}
