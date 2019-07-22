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

package releaseutil

import (
	"bytes"
	"testing"
)

func TestKindSorter(t *testing.T) {
	manifests := []Manifest{
		{
			Name: "i",
			Head: &SimpleHead{Kind: "ClusterRole"},
		},
		{
			Name: "j",
			Head: &SimpleHead{Kind: "ClusterRoleBinding"},
		},
		{
			Name: "e",
			Head: &SimpleHead{Kind: "ConfigMap"},
		},
		{
			Name: "u",
			Head: &SimpleHead{Kind: "CronJob"},
		},
		{
			Name: "2",
			Head: &SimpleHead{Kind: "CustomResourceDefinition"},
		},
		{
			Name: "n",
			Head: &SimpleHead{Kind: "DaemonSet"},
		},
		{
			Name: "r",
			Head: &SimpleHead{Kind: "Deployment"},
		},
		{
			Name: "!",
			Head: &SimpleHead{Kind: "HonkyTonkSet"},
		},
		{
			Name: "v",
			Head: &SimpleHead{Kind: "Ingress"},
		},
		{
			Name: "t",
			Head: &SimpleHead{Kind: "Job"},
		},
		{
			Name: "c",
			Head: &SimpleHead{Kind: "LimitRange"},
		},
		{
			Name: "a",
			Head: &SimpleHead{Kind: "Namespace"},
		},
		{
			Name: "f",
			Head: &SimpleHead{Kind: "PersistentVolume"},
		},
		{
			Name: "g",
			Head: &SimpleHead{Kind: "PersistentVolumeClaim"},
		},
		{
			Name: "o",
			Head: &SimpleHead{Kind: "Pod"},
		},
		{
			Name: "q",
			Head: &SimpleHead{Kind: "ReplicaSet"},
		},
		{
			Name: "p",
			Head: &SimpleHead{Kind: "ReplicationController"},
		},
		{
			Name: "b",
			Head: &SimpleHead{Kind: "ResourceQuota"},
		},
		{
			Name: "k",
			Head: &SimpleHead{Kind: "Role"},
		},
		{
			Name: "l",
			Head: &SimpleHead{Kind: "RoleBinding"},
		},
		{
			Name: "d",
			Head: &SimpleHead{Kind: "Secret"},
		},
		{
			Name: "m",
			Head: &SimpleHead{Kind: "Service"},
		},
		{
			Name: "h",
			Head: &SimpleHead{Kind: "ServiceAccount"},
		},
		{
			Name: "s",
			Head: &SimpleHead{Kind: "StatefulSet"},
		},
		{
			Name: "1",
			Head: &SimpleHead{Kind: "StorageClass"},
		},
		{
			Name: "w",
			Head: &SimpleHead{Kind: "APIService"},
		},
		{
			Name: "x",
			Head: &SimpleHead{Kind: "HorizontalPodAutoscaler"},
		},
	}

	for _, test := range []struct {
		description string
		order       KindSortOrder
		expected    string
	}{
		{"install", InstallOrder, "abcde1fgh2ijklmnopqrxstuvw!"},
		{"uninstall", UninstallOrder, "wvmutsxrqponlkji2hgf1edcba!"},
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
			Head: &SimpleHead{Kind: "ClusterRole"},
		},
		{
			Name: "A",
			Head: &SimpleHead{Kind: "ClusterRole"},
		},
		{
			Name: "0",
			Head: &SimpleHead{Kind: "ConfigMap"},
		},
		{
			Name: "1",
			Head: &SimpleHead{Kind: "ConfigMap"},
		},
		{
			Name: "z",
			Head: &SimpleHead{Kind: "ClusterRoleBinding"},
		},
		{
			Name: "!",
			Head: &SimpleHead{Kind: "ClusterRoleBinding"},
		},
		{
			Name: "u2",
			Head: &SimpleHead{Kind: "Unknown"},
		},
		{
			Name: "u1",
			Head: &SimpleHead{Kind: "Unknown"},
		},
		{
			Name: "t3",
			Head: &SimpleHead{Kind: "Unknown2"},
		},
	}
	for _, test := range []struct {
		description string
		order       KindSortOrder
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
