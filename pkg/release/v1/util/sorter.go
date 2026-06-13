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

package util // import "helm.sh/helm/v4/pkg/release/v1/util"

import (
	"sort"

	rspb "helm.sh/helm/v4/pkg/release/v1"
)

// Reverse reverses the list of releases sorted by the sort func.
func Reverse(list []*rspb.Release, sortFn func([]*rspb.Release)) {
	sortFn(list)
	for i, j := 0, len(list)-1; i < j; i, j = i+1, j-1 {
		list[i], list[j] = list[j], list[i]
	}
}

// SortByName returns the list of releases sorted
// in lexicographical order.
func SortByName(list []*rspb.Release) {
	sort.Slice(list, func(i, j int) bool {
		return list[i].Name < list[j].Name
	})
}

// SortByDate returns the list of releases sorted by a
// release's last deployed time (in seconds).
func SortByDate(list []*rspb.Release) {
	sort.Slice(list, func(i, j int) bool {
		ti := list[i].Info.LastDeployed.Unix()
		tj := list[j].Info.LastDeployed.Unix()
		if ti != tj {
			return ti < tj
		}
		// Use name as tie-breaker for stable sorting
		return list[i].Name < list[j].Name
	})
}

// SortByRevision returns the list of releases sorted by a
// release's revision number (release.Version).
func SortByRevision(list []*rspb.Release) {
	sort.Slice(list, func(i, j int) bool {
		return list[i].Version < list[j].Version
	})
}
