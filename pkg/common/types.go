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

// Expander controls how template/ is evaluated.
type Expander struct {
	// Currently just Expandybird or GoTemplate
	Name string `json:"name"`
	// During evaluation, which file to start from.
	Entrypoint string `json:"entry_point"`
}

// ChartFile is a file in a chart that is not chart.yaml.
type ChartFile struct {
	Path    string `json:"path"`    // Path from the root of the chart.
	Content string `json:"content"` // Base64 encoded file content.
}

// Chart is our internal representation of the chart.yaml (in structured form) + all supporting files.
type Chart struct {
	Name     string       `json:"name"`
	Expander *Expander    `json:"expander"`
	Schema   interface{}  `json:"schema"`
	Files    []*ChartFile `json:"files"`
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
	Path    string `json:"path,omitempty"` // Actual URL for the file
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
	APIVersion string                 `json:"apiVersion"`
	Metadata   map[string]interface{} `json:"metadata"`
	Spec       map[string]interface{} `json:"spec"`
}

// KubernetesSecret represents a Kubernetes secret
type KubernetesSecret struct {
	Kind       string            `json:"kind"`
	APIVersion string            `json:"apiVersion"`
	Metadata   map[string]string `json:"metadata"`
	Data       map[string]string `json:"data,omitempty"`
}

// Repository related types

// BasicAuthCredential holds a username and password.
type BasicAuthCredential struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// APITokenCredential defines an API token.
type APITokenCredential string

// JWTTokenCredential defines a JWT token.
type JWTTokenCredential string

// RegistryCredential holds a credential used to access a registry.
type RegistryCredential struct {
	APIToken       APITokenCredential  `json:"apitoken,omitempty"`
	BasicAuth      BasicAuthCredential `json:"basicauth,omitempty"`
	ServiceAccount JWTTokenCredential  `json:"serviceaccount,omitempty"`
}

// Registry describes a template registry
// TODO(jackr): Fix ambiguity re: whether or not URL has a scheme.
type Registry struct {
	Name           string         `json:"name,omitempty"`           // Friendly name for the registry
	Type           RegistryType   `json:"type,omitempty"`           // Technology implementing the registry
	URL            string         `json:"url,omitempty"`            // URL to the root of the registry
	Format         RegistryFormat `json:"format,omitempty"`         // Format of the registry
	CredentialName string         `json:"credentialname,omitempty"` // Name of the credential to use
}

// RegistryType defines the technology that implements the registry
type RegistryType string

// Constants that identify the supported registry layouts.
const (
	GithubRegistryType RegistryType = "github"
	GCSRegistryType    RegistryType = "gcs"
)

// RegistryFormat is a semi-colon delimited string that describes the format
// of a registry.
type RegistryFormat string

const (
	// Versioning.

	// VersionedRegistry identifies a versioned registry, where types appear under versions.
	VersionedRegistry RegistryFormat = "versioned"
	// UnversionedRegistry identifies an unversioned registry, where types appear under their names.
	UnversionedRegistry RegistryFormat = "unversioned"

	// Organization.

	// CollectionRegistry identfies a collection registry, where types are grouped into collections.
	CollectionRegistry RegistryFormat = "collection"
	// OneLevelRegistry identifies a one level registry, where all types appear at the top level.
	OneLevelRegistry RegistryFormat = "onelevel"
)

// RegistryService maintains a set of registries that defines the scope of all
// registry based operations, such as search and type resolution.
type RegistryService interface {
	// List all the registries
	List() ([]*Registry, error)
	// Create a new registry
	Create(registry *Registry) error
	// Get a registry
	Get(name string) (*Registry, error)
	// Get a registry with credential.
	GetRegistry(name string) (*Registry, error)
	// Delete a registry
	Delete(name string) error
	// Find a registry that backs the given URL
	GetByURL(URL string) (*Registry, error)
	// GetRegistryByURL returns a registry that handles the types for a given URL.
	GetRegistryByURL(URL string) (*Registry, error)
}

// CredentialProvider provides credentials for registries.
type CredentialProvider interface {
	// Set the credential for a registry.
	// May not be supported by some registry services.
	SetCredential(name string, credential *RegistryCredential) error

	// GetCredential returns the specified credential or nil if there's no credential.
	// Error is non-nil if fetching the credential failed.
	GetCredential(name string) (*RegistryCredential, error)
}
