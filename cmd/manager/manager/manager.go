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
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/kubernetes/deployment-manager/cmd/manager/repository"
	"github.com/kubernetes/deployment-manager/pkg/common"
	"github.com/kubernetes/deployment-manager/pkg/registry"
	"github.com/kubernetes/deployment-manager/pkg/util"
)

// Manager manages a persistent set of Deployments.
type Manager interface {
	// Deployments
	ListDeployments() ([]common.Deployment, error)
	GetDeployment(name string) (*common.Deployment, error)
	CreateDeployment(t *common.Template) (*common.Deployment, error)
	DeleteDeployment(name string, forget bool) (*common.Deployment, error)
	PutDeployment(name string, t *common.Template) (*common.Deployment, error)

	// Manifests
	ListManifests(deploymentName string) (map[string]*common.Manifest, error)
	GetManifest(deploymentName string, manifest string) (*common.Manifest, error)
	Expand(t *common.Template) (*common.Manifest, error)

	// Types
	ListTypes() ([]string, error)
	ListInstances(typeName string) ([]*common.TypeInstance, error)
	GetRegistryForType(typeName string) (string, error)
	GetMetadataForType(typeName string) (string, error)

	// Registries
	ListRegistries() ([]*common.Registry, error)
	CreateRegistry(pr *common.Registry) error
	GetRegistry(name string) (*common.Registry, error)
	DeleteRegistry(name string) error

	// Registry Types
	ListRegistryTypes(registryName string, regex *regexp.Regexp) ([]registry.Type, error)
	GetDownloadURLs(registryName string, t registry.Type) ([]*url.URL, error)
	GetFile(registryName string, url string) (string, error)

	// Credentials
	CreateCredential(name string, c *common.RegistryCredential) error
	GetCredential(name string) (*common.RegistryCredential, error)
}

type manager struct {
	expander           Expander
	deployer           Deployer
	repository         repository.Repository
	registryProvider   registry.RegistryProvider
	service            common.RegistryService
	credentialProvider common.CredentialProvider
}

// NewManager returns a new initialized Manager.
func NewManager(expander Expander,
	deployer Deployer,
	repository repository.Repository,
	registryProvider registry.RegistryProvider,
	service common.RegistryService,
	credentialProvider common.CredentialProvider) Manager {
	return &manager{expander, deployer, repository, registryProvider, service, credentialProvider}
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

// CreateDeployment expands the supplied template, creates the resulting
// configuration in the cluster, creates a new deployment that tracks it,
// and stores the deployment in the repository. Returns the deployment.
func (m *manager) CreateDeployment(t *common.Template) (*common.Deployment, error) {
	log.Printf("Creating deployment: %s", t.Name)
	_, err := m.repository.CreateDeployment(t.Name)
	if err != nil {
		log.Printf("CreateDeployment failed %v", err)
		return nil, err
	}

	manifest, err := m.createManifest(t)
	if err != nil {
		log.Printf("Manifest creation failed: %v", err)
		m.repository.SetDeploymentState(t.Name, failState(err))
		return nil, err
	}

	if err := m.repository.AddManifest(manifest); err != nil {
		log.Printf("AddManifest failed %v", err)
		m.repository.SetDeploymentState(t.Name, failState(err))
		return nil, err
	}

	actualConfig, err := m.deployer.CreateConfiguration(manifest.ExpandedConfig)
	if err != nil {
		// Deployment failed, mark as failed
		log.Printf("CreateConfiguration failed: %v", err)
		m.repository.SetDeploymentState(t.Name, failState(err))

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
			m.repository.SetDeploymentState(t.Name, failState(e))
			return nil, e
		}

		m.repository.SetDeploymentState(t.Name, &common.DeploymentState{Status: common.DeployedStatus})
	}

	// Update the manifest with the actual state of the reified resources
	manifest.ExpandedConfig = actualConfig
	if err := m.repository.SetManifest(manifest); err != nil {
		log.Printf("SetManifest failed %v", err)
		m.repository.SetDeploymentState(t.Name, failState(err))
		return nil, err
	}

	// Finally update the type instances for this deployment.
	m.setTypeInstances(t.Name, manifest.Name, manifest.Layout)
	return m.repository.GetValidDeployment(t.Name)
}

func (m *manager) createManifest(t *common.Template) (*common.Manifest, error) {
	et, err := m.expander.ExpandTemplate(t)
	if err != nil {
		log.Printf("Expansion failed %v", err)
		return nil, err
	}

	return &common.Manifest{
		Name:           generateManifestName(),
		Deployment:     t.Name,
		InputConfig:    t,
		ExpandedConfig: et.Config,
		Layout:         et.Layout,
	}, nil
}

func (m *manager) setTypeInstances(deploymentName string, manifestName string, layout *common.Layout) {
	m.repository.ClearTypeInstancesForDeployment(deploymentName)

	instances := make(map[string][]*common.TypeInstance)
	for i, r := range layout.Resources {
		addTypeInstances(&instances, r, deploymentName, manifestName, fmt.Sprintf("$.resources[%d]", i))
	}

	m.repository.AddTypeInstances(instances)
}

