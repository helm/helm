/*
Copyright 2018 The Kubernetes Authors All rights reserved.
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

package hapi

import (
	"k8s.io/helm/pkg/chart"
	"k8s.io/helm/pkg/hapi/release"
)

// SortBy defines sort operations.
type SortBy string

const (
	// SortByName requests releases sorted by name.
	SortByName SortBy = "name"
	// SortByLastReleased requests releases sorted by last released.
	SortByLastReleased SortBy = "last-released"
)

// SortOrder defines sort orders to augment sorting operations.
type SortOrder string

const (
	//SortAsc defines ascending sorting.
	SortAsc SortOrder = "ascending"
	//SortDesc defines descending sorting.
	SortDesc SortOrder = "descending"
)

// ListReleasesRequest requests a list of releases.
//
// Releases can be retrieved in chunks by setting limit and offset.
//
// Releases can be sorted according to a few pre-determined sort stategies.
type ListReleasesRequest struct {
	// Limit is the maximum number of releases to be returned.
	Limit int64 `json:"limit,omitempty"`
	// Offset is the last release name that was seen. The next listing
	// operation will start with the name after this one.
	// Example: If list one returns albert, bernie, carl, and sets 'next: dennis'.
	// dennis is the offset. Supplying 'dennis' for the next request should
	// cause the next batch to return a set of results starting with 'dennis'.
	Offset string `json:"offset,omitempty"`
	// SortBy is the sort field that the ListReleases server should sort data before returning.
	SortBy SortBy `json:"sort_by,omitempty"`
	// Filter is a regular expression used to filter which releases should be listed.
	//
	// Anything that matches the regexp will be included in the results.
	Filter string `json:"filter,omitempty"`
	// SortOrder is the ordering directive used for sorting.
	SortOrder   SortOrder               `json:"sort_order,omitempty"`
	StatusCodes []release.ReleaseStatus `json:"status_codes,omitempty"`
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
	Name string `json:"name,omitempty"`
	// Version is the version of the release
	Version int `json:"version,omitempty"`
}

// UpdateReleaseRequest updates a release.
type UpdateReleaseRequest struct {
	// The name of the release
	Name string `json:"name,omitempty"`
	// Chart is the protobuf representation of a chart.
	Chart *chart.Chart `json:"chart,omitempty"`
	// Values is a string containing (unparsed) YAML values.
	Values []byte `json:"values,omitempty"`
	// dry_run, if true, will run through the release logic, but neither create
	DryRun bool `json:"dry_run,omitempty"`
	// DisableHooks causes the server to skip running any hooks for the upgrade.
	DisableHooks bool `json:"disable_hooks,omitempty"`
	// Performs pods restart for resources if applicable
	Recreate bool `json:"recreate,omitempty"`
	// timeout specifies the max amount of time any kubernetes client command can run.
	Timeout int64 `json:"timeout,omitempty"`
	// ResetValues will cause Tiller to ignore stored values, resetting to default values.
	ResetValues bool `json:"reset_values,omitempty"`
	// wait, if true, will wait until all Pods, PVCs, and Services are in a ready state
	// before marking the release as successful. It will wait for as long as timeout
	Wait bool `json:"wait,omitempty"`
	// ReuseValues will cause Tiller to reuse the values from the last release.
	// This is ignored if reset_values is set.
	ReuseValues bool `json:"reuse_values,omitempty"`
	// Force resource update through delete/recreate if needed.
	Force bool `json:"force,omitempty"`
}

// RollbackReleaseRequest is the request for a release to be rolledback to a
// previous version.
type RollbackReleaseRequest struct {
	// The name of the release
	Name string `json:"name,omitempty"`
	// dry_run, if true, will run through the release logic but no create
	DryRun bool `json:"dry_run,omitempty"`
	// DisableHooks causes the server to skip running any hooks for the rollback
	DisableHooks bool `json:"disable_hooks,omitempty"`
	// Version is the version of the release to deploy.
	Version int `json:"version,omitempty"`
	// Performs pods restart for resources if applicable
	Recreate bool `json:"recreate,omitempty"`
	// timeout specifies the max amount of time any kubernetes client command can run.
	Timeout int64 `json:"timeout,omitempty"`
	// wait, if true, will wait until all Pods, PVCs, and Services are in a ready state
	// before marking the release as successful. It will wait for as long as timeout
	Wait bool `json:"wait,omitempty"`
	// Force resource update through delete/recreate if needed.
	Force bool `json:"force,omitempty"`
}

// InstallReleaseRequest is the request for an installation of a chart.
type InstallReleaseRequest struct {
	// Chart is the protobuf representation of a chart.
	Chart *chart.Chart `json:"chart,omitempty"`
	// Values is a string containing (unparsed) YAML values.
	Values []byte `json:"values,omitempty"`
	// DryRun, if true, will run through the release logic, but neither create
	// a release object nor deploy to Kubernetes. The release object returned
	// in the response will be fake.
	DryRun bool `json:"dry_run,omitempty"`
	// Name is the candidate release name. This must be unique to the
	// namespace, otherwise the server will return an error. If it is not
	// supplied, the server will autogenerate one.
	Name string `json:"name,omitempty"`
	// DisableHooks causes the server to skip running any hooks for the install.
	DisableHooks bool `json:"disable_hooks,omitempty"`
	// Namepace is the kubernetes namespace of the release.
	Namespace string `json:"namespace,omitempty"`
	// ReuseName requests that Tiller re-uses a name, instead of erroring out.
	ReuseName bool `json:"reuse_name,omitempty"`
	// timeout specifies the max amount of time any kubernetes client command can run.
	Timeout int64 `json:"timeout,omitempty"`
	// wait, if true, will wait until all Pods, PVCs, and Services are in a ready state
	// before marking the release as successful. It will wait for as long as timeout
	Wait bool `json:"wait,omitempty"`
}

// UninstallReleaseRequest represents a request to uninstall a named release.
type UninstallReleaseRequest struct {
	// Name is the name of the release to delete.
	Name string `json:"name,omitempty"`
	// DisableHooks causes the server to skip running any hooks for the uninstall.
	DisableHooks bool `json:"disable_hooks,omitempty"`
	// Purge removes the release from the store and make its name free for later use.
	Purge bool `json:"purge,omitempty"`
	// timeout specifies the max amount of time any kubernetes client command can run.
	Timeout int64 `json:"timeout,omitempty"`
}

// UninstallReleaseResponse represents a successful response to an uninstall request.
type UninstallReleaseResponse struct {
	// Release is the release that was marked deleted.
	Release *release.Release `json:"release,omitempty"`
	// Info is an uninstall message
	Info string `json:"info,omitempty"`
}

// GetHistoryRequest requests a release's history.
type GetHistoryRequest struct {
	// The name of the release.
	Name string `json:"name,omitempty"`
	// The maximum number of releases to include.
	Max int `json:"max,omitempty"`
}

// TestReleaseRequest is a request to get the status of a release.
type TestReleaseRequest struct {
	// Name is the name of the release
	Name string `json:"name,omitempty"`
	// timeout specifies the max amount of time any kubernetes client command can run.
	Timeout int64 `json:"timeout,omitempty"`
	// cleanup specifies whether or not to attempt pod deletion after test completes
	Cleanup bool `json:"cleanup,omitempty"`
}

// TestReleaseResponse represents a message from executing a test
type TestReleaseResponse struct {
	Msg    string                `json:"msg,omitempty"`
	Status release.TestRunStatus `json:"status,omitempty"`
}
