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
	"time"
)

// Manager manages a persistent set of Deployments.
type Manager interface {
	ListDeployments() ([]Deployment, error)
	GetDeployment(name string) (*Deployment, error)
	CreateDeployment(t *Template) (*Deployment, error)
	DeleteDeployment(name string, forget bool) (*Deployment, error)
	PutDeployment(name string, t *Template) (*Deployment, error)
	ListManifests(deploymentName string) (map[string]*Manifest, error)
	GetManifest(deploymentName string, manifest string) (*Manifest, error)
	Expand(t *Template) (*Manifest, error)
	ListTypes() []string
	ListInstances(typeName string) []*TypeInstance
}

type manager struct {
	expander   Expander
	deployer   Deployer
	repository Repository
}

// NewManager returns a new initialized Manager.
func NewManager(expander Expander, deployer Deployer, repository Repository) Manager {
	return &manager{expander, deployer, repository}
}

// ListDeployments returns the list of deployments
func (m *manager) ListDeployments() ([]Deployment, error) {
	l, err := m.repository.ListDeployments()
	if err != nil {
		return nil, err
	}
	return l, nil
}

// GetDeployment retrieves the configuration stored for a given deployment
// as well as the current configuration from the cluster.
func (m *manager) GetDeployment(name string) (*Deployment, error) {
	d, err := m.repository.GetDeployment(name)
	if err != nil {
		return nil, err
	}

	return d, nil
}

// ListManifests retrieves the manifests for a given deployment
// of each of the deployments in the repository and returns the deployments.
func (m *manager) ListManifests(deploymentName string) (map[string]*Manifest, error) {
	l, err := m.repository.ListManifests(deploymentName)
	if err != nil {
		return nil, err
	}

	return l, nil
}

// GetManifest retrieves the specified manifest for a given deployment
func (m *manager) GetManifest(deploymentName string, manifestName string) (*Manifest, error) {
	d, err := m.repository.GetManifest(deploymentName, manifestName)
	if err != nil {
		return nil, err
	}

	return d, nil
}

// CreateDeployment expands the supplied template, creates the resulting
// configuration in the cluster, creates a new deployment that tracks it,
// and stores the deployment in the repository. Returns the deployment.
func (m *manager) CreateDeployment(t *Template) (*Deployment, error) {
	log.Printf("Creating deployment: %s", t.Name)
	_, err := m.repository.CreateDeployment(t.Name)
	if err != nil {
		log.Printf("CreateDeployment failed %v", err)
		return nil, err
	}

	manifest, err := m.createManifest(t)
	if err != nil {
		log.Printf("Manifest creation failed: %v", err)
		m.repository.SetDeploymentStatus(t.Name, FailedStatus)
		return nil, err
	}

	actualConfig, createErr := m.deployer.CreateConfiguration(manifest.ExpandedConfig)
	if createErr != nil {
		// Deployment failed, mark as failed
		log.Printf("CreateConfiguration failed: %v", err)
		m.repository.SetDeploymentStatus(t.Name, FailedStatus)
		// If we failed before being able to create some of the resources, then
		// return the failure as such. Otherwise, we're going to add the manifest
		// and hence resource specific errors down below.
		if actualConfig == nil {
			return nil, createErr
		}
	} else {
		m.repository.SetDeploymentStatus(t.Name, DeployedStatus)
	}

	// Update the manifest with the actual state of the reified resources
	manifest.ExpandedConfig = actualConfig
	aErr := m.repository.AddManifest(t.Name, manifest)
	if aErr != nil {
		log.Printf("AddManifest failed %v", aErr)
		m.repository.SetDeploymentStatus(t.Name, FailedStatus)
		// If there's an earlier error, return that instead since it contains
		// more applicable error message. Adding manifest failure is more akin
		// to a check fail (either deployment doesn't exist, or a manifest with the same
		// name already exists).
		// TODO(vaikas): Should we combine both errors and return a nicely formatted error for both?
		if createErr != nil {
			return nil, createErr
		} else {
			return nil, aErr
		}
	}

	// Finally update the type instances for this deployment.
	m.addTypeInstances(t.Name, manifest.Name, manifest.Layout)
	return m.repository.GetValidDeployment(t.Name)
}

