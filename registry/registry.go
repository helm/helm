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

package registry

import (
	"strings"

	"github.com/kubernetes/deployment-manager/common"

	"net/url"
)

// Registry abstracts a registry that holds charts, which can be
// used in a Deployment Manager configuration. There can be multiple
// registry implementations.
type Registry interface {
	// GetRegistryName returns the name of this registry
	GetRegistryName() string
	// GetRegistryType returns the type of this registry.
	GetRegistryType() common.RegistryType
	// GetRegistryURL returns the URL for this registry.
	GetRegistryURL() string

	// ListCharts lists the versioned chart names in this registry.
	ListCharts() ([]string, error)
	// GetChart fetches the contents of a given chart.
	GetChart(chartName string) (*Chart, error)

	// Deprecated: Use ListCharts, instead.
	List() ([]Type, error)
	// Deprecated: Use GetChart, instead.
	GetURLs(t Type) ([]string, error)
}

type RegistryService interface {
	// List all the registries
	List() ([]*common.Registry, error)
	// Create a new registry
	Create(registry *common.Registry) error
	// Get a registry
	Get(name string) (*common.Registry, error)
	// Delete a registry
	Delete(name string) error
	// Find a registry that backs the given URL
	GetByURL(URL string) (*common.Registry, error)
}

// Deprecated: Use Chart, instead
type Type struct {
	Collection string
	Name       string
	Version    string
}

// ParseType takes a registry name and parses it into a *registry.Type.
func ParseType(name string) *Type {
	tt := &Type{}

	tList := strings.Split(name, ":")
	if len(tList) == 2 {
		tt.Version = tList[1]
	}

	cList := strings.Split(tList[0], "/")

	if len(cList) == 1 {
		tt.Name = tList[0]
	} else {
		tt.Collection = cList[0]
		tt.Name = cList[1]
	}
	return tt
}

type Chart struct {
	Name         string
	Version      SemVer
	RegistryURL  string
	DownloadURLs []url.URL

	// TODO(jackgr): Should the metadata be strongly typed?
	Metadata map[string]interface{}
}
