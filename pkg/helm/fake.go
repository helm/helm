/*
Copyright The Helm Authors.

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
	"math/rand"
	"sync"
	"time"

	"github.com/pkg/errors"

	"k8s.io/helm/pkg/chart"
	"k8s.io/helm/pkg/hapi"
	"k8s.io/helm/pkg/hapi/release"
)

// FakeClient implements Interface
type FakeClient struct {
	Rels          []*release.Release
	TestRunStatus map[string]release.TestRunStatus
	Opts          options
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
func (c *FakeClient) ListReleases(opts ...ReleaseListOption) ([]*release.Release, error) {
	return c.Rels, nil
}

// InstallRelease creates a new release and returns the release
func (c *FakeClient) InstallRelease(chStr, ns string, opts ...InstallOption) (*release.Release, error) {
	chart := &chart.Chart{}
	return c.InstallReleaseFromChart(chart, ns, opts...)
}

// InstallReleaseFromChart adds a new MockRelease to the fake client and
// returns the release
func (c *FakeClient) InstallReleaseFromChart(chart *chart.Chart, ns string, opts ...InstallOption) (*release.Release, error) {
	for _, opt := range opts {
		opt(&c.Opts)
	}

	releaseName := c.Opts.instReq.Name

	// Check to see if the release already exists.
	rel, err := c.ReleaseStatus(releaseName, 0)
	if err == nil && rel != nil {
		return nil, errors.New("cannot re-use a name that is still in use")
	}

	release := ReleaseMock(&MockReleaseOptions{Name: releaseName, Namespace: ns})
	c.Rels = append(c.Rels, release)
	return release, nil
}

// UninstallRelease uninstalls a release from the FakeClient
func (c *FakeClient) UninstallRelease(rlsName string, opts ...UninstallOption) (*hapi.UninstallReleaseResponse, error) {
	for i, rel := range c.Rels {
		if rel.Name == rlsName {
			c.Rels = append(c.Rels[:i], c.Rels[i+1:]...)
			return &hapi.UninstallReleaseResponse{
				Release: rel,
			}, nil
		}
	}

	return nil, errors.Errorf("no such release: %s", rlsName)
}

// UpdateRelease returns the updated release, if it exists
func (c *FakeClient) UpdateRelease(rlsName, chStr string, opts ...UpdateOption) (*release.Release, error) {
	return c.UpdateReleaseFromChart(rlsName, &chart.Chart{}, opts...)
}

// UpdateReleaseFromChart returns the updated release, if it exists
func (c *FakeClient) UpdateReleaseFromChart(rlsName string, chart *chart.Chart, opts ...UpdateOption) (*release.Release, error) {
	// Check to see if the release already exists.
	return c.ReleaseContent(rlsName, 0)
}

// RollbackRelease returns nil, nil
func (c *FakeClient) RollbackRelease(rlsName string, opts ...RollbackOption) (*release.Release, error) {
	return nil, nil
}

// ReleaseStatus returns a release status response with info from the matching release name.
func (c *FakeClient) ReleaseStatus(rlsName string, version int) (*hapi.GetReleaseStatusResponse, error) {
	for _, rel := range c.Rels {
		if rel.Name == rlsName {
			return &hapi.GetReleaseStatusResponse{
				Name:      rel.Name,
				Info:      rel.Info,
				Namespace: rel.Namespace,
			}, nil
		}
	}
	return nil, errors.Errorf("no such release: %s", rlsName)
}

// ReleaseContent returns the configuration for the matching release name in the fake release client.
func (c *FakeClient) ReleaseContent(rlsName string, version int) (*release.Release, error) {
	for _, rel := range c.Rels {
		if rel.Name == rlsName {
			return rel, nil
		}
	}
	return nil, errors.Errorf("no such release: %s", rlsName)
}

// ReleaseHistory returns a release's revision history.
func (c *FakeClient) ReleaseHistory(rlsName string, max int) ([]*release.Release, error) {
	return c.Rels, nil
}

// RunReleaseTest executes a pre-defined tests on a release
func (c *FakeClient) RunReleaseTest(rlsName string, opts ...ReleaseTestOption) (<-chan *hapi.TestReleaseResponse, <-chan error) {

	results := make(chan *hapi.TestReleaseResponse)
	errc := make(chan error, 1)

	go func() {
		var wg sync.WaitGroup
		for m, s := range c.TestRunStatus {
			wg.Add(1)

			go func(msg string, status release.TestRunStatus) {
				defer wg.Done()
				results <- &hapi.TestReleaseResponse{Msg: msg, Status: status}
			}(m, s)
		}

		wg.Wait()
		close(results)
		close(errc)
	}()

	return results, errc
}

// MockHookTemplate is the hook template used for all mock release objects.
var MockHookTemplate = `apiVersion: v1
kind: Job
metadata:
  annotations:
    "helm.sh/hook": pre-install
`

// MockManifest is the manifest used for all mock release objects.
var MockManifest = `apiVersion: v1
kind: Secret
metadata:
  name: fixture
`

// MockReleaseOptions allows for user-configurable options on mock release objects.
type MockReleaseOptions struct {
	Name      string
	Version   int
	Chart     *chart.Chart
	Status    release.ReleaseStatus
	Namespace string
}

// ReleaseMock creates a mock release object based on options set by MockReleaseOptions. This function should typically not be used outside of testing.
func ReleaseMock(opts *MockReleaseOptions) *release.Release {
	date := time.Unix(242085845, 0).UTC()

	name := opts.Name
	if name == "" {
		name = "testrelease-" + string(rand.Intn(100))
	}

	var version int = 1
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
			Templates: []*chart.File{
				{Name: "templates/foo.tpl", Data: []byte(MockManifest)},
			},
		}
	}

	scode := release.StatusDeployed
	if len(opts.Status) > 0 {
		scode = opts.Status
	}

	return &release.Release{
		Name: name,
		Info: &release.Info{
			FirstDeployed: date,
			LastDeployed:  date,
			Status:        scode,
			Description:   "Release mock",
		},
		Chart:     ch,
		Config:    map[string]interface{}{"name": "value"},
		Version:   version,
		Namespace: namespace,
		Hooks: []*release.Hook{
			{
				Name:     "pre-install-hook",
				Kind:     "Job",
				Path:     "pre-install-hook.yaml",
				Manifest: MockHookTemplate,
				LastRun:  date,
				Events:   []release.HookEvent{release.HookPreInstall},
			},
		},
		Manifest: MockManifest,
	}
}
