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

package expander

import (
	"github.com/kubernetes/helm/pkg/chart"
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

// LayoutResource defines the structure of resources in the manifest layout.
type LayoutResource struct {
	Resource
	Layout
}

// Layout defines the structure of a layout as returned from expansion.
type Layout struct {
	Resources []*LayoutResource `json:"resources,omitempty"`
}

// ExpansionRequest defines the API to expander.
type ExpansionRequest struct {
	ChartInvocation *Resource           `json:"chart_invocation"`
	Chart           *chart.ChartContent `json:"chart"`
}

// ExpansionResponse defines the API to expander.
type ExpansionResponse struct {
	Resources []interface{} `json:"resources"`
}

// Expander abstracts interactions with the expander and deployer services.
type Expander interface {
	ExpandChart(request *ExpansionRequest) (*ExpansionResponse, error)
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
