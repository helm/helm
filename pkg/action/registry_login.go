/*
Copyright The Helm Authors.

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

package action

import (
	"io"

	"helm.sh/helm/v3/pkg/registry"
)

// RegistryLogin performs a registry login operation.
type RegistryLogin struct {
	cfg *Configuration
}

// NewRegistryLogin creates a new RegistryLogin object with the given configuration.
func NewRegistryLogin(cfg *Configuration) *RegistryLogin {
	return &RegistryLogin{
		cfg: cfg,
	}
}

// Run executes the registry login operation
func (a *RegistryLogin) Run(out io.Writer, hostname string, username string, password string, insecure bool) error {
	return a.cfg.RegistryClient.Login(
		hostname,
		registry.LoginOptBasicAuth(username, password),
		registry.LoginOptInsecure(insecure))
}
