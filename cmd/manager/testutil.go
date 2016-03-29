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

package main

import (
	"errors"
	"fmt"
	"net/http/httptest"
	"regexp"

	"github.com/kubernetes/helm/cmd/manager/router"
	"github.com/kubernetes/helm/pkg/chart"
	"github.com/kubernetes/helm/pkg/common"
	"github.com/kubernetes/helm/pkg/httputil"
	"github.com/kubernetes/helm/pkg/repo"
)

// httpHarness is a simple test server fixture.
// Simple fixture for standing up a test server with a single route.
//
// You must Close() the returned server.
func httpHarness(c *router.Context, route string, fn router.HandlerFunc) *httptest.Server {
	h := router.NewHandler(c)
	h.Add(route, fn)
	return httptest.NewServer(h)
}

// stubContext creates a stub of a Context object.
//
// This creates a stub context with the following properties:
// - Config is initialized to empty values
// - Encoder is initialized to httputil.DefaultEncoder
// - CredentialProvider is initialized to repo.InmemCredentialProvider
// - Manager is initialized to mockManager.
func stubContext() *router.Context {
	return &router.Context{
		Config:             &router.Config{},
		Manager:            newMockManager(),
		CredentialProvider: repo.NewInmemCredentialProvider(),
		Encoder:            httputil.DefaultEncoder,
	}
}

func newMockManager() *mockManager {
	return &mockManager{
		deployments: []*common.Deployment{},
	}
}

type mockManager struct {
	deployments []*common.Deployment
}

func (m *mockManager) ListDeployments() ([]common.Deployment, error) {
	d := make([]common.Deployment, len(m.deployments))
	for i, dd := range m.deployments {
		d[i] = *dd
	}
	return d, nil
}

func (m *mockManager) GetDeployment(name string) (*common.Deployment, error) {

	for _, d := range m.deployments {
		if d.Name == name {
			return d, nil
		}
	}

	return nil, errors.New("mock error: No such deployment")
}

func (m *mockManager) CreateDeployment(depReq *common.DeploymentRequest) (*common.Deployment, error) {
	return &common.Deployment{}, nil
}

func (m *mockManager) DeleteDeployment(name string, forget bool) (*common.Deployment, error) {
	for _, d := range m.deployments {
		if d.Name == name {
			return d, nil
		}
	}
	fmt.Printf("Could not find %s", name)
	return nil, errors.New("Deployment not found")
}

func (m *mockManager) PutDeployment(name string, depReq *common.DeploymentRequest) (*common.Deployment, error) {
	return &common.Deployment{}, nil
}

func (m *mockManager) ListManifests(deploymentName string) (map[string]*common.Manifest, error) {
	return map[string]*common.Manifest{}, nil
}

func (m *mockManager) GetManifest(deploymentName string, manifest string) (*common.Manifest, error) {
	return &common.Manifest{}, nil
}

func (m *mockManager) Expand(depReq *common.DeploymentRequest) (*common.Manifest, error) {
	return &common.Manifest{}, nil
}

func (m *mockManager) ListCharts() ([]string, error) {
	return []string{}, nil
}

func (m *mockManager) ListChartInstances(chartName string) ([]*common.ChartInstance, error) {
	return []*common.ChartInstance{}, nil
}

func (m *mockManager) GetRepoForChart(chartName string) (string, error) {
	return "", nil
}

func (m *mockManager) GetMetadataForChart(chartName string) (*chart.Chartfile, error) {
	return &chart.Chartfile{}, nil
}

func (m *mockManager) GetChart(chartName string) (*chart.Chart, error) {
	return &chart.Chart{}, nil
}

func (m *mockManager) ListRepoCharts(repoName string, regex *regexp.Regexp) ([]string, error) {
	return []string{}, nil
}

func (m *mockManager) GetChartForRepo(repoName, chartName string) (*chart.Chart, error) {
	return &chart.Chart{}, nil
}

func (m *mockManager) CreateCredential(name string, c *repo.Credential) error {
	return nil
}
func (m *mockManager) GetCredential(name string) (*repo.Credential, error) {
	return &repo.Credential{}, nil
}

func (m *mockManager) ListRepos() (map[string]string, error) {
	return map[string]string{}, nil
}

func (m *mockManager) AddRepo(addition repo.IRepo) error {
	return nil
}

func (m *mockManager) RemoveRepo(name string) error {
	return nil
}

func (m *mockManager) GetRepo(name string) (repo.IRepo, error) {
	return &repo.Repo{}, nil
}
