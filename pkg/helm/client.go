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

package helm // import "k8s.io/helm/pkg/helm"

import (
	"io"
	"time"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"k8s.io/helm/pkg/chartutil"
	"k8s.io/helm/pkg/proto/hapi/chart"
	rls "k8s.io/helm/pkg/proto/hapi/services"
)

// Client manages client side of the Helm-Tiller protocol.
type Client struct {
	opts options
}

// NewClient creates a new client.
func NewClient(opts ...Option) *Client {
	var c Client
	return c.Option(opts...)
}

// Option configures the Helm client with the provided options.
func (h *Client) Option(opts ...Option) *Client {
	for _, opt := range opts {
		opt(&h.opts)
	}
	return h
}

// ListReleases lists the current releases.
func (h *Client) ListReleases(opts ...ReleaseListOption) (*rls.ListReleasesResponse, error) {
	for _, opt := range opts {
		opt(&h.opts)
	}
	req := &h.opts.listReq
	ctx := NewContext()

	if h.opts.before != nil {
		if err := h.opts.before(ctx, req); err != nil {
			return nil, err
		}
	}
	return h.list(ctx, req)
}

// InstallRelease loads a chart from chstr, installs it, and returns the release response.
func (h *Client) InstallRelease(chstr, ns string, opts ...InstallOption) (*rls.InstallReleaseResponse, error) {
	// load the chart to install
	chart, err := chartutil.Load(chstr)
	if err != nil {
		return nil, err
	}

	return h.InstallReleaseFromChart(chart, ns, opts...)
}

// InstallReleaseFromChart installs a new chart and returns the release response.
func (h *Client) InstallReleaseFromChart(chart *chart.Chart, ns string, opts ...InstallOption) (*rls.InstallReleaseResponse, error) {
	// apply the install options
	for _, opt := range opts {
		opt(&h.opts)
	}
	req := &h.opts.instReq
	req.Chart = chart
	req.Namespace = ns
	req.DryRun = h.opts.dryRun
	req.DisableHooks = h.opts.disableHooks
	req.ReuseName = h.opts.reuseName
	ctx := NewContext()

	if h.opts.before != nil {
		if err := h.opts.before(ctx, req); err != nil {
			return nil, err
		}
	}
	err := chartutil.ProcessRequirementsEnabled(req.Chart, req.Values)
	if err != nil {
		return nil, err
	}
	err = chartutil.ProcessRequirementsImportValues(req.Chart)
	if err != nil {
		return nil, err
	}

	return h.install(ctx, req)
}

// DeleteRelease uninstalls a named release and returns the response.
func (h *Client) DeleteRelease(rlsName string, opts ...DeleteOption) (*rls.UninstallReleaseResponse, error) {
	// apply the uninstall options
	for _, opt := range opts {
		opt(&h.opts)
	}

	if h.opts.dryRun {
		// In the dry run case, just see if the release exists
		r, err := h.ReleaseContent(rlsName)
		if err != nil {
			return &rls.UninstallReleaseResponse{}, err
		}
		return &rls.UninstallReleaseResponse{Release: r.Release}, nil
	}

	req := &h.opts.uninstallReq
	req.Name = rlsName
	req.DisableHooks = h.opts.disableHooks
	ctx := NewContext()

	if h.opts.before != nil {
		if err := h.opts.before(ctx, req); err != nil {
			return nil, err
		}
	}
	return h.delete(ctx, req)
}

// UpdateRelease loads a chart from chstr and updates a release to a new/different chart.
func (h *Client) UpdateRelease(rlsName string, chstr string, opts ...UpdateOption) (*rls.UpdateReleaseResponse, error) {
	// load the chart to update
	chart, err := chartutil.Load(chstr)
	if err != nil {
		return nil, err
	}

	return h.UpdateReleaseFromChart(rlsName, chart, opts...)
}

