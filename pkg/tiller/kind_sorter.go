/*
Copyright 2016 The Kubernetes Authors All rights reserved.

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
	"sort"
)

// SortOrder is an ordering of Kinds.
type SortOrder []string

// InstallOrder is the order in which manifests should be installed (by Kind)
var InstallOrder SortOrder = []string{"Namespace", "Secret", "ConfigMap", "PersistentVolume", "PersistentVolumeClaim", "ServiceAccount", "Service", "Pod", "ReplicationController", "Deployment", "DaemonSet", "Ingress", "Job"}

// UninstallOrder is the order in which manifests should be uninstalled (by Kind)
var UninstallOrder SortOrder = []string{"Service", "Pod", "ReplicationController", "Deployment", "DaemonSet", "ConfigMap", "Secret", "PersistentVolumeClaim", "PersistentVolume", "ServiceAccount", "Ingress", "Job", "Namespace"}

// sortByKind does an in-place sort of manifests by Kind.
//
// Results are sorted by 'ordering'
func sortByKind(manifests []manifest, ordering SortOrder) []manifest {
	ks := newKindSorter(manifests, ordering)
	sort.Sort(ks)
	return ks.manifests
}

type kindSorter struct {
	ordering  map[string]int
	manifests []manifest
}

func newKindSorter(m []manifest, s SortOrder) *kindSorter {
	o := make(map[string]int, len(s))
	for v, k := range s {
		o[k] = v
	}

	return &kindSorter{
		manifests: m,
		ordering:  o,
	}
}

func (k *kindSorter) Len() int { return len(k.manifests) }

func (k *kindSorter) Swap(i, j int) { k.manifests[i], k.manifests[j] = k.manifests[j], k.manifests[i] }

func (k *kindSorter) Less(i, j int) bool {
	a := k.manifests[i]
	b := k.manifests[j]
	first, ok := k.ordering[a.head.Kind]
	if !ok {
		// Unknown is always last
		return false
	}
	second, ok := k.ordering[b.head.Kind]
	if !ok {
		return true
	}
	return first < second
}
