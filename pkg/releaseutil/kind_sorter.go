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
func sortManifestsByKind(manifests []Manifest, uninstall bool) []Manifest {
	m := manifests
	sort.SliceStable(m, func(i, j int) bool {
		return lessByKind(m[i], m[j], m[i].Head.Kind, m[j].Head.Kind, m[i].InstallBefore, m[j].InstallBefore, uninstall)
	})

	return m
}

// sort hooks by kind, using an out-of-place sort to preserve the input parameters.
//
// Results are sorted by 'ordering', keeping order of items with equal kind/priority
func sortHooksByKind(hooks []*release.Hook, uninstall bool) []*release.Hook {
	h := hooks
	sort.SliceStable(h, func(i, j int) bool {
		return lessByKind(h[i], h[j], h[i].Kind, h[j].Kind, []string{}, []string{}, uninstall)
	})

	return h
}

func lessByKind(a interface{}, b interface{}, kindA string, kindB string, beforeA []string, beforeB []string, uninstall bool) bool {
	first, aok := installOrderIndex(kindA, beforeA, uninstall)
	second, bok := installOrderIndex(kindB, beforeB, uninstall)

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

// installOrderIndex returns the lowest index number of all beforeKinds
func installOrderIndex(kind string, beforeKinds []string, uninstall bool) (int, bool) {
	order := InstallOrder
	if uninstall {
		order = UninstallOrder
	}

	ordering := make(map[string]int, len(order))
	for v, k := range order {
		// In order to allow placing custom resources in between existing resources we need to double the index.
		// for example
		// NetworkPolicy has an index of 1
		// ResourceQuota has an index of 2
		// if we want to place a custom resource in between we would need to make the index of that resource 1.5
		// since we use int numbers we cannot use floating point numbers, so instead we just DOUBLE the index of
		// everything so that our Custom Resource fits in between (2 and 4 in this case)
		ordering[k] = v * 2
	}

	orderIndex, foundIndex := ordering[kind]

	// reset orderIndex for unknown resources
	// when we're uninstalling we're actually searching for the HIGHEST index, so 0 is fine as initial value
	if !foundIndex && !uninstall {
		// see above why we use double the length
		orderIndex = len(order) * 2
	}

	for _, kind := range beforeKinds {
		i, ok := ordering[kind]
		if !ok {
			continue
		}
		// we're searching for the lowest index when installing
		if i < orderIndex && !uninstall {
			foundIndex = true
			// set orderIndex 1 BEFORE the actual index, so it get installed BEFORE it
			orderIndex = i - 1
		}
		// we're searching for the highest index when uninstalling
		if i > orderIndex && uninstall {
			foundIndex = true
			// set orderIndex 1 AFTER the actual index, so it get uninstalled AFTER it
			orderIndex = i + 1
		}
	}

	return orderIndex, foundIndex
}
