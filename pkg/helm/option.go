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

package helm

import (
	"golang.org/x/net/context"
	"google.golang.org/grpc/metadata"

	cpb "k8s.io/helm/pkg/proto/hapi/chart"
	"k8s.io/helm/pkg/proto/hapi/release"
	rls "k8s.io/helm/pkg/proto/hapi/services"
	"k8s.io/helm/pkg/version"
)

// Option allows specifying various settings configurable by
// the helm client user for overriding the defaults used when
// issuing rpc's to the Tiller release server.
type Option func(*options)

// options specify optional settings used by the helm client.
type options struct {
	// value of helm home override
	host string
	// if set dry-run helm client calls
	dryRun bool
	// if set, re-use an existing name
	reuseName bool
	// if set, skip running hooks
	disableHooks bool
	// release list options are applied directly to the list releases request
	listReq rls.ListReleasesRequest
	// release install options are applied directly to the install release request
	instReq rls.InstallReleaseRequest
	// release update options are applied directly to the update release request
	updateReq rls.UpdateReleaseRequest
	// release uninstall options are applied directly to the uninstall release request
	uninstallReq rls.UninstallReleaseRequest
	// release get status options are applied directly to the get release status request
	statusReq rls.GetReleaseStatusRequest
	// release get content options are applied directly to the get release content request
	contentReq rls.GetReleaseContentRequest
	// release rollback options are applied directly to the rollback release request
	rollbackReq rls.RollbackReleaseRequest
}

// Host specifies the host address of the Tiller release server, (default = ":44134").
func Host(host string) Option {
	return func(opts *options) {
		opts.host = host
	}
}

// ReleaseListOption allows specifying various settings
// configurable by the helm client user for overriding
// the defaults used when running the `helm list` command.
type ReleaseListOption func(*options)

// ReleaseListOffset specifies the offset into a list of releases.
func ReleaseListOffset(offset string) ReleaseListOption {
	return func(opts *options) {
		opts.listReq.Offset = offset
	}
}

// ReleaseListFilter specifies a filter to apply a list of releases.
func ReleaseListFilter(filter string) ReleaseListOption {
	return func(opts *options) {
		opts.listReq.Filter = filter
	}
}

// ReleaseListLimit set an upper bound on the number of releases returned.
func ReleaseListLimit(limit int) ReleaseListOption {
	return func(opts *options) {
		opts.listReq.Limit = int64(limit)
	}
}

// ReleaseListOrder specifies how to order a list of releases.
func ReleaseListOrder(order int32) ReleaseListOption {
	return func(opts *options) {
		opts.listReq.SortOrder = rls.ListSort_SortOrder(order)
	}
}

// ReleaseListSort specifies how to sort a release list.
func ReleaseListSort(sort int32) ReleaseListOption {
	return func(opts *options) {
		opts.listReq.SortBy = rls.ListSort_SortBy(sort)
	}
}

// ReleaseListStatuses specifies which status codes should be returned.
func ReleaseListStatuses(statuses []release.Status_Code) ReleaseListOption {
	return func(opts *options) {
		if len(statuses) == 0 {
			statuses = []release.Status_Code{release.Status_DEPLOYED}
		}
		opts.listReq.StatusCodes = statuses
	}
}

// InstallOption allows specifying various settings
// configurable by the helm client user for overriding
// the defaults used when running the `helm install` command.
type InstallOption func(*options)

// ValueOverrides specifies a list of values to include when installing.
func ValueOverrides(raw []byte) InstallOption {
	return func(opts *options) {
		opts.instReq.Values = &cpb.Config{Raw: string(raw)}
	}
}

// ReleaseName specifies the name of the release when installing.
func ReleaseName(name string) InstallOption {
	return func(opts *options) {
		opts.instReq.Name = name
	}
}

// UpdateValueOverrides specifies a list of values to include when upgrading
func UpdateValueOverrides(raw []byte) UpdateOption {
	return func(opts *options) {
		opts.updateReq.Values = &cpb.Config{Raw: string(raw)}
	}
}

