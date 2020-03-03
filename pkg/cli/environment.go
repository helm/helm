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
	"strconv"
	"sync"

	"github.com/spf13/pflag"

	"k8s.io/cli-runtime/pkg/genericclioptions"

	"helm.sh/helm/v3/pkg/helmpath"
	"helm.sh/helm/v3/pkg/kube"
)

// EnvSettings describes all of the environment settings.
type EnvSettings struct {
	namespace  string
	config     genericclioptions.RESTClientGetter
	configOnce sync.Once

	// KubeConfig is the path to the kubeconfig file
	KubeConfig string
	// KubeContext is the name of the kubeconfig context.
	KubeContext string
	// Bearer KubeToken used for authentication
	KubeToken string
	// Kubernetes API Server Endpoint for authentication
	KubeAPIServer string
	// Debug indicates whether or not Helm is running in Debug mode.
	Debug bool
	// RegistryConfig is the path to the registry config file.
	RegistryConfig string
	// RepositoryConfig is the path to the repositories file.
	RepositoryConfig string
	// RepositoryCache is the path to the repository cache directory.
	RepositoryCache string
	// PluginsDirectory is the path to the plugins directory.
	PluginsDirectory string
}

func New() *EnvSettings {

	env := EnvSettings{
		namespace:        os.Getenv("HELM_NAMESPACE"),
		KubeContext:      os.Getenv("HELM_KUBECONTEXT"),
		KubeToken:        os.Getenv("HELM_KUBETOKEN"),
		KubeAPIServer:    os.Getenv("HELM_KUBEAPISERVER"),
		PluginsDirectory: envOr("HELM_PLUGINS", helmpath.DataPath("plugins")),
		RegistryConfig:   envOr("HELM_REGISTRY_CONFIG", helmpath.ConfigPath("registry.json")),
		RepositoryConfig: envOr("HELM_REPOSITORY_CONFIG", helmpath.ConfigPath("repositories.yaml")),
		RepositoryCache:  envOr("HELM_REPOSITORY_CACHE", helmpath.CachePath("repository")),
	}
	env.Debug, _ = strconv.ParseBool(os.Getenv("HELM_DEBUG"))
	return &env
}

// AddFlags binds flags to the given flagset.
func (s *EnvSettings) AddFlags(fs *pflag.FlagSet) {
	fs.StringVarP(&s.namespace, "namespace", "n", s.namespace, "namespace scope for this request")
	fs.StringVar(&s.KubeConfig, "kubeconfig", "", "path to the kubeconfig file")
	fs.StringVar(&s.KubeContext, "kube-context", s.KubeContext, "name of the kubeconfig context to use")
	fs.StringVar(&s.KubeToken, "kube-token", s.KubeToken, "bearer token used for authentication")
	fs.StringVar(&s.KubeAPIServer, "kube-apiserver", s.KubeAPIServer, "the address and the port for the Kubernetes API server")
	fs.BoolVar(&s.Debug, "debug", s.Debug, "enable verbose output")
	fs.StringVar(&s.RegistryConfig, "registry-config", s.RegistryConfig, "path to the registry config file")
	fs.StringVar(&s.RepositoryConfig, "repository-config", s.RepositoryConfig, "path to the file containing repository names and URLs")
	fs.StringVar(&s.RepositoryCache, "repository-cache", s.RepositoryCache, "path to the file containing cached repository indexes")
}

func envOr(name, def string) string {
	if v, ok := os.LookupEnv(name); ok {
		return v
	}
	return def
}

func (s *EnvSettings) EnvVars() map[string]string {
	envvars := map[string]string{
		"HELM_BIN":               os.Args[0],
		"HELM_DEBUG":             fmt.Sprint(s.Debug),
		"HELM_PLUGINS":           s.PluginsDirectory,
		"HELM_REGISTRY_CONFIG":   s.RegistryConfig,
		"HELM_REPOSITORY_CACHE":  s.RepositoryCache,
		"HELM_REPOSITORY_CONFIG": s.RepositoryConfig,
		"HELM_NAMESPACE":         s.Namespace(),
		"HELM_KUBECONTEXT":       s.KubeContext,
		"HELM_KUBETOKEN":         s.KubeToken,
		"HELM_KUBEAPISERVER":     s.KubeAPIServer,
	}

	if s.KubeConfig != "" {
		envvars["KUBECONFIG"] = s.KubeConfig
	}

	return envvars
}

//Namespace gets the namespace from the configuration
func (s *EnvSettings) Namespace() string {
	if s.namespace != "" {
		return s.namespace
	}

	if ns, _, err := s.RESTClientGetter().ToRawKubeConfigLoader().Namespace(); err == nil {
		return ns
	}
	return "default"
}

//RESTClientGetter gets the kubeconfig from EnvSettings
func (s *EnvSettings) RESTClientGetter() genericclioptions.RESTClientGetter {
	s.configOnce.Do(func() {
		clientConfig := kube.GetConfig(s.KubeConfig, s.KubeContext, s.namespace)
		if s.KubeToken != "" {
			clientConfig.BearerToken = &s.KubeToken
		}
		if s.KubeAPIServer != "" {
			clientConfig.APIServer = &s.KubeAPIServer
		}

		s.config = clientConfig
	})
	return s.config
}
