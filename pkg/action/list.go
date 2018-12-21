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

package action

import (
	"regexp"

	"k8s.io/helm/pkg/hapi/release"
	"k8s.io/helm/pkg/releaseutil"
)

// ListStates represents zero or more status codes that a list item may have set
//
// Because this is used as a bitmask filter, more than one one bit can be flipped
// in the ListStates.
type ListStates uint

const (
	// ListDeployed filters on status "deployed"
	ListDeployed ListStates = 1 << iota
	// ListUninstalled filters on status "uninstalled"
	ListUninstalled
	// ListUninstalling filters on status "uninstalling" (uninstall in progress)
	ListUninstalling
	// ListPendingInstall filters on status "pending" (deployment in progress)
	ListPendingInstall
	// ListPendingUpgrade filters on status "pending_upgrade" (upgrade in progress)
	ListPendingUpgrade
	// ListPendingRollback filters on status "pending_rollback" (rollback in progres)
	ListPendingRollback
	// ListSuperseded filters on status "superseded" (historical release version that is no longer deployed)
	ListSuperseded
	// ListFailed filters on status "failed" (release version not deployed because of error)
	ListFailed
	// ListUnknown filters on an unknown status
	ListUnknown
)

// FromName takes a state name and returns a ListStates representation.
//
// Currently, there are only names for individual flipped bits, so the returned
// ListStates will only match one of the constants. However, it is possible that
// this behavior could change in the future.
func (s ListStates) FromName(str string) ListStates {
	switch str {
	case "deployed":
		return ListDeployed
	case "uninstalled":
		return ListUninstalled
	case "superseded":
		return ListSuperseded
	case "failed":
		return ListFailed
	case "uninstalling":
		return ListUninstalling
	case "pending-install":
		return ListPendingInstall
	case "pending-upgrade":
		return ListPendingUpgrade
	case "pending-rollback":
		return ListPendingRollback
	}
	return ListUnknown
}

// ListAll is a convenience for enabling all list filters
const ListAll = ListDeployed | ListUninstalled | ListUninstalling | ListPendingInstall | ListPendingRollback | ListPendingUpgrade | ListSuperseded | ListFailed

// Sorter is a top-level sort
type Sorter uint

const (
	// ByDate sorts by date
	ByDate Sorter = iota
	// ByNameAsc sorts by ascending lexicographic order
	ByNameAsc
	// ByNameDesc sorts by descending lexicographic order
	ByNameDesc
)

// List is the action for listing releases.
//
// It provides, for example, the implementation of 'helm list'.
type List struct {
	// All ignores the limit/offset
	All bool
	// AllNamespaces searches across namespaces
	AllNamespaces bool
	// Sort indicates the sort to use
	//
	// see pkg/releaseutil for several useful sorters
	Sort Sorter
	// StateMask accepts a bitmask of states for items to show.
	// The default is ListDeployed
	StateMask ListStates
	// Limit is the number of items to return per Run()
	Limit int
	// Offset is the starting index for the Run() call
	Offset int
	// Filter is a filter that is applied to the results
	Filter string

	cfg *Configuration
}

// NewList constructs a new *List
func NewList(cfg *Configuration) *List {
	return &List{
		StateMask: ListDeployed | ListFailed,
		cfg:       cfg,
	}
}

// Run executes the list command, returning a set of matches.
func (a *List) Run() ([]*release.Release, error) {
	var filter *regexp.Regexp
	if a.Filter != "" {
		var err error
		filter, err = regexp.Compile(a.Filter)
		if err != nil {
			return nil, err
		}
	}

	results, err := a.cfg.Releases.List(func(rel *release.Release) bool {
		// Skip anything that the mask doesn't cover
		currentStatus := a.StateMask.FromName(rel.Info.Status.String())
		if a.StateMask&currentStatus == 0 {
			return false
		}

		// Skip anything that doesn't match the filter.
		if filter != nil && !filter.MatchString(rel.Name) {
			return false
		}
		return true
	})

	if results == nil {
		return results, nil
	}

	// Unfortunately, we have to sort before truncating, which can incur substantial overhead
	a.sort(results)

	// Guard on offset
	if a.Offset >= len(results) {
		return []*release.Release{}, nil
	}

	// Calculate the limit and offset, and then truncate results if necessary.
	limit := len(results)
	if a.Limit > 0 && a.Limit < limit {
		limit = a.Limit
	}
	last := a.Offset + limit
	if l := len(results); l < last {
		last = l
	}
	results = results[a.Offset:last]

	return results, err
}

// sort is an in-place sort where order is based on the value of a.Sort
func (a *List) sort(rels []*release.Release) {
	switch a.Sort {
	case ByDate:
		releaseutil.SortByDate(rels)
	case ByNameDesc:
		releaseutil.Reverse(rels, releaseutil.SortByName)
	default:
		releaseutil.SortByName(rels)
	}
}
