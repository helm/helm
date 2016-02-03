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

	"fmt"
)

type InmemCredentialProvider struct {
	credentials map[string]*common.RegistryCredential
}

func NewInmemCredentialProvider() common.CredentialProvider {
	return &InmemCredentialProvider{credentials: make(map[string]*common.RegistryCredential)}
}

func (fcp *InmemCredentialProvider) GetCredential(name string) (*common.RegistryCredential, error) {
	if val, ok := fcp.credentials[name]; ok {
		return val, nil
	}
	return nil, fmt.Errorf("no such credential : %s", name)
}

func (fcp *InmemCredentialProvider) SetCredential(name string, credential *common.RegistryCredential) error {
	fcp.credentials[name] = &common.RegistryCredential{credential.APIToken, credential.BasicAuth, credential.ServiceAccount}
	return nil
}
