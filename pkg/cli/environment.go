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

/*Package cli describes the operating environment for the Helm CLI.

Helm's environment encapsulates all of the service dependencies Helm has.
These dependencies are expressed as interfaces so that alternate implementations
(mocks, etc.) can be easily generated.
*/
package cli

import (
	"fmt"
	"os"

	"github.com/spf13/pflag"

	"helm.sh/helm/pkg/helmpath"
)

// EnvSettings describes all of the environment settings.
type EnvSettings struct {
	// Namespace is the namespace scope.
	Namespace string
	// KubeConfig is the path to the kubeconfig file.
	KubeConfig string
	// KubeContext is the name of the kubeconfig context.
	KubeContext string
	// Debug indicates whether or not Helm is running in Debug mode.
	Debug bool

	// RegistryConfig is the path to the registry config file.
	RegistryConfig string
	// RepositoryConfig is the path to the repositories file.
	RepositoryConfig string
	// Repositoryache is the path to the repository cache directory.
	RepositoryCache string
	// PluginsDirectory is the path to the plugins directory.
	PluginsDirectory string

	// Environment Variables Store.
	EnvironmentVariables map[string]string
}

func New() *EnvSettings {
	envSettings := EnvSettings{
		PluginsDirectory:     helmpath.DataPath("plugins"),
		RegistryConfig:       helmpath.ConfigPath("registry.json"),
		RepositoryConfig:     helmpath.ConfigPath("repositories.yaml"),
		RepositoryCache:      helmpath.CachePath("repository"),
		EnvironmentVariables: make(map[string]string),
	}
	envSettings.setHelmEnvVars()
	return &envSettings
}

// AddFlags binds flags to the given flagset.
func (s *EnvSettings) AddFlags(fs *pflag.FlagSet) {
	fs.StringVarP(&s.Namespace, "namespace", "n", "", "namespace scope for this request")
	fs.StringVar(&s.KubeConfig, "kubeconfig", "", "path to the kubeconfig file")
	fs.StringVar(&s.KubeContext, "kube-context", "", "name of the kubeconfig context to use")
	fs.BoolVar(&s.Debug, "debug", false, "enable verbose output")

	fs.StringVar(&s.RegistryConfig, "registry-config", s.RegistryConfig, "path to the registry config file")
	fs.StringVar(&s.RepositoryConfig, "repository-config", s.RepositoryConfig, "path to the file containing repository names and URLs")
	fs.StringVar(&s.RepositoryCache, "repository-cache", s.RepositoryCache, "path to the file containing cached repository indexes")
}

// envMap maps flag names to envvars
var envMap = map[string]string{
	"debug":             "HELM_DEBUG",
	"namespace":         "HELM_NAMESPACE",
	"registry-config":   "HELM_REGISTRY_CONFIG",
	"repository-config": "HELM_REPOSITORY_CONFIG",
}

func setFlagFromEnv(name, envar string, fs *pflag.FlagSet) {
	if fs.Changed(name) {
		return
	}
	if v, ok := os.LookupEnv(envar); ok {
		fs.Set(name, v)
	}
}

func (s *EnvSettings) setHelmEnvVars() {
	for key, val := range map[string]string{
		"HELM_HOME":              helmpath.DataPath(),
		"HELM_PATH_STARTER":      helmpath.DataPath("starters"),
		"HELM_DEBUG":             fmt.Sprint(s.Debug),
		"HELM_REGISTRY_CONFIG":   s.RegistryConfig,
		"HELM_REPOSITORY_CONFIG": s.RepositoryConfig,
		"HELM_REPOSITORY_CACHE":  s.RepositoryCache,
		"HELM_PLUGIN":            s.PluginsDirectory,
	} {
		if eVal := os.Getenv(key); len(eVal) > 0 {
			val = eVal
		}
		s.EnvironmentVariables[key] = val
	}
}

// Init sets values from the environment.
func (s *EnvSettings) Init(fs *pflag.FlagSet) {
	for name, envar := range envMap {
		setFlagFromEnv(name, envar, fs)
	}
}