// DeleteDisableHooks will disable hooks for a deletion operation.
func DeleteDisableHooks(disable bool) DeleteOption {
	return func(opts *options) {
		opts.disableHooks = disable
	}
}

// DeleteDryRun will (if true) execute a deletion as a dry run.
func DeleteDryRun(dry bool) DeleteOption {
	return func(opts *options) {
		opts.dryRun = dry
	}
}

// DeletePurge removes the release from the store and make its name free for later use.
func DeletePurge(purge bool) DeleteOption {
	return func(opts *options) {
		opts.uninstallReq.Purge = purge
	}
}

// InstallDryRun will (if true) execute an installation as a dry run.
func InstallDryRun(dry bool) InstallOption {
	return func(opts *options) {
		opts.dryRun = dry
	}
}

// InstallDisableHooks disables hooks during installation.
func InstallDisableHooks(disable bool) InstallOption {
	return func(opts *options) {
		opts.disableHooks = disable
	}
}

// InstallReuseName will (if true) instruct Tiller to re-use an existing name.
func InstallReuseName(reuse bool) InstallOption {
	return func(opts *options) {
		opts.reuseName = reuse
	}
}

// RollbackDisableHooks will disable hooks for a rollback operation
func RollbackDisableHooks(disable bool) RollbackOption {
	return func(opts *options) {
		opts.disableHooks = disable
	}
}

// RollbackDryRun will (if true) execute a rollback as a dry run.
func RollbackDryRun(dry bool) RollbackOption {
	return func(opts *options) {
		opts.dryRun = dry
	}
}

// RollbackVersion sets the version of the release to deploy.
func RollbackVersion(ver int32) RollbackOption {
	return func(opts *options) {
		opts.rollbackReq.Version = ver
	}
}

// UpgradeDisableHooks will disable hooks for an upgrade operation.
func UpgradeDisableHooks(disable bool) UpdateOption {
	return func(opts *options) {
		opts.disableHooks = disable
	}
}

// UpgradeDryRun will (if true) execute an upgrade as a dry run.
func UpgradeDryRun(dry bool) UpdateOption {
	return func(opts *options) {
		opts.dryRun = dry
	}
}

// ContentOption allows setting optional attributes when
// performing a GetReleaseContent tiller rpc.
type ContentOption func(*options)

// ContentReleaseVersion will instruct Tiller to retrieve the content
// of a paritcular version of a release.
func ContentReleaseVersion(version int32) ContentOption {
	return func(opts *options) {
		opts.contentReq.Version = version
	}
}

// StatusOption allows setting optional attributes when
// performing a GetReleaseStatus tiller rpc.
type StatusOption func(*options)

// StatusReleaseVersion will instruct Tiller to retrieve the status
// of a particular version of a release.
func StatusReleaseVersion(version int32) StatusOption {
	return func(opts *options) {
		opts.statusReq.Version = version
	}
}

// DeleteOption allows setting optional attributes when
// performing a UninstallRelease tiller rpc.
type DeleteOption func(*options)

// VersionOption -- TODO
type VersionOption func(*options)

// UpdateOption allows specifying various settings
// configurable by the helm client user for overriding
// the defaults used when running the `helm upgrade` command.
type UpdateOption func(*options)

// RollbackOption allows specififying various settings configurable
// by the helm client user for overriding the defaults used when
// running the `helm rollback` command.
type RollbackOption func(*options)

// RPC helpers defined on `options` type. Note: These actually execute the
// the corresponding tiller RPC. There is no particular reason why these
// are APIs are hung off `options`, they are internal to pkg/helm to remain
// malleable.

// Executes tiller.ListReleases RPC.
func (o *options) rpcListReleases(rlc rls.ReleaseServiceClient, opts ...ReleaseListOption) (*rls.ListReleasesResponse, error) {
	// apply release list options
	for _, opt := range opts {
		opt(o)
	}
	s, err := rlc.ListReleases(NewContext(), &o.listReq)
	if err != nil {
		return nil, err
	}

	return s.Recv()
}

