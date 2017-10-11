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
	"strings"
	"sync"

	"github.com/technosophos/moniker"
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

// InstallRelease returns a response with the first Release on the fake release client
func (c *FakeClient) InstallRelease(chStr, ns string, opts ...InstallOption) (*rls.InstallReleaseResponse, error) {
	chart := &chart.Chart{}
	return c.InstallReleaseFromChart(chart, ns, opts...)
}

// InstallReleaseFromChart returns a response with the first Release on the fake release client
func (c *FakeClient) InstallReleaseFromChart(chart *chart.Chart, ns string, opts ...InstallOption) (*rls.InstallReleaseResponse, error) {
	for _, opt := range opts {
		opt(&c.Opts)
	}

	releaseName := c.Opts.instReq.Name
	if releaseName == "" {
		for i := 0; i < 5; i++ {
			namer := moniker.New()
			name := namer.NameSep("-")
			if len(name) > 53 {
				name = name[:53]
			}
			if _, err := c.ReleaseStatus(name, nil); strings.Contains(err.Error(), "No such release") {
				releaseName = name
				break
			}
		}
	}

	// Check to see if the release already exists.
	rel, err := c.ReleaseStatus(releaseName, nil)
	if err == nil && rel != nil {
		return nil, errors.New("cannot re-use a name that is still in use")
	}

	release := &release.Release{
		Name:      releaseName,
		Namespace: ns,
	}

	c.Rels = append(c.Rels, release)

	return &rls.InstallReleaseResponse{
		Release: release,
	}, nil
}

// DeleteRelease returns nil, nil
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

// UpdateRelease returns nil, nil
func (c *FakeClient) UpdateRelease(rlsName string, chStr string, opts ...UpdateOption) (*rls.UpdateReleaseResponse, error) {
	return c.UpdateReleaseFromChart(rlsName, &chart.Chart{}, opts...)
}

// UpdateReleaseFromChart returns nil, nil
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

// ReleaseStatus returns a release status response with info from the matching release name in the fake release client.
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