// UpdateReleaseFromChart updates a release to a new/different chart.
func (h *Client) UpdateReleaseFromChart(rlsName string, chart *chart.Chart, opts ...UpdateOption) (*rls.UpdateReleaseResponse, error) {

	// apply the update options
	for _, opt := range opts {
		opt(&h.opts)
	}
	req := &h.opts.updateReq
	req.Chart = chart
	req.DryRun = h.opts.dryRun
	req.Name = rlsName
	req.DisableHooks = h.opts.disableHooks
	req.Recreate = h.opts.recreate
	req.Force = h.opts.force
	req.ResetValues = h.opts.resetValues
	req.ReuseValues = h.opts.reuseValues
	ctx := NewContext()

	if h.opts.before != nil {
		if err := h.opts.before(ctx, req); err != nil {
			return nil, err
		}
	}
	err := chartutil.ProcessRequirementsEnabled(req.Chart, req.Values)
	if err != nil {
		return nil, err
	}
	err = chartutil.ProcessRequirementsImportValues(req.Chart)
	if err != nil {
		return nil, err
	}

	return h.update(ctx, req)
}

// GetVersion returns the server version.
func (h *Client) GetVersion(opts ...VersionOption) (*rls.GetVersionResponse, error) {
	for _, opt := range opts {
		opt(&h.opts)
	}
	req := &rls.GetVersionRequest{}
	ctx := NewContext()

	if h.opts.before != nil {
		if err := h.opts.before(ctx, req); err != nil {
			return nil, err
		}
	}
	return h.version(ctx, req)
}

// RollbackRelease rolls back a release to the previous version.
func (h *Client) RollbackRelease(rlsName string, opts ...RollbackOption) (*rls.RollbackReleaseResponse, error) {
	for _, opt := range opts {
		opt(&h.opts)
	}
	req := &h.opts.rollbackReq
	req.Recreate = h.opts.recreate
	req.Force = h.opts.force
	req.DisableHooks = h.opts.disableHooks
	req.DryRun = h.opts.dryRun
	req.Name = rlsName
	ctx := NewContext()

	if h.opts.before != nil {
		if err := h.opts.before(ctx, req); err != nil {
			return nil, err
		}
	}
	return h.rollback(ctx, req)
}

// ReleaseStatus returns the given release's status.
func (h *Client) ReleaseStatus(rlsName string, opts ...StatusOption) (*rls.GetReleaseStatusResponse, error) {
	for _, opt := range opts {
		opt(&h.opts)
	}
	req := &h.opts.statusReq
	req.Name = rlsName
	ctx := NewContext()

	if h.opts.before != nil {
		if err := h.opts.before(ctx, req); err != nil {
			return nil, err
		}
	}
	return h.status(ctx, req)
}

// ReleaseContent returns the configuration for a given release.
func (h *Client) ReleaseContent(rlsName string, opts ...ContentOption) (*rls.GetReleaseContentResponse, error) {
	for _, opt := range opts {
		opt(&h.opts)
	}
	req := &h.opts.contentReq
	req.Name = rlsName
	ctx := NewContext()

	if h.opts.before != nil {
		if err := h.opts.before(ctx, req); err != nil {
			return nil, err
		}
	}
	return h.content(ctx, req)
}

// ReleaseHistory returns a release's revision history.
func (h *Client) ReleaseHistory(rlsName string, opts ...HistoryOption) (*rls.GetHistoryResponse, error) {
	for _, opt := range opts {
		opt(&h.opts)
	}

	req := &h.opts.histReq
	req.Name = rlsName
	ctx := NewContext()

	if h.opts.before != nil {
		if err := h.opts.before(ctx, req); err != nil {
			return nil, err
		}
	}
	return h.history(ctx, req)
}

// RunReleaseTest executes a pre-defined test on a release.
func (h *Client) RunReleaseTest(rlsName string, opts ...ReleaseTestOption) (<-chan *rls.TestReleaseResponse, <-chan error) {
	for _, opt := range opts {
		opt(&h.opts)
	}

	req := &h.opts.testReq
	req.Name = rlsName
	ctx := NewContext()

	return h.test(ctx, req)
}

// connect returns a gRPC connection to Tiller or error. The gRPC dial options
// are constructed here.
func (h *Client) connect(ctx context.Context) (conn *grpc.ClientConn, err error) {
	opts := []grpc.DialOption{
		grpc.WithTimeout(5 * time.Second),
		grpc.WithBlock(),
	}
	switch {
	case h.opts.useTLS:
		opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(h.opts.tlsConfig)))
	default:
		opts = append(opts, grpc.WithInsecure())
	}
	if conn, err = grpc.Dial(h.opts.host, opts...); err != nil {
		return nil, err
	}
	return conn, nil
}

