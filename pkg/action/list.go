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
	"path"
	"regexp"

	"k8s.io/apimachinery/pkg/labels"

	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/releaseutil"
)

// ListStates represents zero or more status codes that a list item may have set
//
// Because this is used as a bitmask filter, more than one bit can be flipped
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
	// ListPendingRollback filters on status "pending_rollback" (rollback in progress)
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
	// ByNameDesc sorts by descending lexicographic order
	ByNameDesc Sorter = iota + 1
	// ByDateAsc sorts by ascending dates (oldest updated release first)
	ByDateAsc
	// ByDateDesc sorts by descending dates (latest updated release first)
	ByDateDesc
)

// List is the action for listing releases.
//
// It provides, for example, the implementation of 'helm list'.
// It returns no more than one revision of every release in one specific, or in
// all, namespaces.
// To list all the revisions of a specific release, see the History action.
type List struct {
	cfg *Configuration

	// All ignores the limit/offset
	All bool
	// AllNamespaces searches across namespaces
	AllNamespaces bool
	// Sort indicates the sort to use
	//
	// see pkg/releaseutil for several useful sorters
	Sort Sorter
	// Overrides the default lexicographic sorting
	ByDate      bool
	SortReverse bool
	// StateMask accepts a bitmask of states for items to show.
	// The default is ListDeployed
	StateMask ListStates
	// Limit is the number of items to return per Run()
	Limit int
	// Offset is the starting index for the Run() call
	Offset int
	// Filter is a filter that is applied to the results
	Filter       string
	Short        bool
	NoHeaders    bool
	TimeFormat   string
	Uninstalled  bool
	Superseded   bool
	Uninstalling bool
	Deployed     bool
	Failed       bool
	Pending      bool
	Selector     string
}

// NewList constructs a new *List
func NewList(cfg *Configuration) *List {
	return &List{
		StateMask: ListDeployed | ListFailed,
		cfg:       cfg,
	}
}

// Run executes the list command, returning a set of matches.
func (l *List) Run() ([]*release.Release, error) {
	if err := l.cfg.KubeClient.IsReachable(); err != nil {
		return nil, err
	}

	var filter *regexp.Regexp
	if l.Filter != "" {
		var err error
		filter, err = regexp.Compile(l.Filter)
		if err != nil {
			return nil, err
		}
	}

	results, err := l.cfg.Releases.List(func(rel *release.Release) bool {
		// Skip anything that doesn't match the filter.
		if filter != nil && !filter.MatchString(rel.Name) {
			return false
		}

		return true
	})

	if err != nil {
		return nil, err
	}

	if results == nil {
		return results, nil
	}

	// by definition, superseded releases are never shown if
	// only the latest releases are returned. so if requested statemask
	// is _only_ ListSuperseded, skip the latest release filter
	if l.StateMask != ListSuperseded {
		results = filterLatestReleases(results)
	}

	// State mask application must occur after filtering to
	// latest releases, otherwise outdated entries can be returned
	results = l.filterStateMask(results)

	// Skip anything that doesn't match the selector
	selectorObj, err := labels.Parse(l.Selector)
	if err != nil {
		return nil, err
	}
	results = l.filterSelector(results, selectorObj)

	// Unfortunately, we have to sort before truncating, which can incur substantial overhead
	l.sort(results)

	// Guard on offset
	if l.Offset >= len(results) {
		return []*release.Release{}, nil
	}

	// Calculate the limit and offset, and then truncate results if necessary.
	limit := len(results)
	if l.Limit > 0 && l.Limit < limit {
		limit = l.Limit
	}
	last := l.Offset + limit
	if l := len(results); l < last {
		last = l
	}
	results = results[l.Offset:last]

	return results, err
}

// sort is an in-place sort where order is based on the value of a.Sort
func (l *List) sort(rels []*release.Release) {
	if l.SortReverse {
		l.Sort = ByNameDesc
	}

	if l.ByDate {
		l.Sort = ByDateDesc
		if l.SortReverse {
			l.Sort = ByDateAsc
		}
	}

	switch l.Sort {
	case ByDateDesc:
		releaseutil.SortByDate(rels)
	case ByDateAsc:
		releaseutil.Reverse(rels, releaseutil.SortByDate)
	case ByNameDesc:
		releaseutil.Reverse(rels, releaseutil.SortByName)
	default:
		releaseutil.SortByName(rels)
	}
}

// filterLatestReleases returns a list scrubbed of old releases.
func filterLatestReleases(releases []*release.Release) []*release.Release {
	latestReleases := make(map[string]*release.Release)

	for _, rls := range releases {
		name, namespace := rls.Name, rls.Namespace
		key := path.Join(namespace, name)
		if latestRelease, exists := latestReleases[key]; exists && latestRelease.Version > rls.Version {
			continue
		}
		latestReleases[key] = rls
	}

	var list = make([]*release.Release, 0, len(latestReleases))
	for _, rls := range latestReleases {
		list = append(list, rls)
	}
	return list
}

func (l *List) filterStateMask(releases []*release.Release) []*release.Release {
	desiredStateReleases := make([]*release.Release, 0)

	for _, rls := range releases {
		currentStatus := l.StateMask.FromName(rls.Info.Status.String())
		mask := l.StateMask & currentStatus
		if mask == 0 {
			continue
		}
		desiredStateReleases = append(desiredStateReleases, rls)
	}

	return desiredStateReleases
}

func (l *List) filterSelector(releases []*release.Release, selector labels.Selector) []*release.Release {
	desiredStateReleases := make([]*release.Release, 0)

	for _, rls := range releases {
		if selector.Matches(labels.Set(rls.Labels)) {
			desiredStateReleases = append(desiredStateReleases, rls)
		}
	}

	return desiredStateReleases
}

// SetStateMask calculates the state mask based on parameters.
func (l *List) SetStateMask() {
	if l.All {
		l.StateMask = ListAll
		return
	}

	state := ListStates(0)
	if l.Deployed {
		state |= ListDeployed
	}
	if l.Uninstalled {
		state |= ListUninstalled
	}
	if l.Uninstalling {
		state |= ListUninstalling
	}
	if l.Pending {
		state |= ListPendingInstall | ListPendingRollback | ListPendingUpgrade
	}
	if l.Failed {
		state |= ListFailed
	}
	if l.Superseded {
		state |= ListSuperseded
	}

	// Apply a default
	if state == 0 {
		state = ListDeployed | ListFailed
	}

	l.StateMask = state
}
