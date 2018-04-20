package hapi

import (
	"k8s.io/helm/pkg/hapi/chart"
	"k8s.io/helm/pkg/hapi/release"
)

// SortBy defines sort operations.
type ListSortBy int

const (
	ListSort_UNKNOWN ListSortBy = iota
	ListSort_NAME
	ListSort_LAST_RELEASED
)

var sortByNames = [...]string{
	"UNKNOWN",
	"NAME",
	"LAST_RELEASED",
}

func (x ListSortBy) String() string { return sortByNames[x] }

// SortOrder defines sort orders to augment sorting operations.
type ListSortOrder int

const (
	ListSort_ASC ListSortOrder = iota
	ListSort_DESC
)

var sortOrderNames = [...]string{
	"ASC",
	"DESC",
}

func (x ListSortOrder) String() string { return sortOrderNames[x] }

// ListReleasesRequest requests a list of releases.
//
// Releases can be retrieved in chunks by setting limit and offset.
//
// Releases can be sorted according to a few pre-determined sort stategies.
type ListReleasesRequest struct {
	// Limit is the maximum number of releases to be returned.
	Limit int64 `json:"limit,omityempty"`
	// Offset is the last release name that was seen. The next listing
	// operation will start with the name after this one.
	// Example: If list one returns albert, bernie, carl, and sets 'next: dennis'.
	// dennis is the offset. Supplying 'dennis' for the next request should
	// cause the next batch to return a set of results starting with 'dennis'.
	Offset string `json:"offset,omityempty"`
	// SortBy is the sort field that the ListReleases server should sort data before returning.
	SortBy ListSortBy `json:"sort_by,omityempty"`
	// Filter is a regular expression used to filter which releases should be listed.
	//
	// Anything that matches the regexp will be included in the results.
	Filter string `json:"filter,omityempty"`
	// SortOrder is the ordering directive used for sorting.
	SortOrder   ListSortOrder        `json:"sort_order,omityempty"`
	StatusCodes []release.StatusCode `json:"status_codes,omityempty"`
	// Namespace is the filter to select releases only from a specific namespace.
	Namespace string `json:"namespace,omityempty"`
}

// ListReleasesResponse is a list of releases.
type ListReleasesResponse struct {
	// Count is the expected total number of releases to be returned.
	Count int64 `json:"count,omityempty"`
	// Next is the name of the next release. If this is other than an empty
	// string, it means there are more results.
	Next string `json:"next,omityempty"`
	// Total is the total number of queryable releases.
	Total int64 `json:"total,omityempty"`
	// Releases is the list of found release objects.
	Releases []*release.Release `json:"releases,omityempty"`
}

// GetReleaseStatusRequest is a request to get the status of a release.
type GetReleaseStatusRequest struct {
	// Name is the name of the release
	Name string `json:"name,omitempty"`
	// Version is the version of the release
	Version int `json:"version,omitempty"`
}

// GetReleaseStatusResponse is the response indicating the status of the named release.
type GetReleaseStatusResponse struct {
	// Name is the name of the release.
	Name string `json:"name,omitempty"`
	// Info contains information about the release.
	Info *release.Info `json:"info,omitempty"`
	// Namespace the release was released into
	Namespace string `json:"namespace,omitempty"`
}

// GetReleaseContentRequest is a request to get the contents of a release.
type GetReleaseContentRequest struct {
	// The name of the release
	Name string `json:"name,omityempty"`
	// Version is the version of the release
	Version int `json:"version,omityempty"`
}

// UpdateReleaseRequest updates a release.
type UpdateReleaseRequest struct {
	// The name of the release
	Name string `json:"name,omityempty"`
	// Chart is the protobuf representation of a chart.
	Chart *chart.Chart `json:"chart,omityempty"`
	// Values is a string containing (unparsed) YAML values.
	Values []byte `json:"values,omityempty"`
	// dry_run, if true, will run through the release logic, but neither create
	DryRun bool `json:"dry_run,omityempty"`
	// DisableHooks causes the server to skip running any hooks for the upgrade.
	DisableHooks bool `json:"disable_hooks,omityempty"`
	// Performs pods restart for resources if applicable
	Recreate bool `json:"recreate,omityempty"`
	// timeout specifies the max amount of time any kubernetes client command can run.
	Timeout int64 `json:"timeout,omityempty"`
	// ResetValues will cause Tiller to ignore stored values, resetting to default values.
	ResetValues bool `json:"reset_values,omityempty"`
	// wait, if true, will wait until all Pods, PVCs, and Services are in a ready state
	// before marking the release as successful. It will wait for as long as timeout
	Wait bool `json:"wait,omityempty"`
	// ReuseValues will cause Tiller to reuse the values from the last release.
	// This is ignored if reset_values is set.
	ReuseValues bool `json:"reuse_values,omityempty"`
	// Force resource update through delete/recreate if needed.
	Force bool `json:"force,omityempty"`
}

