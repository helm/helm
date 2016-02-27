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

// Package transient implements a transient deployment repository.
package transient

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/kubernetes/deployment-manager/pkg/common"
	"github.com/kubernetes/deployment-manager/cmd/manager/repository"
)

// deploymentTypeInstanceMap stores type instances mapped by deployment name.
// This allows for simple updating and deleting of per-deployment instances
// when deployments are created/updated/deleted.
type deploymentTypeInstanceMap map[string][]*common.TypeInstance

type tRepository struct {
	sync.RWMutex
	deployments map[string]common.Deployment
	manifests   map[string]map[string]*common.Manifest
	instances   map[string]deploymentTypeInstanceMap
}

// NewRepository returns a new transient repository. Its lifetime is coupled
// to the lifetime of the current process. When the process dies, its contents
// will be permanently destroyed.
func NewRepository() repository.Repository {
	return &tRepository{
		deployments: make(map[string]common.Deployment, 0),
		manifests:   make(map[string]map[string]*common.Manifest, 0),
		instances:   make(map[string]deploymentTypeInstanceMap, 0),
	}
}

func (r *tRepository) Close() {
	r.deployments = make(map[string]common.Deployment, 0)
	r.manifests = make(map[string]map[string]*common.Manifest, 0)
	r.instances = make(map[string]deploymentTypeInstanceMap, 0)
}

// ListDeployments returns of all of the deployments in the repository.
func (r *tRepository) ListDeployments() ([]common.Deployment, error) {
	l := []common.Deployment{}
	for _, deployment := range r.deployments {
		l = append(l, deployment)
	}

	return l, nil
}

// GetDeployment returns the deployment with the supplied name.
// If the deployment is not found, it returns an error.
func (r *tRepository) GetDeployment(name string) (*common.Deployment, error) {
	d, ok := r.deployments[name]
	if !ok {
		return nil, fmt.Errorf("deployment %s not found", name)
	}

	return &d, nil
}

// GetValidDeployment returns the deployment with the supplied name.
// If the deployment is not found or marked as deleted, it returns an error.
func (r *tRepository) GetValidDeployment(name string) (*common.Deployment, error) {
	d, err := r.GetDeployment(name)
	if err != nil {
		return nil, err
	}

	if d.State.Status == common.DeletedStatus {
		return nil, fmt.Errorf("deployment %s is deleted", name)
	}

	return d, nil
}

// SetDeploymentState sets the DeploymentState of the deployment and updates ModifiedAt
func (r *tRepository) SetDeploymentState(name string, state *common.DeploymentState) error {
	r.Lock()
	defer r.Unlock()

	d, err := r.GetValidDeployment(name)
	if err != nil {
		return err
	}

	d.State = state
	d.ModifiedAt = time.Now()
	r.deployments[name] = *d
	return nil
}

// CreateDeployment creates a new deployment and stores it in the repository.
func (r *tRepository) CreateDeployment(name string) (*common.Deployment, error) {
	r.Lock()
	defer r.Unlock()

	exists, _ := r.GetValidDeployment(name)
	if exists != nil {
		return nil, fmt.Errorf("Deployment %s already exists", name)
	}

	d := common.NewDeployment(name)
	r.deployments[name] = *d

	log.Printf("created deployment: %v", d)
	return d, nil
}

// AddManifest adds a manifest to the repository and repoints the latest
// manifest to it for the corresponding deployment.
func (r *tRepository) AddManifest(manifest *common.Manifest) error {
	r.Lock()
	defer r.Unlock()

	deploymentName := manifest.Deployment
	l, err := r.ListManifests(deploymentName)
	if err != nil {
		return err
	}

	// Make sure the manifest doesn't already exist, and if not, add the manifest to
	// map of manifests this deployment has
	if _, ok := l[manifest.Name]; ok {
		return fmt.Errorf("Manifest %s already exists in deployment %s", manifest.Name, deploymentName)
	}

	d, err := r.GetValidDeployment(deploymentName)
	if err != nil {
		return err
	}

	l[manifest.Name] = manifest
	d.LatestManifest = manifest.Name
	d.ModifiedAt = time.Now()
	r.deployments[deploymentName] = *d

	log.Printf("Added manifest %s to deployment: %s", manifest.Name, deploymentName)
	return nil
}

