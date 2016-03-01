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

// Package repository defines a deployment repository.
package repository

import (
	"github.com/kubernetes/deployment-manager/pkg/common"
)

// Repository manages storage for all Deployment Manager entities, as well as
// the common operations to store, access and manage them.
type Repository interface {
	// Deployments.
	ListDeployments() ([]common.Deployment, error)
	GetDeployment(name string) (*common.Deployment, error)
	GetValidDeployment(name string) (*common.Deployment, error)
	CreateDeployment(name string) (*common.Deployment, error)
	DeleteDeployment(name string, forget bool) (*common.Deployment, error)
	SetDeploymentState(name string, state *common.DeploymentState) error

	// Manifests.
	AddManifest(manifest *common.Manifest) error
	SetManifest(manifest *common.Manifest) error
	ListManifests(deploymentName string) (map[string]*common.Manifest, error)
	GetManifest(deploymentName string, manifestName string) (*common.Manifest, error)
	GetLatestManifest(deploymentName string) (*common.Manifest, error)

	// Types.
	ListTypes() ([]string, error)
	GetTypeInstances(typeName string) ([]*common.TypeInstance, error)
	ClearTypeInstancesForDeployment(deploymentName string) error
	AddTypeInstances(instances map[string][]*common.TypeInstance) error

	Close()
}
