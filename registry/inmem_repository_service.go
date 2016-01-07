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

type inmemRepositoryService struct {
	repositories map[string]*common.Registry
}

func NewInmemRepositoryService() RegistryService {
	rs := &inmemRepositoryService{
		repositories: make(map[string]*common.Registry),
	}
	rs.Create(&common.Registry{
		Name:   "charts",
		Type:   common.Github,
		URL:    "github.com/helm/charts",
		Format: common.UnversionedRegistry,
	})
	rs.Create(&common.Registry{
		Name:   "application-dm-templates",
		Type:   common.Github,
		URL:    "github.com/kubernetes/application-dm-templates",
		Format: common.VersionedRegistry,
	})
	return rs
}

func (rs *inmemRepositoryService) List() ([]*common.Registry, error) {
	ret := []*common.Registry{}
	for _, r := range rs.repositories {
		ret = append(ret, r)
	}
	return ret, nil
}

func (rs *inmemRepositoryService) Create(repository *common.Registry) error {
	rs.repositories[repository.URL] = repository
	return nil
}

func (rs *inmemRepositoryService) Get(name string) (*common.Registry, error) {
	return &common.Registry{}, nil
}

func (rs *inmemRepositoryService) Delete(name string) error {
	return nil
}

// GetByURL returns a registry that handles the types for a given URL.
func (rs *inmemRepositoryService) GetByURL(URL string) (*common.Registry, error) {
	for _, r := range rs.repositories {
		if strings.HasPrefix(URL, r.URL) {
			return r, nil
		}
	}
	return nil, fmt.Errorf("Failed to find registry for github url: %s", URL)
}
