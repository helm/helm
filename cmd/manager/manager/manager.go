/*
Copyright 2015 The Kubernetes Authors All rights reserved.

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

package manager

import (
	"fmt"
	"log"
	"regexp"
	"time"

	"github.com/kubernetes/helm/cmd/manager/repository"
	"github.com/kubernetes/helm/pkg/chart"
	"github.com/kubernetes/helm/pkg/common"
	"github.com/kubernetes/helm/pkg/repo"
)

// Manager manages a persistent set of Deployments.
type Manager interface {
	// Deployments
	ListDeployments() ([]common.Deployment, error)
	GetDeployment(name string) (*common.Deployment, error)
	CreateDeployment(depReq *common.DeploymentRequest) (*common.Deployment, error)
	DeleteDeployment(name string, forget bool) (*common.Deployment, error)
	PutDeployment(name string, depReq *common.DeploymentRequest) (*common.Deployment, error)

	// Manifests
	ListManifests(deploymentName string) (map[string]*common.Manifest, error)
	GetManifest(deploymentName string, manifest string) (*common.Manifest, error)
	Expand(t *common.DeploymentRequest) (*common.Manifest, error)

	// Charts
	ListCharts() ([]string, error)
	ListChartInstances(chartName string) ([]*common.ChartInstance, error)
	GetRepoForChart(chartName string) (string, error)
	GetMetadataForChart(chartName string) (*chart.Chartfile, error)
	GetChart(chartName string) (*chart.Chart, error)

	// Repo Charts
	ListRepoCharts(repoName string, regex *regexp.Regexp) ([]string, error)
	GetChartForRepo(repoName, chartName string) (*chart.Chart, error)

	// Credentials
	CreateCredential(name string, c *repo.Credential) error
	GetCredential(name string) (*repo.Credential, error)

	// Chart Repositories
	ListRepos() (map[string]string, error)
	AddRepo(addition repo.IRepo) error
	RemoveRepo(repoName string) error
	GetRepo(repoName string) (repo.IRepo, error)
}

type manager struct {
	expander     Expander
	deployer     Deployer
	repository   repository.Repository
	repoProvider repo.IRepoProvider
	service      repo.IRepoService
	//TODO: add chart repo service
	credentialProvider repo.ICredentialProvider
}

// NewManager returns a new initialized Manager.
func NewManager(expander Expander,
	deployer Deployer,
	repository repository.Repository,
	repoProvider repo.IRepoProvider,
	service repo.IRepoService,
	credentialProvider repo.ICredentialProvider) Manager {
	return &manager{expander, deployer, repository, repoProvider, service, credentialProvider}
}

// ListDeployments returns the list of deployments
func (m *manager) ListDeployments() ([]common.Deployment, error) {
	l, err := m.repository.ListDeployments()
	if err != nil {
		return nil, err
	}
	return l, nil
}

// GetDeployment retrieves the configuration stored for a given deployment
// as well as the current configuration from the cluster.
func (m *manager) GetDeployment(name string) (*common.Deployment, error) {
	d, err := m.repository.GetDeployment(name)
	if err != nil {
		return nil, err
	}

	return d, nil
}

// ListManifests retrieves the manifests for a given deployment
// of each of the deployments in the repository and returns the deployments.
func (m *manager) ListManifests(deploymentName string) (map[string]*common.Manifest, error) {
	l, err := m.repository.ListManifests(deploymentName)
	if err != nil {
		return nil, err
	}

	return l, nil
}

// GetManifest retrieves the specified manifest for a given deployment
func (m *manager) GetManifest(deploymentName string, manifestName string) (*common.Manifest, error) {
	d, err := m.repository.GetManifest(deploymentName, manifestName)
	if err != nil {
		return nil, err
	}

	return d, nil
}

// CreateDeployment expands the supplied configuration, creates the resulting
// resources in the cluster, creates a new deployment that tracks it, stores the
// deployment in the repository and returns the deployment.
func (m *manager) CreateDeployment(depReq *common.DeploymentRequest) (*common.Deployment, error) {
	log.Printf("Creating deployment: %s", depReq.Name)
	_, err := m.repository.CreateDeployment(depReq.Name)
	if err != nil {
		log.Printf("CreateDeployment failed %v", err)
		return nil, err
	}

	manifest, err := m.Expand(depReq)
	if err != nil {
		log.Printf("Manifest creation failed: %v", err)
		m.repository.SetDeploymentState(depReq.Name, failState(err))
		return nil, err
	}

	if err := m.repository.AddManifest(manifest); err != nil {
		log.Printf("AddManifest failed %v", err)
		m.repository.SetDeploymentState(depReq.Name, failState(err))
		return nil, err
	}

	actualConfig, err := m.deployer.CreateConfiguration(manifest.ExpandedConfig)
	if err != nil {
		// Deployment failed, mark as failed
		log.Printf("CreateConfiguration failed: %v", err)
		m.repository.SetDeploymentState(depReq.Name, failState(err))

		// If we failed before being able to create some of the resources, then
		// return the failure as such. Otherwise, we're going to add the manifest
		// and hence resource specific errors down below.
		if actualConfig == nil {
			return nil, err
		}
	} else {
		// May be errors in the resources themselves.
		errs := getResourceErrors(actualConfig)
		if len(errs) > 0 {
			e := fmt.Errorf("Found resource errors during deployment: %v", errs)
			m.repository.SetDeploymentState(depReq.Name, failState(e))
			return nil, e
		}

		m.repository.SetDeploymentState(depReq.Name, &common.DeploymentState{Status: common.DeployedStatus})
	}

	// Update the manifest with the actual state of the reified resources
	manifest.ExpandedConfig = actualConfig
	if err := m.repository.SetManifest(manifest); err != nil {
		log.Printf("SetManifest failed %v", err)
		m.repository.SetDeploymentState(depReq.Name, failState(err))
		return nil, err
	}

	// Finally update the type instances for this deployment.
	m.setChartInstances(depReq.Name, manifest.Name, manifest.Layout)
	return m.repository.GetValidDeployment(depReq.Name)
}

func (m *manager) setChartInstances(deploymentName string, manifestName string, layout *common.Layout) {
	m.repository.ClearChartInstancesForDeployment(deploymentName)

	instances := make(map[string][]*common.ChartInstance)
	for i, r := range layout.Resources {
		addChartInstances(&instances, r, deploymentName, manifestName, fmt.Sprintf("$.resources[%d]", i))
	}

	m.repository.AddChartInstances(instances)
}

func addChartInstances(instances *map[string][]*common.ChartInstance, r *common.LayoutResource, deploymentName string, manifestName string, jsonPath string) {
	// Add this resource.
	inst := &common.ChartInstance{
		Name:       r.Name,
		Type:       r.Type,
		Deployment: deploymentName,
		Manifest:   manifestName,
		Path:       jsonPath,
	}

	(*instances)[r.Type] = append((*instances)[r.Type], inst)

	// Add all sub resources if they exist.
	for i, sr := range r.Resources {
		addChartInstances(instances, sr, deploymentName, manifestName, fmt.Sprintf("%s.resources[%d]", jsonPath, i))
	}
}

// DeleteDeployment deletes the configuration for the deployment with
// the supplied identifier from the cluster.repository. If forget is true, then
// the deployment is removed from the repository. Otherwise, it is marked
// as deleted and retained.
func (m *manager) DeleteDeployment(name string, forget bool) (*common.Deployment, error) {
	log.Printf("Deleting deployment: %s", name)
	d, err := m.repository.GetValidDeployment(name)
	if err != nil {
		return nil, err
	}

	// If there's a latest manifest, delete the underlying resources.
	latest, err := m.repository.GetLatestManifest(name)
	if err != nil {
		m.repository.SetDeploymentState(name, failState(err))
		return nil, err
	}

	if latest != nil {
		log.Printf("Deleting resources from the latest manifest")
		// Clear previous state.
		for _, r := range latest.ExpandedConfig.Resources {
			r.State = nil
		}

		if _, err := m.deployer.DeleteConfiguration(latest.ExpandedConfig); err != nil {
			log.Printf("Failed to delete resources from the latest manifest: %v", err)
			m.repository.SetDeploymentState(name, failState(err))
			return nil, err
		}

		// Create an empty manifest since resources have been deleted.
		if !forget {
			manifest := &common.Manifest{Deployment: name, Name: generateManifestName()}
			if err := m.repository.AddManifest(manifest); err != nil {
				log.Printf("Failed to add empty manifest")
				return nil, err
			}
		}
	}

	d, err = m.repository.DeleteDeployment(name, forget)
	if err != nil {
		return nil, err
	}

	// Finally remove the type instances for this deployment.
	m.repository.ClearChartInstancesForDeployment(name)
	return d, nil
}

// PutDeployment replaces the configuration of the deployment with
// the supplied identifier in the cluster, and returns the deployment.
func (m *manager) PutDeployment(name string, depReq *common.DeploymentRequest) (*common.Deployment, error) {
	_, err := m.repository.GetValidDeployment(name)
	if err != nil {
		return nil, err
	}

	manifest, err := m.Expand(depReq)
	if err != nil {
		log.Printf("Manifest creation failed: %v", err)
		m.repository.SetDeploymentState(name, failState(err))
		return nil, err
	}

	actualConfig, err := m.deployer.PutConfiguration(manifest.ExpandedConfig)
	if err != nil {
		m.repository.SetDeploymentState(name, failState(err))
		return nil, err
	}

	manifest.ExpandedConfig = actualConfig
	err = m.repository.AddManifest(manifest)
	if err != nil {
		m.repository.SetDeploymentState(name, failState(err))
		return nil, err
	}

	// Finally update the type instances for this deployment.
	m.setChartInstances(depReq.Name, manifest.Name, manifest.Layout)
	return m.repository.GetValidDeployment(depReq.Name)
}

func (m *manager) Expand(depReq *common.DeploymentRequest) (*common.Manifest, error) {
	expConf, err := m.expander.ExpandConfiguration(&depReq.Configuration)
	if err != nil {
		log.Printf("Expansion failed %v", err)
		return nil, err
	}

	return &common.Manifest{
		Name:           generateManifestName(),
		Deployment:     depReq.Name,
		InputConfig:    &depReq.Configuration,
		ExpandedConfig: expConf.Config,
		Layout:         expConf.Layout,
	}, nil
}

func (m *manager) ListCharts() ([]string, error) {
	return m.repository.ListCharts()
}

func (m *manager) ListChartInstances(chartName string) ([]*common.ChartInstance, error) {
	return m.repository.GetChartInstances(chartName)
}

// GetRepoForChart returns the repository where the referenced chart resides.
func (m *manager) GetRepoForChart(reference string) (string, error) {
	_, r, err := m.repoProvider.GetChartByReference(reference)
	if err != nil {
		return "", err
	}

	return r.GetURL(), nil
}

// GetMetadataForChart returns the metadata for the referenced chart.
func (m *manager) GetMetadataForChart(reference string) (*chart.Chartfile, error) {
	c, _, err := m.repoProvider.GetChartByReference(reference)
	if err != nil {
		return nil, err
	}

	return c.Chartfile(), nil
}

// GetChart returns the referenced chart.
func (m *manager) GetChart(reference string) (*chart.Chart, error) {
	c, _, err := m.repoProvider.GetChartByReference(reference)
	if err != nil {
		return nil, err
	}

	return c, nil
}

// ListRepos returns the list of available repository URLs
func (m *manager) ListRepos() (map[string]string, error) {
	return m.service.ListRepos()
}

// AddRepo adds a repository to the list
func (m *manager) AddRepo(addition repo.IRepo) error {
	return m.service.CreateRepo(addition)
}

// RemoveRepo removes a repository from the list by URL
func (m *manager) RemoveRepo(repoName string) error {
	repoURL, err := m.service.GetRepoURLByName(repoName)
	if err != nil {
		return err
	}

	return m.service.DeleteRepo(repoURL)
}

// GetRepo returns the repository with the given name
func (m *manager) GetRepo(repoName string) (repo.IRepo, error) {
	repoURL, err := m.service.GetRepoURLByName(repoName)
	if err != nil {
		return nil, err
	}

	return m.service.GetRepoByURL(repoURL)
}

func generateManifestName() string {
	return fmt.Sprintf("manifest-%d", time.Now().UTC().UnixNano())
}

func failState(e error) *common.DeploymentState {
	return &common.DeploymentState{
		Status: common.FailedStatus,
		Errors: []string{e.Error()},
	}
}

func getResourceErrors(c *common.Configuration) []string {
	var errs []string
	for _, r := range c.Resources {
		if r.State.Status == common.Failed {
			errs = append(errs, r.State.Errors...)
		}
	}

	return errs
}

// ListRepoCharts lists charts in a given repository whose names
// conform to the supplied regular expression, or all charts, if the regular
// expression is nil.
func (m *manager) ListRepoCharts(repoName string, regex *regexp.Regexp) ([]string, error) {
	repoURL, err := m.service.GetRepoURLByName(repoName)
	if err != nil {
		return nil, err
	}

	r, err := m.repoProvider.GetRepoByURL(repoURL)
	if err != nil {
		return nil, err
	}

	return r.ListCharts(regex)
}

// GetChartForRepo returns a chart by name from a given repository.
func (m *manager) GetChartForRepo(repoName, chartName string) (*chart.Chart, error) {
	repoURL, err := m.service.GetRepoURLByName(repoName)
	if err != nil {
		return nil, err
	}

	r, err := m.repoProvider.GetRepoByURL(repoURL)
	if err != nil {
		return nil, err
	}

	return r.GetChart(chartName)
}

// CreateCredential creates a credential that can be used to authenticate to repository
func (m *manager) CreateCredential(name string, c *repo.Credential) error {
	return m.credentialProvider.SetCredential(name, c)
}

func (m *manager) GetCredential(name string) (*repo.Credential, error) {
	return m.credentialProvider.GetCredential(name)
}
