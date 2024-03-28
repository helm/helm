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

// ResourceDependencyOrder is the order in which manifests should be installed (by Kind).
//
// Those occurring earlier in the list get installed before those occurring later in the list.
var ResourceDependencyOrder KindSortOrder = []string{
	"PriorityClass",
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
	"ClusterRoleBinding",
	"Role",
	"RoleBinding",
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

// addListKind adds the List equivalent for each resource kind in the list.
func addListKind(resources KindSortOrder) KindSortOrder {
	output := make(KindSortOrder, 2*len(resources))
	for _, resource := range resources {
		output = append(output, resource)
		output = append(output, resource+"List")
	}
	return output
}

// reverseKindOrder reverses the order of the KindSortOrder.
func reverseKindOrder(resources KindSortOrder) KindSortOrder {
	for i := 0; i < len(resources)/2; i++ {
		j := len(resources) - i - 1
		resources[i], resources[j] = resources[j], resources[i]
	}
	return resources
}

// InstallOrder is the order in which manifests should be installed (by Kind).
//
// Those occurring earlier in the list get installed before those occurring later in the list.
var InstallOrder = addListKind(ResourceDependencyOrder)

// UninstallOrder is the order in which manifests should be uninstalled (by Kind).
//
// Those occurring earlier in the list get uninstalled before those occurring later in the list.
var UninstallOrder = reverseKindOrder(addListKind(ResourceDependencyOrder))

// sort manifests by kind.
//
// Results are sorted by 'ordering', keeping order of items with equal kind/priority
func sortManifestsByKind(manifests []Manifest, ordering KindSortOrder) []Manifest {
	sort.SliceStable(manifests, func(i, j int) bool {
		return lessByKind(manifests[i], manifests[j], manifests[i].Head.Kind, manifests[j].Head.Kind, ordering)
	})

	return manifests
}

// sort hooks by kind, using an out-of-place sort to preserve the input parameters.
//
// Results are sorted by 'ordering', keeping order of items with equal kind/priority
func sortHooksByKind(hooks []*release.Hook, ordering KindSortOrder) []*release.Hook {
	h := hooks
	sort.SliceStable(h, func(i, j int) bool {
		return lessByKind(h[i], h[j], h[i].Kind, h[j].Kind, ordering)
	})

	return h
}

func lessByKind(_ interface{}, _ interface{}, kindA string, kindB string, o KindSortOrder) bool {
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