func (m *manager) createManifest(t *Template) (*Manifest, error) {
	et, err := m.expander.ExpandTemplate(t)
	if err != nil {
		log.Printf("Expansion failed %v", err)
		return nil, err
	}

	return &Manifest{
		Name:           generateManifestName(),
		Deployment:     t.Name,
		InputConfig:    t,
		ExpandedConfig: et.Config,
		Layout:         et.Layout,
	}, nil
}

func (m *manager) addTypeInstances(deploymentName string, manifestName string, layout *Layout) {
	m.repository.ClearTypeInstances(deploymentName)

	instances := make(map[string][]*TypeInstance)
	for i, r := range layout.Resources {
		addTypeInstances(&instances, r, deploymentName, manifestName, fmt.Sprintf("$.resources[%d]", i))
	}

	m.repository.SetTypeInstances(deploymentName, instances)
}

func addTypeInstances(instances *map[string][]*TypeInstance, r *LayoutResource, deploymentName string, manifestName string, jsonPath string) {
	// Add this resource.
	inst := &TypeInstance{
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
func (m *manager) DeleteDeployment(name string, forget bool) (*Deployment, error) {
	log.Printf("Deleting deployment: %s", name)
	d, err := m.repository.GetValidDeployment(name)
	if err != nil {
		return nil, err
	}

	// If there's a latest manifest, delete the underlying resources.
	latest, err := m.repository.GetLatestManifest(name)
	if err != nil {
		return nil, err
	}

	if latest != nil {
		log.Printf("Deleting resources from the latest manifest")
		if _, err := m.deployer.DeleteConfiguration(latest.ExpandedConfig); err != nil {
			log.Printf("Failed to delete resources from the latest manifest: %v", err)
			return nil, err
		}

		// Create an empty manifest since resources have been deleted.
		err = m.repository.AddManifest(name, &Manifest{Deployment: name, Name: generateManifestName()})
		if err != nil {
			log.Printf("Failed to add empty manifest")
			return nil, err
		}
	}

	d, err = m.repository.DeleteDeployment(name, forget)
	if err != nil {
		return nil, err
	}

	// Finally remove the type instances for this deployment.
	m.repository.ClearTypeInstances(name)

	return d, nil
}

// PutDeployment replaces the configuration of the deployment with
// the supplied identifier in the cluster, and returns the deployment.
func (m *manager) PutDeployment(name string, t *Template) (*Deployment, error) {
	_, err := m.repository.GetValidDeployment(name)
	if err != nil {
		return nil, err
	}

	manifest, err := m.createManifest(t)
	if err != nil {
		log.Printf("Manifest creation failed: %v", err)
		m.repository.SetDeploymentStatus(name, FailedStatus)
		return nil, err
	}

	actualConfig, err := m.deployer.PutConfiguration(manifest.ExpandedConfig)
	if err != nil {
		m.repository.SetDeploymentStatus(name, FailedStatus)
		return nil, err
	}

	manifest.ExpandedConfig = actualConfig
	err = m.repository.AddManifest(t.Name, manifest)
	if err != nil {
		m.repository.SetDeploymentStatus(name, FailedStatus)
		return nil, err
	}

	// Finally update the type instances for this deployment.
	m.addTypeInstances(t.Name, manifest.Name, manifest.Layout)

	return m.repository.GetValidDeployment(t.Name)
}

func (m *manager) Expand(t *Template) (*Manifest, error) {
	et, err := m.expander.ExpandTemplate(t)
	if err != nil {
		log.Printf("Expansion failed %v", err)
		return nil, err
	}

	return &Manifest{
		ExpandedConfig: et.Config,
		Layout:         et.Layout,
	}, nil
}

func (m *manager) ListTypes() []string {
	return m.repository.ListTypes()
}

func (m *manager) ListInstances(typeName string) []*TypeInstance {
	return m.repository.GetTypeInstances(typeName)
}

func generateManifestName() string {
	return fmt.Sprintf("manifest-%d", time.Now().UTC().UnixNano())
}
