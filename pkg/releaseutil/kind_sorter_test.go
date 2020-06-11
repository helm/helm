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
	"sort"
	"testing"

	"helm.sh/helm/v3/pkg/release"
)

func buildManifestsForTestKindSorter() []Manifest {
	return []Manifest{
		{
			Name: "E",
			Head: &SimpleHead{Kind: "SecretList"},
		},
		{
			Name: "i",
			Head: &SimpleHead{Kind: "ClusterRole"},
		},
		{
			Name: "I",
			Head: &SimpleHead{Kind: "ClusterRoleList"},
		},
		{
			Name: "j",
			Head: &SimpleHead{Kind: "ClusterRoleBinding"},
		},
		{
			Name: "J",
			Head: &SimpleHead{Kind: "ClusterRoleBindingList"},
		},
		{
			Name: "f",
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
			Name: "A",
			Head: &SimpleHead{Kind: "NetworkPolicy"},
		},
		{
			Name: "g",
			Head: &SimpleHead{Kind: "PersistentVolume"},
		},
		{
			Name: "h",
			Head: &SimpleHead{Kind: "PersistentVolumeClaim"},
		},
		{
			Name: "o",
			Head: &SimpleHead{Kind: "Pod"},
		},
		{
			Name: "3",
			Head: &SimpleHead{Kind: "PodDisruptionBudget"},
		},
		{
			Name: "C",
			Head: &SimpleHead{Kind: "PodSecurityPolicy"},
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
			Name: "K",
			Head: &SimpleHead{Kind: "RoleList"},
		},
		{
			Name: "l",
			Head: &SimpleHead{Kind: "RoleBinding"},
		},
		{
			Name: "L",
			Head: &SimpleHead{Kind: "RoleBindingList"},
		},
		{
			Name: "e",
			Head: &SimpleHead{Kind: "Secret"},
		},
		{
			Name: "m",
			Head: &SimpleHead{Kind: "Service"},
		},
		{
			Name: "d",
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
		{
			Name: "N",
			Head: &SimpleHead{Kind: "NginxVhost"},
		},
		{
			Name: "U",
			Head: &SimpleHead{Kind: "Unknown"},
		},
		{
			Name: "R",
			Head: &SimpleHead{Kind: "Registration"},
		},
	}
}

func TestKindSorter(t *testing.T) {
	manifests := buildManifestsForTestKindSorter()

	for _, test := range []struct {
		description string
		order       KindSortOrder
		expected    string
	}{
		{"install", InstallOrder, "aAbcC3deEf1gh2iIjJkKlLmnopqrxstuvw!NUR"},
		{"uninstall", UninstallOrder, "wvmutsxrqponLlKkJjIi2hg1fEed3CcbAaRUN!"},
	} {
		var buf bytes.Buffer
		t.Run(test.description, func(t *testing.T) {
			if got, want := len(test.expected), len(manifests); got != want {
				t.Fatalf("Expected %d names in order, got %d", want, got)
			}
			defer buf.Reset()
			orig := manifests
			for _, r := range sortManifestsByKind(manifests, test.order) {
				buf.WriteString(r.Name)
			}
			if got := buf.String(); got != test.expected {
				t.Errorf("Expected %q, got %q", test.expected, got)
			}
			for i, manifest := range orig {
				if manifest != manifests[i] {
					t.Fatal("Expected input to sortManifestsByKind to stay the same")
				}
			}
		})
	}
}

func TestKindSorterFailedUsingAlphabeticalOrder(t *testing.T) {
	manifests := buildManifestsForTestKindSorter()

	for _, test := range []struct {
		description string
		order       KindSortOrder
		expected    string
		actual      string
	}{
		{"install", InstallOrder, "aAbcC3deEf1gh2iIjJkKlLmnopqrxstuvw!NUR", "aAbcC3deEf1gh2iIjJkKlLmnopqrxstuvw!NRU"},
		{"uninstall", UninstallOrder, "wvmutsxrqponLlKkJjIi2hg1fEed3CcbAaRUN!", "wvmutsxrqponLlKkJjIi2hg1fEed3CcbAa!NRU"},
	} {
		var buf bytes.Buffer
		t.Run(test.description, func(t *testing.T) {
			if got, want := len(test.expected), len(manifests); got != want {
				t.Fatalf("Expected %d names in order, got %d", want, got)
			}
			defer buf.Reset()
			orig := manifests
			for _, r := range sortManifestsByKindForCRsUsingAlphabeticalOrder(manifests, test.order) {
				buf.WriteString(r.Name)
			}
			if got := buf.String(); got == test.actual && got != test.expected {
				t.Logf("Expected %q, got %q", test.expected, got)
			}
			for i, manifest := range orig {
				if manifest != manifests[i] {
					t.Fatal("Expected input to sortManifestsByKind to stay the same")
				}
			}
		})
	}
}

// TestKindSorterKeepOriginalOrder verifies manifests of same kind are kept in original order
func TestKindSorterKeepOriginalOrder(t *testing.T) {
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
		// expectation is sorted by kind (unknown is last) and within each group of same kind, the order is kept
		{"cm,clusterRole,clusterRoleBinding,Unknown,Unknown2", InstallOrder, "01aAz!u2u1t3"},
	} {
		var buf bytes.Buffer
		t.Run(test.description, func(t *testing.T) {
			defer buf.Reset()
			for _, r := range sortManifestsByKind(manifests, test.order) {
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
		Head: &SimpleHead{Kind: "Unknown"},
	}
	namespace := Manifest{
		Name: "b",
		Head: &SimpleHead{Kind: "Namespace"},
	}

	manifests := []Manifest{unknown, namespace}
	manifests = sortManifestsByKind(manifests, InstallOrder)

	expectedOrder := []Manifest{namespace, unknown}
	for i, manifest := range manifests {
		if expectedOrder[i].Name != manifest.Name {
			t.Errorf("Expected %s, got %s", expectedOrder[i].Name, manifest.Name)
		}
	}
}

// test hook sorting with a small subset of kinds, since it uses the same algorithm as sortManifestsByKind
func TestKindSorterForHooks(t *testing.T) {
	hooks := []*release.Hook{
		{
			Name: "i",
			Kind: "ClusterRole",
		},
		{
			Name: "j",
			Kind: "ClusterRoleBinding",
		},
		{
			Name: "c",
			Kind: "LimitRange",
		},
		{
			Name: "a",
			Kind: "Namespace",
		},
	}

	for _, test := range []struct {
		description string
		order       KindSortOrder
		expected    string
	}{
		{"install", InstallOrder, "acij"},
		{"uninstall", UninstallOrder, "jica"},
	} {
		var buf bytes.Buffer
		t.Run(test.description, func(t *testing.T) {
			if got, want := len(test.expected), len(hooks); got != want {
				t.Fatalf("Expected %d names in order, got %d", want, got)
			}
			defer buf.Reset()
			orig := hooks
			for _, r := range sortHooksByKind(hooks, test.order) {
				buf.WriteString(r.Name)
			}
			for i, hook := range orig {
				if hook != hooks[i] {
					t.Fatal("Expected input to sortHooksByKind to stay the same")
				}
			}
			if got := buf.String(); got != test.expected {
				t.Errorf("Expected %q, got %q", test.expected, got)
			}
		})
	}
}

func sortManifestsByKindForCRsUsingAlphabeticalOrder(manifests []Manifest, ordering KindSortOrder) []Manifest {
	sort.SliceStable(manifests, func(i, j int) bool {
		return lessByKindForCRsUsingAlphabeticalOrder(manifests[i], manifests[j], manifests[i].Head.Kind, manifests[j].Head.Kind, ordering)
	})

	return manifests
}

func lessByKindForCRsUsingAlphabeticalOrder(a interface{}, b interface{}, kindA string, kindB string, o KindSortOrder) bool {
	ordering := make(map[string]int, len(o))
	for v, k := range o {
		ordering[k] = v
	}

	first, aok := ordering[kindA]
	second, bok := ordering[kindB]

	if !aok && !bok {
		// if both are unknown then sort alphabetically by kind, keep original order if same kind
		if kindA != kindB {
			return kindA < kindB
		}
		return first < second
	}
	// unknown kind is last
	if !aok {
		return false
	}
	if !bok {
		return true
	}
	// sort different kinds, keep original order if same priority
	return first < second
}
