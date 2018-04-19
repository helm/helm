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
	"k8s.io/helm/pkg/chartutil"
	"k8s.io/helm/pkg/hapi"
	"k8s.io/helm/pkg/hapi/chart"
	"k8s.io/helm/pkg/hapi/release"
	"k8s.io/helm/pkg/storage"
	"k8s.io/helm/pkg/tiller"
	"k8s.io/helm/pkg/tiller/environment"
)

// Client manages client side of the Helm-Tiller protocol.
type Client struct {
	opts   options
	tiller *tiller.ReleaseServer
}

// NewClient creates a new client.
func NewClient(opts ...Option) *Client {
	var c Client
	return c.Option(opts...).init()
}

func (c *Client) init() *Client {
	env := environment.New()
	env.Releases = storage.Init(c.opts.driver)
	env.KubeClient = c.opts.kubeClient

	c.tiller = tiller.NewReleaseServer(env, c.opts.discovery)
	return c
}

// Option configures the Helm client with the provided options.
func (c *Client) Option(opts ...Option) *Client {
	for _, opt := range opts {
		opt(&c.opts)
	}
	return c
}

// ListReleases lists the current releases.
func (c *Client) ListReleases(opts ...ReleaseListOption) ([]*release.Release, error) {
	reqOpts := c.opts
	for _, opt := range opts {
		opt(&reqOpts)
	}
	req := &reqOpts.listReq
	if err := reqOpts.runBefore(req); err != nil {
		return nil, err
	}
	return c.tiller.ListReleases(req)
}

// InstallRelease loads a chart from chstr, installs it, and returns the release response.
func (c *Client) InstallRelease(chstr, ns string, opts ...InstallOption) (*release.Release, error) {
	// load the chart to install
	chart, err := chartutil.Load(chstr)
	if err != nil {
		return nil, err
	}

	return c.InstallReleaseFromChart(chart, ns, opts...)
}

// InstallReleaseFromChart installs a new chart and returns the release response.
func (c *Client) InstallReleaseFromChart(chart *chart.Chart, ns string, opts ...InstallOption) (*release.Release, error) {
	// apply the install options
	reqOpts := c.opts
	for _, opt := range opts {
		opt(&reqOpts)
	}
	req := &reqOpts.instReq
	req.Chart = chart
	req.Namespace = ns
	req.DryRun = reqOpts.dryRun
	req.DisableHooks = reqOpts.disableHooks
	req.ReuseName = reqOpts.reuseName

	if err := reqOpts.runBefore(req); err != nil {
		return nil, err
	}
	err := chartutil.ProcessRequirementsEnabled(req.Chart, req.Values)
	if err != nil {
		return nil, err
	}
	err = chartutil.ProcessRequirementsImportValues(req.Chart)
	if err != nil {
		return nil, err
	}

	return c.tiller.InstallRelease(req)
}

// DeleteRelease uninstalls a named release and returns the response.
func (c *Client) DeleteRelease(rlsName string, opts ...DeleteOption) (*hapi.UninstallReleaseResponse, error) {
	// apply the uninstall options
	reqOpts := c.opts
	for _, opt := range opts {
		opt(&reqOpts)
	}

	if reqOpts.dryRun {
		// In the dry run case, just see if the release exists
		r, err := c.ReleaseContent(rlsName, 0)
		if err != nil {
			return &hapi.UninstallReleaseResponse{}, err
		}
		return &hapi.UninstallReleaseResponse{Release: r}, nil
	}

	req := &reqOpts.uninstallReq
	req.Name = rlsName
	req.DisableHooks = reqOpts.disableHooks

	if err := reqOpts.runBefore(req); err != nil {
		return nil, err
	}
	return c.tiller.UninstallRelease(req)
}

// UpdateRelease loads a chart from chstr and updates a release to a new/different chart.
func (c *Client) UpdateRelease(rlsName string, chstr string, opts ...UpdateOption) (*release.Release, error) {
	// load the chart to update
	chart, err := chartutil.Load(chstr)
	if err != nil {
		return nil, err
	}

	return c.UpdateReleaseFromChart(rlsName, chart, opts...)
}

// UpdateReleaseFromChart updates a release to a new/different chart.
func (c *Client) UpdateReleaseFromChart(rlsName string, chart *chart.Chart, opts ...UpdateOption) (*release.Release, error) {
	// apply the update options
	reqOpts := c.opts
	for _, opt := range opts {
		opt(&reqOpts)
	}
	req := &reqOpts.updateReq
	req.Chart = chart
	req.DryRun = reqOpts.dryRun
	req.Name = rlsName
	req.DisableHooks = reqOpts.disableHooks
	req.Recreate = reqOpts.recreate
	req.Force = reqOpts.force
	req.ResetValues = reqOpts.resetValues
	req.ReuseValues = reqOpts.reuseValues

	if err := reqOpts.runBefore(req); err != nil {
		return nil, err
	}
	err := chartutil.ProcessRequirementsEnabled(req.Chart, req.Values)
	if err != nil {
		return nil, err
	}
	err = chartutil.ProcessRequirementsImportValues(req.Chart)
	if err != nil {
		return nil, err
	}

	return c.tiller.UpdateRelease(req)
}

// RollbackRelease rolls back a release to the previous version.
func (c *Client) RollbackRelease(rlsName string, opts ...RollbackOption) (*release.Release, error) {
	reqOpts := c.opts
	for _, opt := range opts {
		opt(&reqOpts)
	}
	req := &reqOpts.rollbackReq
	req.Recreate = reqOpts.recreate
	req.Force = reqOpts.force
	req.DisableHooks = reqOpts.disableHooks
	req.DryRun = reqOpts.dryRun
	req.Name = rlsName

	if err := reqOpts.runBefore(req); err != nil {
		return nil, err
	}
	return c.tiller.RollbackRelease(req)
}

// ReleaseStatus returns the given release's status.
func (c *Client) ReleaseStatus(rlsName string, version int) (*hapi.GetReleaseStatusResponse, error) {
	reqOpts := c.opts
	req := &reqOpts.statusReq
	req.Name = rlsName
	req.Version = version

	if err := reqOpts.runBefore(req); err != nil {
		return nil, err
	}
	return c.tiller.GetReleaseStatus(req)
}

// ReleaseContent returns the configuration for a given release.
func (c *Client) ReleaseContent(name string, version int) (*release.Release, error) {
	reqOpts := c.opts
	req := &reqOpts.contentReq
	req.Name = name
	req.Version = version

	if err := reqOpts.runBefore(req); err != nil {
		return nil, err
	}
	return c.tiller.GetReleaseContent(req)
}

// ReleaseHistory returns a release's revision history.
func (c *Client) ReleaseHistory(rlsName string, max int) ([]*release.Release, error) {
	reqOpts := c.opts
	req := &reqOpts.histReq
	req.Name = rlsName
	req.Max = max

	if err := reqOpts.runBefore(req); err != nil {
		return nil, err
	}
	return c.tiller.GetHistory(req)
}

// RunReleaseTest executes a pre-defined test on a release.
func (c *Client) RunReleaseTest(rlsName string, opts ...ReleaseTestOption) (<-chan *hapi.TestReleaseResponse, <-chan error) {
	reqOpts := c.opts
	for _, opt := range opts {
		opt(&reqOpts)
	}

	req := &reqOpts.testReq
	req.Name = rlsName

	return c.tiller.RunReleaseTest(req)
}
