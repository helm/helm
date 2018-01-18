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
	"errors"
	"fmt"
	"math/rand"
	"sync"

	"github.com/golang/protobuf/ptypes/timestamp"
	"k8s.io/helm/pkg/proto/hapi/chart"
	"k8s.io/helm/pkg/proto/hapi/release"
	rls "k8s.io/helm/pkg/proto/hapi/services"
	"k8s.io/helm/pkg/proto/hapi/version"
)

// FakeClient implements Interface
type FakeClient struct {
	Rels      []*release.Release
	Responses map[string]release.TestRun_Status
	Opts      options
}

// Option returns the fake release client
func (c *FakeClient) Option(opts ...Option) Interface {
	for _, opt := range opts {
		opt(&c.Opts)
	}
	return c
}

var _ Interface = &FakeClient{}
var _ Interface = (*FakeClient)(nil)

// ListReleases lists the current releases
func (c *FakeClient) ListReleases(opts ...ReleaseListOption) (*rls.ListReleasesResponse, error) {
	resp := &rls.ListReleasesResponse{
		Count:    int64(len(c.Rels)),
		Releases: c.Rels,
	}
	return resp, nil
}

// InstallRelease creates a new release and returns a InstallReleaseResponse containing that release
func (c *FakeClient) InstallRelease(chStr, ns string, opts ...InstallOption) (*rls.InstallReleaseResponse, error) {
	chart := &chart.Chart{}
	return c.InstallReleaseFromChart(chart, ns, opts...)
}

// InstallReleaseFromChart adds a new MockRelease to the fake client and returns a InstallReleaseResponse containing that release
func (c *FakeClient) InstallReleaseFromChart(chart *chart.Chart, ns string, opts ...InstallOption) (*rls.InstallReleaseResponse, error) {
	for _, opt := range opts {
		opt(&c.Opts)
	}

	releaseName := c.Opts.instReq.Name

	// Check to see if the release already exists.
	rel, err := c.ReleaseStatus(releaseName, nil)
	if err == nil && rel != nil {
		return nil, errors.New("cannot re-use a name that is still in use")
	}

	release := ReleaseMock(&MockReleaseOptions{Name: releaseName, Namespace: ns})
	c.Rels = append(c.Rels, release)

	return &rls.InstallReleaseResponse{
		Release: release,
	}, nil
}

// DeleteRelease deletes a release from the FakeClient
func (c *FakeClient) DeleteRelease(rlsName string, opts ...DeleteOption) (*rls.UninstallReleaseResponse, error) {
	for i, rel := range c.Rels {
		if rel.Name == rlsName {
			c.Rels = append(c.Rels[:i], c.Rels[i+1:]...)
			return &rls.UninstallReleaseResponse{
				Release: rel,
			}, nil
		}
	}

	return nil, fmt.Errorf("No such release: %s", rlsName)
}

// GetVersion returns a fake version
func (c *FakeClient) GetVersion(opts ...VersionOption) (*rls.GetVersionResponse, error) {
	return &rls.GetVersionResponse{
		Version: &version.Version{
			SemVer: "1.2.3-fakeclient+testonly",
		},
	}, nil
}

// UpdateRelease returns an UpdateReleaseResponse containing the updated release, if it exists
func (c *FakeClient) UpdateRelease(rlsName string, chStr string, opts ...UpdateOption) (*rls.UpdateReleaseResponse, error) {
	return c.UpdateReleaseFromChart(rlsName, &chart.Chart{}, opts...)
}

// UpdateReleaseFromChart returns an UpdateReleaseResponse containing the updated release, if it exists
func (c *FakeClient) UpdateReleaseFromChart(rlsName string, chart *chart.Chart, opts ...UpdateOption) (*rls.UpdateReleaseResponse, error) {
	// Check to see if the release already exists.
	rel, err := c.ReleaseContent(rlsName, nil)
	if err != nil {
		return nil, err
	}

	return &rls.UpdateReleaseResponse{Release: rel.Release}, nil
}

// RollbackRelease returns nil, nil
func (c *FakeClient) RollbackRelease(rlsName string, opts ...RollbackOption) (*rls.RollbackReleaseResponse, error) {
	return nil, nil
}

