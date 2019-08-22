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

	util "k8s.io/helm/pkg/releaseutil"
)

func TestKindSorter(t *testing.T) {
	manifests := []Manifest{
		{
			Name: "i",
			Head: &util.SimpleHead{Kind: "ClusterRole"},
		},
		{
			Name: "I",
			Head: &util.SimpleHead{Kind: "ClusterRoleList"},
		},
		{
			Name: "j",
			Head: &util.SimpleHead{Kind: "ClusterRoleBinding"},
		},
		{
			Name: "J",
			Head: &util.SimpleHead{Kind: "ClusterRoleBindingList"},
		},
		{
			Name: "e",
			Head: &util.SimpleHead{Kind: "ConfigMap"},
		},
		{
			Name: "u",
			Head: &util.SimpleHead{Kind: "CronJob"},
		},
		{
			Name: "2",
			Head: &util.SimpleHead{Kind: "CustomResourceDefinition"},
		},
		{
			Name: "n",
			Head: &util.SimpleHead{Kind: "DaemonSet"},
		},
		{
			Name: "r",
			Head: &util.SimpleHead{Kind: "Deployment"},
		},
		{
			Name: "!",
			Head: &util.SimpleHead{Kind: "HonkyTonkSet"},
		},
		{
			Name: "v",
			Head: &util.SimpleHead{Kind: "Ingress"},
		},
		{
			Name: "t",
			Head: &util.SimpleHead{Kind: "Job"},
		},
		{
			Name: "c",
			Head: &util.SimpleHead{Kind: "LimitRange"},
		},
		{
			Name: "a",
			Head: &util.SimpleHead{Kind: "Namespace"},
		},
		{
			Name: "f",
			Head: &util.SimpleHead{Kind: "PersistentVolume"},
		},
		{
			Name: "g",
			Head: &util.SimpleHead{Kind: "PersistentVolumeClaim"},
		},
		{
			Name: "o",
			Head: &util.SimpleHead{Kind: "Pod"},
		},
		{
			Name: "3",
			Head: &util.SimpleHead{Kind: "PodSecurityPolicy"},
		},
		{
			Name: "q",
			Head: &util.SimpleHead{Kind: "ReplicaSet"},
		},
		{
			Name: "p",
			Head: &util.SimpleHead{Kind: "ReplicationController"},
		},
		{
			Name: "b",
			Head: &util.SimpleHead{Kind: "ResourceQuota"},
		},
		{
			Name: "k",
			Head: &util.SimpleHead{Kind: "Role"},
		},
		{
			Name: "K",
			Head: &util.SimpleHead{Kind: "RoleList"},
		},
		{
			Name: "l",
			Head: &util.SimpleHead{Kind: "RoleBinding"},
		},
		{
			Name: "L",
			Head: &util.SimpleHead{Kind: "RoleBindingList"},
		},
		{
			Name: "d",
			Head: &util.SimpleHead{Kind: "Secret"},
		},
		{
			Name: "m",
			Head: &util.SimpleHead{Kind: "Service"},
		},
		{
			Name: "h",
			Head: &util.SimpleHead{Kind: "ServiceAccount"},
		},
		{
			Name: "s",
			Head: &util.SimpleHead{Kind: "StatefulSet"},
		},
		{
			Name: "1",
			Head: &util.SimpleHead{Kind: "StorageClass"},
		},
		{
			Name: "w",
			Head: &util.SimpleHead{Kind: "APIService"},
		},
		{
			Name: "z",
			Head: &util.SimpleHead{Kind: "PodDisruptionBudget"},
		},
		{
			Name: "x",
			Head: &util.SimpleHead{Kind: "HorizontalPodAutoscaler"},
		},
		{
			Name: "B",
			Head: &util.SimpleHead{Kind: "NetworkPolicy"},
		},
	}

	for _, test := range []struct {
		description string
		order       SortOrder
		expected    string
	}{
		{"install", InstallOrder, "aBbc3zde1fgh2iIjJkKlLmnopqrxstuvw!"},
		{"uninstall", UninstallOrder, "wvmutsxrqponLlKkJjIi2hgf1edz3cbBa!"},
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
			Head: &util.SimpleHead{Kind: "ClusterRole"},
		},
		{
			Name: "A",
			Head: &util.SimpleHead{Kind: "ClusterRole"},
		},
		{
			Name: "0",
			Head: &util.SimpleHead{Kind: "ConfigMap"},
		},
		{
			Name: "1",
			Head: &util.SimpleHead{Kind: "ConfigMap"},
		},
		{
			Name: "z",
			Head: &util.SimpleHead{Kind: "ClusterRoleBinding"},
		},
		{
			Name: "!",
			Head: &util.SimpleHead{Kind: "ClusterRoleBinding"},
		},
		{
			Name: "u2",
			Head: &util.SimpleHead{Kind: "Unknown"},
		},
		{
			Name: "u1",
			Head: &util.SimpleHead{Kind: "Unknown"},
		},
		{
			Name: "t3",
			Head: &util.SimpleHead{Kind: "Unknown2"},
		},
	}
	for _, test := range []struct {
		description string
		order       SortOrder
		expected    string
	}{
		// expectation is sorted by kind (unknown is last) and then sub sorted alphabetically within each group
		{"cm,clusterRole,clusterRoleBinding,Unknown,Unknown2", InstallOrder, "01Aa!zu1u2t3"},
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
		Head: &util.SimpleHead{Kind: "Unknown"},
	}
	namespace := Manifest{
		Name: "b",
		Head: &util.SimpleHead{Kind: "Namespace"},
	}

	manifests := []Manifest{unknown, namespace}
	sortByKind(manifests, InstallOrder)

	expectedOrder := []Manifest{namespace, unknown}
	for i, manifest := range manifests {
		if expectedOrder[i].Name != manifest.Name {
			t.Errorf("Expected %s, got %s", expectedOrder[i].Name, manifest.Name)
		}
	}
}
