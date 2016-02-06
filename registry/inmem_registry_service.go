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
	"github.com/kubernetes/deployment-manager/common"
	"github.com/kubernetes/deployment-manager/util"

	"fmt"
	"strings"
)

type inmemRegistryService struct {
	registries map[string]*common.Registry
}

// NewInmemRegistryService returns a new memory based registry service.
func NewInmemRegistryService() common.RegistryService {
	rs := &inmemRegistryService{
		registries: make(map[string]*common.Registry),
	}

	pFormat := fmt.Sprintf("%s;%s", common.UnversionedRegistry, common.OneLevelRegistry)
	rs.Create(&common.Registry{
		Name:           "charts",
		Type:           common.GithubRegistryType,
		URL:            "github.com/helm/charts",
		Format:         common.RegistryFormat(pFormat),
		CredentialName: "default",
	})

	tFormat := fmt.Sprintf("%s;%s", common.VersionedRegistry, common.CollectionRegistry)
	rs.Create(&common.Registry{
		Name:           "application-dm-templates",
		Type:           common.GithubRegistryType,
		URL:            "github.com/kubernetes/application-dm-templates",
		Format:         common.RegistryFormat(tFormat),
		CredentialName: "default",
	})

	return rs
}

// List returns the list of known registries.
func (rs *inmemRegistryService) List() ([]*common.Registry, error) {
	ret := []*common.Registry{}
	for _, r := range rs.registries {
		ret = append(ret, r)
	}

	return ret, nil
}

// Create creates a registry.
func (rs *inmemRegistryService) Create(registry *common.Registry) error {
	rs.registries[registry.Name] = registry
	return nil
}

// Get returns a registry with a given name.
func (rs *inmemRegistryService) Get(name string) (*common.Registry, error) {
	r, ok := rs.registries[name]
	if !ok {
		return nil, fmt.Errorf("Failed to find registry named %s", name)
	}

	return r, nil
}

// GetRegistry returns a registry with a given name.
func (rs *inmemRegistryService) GetRegistry(name string) (*common.Registry, error) {
	r, ok := rs.registries[name]
	if !ok {
		return nil, fmt.Errorf("Failed to find registry named %s", name)
	}

	return r, nil
}

// Delete deletes the registry with a given name.
func (rs *inmemRegistryService) Delete(name string) error {
	_, ok := rs.registries[name]
	if !ok {
		return fmt.Errorf("Failed to find registry named %s", name)
	}

	delete(rs.registries, name)
	return nil
}

// GetByURL returns a registry that handles the types for a given URL.
func (rs *inmemRegistryService) GetByURL(URL string) (*common.Registry, error) {
	trimmed := util.TrimURLScheme(URL)
	for _, r := range rs.registries {
		if strings.HasPrefix(trimmed, util.TrimURLScheme(r.URL)) {
			return r, nil
		}
	}

	return nil, fmt.Errorf("Failed to find registry for url: %s", URL)
}

// GetRegistryByURL returns a registry that handles the types for a given URL.
func (rs *inmemRegistryService) GetRegistryByURL(URL string) (*common.Registry, error) {
	trimmed := util.TrimURLScheme(URL)
	for _, r := range rs.registries {
		if strings.HasPrefix(trimmed, util.TrimURLScheme(r.URL)) {
			return r, nil
		}
	}

	return nil, fmt.Errorf("Failed to find registry for url: %s", URL)
}
