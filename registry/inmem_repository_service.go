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
	return &inmemRepositoryService{
		repositories: make(map[string]*common.Registry),
	}
}

func (rs *inmemRepositoryService) List() ([]*common.Registry, error) {
	return nil, nil
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
func (rs *inmemRepositoryService) GetByURL(URL string) (*common.Registry, error) {
	if !strings.HasPrefix(URL, "github.com/") {
		return nil, fmt.Errorf("Failed to parse short github url: %s", URL)
	}
	s := strings.Split(URL, "/")
	if len(s) < 3 {
		panic(fmt.Errorf("invalid template registry: %s", URL))
	}

	toFind := "github.com/" + s[1] + "/" + s[2]
	fmt.Printf("toFind: %s", toFind)
	for _, r := range rs.repositories {
		fmt.Printf("Checking: %s", r.URL)
		if r.URL == toFind {
			return r, nil
		}
	}
	return nil, fmt.Errorf("Failed to find registry for github url: %s", URL)
}
