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

// Package repository implements a deployment repository using a map.
// It can be easily replaced by a deployment repository that uses some
// form of persistent storage.
package repository

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/kubernetes/deployment-manager/manager/manager"
)

// deploymentTypeInstanceMap stores type instances mapped by deployment name.
// This allows for simple updating and deleting of per-deployment instances
// when deployments are created/updated/deleted.
type deploymentTypeInstanceMap map[string][]*manager.TypeInstance
type typeInstanceMap map[string]deploymentTypeInstanceMap

type mapBasedRepository struct {
	sync.RWMutex
	deployments map[string]manager.Deployment
	instances   typeInstanceMap
	lastID      int
}

// NewMapBasedRepository returns a new map based repository.
func NewMapBasedRepository() manager.Repository {
	return &mapBasedRepository{
		deployments: make(map[string]manager.Deployment, 0),
		instances:   typeInstanceMap{},
	}
}

// ListDeployments returns of all of the deployments in the repository.
func (r *mapBasedRepository) ListDeployments() ([]manager.Deployment, error) {
	r.RLock()
	defer r.RUnlock()

	l := []manager.Deployment{}
	for _, deployment := range r.deployments {
		l = append(l, deployment)
	}

	return l, nil
}

// GetDeployment returns the deployment with the supplied name.
// If the deployment is not found, it returns an error.
func (r *mapBasedRepository) GetDeployment(name string) (*manager.Deployment, error) {
	d, ok := r.deployments[name]
	if !ok {
		return nil, fmt.Errorf("deployment %s not found", name)
	}
	return &d, nil
}

// GetValidDeployment returns the deployment with the supplied name.
// If the deployment is not found or marked as deleted, it returns an error.
func (r *mapBasedRepository) GetValidDeployment(name string) (*manager.Deployment, error) {
	d, err := r.GetDeployment(name)
	if err != nil {
		return nil, err
	}

	if d.Status == manager.DeletedStatus {
		return nil, fmt.Errorf("deployment %s is deleted", name)
	}

	return d, nil
}

// CreateDeployment creates a new deployment and stores it in the repository.
func (r *mapBasedRepository) CreateDeployment(name string) (*manager.Deployment, error) {
	d, err := func() (*manager.Deployment, error) {
		r.Lock()
		defer r.Unlock()

		exists, _ := r.GetValidDeployment(name)
		if exists != nil {
			return nil, fmt.Errorf("Deployment %s already exists", name)
		}

		r.lastID++

		d := manager.NewDeployment(name, r.lastID)
		d.Status = manager.CreatedStatus
		d.DeployedAt = time.Now()
		r.deployments[name] = *d
		return d, nil
	}()

	if err != nil {
		return nil, err
	}

	log.Printf("created deployment: %v", d)
	return d, nil
}

func (r *mapBasedRepository) AddManifest(deploymentName string, manifest *manager.Manifest) error {
	err := func() error {
		r.Lock()
		defer r.Unlock()
		d, err := r.GetValidDeployment(deploymentName)
		if err != nil {
			return err
		}

		// Make sure the manifest doesn't already exist, and if not, add the manifest to
		// map of manifests this deployment has
		if _, ok := d.Manifests[manifest.Name]; ok {
			return fmt.Errorf("Manifest %s already exists in deployment %s", manifest.Name, deploymentName)
		}
		d.Manifests[manifest.Name] = manifest
		r.deployments[deploymentName] = *d
		return nil
	}()
	if err != nil {
		return err
	}
	log.Printf("Added manifest %s to deployment: %s", manifest.Name, deploymentName)
	return nil
}

// DeleteDeployment deletes the deployment with the supplied name.
// If forget is true, then the deployment is removed from the repository.
// Otherwise, it is marked as deleted and retained.
func (r *mapBasedRepository) DeleteDeployment(name string, forget bool) (*manager.Deployment, error) {
	d, err := func() (*manager.Deployment, error) {
		r.Lock()
		defer r.Unlock()

		d, err := r.GetValidDeployment(name)
		if err != nil {
			return nil, err
		}

		if !forget {
			d.DeletedAt = time.Now()
			d.Status = manager.DeletedStatus
			r.deployments[name] = *d
		} else {
			delete(r.deployments, name)
		}

		return d, nil
	}()

	if err != nil {
		return nil, err
	}

	log.Printf("deleted deployment: %v", d)
	return d, nil
}

func (r *mapBasedRepository) ListManifests(deploymentName string) (map[string]*manager.Manifest, error) {
	d, err := r.GetValidDeployment(deploymentName)
	if err != nil {
		return nil, err
	}
	return d.Manifests, nil
}

func (r *mapBasedRepository) GetManifest(deploymentName string, manifestName string) (*manager.Manifest, error) {
	d, err := r.GetValidDeployment(deploymentName)
	if err != nil {
		return nil, err
	}
	if m, ok := d.Manifests[manifestName]; ok {
		return m, nil
	}
	return nil, fmt.Errorf("manifest %s not found in deployment %s", manifestName, deploymentName)
}

// ListTypes returns all types known from existing instances.
func (r *mapBasedRepository) ListTypes() []string {
	var keys []string
	for k := range r.instances {
		keys = append(keys, k)
	}

	return keys
}

// GetTypeInstances returns all instances of a given type. If type is empty,
// returns all instances for all types.
func (r *mapBasedRepository) GetTypeInstances(typeName string) []*manager.TypeInstance {
	r.Lock()
	defer r.Unlock()

	var instances []*manager.TypeInstance
	for t, dInstMap := range r.instances {
		if t == typeName || typeName == "all" {
			for _, i := range dInstMap {
				instances = append(instances, i...)
			}
		}
	}

	return instances
}

// ClearTypeInstances deletes all instances associated with the given
// deployment name from the type instance repository.
func (r *mapBasedRepository) ClearTypeInstances(deploymentName string) {
	r.Lock()
	defer r.Unlock()

	for t, dMap := range r.instances {
		delete(dMap, deploymentName)
		if len(dMap) == 0 {
			delete(r.instances, t)
		}
	}
}

// SetTypeInstances sets all type instances for a given deployment name.
//
// To clear the current set of instances first, caller should first use
// ClearTypeInstances().
func (r *mapBasedRepository) SetTypeInstances(deploymentName string, instances map[string][]*manager.TypeInstance) {
	r.Lock()
	defer r.Unlock()

	// Add each instance list to the appropriate type map.
	for t, is := range instances {
		if r.instances[t] == nil {
			r.instances[t] = make(deploymentTypeInstanceMap)
		}

		r.instances[t][deploymentName] = is
	}
}
