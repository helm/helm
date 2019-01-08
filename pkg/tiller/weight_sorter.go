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
	"sort"
)

// sortByWeight does an in-place sort of manifests by Kind.
//
// Results are sorted by weight
func sortByWeight(manifests []Manifest, st SortType) []Manifest {
	ws := newWeightSorter(manifests, st)
	sort.Stable(ws)
	return ws.manifests
}

type weightSorter struct {
	manifests []Manifest
	stype     SortType
}

func newWeightSorter(m []Manifest, t SortType) *weightSorter {
	return &weightSorter{
		manifests: m,
		stype:     t,
	}
}

func (w *weightSorter) Len() int { return len(w.manifests) }

func (w *weightSorter) Swap(i, j int) { w.manifests[i], w.manifests[j] = w.manifests[j], w.manifests[i] }

func (w *weightSorter) Less(i, j int) bool {
	a := w.manifests[i]
	b := w.manifests[j]

	if w.stype == SortInstall {
		return a.Weight.GreaterThan(b.Weight)
	}
	return a.Weight.LessThan(b.Weight)
}
