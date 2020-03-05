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

package kube

import (
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/rest"

	"helm.sh/helm/v3/internal/version"
)

type ConfigFlags struct {
	genericclioptions.ConfigFlags
}

// ToRESTConfig implements RESTClientGetter.
// Returns a REST client configuration based on a provided path
// to a .kubeconfig file, loading rules, and config flag overrides.
// Expects the AddFlags method to have been called.
func (f *ConfigFlags) ToRESTConfig() (*rest.Config, error) {
	config, err := f.ToRawKubeConfigLoader().ClientConfig()
	if err != nil {
		return nil, err
	}
	config.UserAgent = version.GetUserAgent()
	return config, err
}