func addTypeInstances(instances *map[string][]*common.TypeInstance, r *common.LayoutResource, deploymentName string, manifestName string, jsonPath string) {
	// Add this resource.
	inst := &common.TypeInstance{
		Name:       r.Name,
		Type:       r.Type,
		Deployment: deploymentName,
		Manifest:   manifestName,
		Path:       jsonPath,
	}

	(*instances)[r.Type] = append((*instances)[r.Type], inst)

	// Add all sub resources if they exist.
	for i, sr := range r.Resources {
		addTypeInstances(instances, sr, deploymentName, manifestName, fmt.Sprintf("%s.resources[%d]", jsonPath, i))
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
	m.repository.ClearTypeInstancesForDeployment(name)
	return d, nil
}

// PutDeployment replaces the configuration of the deployment with
// the supplied identifier in the cluster, and returns the deployment.
func (m *manager) PutDeployment(name string, t *common.Template) (*common.Deployment, error) {
	_, err := m.repository.GetValidDeployment(name)
	if err != nil {
		return nil, err
	}

	manifest, err := m.createManifest(t)
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
	m.setTypeInstances(t.Name, manifest.Name, manifest.Layout)
	return m.repository.GetValidDeployment(t.Name)
}

func (m *manager) Expand(t *common.Template) (*common.Manifest, error) {
	et, err := m.expander.ExpandTemplate(t)
	if err != nil {
		log.Printf("Expansion failed %v", err)
		return nil, err
	}

	return &common.Manifest{
		ExpandedConfig: et.Config,
		Layout:         et.Layout,
	}, nil
}

func (m *manager) ListTypes() ([]string, error) {
	return m.repository.ListTypes()
}

func (m *manager) ListInstances(typeName string) ([]*common.TypeInstance, error) {
	return m.repository.GetTypeInstances(typeName)
}

// GetRegistryForType returns the registry where a type resides.
func (m *manager) GetRegistryForType(typeName string) (string, error) {
	_, r, err := registry.GetDownloadURLs(m.registryProvider, typeName)
	if err != nil {
		return "", err
	}

	return r.GetRegistryName(), nil
}

// GetMetadataForType returns the metadata for type.
func (m *manager) GetMetadataForType(typeName string) (string, error) {
	URLs, r, err := registry.GetDownloadURLs(m.registryProvider, typeName)
	if err != nil {
		return "", err
	}

	if len(URLs) < 1 {
		return "", nil
	}

	// If it's a chart, we want the provenance file
	fPath := URLs[0]
	if !strings.Contains(fPath, ".prov") {
		// It's not a chart, so we want the schema
		fPath += ".schema"
	}

	metadata, err := getFileFromRegistry(fPath, r)
	if err != nil {
		return "", fmt.Errorf("cannot get metadata for type (%s): %s", typeName, err)
	}

	return metadata, nil
}

// ListRegistries returns the list of registries
func (m *manager) ListRegistries() ([]*common.Registry, error) {
	return m.service.List()
}

func (m *manager) CreateRegistry(pr *common.Registry) error {
	return m.service.Create(pr)
}

func (m *manager) GetRegistry(name string) (*common.Registry, error) {
	return m.service.Get(name)
}

func (m *manager) DeleteRegistry(name string) error {
	return m.service.Delete(name)
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

// ListRegistryTypes lists types in a given registry whose string values
// conform to the supplied regular expression, or all types, if the regular
// expression is nil.
func (m *manager) ListRegistryTypes(registryName string, regex *regexp.Regexp) ([]registry.Type, error) {
	r, err := m.registryProvider.GetRegistryByName(registryName)
	if err != nil {
		return nil, err
	}

	return r.ListTypes(regex)
}

// GetDownloadURLs returns the URLs required to download the contents
// of a given type in a given registry.
func (m *manager) GetDownloadURLs(registryName string, t registry.Type) ([]*url.URL, error) {
	r, err := m.registryProvider.GetRegistryByName(registryName)
	if err != nil {
		return nil, err
	}

	return r.GetDownloadURLs(t)
}

// GetFile returns a file from the backing registry
func (m *manager) GetFile(registryName string, url string) (string, error) {
	r, err := m.registryProvider.GetRegistryByName(registryName)
	if err != nil {
		return "", err
	}

	return getFileFromRegistry(url, r)
}

func getFileFromRegistry(url string, r registry.Registry) (string, error) {
	getter := util.NewHTTPClient(3, r, util.NewSleeper())
	body, _, err := getter.Get(url)
	if err != nil {
		return "", err
	}

	return body, nil
}

// CreateCredential creates a credential that can be used to authenticate to registry
func (m *manager) CreateCredential(name string, c *common.RegistryCredential) error {
	return m.credentialProvider.SetCredential(name, c)
}

func (m *manager) GetCredential(name string) (*common.RegistryCredential, error) {
	return m.credentialProvider.GetCredential(name)
}
