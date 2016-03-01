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
	"io/ioutil"
	"log"

	"github.com/ghodss/yaml"
	"github.com/kubernetes/deployment-manager/pkg/common"
)

// FilebasedCredentialProvider provides credentials for registries.
type FilebasedCredentialProvider struct {
	// Actual backing store
	backingCredentialProvider common.CredentialProvider
}

// NamedRegistryCredential associates a name with a RegistryCredential.
type NamedRegistryCredential struct {
	Name string `json:"name,omitempty"`
	common.RegistryCredential
}

// NewFilebasedCredentialProvider creates a file based credential provider.
func NewFilebasedCredentialProvider(filename string) (common.CredentialProvider, error) {
	icp := NewInmemCredentialProvider()
	c, err := readCredentialsFile(filename)
	if err != nil {
		return &FilebasedCredentialProvider{}, err
	}
	for _, nc := range c {
		log.Printf("Adding credential %s", nc.Name)
		icp.SetCredential(nc.Name, &nc.RegistryCredential)
	}

	return &FilebasedCredentialProvider{icp}, nil
}

func readCredentialsFile(filename string) ([]NamedRegistryCredential, error) {
	bytes, err := ioutil.ReadFile(filename)
	if err != nil {
		return []NamedRegistryCredential{}, err
	}
	return parseCredentials(bytes)
}

func parseCredentials(bytes []byte) ([]NamedRegistryCredential, error) {
	r := []NamedRegistryCredential{}
	if err := yaml.Unmarshal(bytes, &r); err != nil {
		return []NamedRegistryCredential{}, fmt.Errorf("cannot unmarshal credentials file (%#v)", err)
	}
	return r, nil
}

// GetCredential returns a credential by name.
func (fcp *FilebasedCredentialProvider) GetCredential(name string) (*common.RegistryCredential, error) {
	return fcp.backingCredentialProvider.GetCredential(name)
}

// SetCredential sets a credential by name.
func (fcp *FilebasedCredentialProvider) SetCredential(name string, credential *common.RegistryCredential) error {
	return fmt.Errorf("SetCredential operation not supported with FilebasedCredentialProvider")
}
