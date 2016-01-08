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
	"sync"

	"github.com/google/go-github/github"
	"github.com/kubernetes/deployment-manager/common"
)

// RegistryProvider returns factories for creating registry clients.
type RegistryProvider interface {
	GetRegistryByURL(URL string) (Registry, error)
	GetRegistryByName(registryName string) (Registry, error)
}

func NewDefaultRegistryProvider() RegistryProvider {
	registries := make(map[string]Registry)
	rs := NewInmemRegistryService()
	return &DefaultRegistryProvider{registries: registries, rs: rs}
}

type DefaultRegistryProvider struct {
	sync.RWMutex
	registries map[string]Registry
	rs         RegistryService
}

func (drp *DefaultRegistryProvider) GetRegistryByURL(URL string) (Registry, error) {
	drp.RLock()
	defer drp.RUnlock()

	ghr := drp.findRegistryByURL(URL)
	if ghr == nil {
		cr, err := drp.rs.GetByURL(URL)
		if err != nil {
			return nil, err
		}

		ghr, err := drp.getGithubRegistry(cr)
		if err != nil {
			return nil, err
		}

		drp.registries[ghr.GetRegistryName()] = ghr
	}

	return ghr, nil
}

func (drp *DefaultRegistryProvider) findRegistryByURL(URL string) Registry {
	for _, ghr := range drp.registries {
		if strings.HasPrefix(URL, ghr.GetRegistryURL()) {
			return ghr
		}
	}

	return nil
}

func (drp *DefaultRegistryProvider) GetRegistryByName(registryName string) (Registry, error) {
	drp.RLock()
	defer drp.RUnlock()

	ghr, ok := drp.registries[registryName]
	if !ok {
		cr, err := drp.rs.Get(registryName)
		if err != nil {
			return nil, err
		}

		ghr, err := drp.getGithubRegistry(cr)
		if err != nil {
			return nil, err
		}

		drp.registries[ghr.GetRegistryName()] = ghr
	}

	return ghr, nil
}

func (drp *DefaultRegistryProvider) getGithubRegistry(cr *common.Registry) (Registry, error) {
	// TODO(jackgr): Take owner and repository from cr instead of hard wiring
	if cr.Type == common.GithubRegistryType {
		switch cr.Format {
		case common.UnversionedRegistry:
			return NewGithubPackageRegistry("helm", "charts", github.NewClient(nil)), nil
		case common.VersionedRegistry:
			return NewGithubRegistry("kubernetes", "application-dm-templates", "", github.NewClient(nil)), nil
		default:
			return nil, fmt.Errorf("unknown registry format: %s", cr.Format)
		}
	}

	return nil, fmt.Errorf("unknown registry type: %s", cr.Type)
}
