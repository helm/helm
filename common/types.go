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

package common

import (
	"time"
)

// SchemaImport represents an import as declared in a schema file.
type SchemaImport struct {
	Path string `json:"path"`
	Name string `json:"name"`
}

// Schema is a partial DM schema. We only need access to the imports object at this level.
type Schema struct {
	Imports []SchemaImport `json:"imports"`
}

// Deployment defines a deployment that describes
// the creation, modification and/or deletion of a set of resources.
type Deployment struct {
	Name           string           `json:"name"`
	ID             int              `json:"id"`
	CreatedAt      time.Time        `json:"createdAt,omitempty"`
	DeployedAt     time.Time        `json:"deployedAt,omitempty"`
	ModifiedAt     time.Time        `json:"modifiedAt,omitempty"`
	DeletedAt      time.Time        `json:"deletedAt,omitempty"`
	State          *DeploymentState `json:"state,omitempty"`
	LatestManifest string           `json:"latestManifest,omitEmpty"`
}

// NewDeployment creates a new deployment.
func NewDeployment(name string) *Deployment {
	return &Deployment{
		Name:      name,
		CreatedAt: time.Now(),
		State:     &DeploymentState{Status: CreatedStatus},
	}
}

// DeploymentState describes the state of a resource. It is set to the terminal
// state depending on the outcome of an operation on the deployment.
type DeploymentState struct {
	Status DeploymentStatus `json:"status,omitempty"`
	Errors []string         `json:"errors,omitempty"`
}

// DeploymentStatus is an enumeration type for the status of a deployment.
type DeploymentStatus string

// These constants implement the DeploymentStatus enumeration type.
const (
	CreatedStatus  DeploymentStatus = "Created"
	DeletedStatus  DeploymentStatus = "Deleted"
	DeployedStatus DeploymentStatus = "Deployed"
	FailedStatus   DeploymentStatus = "Failed"
	ModifiedStatus DeploymentStatus = "Modified"
)

func (s DeploymentStatus) String() string {
	return string(s)
}

// LayoutResource defines the structure of resources in the manifest layout.
type LayoutResource struct {
	Resource
	Layout
}

// Layout defines the structure of a layout as returned from expansion.
type Layout struct {
	Resources []*LayoutResource `json:"resources,omitempty"`
}

// Manifest contains the input configuration for a deployment, the fully
// expanded configuration, and the layout structure of the manifest.
//
type Manifest struct {
	Deployment     string         `json:"deployment,omitempty"`
	Name           string         `json:"name,omitempty"`
	InputConfig    *Template      `json:"inputConfig,omitempty"`
	ExpandedConfig *Configuration `json:"expandedConfig,omitempty"`
	Layout         *Layout        `json:"layout,omitempty"`
}

// Template describes a set of resources to be deployed.
// Manager expands a Template into a Configuration, which
// describes the set in a form that can be instantiated.
type Template struct {
	Name    string        `json:"name"`
	Content string        `json:"content"`
	Imports []*ImportFile `json:"imports"`
}

// ImportFile describes a base64 encoded file imported by a Template.
type ImportFile struct {
	Name    string `json:"name,omitempty"`
	Path    string `json:"path",omitempty` // Actual URL for the file
	Content string `json:"content"`
}

// Configuration describes a set of resources in a form
// that can be instantiated.
type Configuration struct {
	Resources []*Resource `json:"resources"`
}

// ResourceStatus is an enumeration type for the status of a resource.
type ResourceStatus string

// These constants implement the resourceStatus enumeration type.
const (
	Created ResourceStatus = "Created"
	Failed  ResourceStatus = "Failed"
	Aborted ResourceStatus = "Aborted"
)

// ResourceState describes the state of a resource.
// Status is set during resource creation and is a terminal state.
type ResourceState struct {
	Status   ResourceStatus `json:"status,omitempty"`
	SelfLink string         `json:"selflink,omitempty"`
	Errors   []string       `json:"errors,omitempty"`
}

// Resource describes a resource in a configuration. A resource has
// a name, a type and a set of properties. The name and type are used
// to identify the resource in Kubernetes. The properties are passed
// to Kubernetes as the resource configuration.
type Resource struct {
	Name       string                 `json:"name"`
	Type       string                 `json:"type"`
	Properties map[string]interface{} `json:"properties,omitempty"`
	State      *ResourceState         `json:"state,omitempty"`
}

// TypeInstance defines the metadata for an instantiation of a template type
// in a deployment.
type TypeInstance struct {
	Name       string `json:"name"`       // instance name
	Type       string `json:"type"`       // instance type
	Deployment string `json:"deployment"` // deployment name
	Manifest   string `json:"manifest"`   // manifest name
	Path       string `json:"path"`       // JSON path within manifest
}

// KubernetesObject represents a native 'bare' Kubernetes object.
type KubernetesObject struct {
	Kind       string                 `json:"kind"`
	ApiVersion string                 `json:"apiVersion"`
	Metadata   map[string]interface{} `json:"metadata"`
	Spec       map[string]interface{} `json:"spec"`
}
