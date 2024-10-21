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

// SortHooksToDelete sorts a list of hooks to be deleted.
func SortHooksToDelete(hooks []*release.Hook) []*release.Hook {
	h := hooks

	// sort by weight, then by UninstallOrder
	sort.SliceStable(
		h, func(i, j int) bool {
			if h[i].Weight == h[j].Weight {
				return lessByKind(h[i], h[j], h[i].Kind, h[j].Kind, UninstallOrder)
			}
			return h[i].Weight > h[j].Weight
		},
	)

	return h
}
