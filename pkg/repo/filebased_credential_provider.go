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
	"github.com/ghodss/yaml"

	"fmt"
	"io/ioutil"
	"log"
)

// FilebasedCredentialProvider provides credentials for registries.
type FilebasedCredentialProvider struct {
	// Actual backing store
	backingCredentialProvider CredentialProvider
}

// NamedRepoCredential associates a name with a RepoCredential.
type NamedRepoCredential struct {
	Name string `json:"name,omitempty"`
	RepoCredential
}

// NewFilebasedCredentialProvider creates a file based credential provider.
func NewFilebasedCredentialProvider(filename string) (CredentialProvider, error) {
	icp := NewInmemCredentialProvider()
	log.Printf("Using credentials file %s", filename)
	c, err := readCredentialsFile(filename)
	if err != nil {
		return nil, err
	}

	for _, nc := range c {
		log.Printf("Loading credential named %s", nc.Name)
		icp.SetCredential(nc.Name, &nc.RepoCredential)
	}

	return &FilebasedCredentialProvider{icp}, nil
}

func readCredentialsFile(filename string) ([]NamedRepoCredential, error) {
	bytes, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	return parseCredentials(bytes)
}

func parseCredentials(bytes []byte) ([]NamedRepoCredential, error) {
	r := []NamedRepoCredential{}
	if err := yaml.Unmarshal(bytes, &r); err != nil {
		return nil, fmt.Errorf("cannot unmarshal credentials file (%#v)", err)
	}

	return r, nil
}

// GetCredential returns a credential by name.
func (fcp *FilebasedCredentialProvider) GetCredential(name string) (*RepoCredential, error) {
	return fcp.backingCredentialProvider.GetCredential(name)
}

// SetCredential sets a credential by name.
func (fcp *FilebasedCredentialProvider) SetCredential(name string, credential *RepoCredential) error {
	return fmt.Errorf("SetCredential operation not supported with FilebasedCredentialProvider")
}