// SetManifest sets an existing manifest in the repository to provided manifest.
func (r *tRepository) SetManifest(manifest *common.Manifest) error {
	r.Lock()
	defer r.Unlock()

	l, err := r.ListManifests(manifest.Deployment)
	if err != nil {
		return err
	}

	if _, ok := l[manifest.Name]; !ok {
		return fmt.Errorf("manifest %s not found", manifest.Name)
	}

	l[manifest.Name] = manifest
	return nil
}

// DeleteDeployment deletes the deployment with the supplied name.
// If forget is true, then the deployment is removed from the repository.
// Otherwise, it is marked as deleted and retained.
func (r *tRepository) DeleteDeployment(name string, forget bool) (*common.Deployment, error) {
	r.Lock()
	defer r.Unlock()

	d, err := r.GetValidDeployment(name)
	if err != nil {
		return nil, err
	}

	if !forget {
		d.DeletedAt = time.Now()
		d.State = &common.DeploymentState{Status: common.DeletedStatus}
		r.deployments[name] = *d
	} else {
		delete(r.deployments, name)
		delete(r.manifests, name)
		d.LatestManifest = ""
	}

	log.Printf("deleted deployment: %v", d)
	return d, nil
}

func (r *tRepository) ListManifests(deploymentName string) (map[string]*common.Manifest, error) {
	_, err := r.GetValidDeployment(deploymentName)
	if err != nil {
		return nil, err
	}

	return r.listManifestsForDeployment(deploymentName)
}

func (r *tRepository) listManifestsForDeployment(deploymentName string) (map[string]*common.Manifest, error) {
	l, ok := r.manifests[deploymentName]
	if !ok {
		l = make(map[string]*common.Manifest, 0)
		r.manifests[deploymentName] = l
	}

	return l, nil
}

func (r *tRepository) GetManifest(deploymentName string, manifestName string) (*common.Manifest, error) {
	_, err := r.GetValidDeployment(deploymentName)
	if err != nil {
		return nil, err
	}

	return r.getManifestForDeployment(deploymentName, manifestName)
}

func (r *tRepository) getManifestForDeployment(deploymentName string, manifestName string) (*common.Manifest, error) {
	l, err := r.listManifestsForDeployment(deploymentName)
	if err != nil {
		return nil, err
	}

	m, ok := l[manifestName]
	if !ok {
		return nil, fmt.Errorf("manifest %s not found in deployment %s", manifestName, deploymentName)
	}

	return m, nil
}

// GetLatestManifest returns the latest manifest for a given deployment,
// which by definition is the manifest with the largest time stamp.
func (r *tRepository) GetLatestManifest(deploymentName string) (*common.Manifest, error) {
	d, err := r.GetValidDeployment(deploymentName)
	if err != nil {
		return nil, err
	}

	if d.LatestManifest == "" {
		return nil, nil
	}

	return r.getManifestForDeployment(deploymentName, d.LatestManifest)
}

// ListTypes returns all types known from existing instances.
func (r *tRepository) ListTypes() ([]string, error) {
	var keys []string
	for k := range r.instances {
		keys = append(keys, k)
	}

	return keys, nil
}

// GetTypeInstances returns all instances of a given type. If type is empty,
// returns all instances for all types.
func (r *tRepository) GetTypeInstances(typeName string) ([]*common.TypeInstance, error) {
	var instances []*common.TypeInstance
	for t, dInstMap := range r.instances {
		if t == typeName || typeName == "" || typeName == "all" {
			for _, i := range dInstMap {
				instances = append(instances, i...)
			}
		}
	}

	return instances, nil
}

// ClearTypeInstancesForDeployment deletes all type instances associated with the given
// deployment from the repository.
func (r *tRepository) ClearTypeInstancesForDeployment(deploymentName string) error {
	r.Lock()
	defer r.Unlock()

	for t, dMap := range r.instances {
		delete(dMap, deploymentName)
		if len(dMap) == 0 {
			delete(r.instances, t)
		}
	}

	return nil
}

// AddTypeInstances adds the supplied type instances to the repository.
func (r *tRepository) AddTypeInstances(instances map[string][]*common.TypeInstance) error {
	r.Lock()
	defer r.Unlock()

	// Add instances to the appropriate type and deployment maps.
	for t, is := range instances {
		if r.instances[t] == nil {
			r.instances[t] = make(deploymentTypeInstanceMap)
		}

		tmap := r.instances[t]
		for _, instance := range is {
			deployment := instance.Deployment
			if tmap[deployment] == nil {
				tmap[deployment] = make([]*common.TypeInstance, 0)
			}

			tmap[deployment] = append(tmap[deployment], instance)
		}
	}

	return nil
}