type RollbackReleaseRequest struct {
	// The name of the release
	Name string `json:"name,omityempty"`
	// dry_run, if true, will run through the release logic but no create
	DryRun bool `json:"dry_run,omityempty"`
	// DisableHooks causes the server to skip running any hooks for the rollback
	DisableHooks bool `json:"disable_hooks,omityempty"`
	// Version is the version of the release to deploy.
	Version int `json:"version,omityempty"`
	// Performs pods restart for resources if applicable
	Recreate bool `json:"recreate,omityempty"`
	// timeout specifies the max amount of time any kubernetes client command can run.
	Timeout int64 `json:"timeout,omityempty"`
	// wait, if true, will wait until all Pods, PVCs, and Services are in a ready state
	// before marking the release as successful. It will wait for as long as timeout
	Wait bool `json:"wait,omityempty"`
	// Force resource update through delete/recreate if needed.
	Force bool `json:"force,omityempty"`
}

// InstallReleaseRequest is the request for an installation of a chart.
type InstallReleaseRequest struct {
	// Chart is the protobuf representation of a chart.
	Chart *chart.Chart `json:"chart,omityempty"`
	// Values is a string containing (unparsed) YAML values.
	Values []byte `json:"values,omityempty"`
	// DryRun, if true, will run through the release logic, but neither create
	// a release object nor deploy to Kubernetes. The release object returned
	// in the response will be fake.
	DryRun bool `json:"dry_run,omityempty"`
	// Name is the candidate release name. This must be unique to the
	// namespace, otherwise the server will return an error. If it is not
	// supplied, the server will autogenerate one.
	Name string `json:"name,omityempty"`
	// DisableHooks causes the server to skip running any hooks for the install.
	DisableHooks bool `json:"disable_hooks,omityempty"`
	// Namepace is the kubernetes namespace of the release.
	Namespace string `json:"namespace,omityempty"`
	// ReuseName requests that Tiller re-uses a name, instead of erroring out.
	ReuseName bool `json:"reuse_name,omityempty"`
	// timeout specifies the max amount of time any kubernetes client command can run.
	Timeout int64 `json:"timeout,omityempty"`
	// wait, if true, will wait until all Pods, PVCs, and Services are in a ready state
	// before marking the release as successful. It will wait for as long as timeout
	Wait bool `json:"wait,omityempty"`
}

// UninstallReleaseRequest represents a request to uninstall a named release.
type UninstallReleaseRequest struct {
	// Name is the name of the release to delete.
	Name string `json:"name,omityempty"`
	// DisableHooks causes the server to skip running any hooks for the uninstall.
	DisableHooks bool `json:"disable_hooks,omityempty"`
	// Purge removes the release from the store and make its name free for later use.
	Purge bool `json:"purge,omityempty"`
	// timeout specifies the max amount of time any kubernetes client command can run.
	Timeout int64 `json:"timeout,omityempty"`
}

// UninstallReleaseResponse represents a successful response to an uninstall request.
type UninstallReleaseResponse struct {
	// Release is the release that was marked deleted.
	Release *release.Release `json:"release,omityempty"`
	// Info is an uninstall message
	Info string `json:"info,omityempty"`
}

// GetHistoryRequest requests a release's history.
type GetHistoryRequest struct {
	// The name of the release.
	Name string `json:"name,omityempty"`
	// The maximum number of releases to include.
	Max int `json:"max,omityempty"`
}

// TestReleaseRequest is a request to get the status of a release.
type TestReleaseRequest struct {
	// Name is the name of the release
	Name string `json:"name,omityempty"`
	// timeout specifies the max amount of time any kubernetes client command can run.
	Timeout int64 `json:"timeout,omityempty"`
	// cleanup specifies whether or not to attempt pod deletion after test completes
	Cleanup bool `json:"cleanup,omityempty"`
}

// TestReleaseResponse represents a message from executing a test
type TestReleaseResponse struct {
	Msg    string                `json:"msg,omityempty"`
	Status release.TestRunStatus `json:"status,omityempty"`
}
