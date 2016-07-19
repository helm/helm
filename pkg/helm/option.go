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
	cpb "k8s.io/helm/pkg/proto/hapi/chart"
	rls "k8s.io/helm/pkg/proto/hapi/services"
)

// Option allows specifying various settings configurable by
// the helm client user for overriding the defaults used when
// issuing rpc's to the Tiller release server.
type Option func(*options)

// options specify optional settings used by the helm client.
type options struct {
	// value of helm host override
	home string
	// value of helm home override
	host string
	// name of chart
	chart string
	// if set dry-run helm client calls
	dryRun bool
	// if set, skip running hooks
	disableHooks bool
	// release list options are applied directly to the list releases request
	listReq rls.ListReleasesRequest
	// release install options are applied directly to the install release request
	instReq rls.InstallReleaseRequest
	// release update options are applied directly to the update release request
	updateReq rls.UpdateReleaseRequest
}

// Home specifies the location of helm home, (default = "$HOME/.helm").
func Home(home string) Option {
	return func(opts *options) {
		opts.home = home
	}
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

// UpdateValueOverrides specifies a list of values to include when upgrading
func UpdateValueOverrides(raw []byte) UpdateOption {
	return func(opts *options) {
		opts.updateReq.Values = &cpb.Config{Raw: string(raw)}
	}
}

// ReleaseName specifies the name of the release when installing.
func ReleaseName(name string) InstallOption {
	return func(opts *options) {
		opts.instReq.Name = name
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

// UpgradeDryRun will (if true) execute an upgrade as a dry run.
func UpgradeDryRun(dry bool) UpdateOption {
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

// InstallDryRun will (if true) execute an installation as a dry run.
func InstallDryRun(dry bool) InstallOption {
	return func(opts *options) {
		opts.dryRun = dry
	}
}

// ContentOption -- TODO
type ContentOption func(*options)

// StatusOption -- TODO
type StatusOption func(*options)

// DeleteOption -- TODO
type DeleteOption func(*options)

// UpdateOption allows specifying various settings
// configurable by the helm client user for overriding
// the defaults used when running the `helm upgrade` command.
type UpdateOption func(*options)

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
	s, err := rlc.ListReleases(context.TODO(), &o.listReq)
	if err != nil {
		return nil, err
	}

	return s.Recv()
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

	return rlc.InstallRelease(context.TODO(), &o.instReq)
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
	return rlc.UninstallRelease(context.TODO(), &rls.UninstallReleaseRequest{Name: rlsName, DisableHooks: o.disableHooks})
}

// Executes tiller.UpdateRelease RPC.
func (o *options) rpcUpdateRelease(rlsName string, chr *cpb.Chart, rlc rls.ReleaseServiceClient, opts ...UpdateOption) (*rls.UpdateReleaseResponse, error) {
	//TODO: handle dryRun

	return rlc.UpdateRelease(context.TODO(), &rls.UpdateReleaseRequest{Name: rlsName, Chart: chr})
}

// Executes tiller.GetReleaseStatus RPC.
func (o *options) rpcGetReleaseStatus(rlsName string, rlc rls.ReleaseServiceClient, opts ...StatusOption) (*rls.GetReleaseStatusResponse, error) {
	req := &rls.GetReleaseStatusRequest{Name: rlsName}
	return rlc.GetReleaseStatus(context.TODO(), req)
}

// Executes tiller.GetReleaseContent.
func (o *options) rpcGetReleaseContent(rlsName string, rlc rls.ReleaseServiceClient, opts ...ContentOption) (*rls.GetReleaseContentResponse, error) {
	req := &rls.GetReleaseContentRequest{Name: rlsName}
	return rlc.GetReleaseContent(context.TODO(), req)
}
