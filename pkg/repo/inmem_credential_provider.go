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

package repo

import (
	"fmt"
	"sync"
)

// InmemCredentialProvider is a memory based credential provider.
type InmemCredentialProvider struct {
	sync.RWMutex
	credentials map[string]*Credential
}

// NewInmemCredentialProvider creates a new memory based credential provider.
func NewInmemCredentialProvider() ICredentialProvider {
	return &InmemCredentialProvider{credentials: make(map[string]*Credential)}
}

// GetCredential returns a credential by name.
func (fcp *InmemCredentialProvider) GetCredential(name string) (*Credential, error) {
	fcp.RLock()
	defer fcp.RUnlock()

	if val, ok := fcp.credentials[name]; ok {
		return val, nil
	}

	return nil, fmt.Errorf("no such credential: %s", name)
}

// SetCredential sets a credential by name.
func (fcp *InmemCredentialProvider) SetCredential(name string, credential *Credential) error {
	fcp.Lock()
	defer fcp.Unlock()

	fcp.credentials[name] = &Credential{APIToken: credential.APIToken, BasicAuth: credential.BasicAuth, ServiceAccount: credential.ServiceAccount}
	return nil
}
