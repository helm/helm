package action

import (
	"errors"
	"regexp"
	"sort"

	"k8s.io/helm/pkg/hapi/release"
	"k8s.io/helm/pkg/releaseutil"
)

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

type ByDate struct{}
type ByNameAsc struct{}
type ByNameDesc struct{}

// ReleaseSorter is a sorter for releases
type ReleaseSorter interface {
	SetList([]*release.Release)
	Len() int
	Less(i, j int) bool
	Swap(i, j int)
}

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
	Sort ReleaseSorter
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
	}
}

func (a *List) Run() ([]*release.Release, error) {
	return []*release.Release{}, errors.New("not implemented")
}

func (a *List) listReleases() ([]*release.Release, error) {
	rels, err := a.cfg.Releases.ListReleases()
	if err != nil {
		return rels, err
	}

	// TODO: add the rest of the filters here

	// Run filter
	rels, err = a.filterReleases(rels)
	if err != nil {
		return rels, err
	}

	// Run sort
	rels = a.sort(rels)

	return rels, nil
}

func (a *List) filterReleases(rels []*release.Release) ([]*release.Release, error) {
	if a.Filter == "" {
		return rels, nil
	}
	preg, err := regexp.Compile(a.Filter)
	if err != nil {
		return rels, err
	}
	matches := []*release.Release{}
	for _, r := range rels {
		if preg.MatchString(r.Name) {
			matches = append(matches, r)
		}
	}
	return matches, nil
}

func (a *List) sort(rels []*release.Release) []*release.Release {
	if a.Sort == nil {
		releaseutil.SortByName(rels)
		return rels
	}
	a.Sort.SetList(rels)
	sort.Sort(a.Sort)
	return rels
}