// Executes tiller.ListReleases RPC.
func (h *Client) list(ctx context.Context, req *rls.ListReleasesRequest) (*rls.ListReleasesResponse, error) {
	c, err := h.connect(ctx)
	if err != nil {
		return nil, err
	}
	defer c.Close()

	rlc := rls.NewReleaseServiceClient(c)
	s, err := rlc.ListReleases(ctx, req)
	if err != nil {
		return nil, err
	}

	return s.Recv()
}

// Executes tiller.InstallRelease RPC.
func (h *Client) install(ctx context.Context, req *rls.InstallReleaseRequest) (*rls.InstallReleaseResponse, error) {
	c, err := h.connect(ctx)
	if err != nil {
		return nil, err
	}
	defer c.Close()

	rlc := rls.NewReleaseServiceClient(c)
	return rlc.InstallRelease(ctx, req)
}

// Executes tiller.UninstallRelease RPC.
func (h *Client) delete(ctx context.Context, req *rls.UninstallReleaseRequest) (*rls.UninstallReleaseResponse, error) {
	c, err := h.connect(ctx)
	if err != nil {
		return nil, err
	}
	defer c.Close()

	rlc := rls.NewReleaseServiceClient(c)
	return rlc.UninstallRelease(ctx, req)
}

// Executes tiller.UpdateRelease RPC.
func (h *Client) update(ctx context.Context, req *rls.UpdateReleaseRequest) (*rls.UpdateReleaseResponse, error) {
	c, err := h.connect(ctx)
	if err != nil {
		return nil, err
	}
	defer c.Close()

	rlc := rls.NewReleaseServiceClient(c)
	return rlc.UpdateRelease(ctx, req)
}

// Executes tiller.RollbackRelease RPC.
func (h *Client) rollback(ctx context.Context, req *rls.RollbackReleaseRequest) (*rls.RollbackReleaseResponse, error) {
	c, err := h.connect(ctx)
	if err != nil {
		return nil, err
	}
	defer c.Close()

	rlc := rls.NewReleaseServiceClient(c)
	return rlc.RollbackRelease(ctx, req)
}

// Executes tiller.GetReleaseStatus RPC.
func (h *Client) status(ctx context.Context, req *rls.GetReleaseStatusRequest) (*rls.GetReleaseStatusResponse, error) {
	c, err := h.connect(ctx)
	if err != nil {
		return nil, err
	}
	defer c.Close()

	rlc := rls.NewReleaseServiceClient(c)
	return rlc.GetReleaseStatus(ctx, req)
}

// Executes tiller.GetReleaseContent RPC.
func (h *Client) content(ctx context.Context, req *rls.GetReleaseContentRequest) (*rls.GetReleaseContentResponse, error) {
	c, err := h.connect(ctx)
	if err != nil {
		return nil, err
	}
	defer c.Close()

	rlc := rls.NewReleaseServiceClient(c)
	return rlc.GetReleaseContent(ctx, req)
}

// Executes tiller.GetVersion RPC.
func (h *Client) version(ctx context.Context, req *rls.GetVersionRequest) (*rls.GetVersionResponse, error) {
	c, err := h.connect(ctx)
	if err != nil {
		return nil, err
	}
	defer c.Close()

	rlc := rls.NewReleaseServiceClient(c)
	return rlc.GetVersion(ctx, req)
}

// Executes tiller.GetHistory RPC.
func (h *Client) history(ctx context.Context, req *rls.GetHistoryRequest) (*rls.GetHistoryResponse, error) {
	c, err := h.connect(ctx)
	if err != nil {
		return nil, err
	}
	defer c.Close()

	rlc := rls.NewReleaseServiceClient(c)
	return rlc.GetHistory(ctx, req)
}

// Executes tiller.TestRelease RPC.
func (h *Client) test(ctx context.Context, req *rls.TestReleaseRequest) (<-chan *rls.TestReleaseResponse, <-chan error) {
	errc := make(chan error, 1)
	c, err := h.connect(ctx)
	if err != nil {
		errc <- err
		return nil, errc
	}

	ch := make(chan *rls.TestReleaseResponse, 1)
	go func() {
		defer close(errc)
		defer close(ch)
		defer c.Close()

		rlc := rls.NewReleaseServiceClient(c)
		s, err := rlc.RunReleaseTest(ctx, req)
		if err != nil {
			errc <- err
			return
		}

		for {
			msg, err := s.Recv()
			if err == io.EOF {
				return
			}
			if err != nil {
				errc <- err
				return
			}
			ch <- msg
		}
	}()

	return ch, errc
}
