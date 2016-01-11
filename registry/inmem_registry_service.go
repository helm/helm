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
	"fmt"
	"strings"

	"github.com/kubernetes/deployment-manager/common"
)

type inmemRegistryService struct {
	registries map[string]*common.Registry
}

func NewInmemRegistryService() common.RegistryService {
	rs := &inmemRegistryService{
		registries: make(map[string]*common.Registry),
	}

	pFormat := fmt.Sprintf("%s;%s", common.UnversionedRegistry, common.OneLevelRegistry)
	rs.Create(&common.Registry{
		Name:   "charts",
		Type:   common.GithubRegistryType,
		URL:    "github.com/helm/charts",
		Format: common.RegistryFormat(pFormat),
	})

	tFormat := fmt.Sprintf("%s;%s", common.VersionedRegistry, common.CollectionRegistry)
	rs.Create(&common.Registry{
		Name:   "application-dm-templates",
		Type:   common.GithubRegistryType,
		URL:    "github.com/kubernetes/application-dm-templates",
		Format: common.RegistryFormat(tFormat),
	})
	return rs
}

func (rs *inmemRegistryService) List() ([]*common.Registry, error) {
	ret := []*common.Registry{}
	for _, r := range rs.registries {
		ret = append(ret, r)
	}
	return ret, nil
}

func (rs *inmemRegistryService) Create(registry *common.Registry) error {
	rs.registries[registry.URL] = registry
	return nil
}

func (rs *inmemRegistryService) Get(name string) (*common.Registry, error) {
	return &common.Registry{}, nil
}

func (rs *inmemRegistryService) Delete(name string) error {
	return nil
}

// GetByURL returns a registry that handles the types for a given URL.
func (rs *inmemRegistryService) GetByURL(URL string) (*common.Registry, error) {
	for _, r := range rs.registries {
		if strings.HasPrefix(URL, r.URL) {
			return r, nil
		}
	}
	return nil, fmt.Errorf("Failed to find registry for github url: %s", URL)
}
