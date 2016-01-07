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

	"github.com/google/go-github/github"
	"github.com/kubernetes/deployment-manager/common"
)

// RegistryProvider returns factories for creating registries for a given RegistryType.
type RegistryProvider interface {
	GetRegistry(prefix string) (Registry, error)
}

func NewDefaultRegistryProvider() RegistryProvider {
	rs := NewInmemRepositoryService()
	return &DefaultRegistryProvider{rs: rs}

}

type DefaultRegistryProvider struct {
	rs RegistryService
}

func (drp *DefaultRegistryProvider) GetRegistry(URL string) (Registry, error) {
	r, err := drp.rs.GetByURL(URL)
	if err != nil {
		return nil, err
	}
	if r.Type == common.Github {
		if r.Format == common.UnversionedRegistry {
			return NewGithubPackageRegistry("helm", "charts", github.NewClient(nil)), nil
		}
		if r.Format == common.VersionedRegistry {
			return NewGithubRegistry("kubernetes", "application-dm-templates", "", github.NewClient(nil)), nil
		}
	}
	return nil, fmt.Errorf("cannot find registry backing url %s", URL)
}