// NewContext creates a versioned context.
func NewContext() context.Context {
	md := metadata.Pairs("x-helm-api-client", version.Version)
	return metadata.NewContext(context.TODO(), md)
}

// Executes tiller.InstallRelease RPC.
func (o *options) rpcInstallRelease(chr *cpb.Chart, rlc rls.ReleaseServiceClient, ns string, opts ...InstallOption) (*rls.InstallReleaseResponse, error) {
	// apply the install options
	for _, opt := range opts {
		opt(o)
	}
	o.instReq.Chart = chr
	o.instReq.Namespace = ns
	o.instReq.DryRun = o.dryRun
	o.instReq.DisableHooks = o.disableHooks
	o.instReq.ReuseName = o.reuseName

	return rlc.InstallRelease(NewContext(), &o.instReq)
}

// Executes tiller.UninstallRelease RPC.
func (o *options) rpcDeleteRelease(rlsName string, rlc rls.ReleaseServiceClient, opts ...DeleteOption) (*rls.UninstallReleaseResponse, error) {
	for _, opt := range opts {
		opt(o)
	}
	if o.dryRun {
		// In the dry run case, just see if the release exists
		r, err := o.rpcGetReleaseContent(rlsName, rlc)
		if err != nil {
			return &rls.UninstallReleaseResponse{}, err
		}
		return &rls.UninstallReleaseResponse{Release: r.Release}, nil
	}

	o.uninstallReq.Name = rlsName
	o.uninstallReq.DisableHooks = o.disableHooks

	return rlc.UninstallRelease(NewContext(), &o.uninstallReq)
}

// Executes tiller.UpdateRelease RPC.
func (o *options) rpcUpdateRelease(rlsName string, chr *cpb.Chart, rlc rls.ReleaseServiceClient, opts ...UpdateOption) (*rls.UpdateReleaseResponse, error) {
	for _, opt := range opts {
		opt(o)
	}

	o.updateReq.Chart = chr
	o.updateReq.DryRun = o.dryRun
	o.updateReq.Name = rlsName

	return rlc.UpdateRelease(NewContext(), &o.updateReq)
}

// Executes tiller.UpdateRelease RPC.
func (o *options) rpcRollbackRelease(rlsName string, rlc rls.ReleaseServiceClient, opts ...RollbackOption) (*rls.RollbackReleaseResponse, error) {
	for _, opt := range opts {
		opt(o)
	}

	o.rollbackReq.DryRun = o.dryRun
	o.rollbackReq.Name = rlsName

	return rlc.RollbackRelease(NewContext(), &o.rollbackReq)
}

// Executes tiller.GetReleaseStatus RPC.
func (o *options) rpcGetReleaseStatus(rlsName string, rlc rls.ReleaseServiceClient, opts ...StatusOption) (*rls.GetReleaseStatusResponse, error) {
	for _, opt := range opts {
		opt(o)
	}
	o.statusReq.Name = rlsName
	return rlc.GetReleaseStatus(NewContext(), &o.statusReq)
}

// Executes tiller.GetReleaseContent.
func (o *options) rpcGetReleaseContent(rlsName string, rlc rls.ReleaseServiceClient, opts ...ContentOption) (*rls.GetReleaseContentResponse, error) {
	for _, opt := range opts {
		opt(o)
	}
	o.contentReq.Name = rlsName
	return rlc.GetReleaseContent(NewContext(), &o.contentReq)
}

// Executes tiller.GetVersion RPC.
func (o *options) rpcGetVersion(rlc rls.ReleaseServiceClient, opts ...VersionOption) (*rls.GetVersionResponse, error) {
	req := &rls.GetVersionRequest{}
	return rlc.GetVersion(NewContext(), req)
}
