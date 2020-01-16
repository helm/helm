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
	"sort"

	"helm.sh/helm/v3/pkg/release"
)

// KindSortOrder is an ordering of Kinds.
type KindSortOrder []string

// InstallOrder is the order in which manifests should be installed (by Kind).
//
// Those occurring earlier in the list get installed before those occurring later in the list.
var InstallOrder KindSortOrder = []string{
	"Namespace",
	"NetworkPolicy",
	"ResourceQuota",
	"LimitRange",
	"PodSecurityPolicy",
	"PodDisruptionBudget",
	"ServiceAccount",
	"Secret",
	"ConfigMap",
	"StorageClass",
	"PersistentVolume",
	"PersistentVolumeClaim",
	"CustomResourceDefinition",
	"ClusterRole",
	"ClusterRoleList",
	"ClusterRoleBinding",
	"ClusterRoleBindingList",
	"Role",
	"RoleList",
	"RoleBinding",
	"RoleBindingList",
	"Service",
	"DaemonSet",
	"Pod",
	"ReplicationController",
	"ReplicaSet",
	"Deployment",
	"HorizontalPodAutoscaler",
	"StatefulSet",
	"Job",
	"CronJob",
	"Ingress",
	"APIService",
}

// UninstallOrder is the order in which manifests should be uninstalled (by Kind).
//
// Those occurring earlier in the list get uninstalled before those occurring later in the list.
var UninstallOrder KindSortOrder = []string{
	"APIService",
	"Ingress",
	"Service",
	"CronJob",
	"Job",
	"StatefulSet",
	"HorizontalPodAutoscaler",
	"Deployment",
	"ReplicaSet",
	"ReplicationController",
	"Pod",
	"DaemonSet",
	"RoleBindingList",
	"RoleBinding",
	"RoleList",
	"Role",
	"ClusterRoleBindingList",
	"ClusterRoleBinding",
	"ClusterRoleList",
	"ClusterRole",
	"CustomResourceDefinition",
	"PersistentVolumeClaim",
	"PersistentVolume",
	"StorageClass",
	"ConfigMap",
	"Secret",
	"ServiceAccount",
	"PodDisruptionBudget",
	"PodSecurityPolicy",
	"LimitRange",
	"ResourceQuota",
	"NetworkPolicy",
	"Namespace",
}

// sort manifests by kind (out of place sort)
//
// Results are sorted by 'ordering', keeping order of items with equal kind/priority
func manifestsSortedByKind(manifests []Manifest, ordering KindSortOrder) []Manifest {
	k := make([]string, len(manifests))
	for i, h := range manifests {
		k[i] = h.Head.Kind
	}
	ks := newKindSorter(k, ordering)
	sort.Stable(ks)

	// apply permutation
	sorted := make([]Manifest, len(manifests))
	for i, p := range ks.permutation {
		sorted[i] = manifests[p]
	}
	return sorted
}

// sort hooks by kind (out of place sort)
//
// Results are sorted by 'ordering', keeping order of items with equal kind/priority
func hooksSortedByKind(hooks []*release.Hook, ordering KindSortOrder) []*release.Hook {
	k := make([]string, len(hooks))
	for i, h := range hooks {
		k[i] = h.Kind
	}

	ks := newKindSorter(k, ordering)
	sort.Stable(ks)

	// apply permutation
	sorted := make([]*release.Hook, len(hooks))
	for i, p := range ks.permutation {
		sorted[i] = hooks[p]
	}
	return sorted
}

type kindSorter struct {
	permutation []int
	ordering    map[string]int
	kinds       []string
}

func newKindSorter(kinds []string, s KindSortOrder) *kindSorter {
	o := make(map[string]int, len(s))
	for v, k := range s {
		o[k] = v
	}

	p := make([]int, len(kinds))
	for i := range p {
		p[i] = i
	}
	return &kindSorter{
		permutation: p,
		kinds:       kinds,
		ordering:    o,
	}
}

func (k *kindSorter) Len() int { return len(k.kinds) }

func (k *kindSorter) Swap(i, j int) {
	k.permutation[i], k.permutation[j] = k.permutation[j], k.permutation[i]
}

func (k *kindSorter) Less(i, j int) bool {
	a := k.kinds[k.permutation[i]]
	b := k.kinds[k.permutation[j]]
	first, aok := k.ordering[a]
	second, bok := k.ordering[b]

	if !aok && !bok {
		// if both are unknown then sort alphabetically by kind, keep original order if same kind
		if a != b {
			return a < b
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
