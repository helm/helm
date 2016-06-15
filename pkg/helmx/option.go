package helmx

import (
	"fmt"
	"golang.org/x/net/context"
	cpb "k8s.io/helm/pkg/proto/hapi/chart"
	rls "k8s.io/helm/pkg/proto/hapi/services"
)

// # Options
//
//      Option allows specifying various settings configurable by
//      the helm client user for overriding the defaults used when
//      issuing rpc's to the Tiller release server.
//
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
	// release list options are applied directly to the list releases request
	listReq rls.ListReleasesRequest
	// release install options are applied directly to the install release request
	instReq rls.InstallReleaseRequest
}

// DryRun returns an Option which instructs the helm client to dry-run tiller rpcs.
func DryRun() Option {
	return func(opts *options) {
		opts.dryRun = true
	}
}

// HelmHome specifies the location of helm home, (default = "$HOME/.helm").
func HelmHome(home string) Option {
	return func(opts *options) {
		opts.home = home
	}
}

// HelmHost specifies the host address of the Tiller release server, (default = ":44134").
func HelmHost(host string) Option {
	return func(opts *options) {
		opts.host = host
	}
}

// # Release List Options
//
//      ReleaseListOption allows specifying various settings
//      configurable by the helm client user for overriding
//      the defaults used when running the `helm list` command.
//
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

// # Install Options
//
//		InstallOption allows specifying various settings
//	    configurable by the helm client user for overriding
//	    the defaults used when running the `helm install` command.
//
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

// # ContentOption -- TODO
type ContentOption func(*options)

// # StatusOption -- TODO
type StatusOption func(*options)

// # DeleteOption -- TODO
type DeleteOption func(*options)

// # UpdateOption -- TODO
type UpdateOption func(*options)

// # RPC helpers defined on `options` type. Note: These actually execute the
//   the corresponding tiller RPC. There is no particular reason why these
//   are APIs are hung off `options`, they are internal to pkg/helm to remain
//   malleable.

// TODO: Executes tiller.ListReleases RPC. See issue #828
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

// TODO: Executes tiller.InstallRelease RPC. See issue #828
func (o *options) rpcInstallRelease(chr *cpb.Chart, rlc rls.ReleaseServiceClient, opts ...InstallOption) (*rls.InstallReleaseResponse, error) {
	// apply the install options
	for _, opt := range opts {
		opt(o)
	}

	o.instReq.Chart = chr
	return rlc.InstallRelease(context.TODO(), &o.instReq)
}

// TODO: Executes tiller.UninstallRelease RPC. See issue #828
func (o *options) rpcDeleteRelease(rlsName string, rlc rls.ReleaseServiceClient, opts ...DeleteOption) (*rls.UninstallReleaseResponse, error) {
	return nil, fmt.Errorf("helm: rpcDeleteRelease: not implemented")
}

// TODO: Executes tiller.UpdateRelease RPC. See issue #828
func (o *options) rpcUpdateRelease(rlsName string, rlc rls.ReleaseServiceClient, opts ...UpdateOption) (*rls.UpdateReleaseResponse, error) {
	return nil, fmt.Errorf("helm: rpcUpdateRelease: not implemented")
}

// TODO: Executes tiller.GetReleaseStatus RPC. See issue #828
func (o *options) rpcGetReleaseStatus(rlsName string, rlc rls.ReleaseServiceClient, opts ...StatusOption) (*rls.GetReleaseStatusResponse, error) {
	return nil, fmt.Errorf("helm: rpcGetReleaseStatus: not implemented")
}

// TODO: Executes tiller.GetReleaseConent. See issue #828
func (o *options) rpcGetReleaseContent(rlsName string, rlc rls.ReleaseServiceClient, opts ...ContentOption) (*rls.GetReleaseContentResponse, error) {
	return nil, fmt.Errorf("helm: getReleaseContent: not implemented")
}