// ReleaseStatus returns a release status response with info from the matching release name.
func (c *FakeClient) ReleaseStatus(rlsName string, opts ...StatusOption) (*rls.GetReleaseStatusResponse, error) {
	for _, rel := range c.Rels {
		if rel.Name == rlsName {
			return &rls.GetReleaseStatusResponse{
				Name:      rel.Name,
				Info:      rel.Info,
				Namespace: rel.Namespace,
			}, nil
		}
	}
	return nil, fmt.Errorf("No such release: %s", rlsName)
}

// ReleaseContent returns the configuration for the matching release name in the fake release client.
func (c *FakeClient) ReleaseContent(rlsName string, opts ...ContentOption) (resp *rls.GetReleaseContentResponse, err error) {
	for _, rel := range c.Rels {
		if rel.Name == rlsName {
			return &rls.GetReleaseContentResponse{
				Release: rel,
			}, nil
		}
	}
	return resp, fmt.Errorf("No such release: %s", rlsName)
}

// ReleaseHistory returns a release's revision history.
func (c *FakeClient) ReleaseHistory(rlsName string, opts ...HistoryOption) (*rls.GetHistoryResponse, error) {
	return &rls.GetHistoryResponse{Releases: c.Rels}, nil
}

// RunReleaseTest executes a pre-defined tests on a release
func (c *FakeClient) RunReleaseTest(rlsName string, opts ...ReleaseTestOption) (<-chan *rls.TestReleaseResponse, <-chan error) {

	results := make(chan *rls.TestReleaseResponse)
	errc := make(chan error, 1)

	go func() {
		var wg sync.WaitGroup
		for m, s := range c.Responses {
			wg.Add(1)

			go func(msg string, status release.TestRun_Status) {
				defer wg.Done()
				results <- &rls.TestReleaseResponse{Msg: msg, Status: status}
			}(m, s)
		}

		wg.Wait()
		close(results)
		close(errc)
	}()

	return results, errc
}

// PingTiller pings the Tiller pod and ensure's that it is up and runnning
func (c *FakeClient) PingTiller() error {
	return nil
}

// MockHookTemplate is the hook template used for all mock release objects.
var MockHookTemplate = `apiVersion: v1
kind: Job
metadata:
  annotations:
    "helm.sh/hooks": pre-install
`

// MockManifest is the manifest used for all mock release objects.
var MockManifest = `apiVersion: v1
kind: Secret
metadata:
  name: fixture
`

// MockReleaseOptions allows for user-configurable options on mock release objects.
type MockReleaseOptions struct {
	Name       string
	Version    int32
	Chart      *chart.Chart
	StatusCode release.Status_Code
	Namespace  string
}

// ReleaseMock creates a mock release object based on options set by MockReleaseOptions. This function should typically not be used outside of testing.
func ReleaseMock(opts *MockReleaseOptions) *release.Release {
	date := timestamp.Timestamp{Seconds: 242085845, Nanos: 0}

	name := opts.Name
	if name == "" {
		name = "testrelease-" + string(rand.Intn(100))
	}

	var version int32 = 1
	if opts.Version != 0 {
		version = opts.Version
	}

	namespace := opts.Namespace
	if namespace == "" {
		namespace = "default"
	}

	ch := opts.Chart
	if opts.Chart == nil {
		ch = &chart.Chart{
			Metadata: &chart.Metadata{
				Name:    "foo",
				Version: "0.1.0-beta.1",
			},
			Templates: []*chart.Template{
				{Name: "templates/foo.tpl", Data: []byte(MockManifest)},
			},
		}
	}

	scode := release.Status_DEPLOYED
	if opts.StatusCode > 0 {
		scode = opts.StatusCode
	}

	return &release.Release{
		Name: name,
		Info: &release.Info{
			FirstDeployed: &date,
			LastDeployed:  &date,
			Status:        &release.Status{Code: scode},
			Description:   "Release mock",
		},
		Chart:     ch,
		Config:    &chart.Config{Raw: `name: "value"`},
		Version:   version,
		Namespace: namespace,
		Hooks: []*release.Hook{
			{
				Name:     "pre-install-hook",
				Kind:     "Job",
				Path:     "pre-install-hook.yaml",
				Manifest: MockHookTemplate,
				LastRun:  &date,
				Events:   []release.Hook_Event{release.Hook_PRE_INSTALL},
			},
		},
		Manifest: MockManifest,
	}
}
