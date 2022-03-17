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
	"SecretList",
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
	"IngressClass",
	"Ingress",
	"APIService",
}

// UninstallOrder is the order in which manifests should be uninstalled (by Kind).
//
// Those occurring earlier in the list get uninstalled before those occurring later in the list.
var UninstallOrder KindSortOrder = []string{
	"APIService",
	"Ingress",
	"IngressClass",
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
	"SecretList",
	"Secret",
	"ServiceAccount",
	"PodDisruptionBudget",
	"PodSecurityPolicy",
	"LimitRange",
	"ResourceQuota",
	"NetworkPolicy",
	"Namespace",
}

// sort manifests by kind.
//
// Results are sorted by 'ordering', keeping order of items with equal kind/priority
func sortManifestsByKind(manifests []Manifest, ordering KindSortOrder) []Manifest {
	lessFunc := lessByKind(ordering)
	sort.SliceStable(manifests, func(i, j int) bool {
		return lessFunc(manifests[i].Head.Kind, manifests[j].Head.Kind)
	})

	return manifests
}

// sortHooks sorts hooks by weight, kind, and finally by name.
// Kind order is defined by ordering.
func sortHooks(hooks []*release.Hook, ordering KindSortOrder) []*release.Hook {
	h := hooks

	// Sort first by name, the least important ordering.
	sort.Slice(h, func(i, j int) bool {
		return h[i].Name < h[j].Name
	})

	// Then sort by kind, keeping equal elements in their original order (Stable).
	lessFunc := lessByKind(ordering)
	sort.SliceStable(h, func(i, j int) bool {
		return lessFunc(h[i].Kind, h[j].Kind)
	})

	// Finally, sort by weight, again keeping equal elements in their original order.
	sort.SliceStable(h, func(i, j int) bool {
		return h[i].Weight < h[j].Weight
	})

	return h
}

// lessByKind takes a KindSortOrder and returns a LessFunc that compares two kinds according to said order.
func lessByKind(o KindSortOrder) func(kindA string, kindB string) bool {
	ordering := make(map[string]int, len(o))
	for v, k := range o {
		ordering[k] = v
	}

	return func(kindA string, kindB string) bool {
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
}
