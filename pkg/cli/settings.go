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

package cli

import (
	"fmt"
	"github.com/spf13/pflag"
	"helm.sh/helm/v3/pkg/helmpath"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"os"
	"strconv"
)

// Settings describes all of the configuration options required by the Helm client.
type Settings struct {
	// The Kubernetes namespace
	Namespace string
	// The Helm driver ("memory", "secret", or "configmap")
	HelmDriver string
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

	// Kubernetes configuration flags
	config *genericclioptions.ConfigFlags
}

// The default Settings struct for the Helm client, largely drawn from environment variables.
func SettingsFromEnv() *Settings {
	env := &Settings{
		Namespace:        os.Getenv("HELM_NAMESPACE"),
		HelmDriver:       os.Getenv("HELM_DRIVER"),
		KubeContext:      os.Getenv("HELM_KUBECONTEXT"),
		KubeToken:        os.Getenv("HELM_KUBETOKEN"),
		KubeAPIServer:    os.Getenv("HELM_KUBEAPISERVER"),
		PluginsDirectory: envOr("HELM_PLUGINS", helmpath.DataPath("plugins")),
		RegistryConfig:   envOr("HELM_REGISTRY_CONFIG", helmpath.ConfigPath("registry.json")),
		RepositoryConfig: envOr("HELM_REPOSITORY_CONFIG", helmpath.ConfigPath("repositories.yaml")),
		RepositoryCache:  envOr("HELM_REPOSITORY_CACHE", helmpath.CachePath("repository")),
	}
	env.Debug, _ = strconv.ParseBool(os.Getenv("HELM_DEBUG"))

	// bind to kubernetes config flags
	env.config = &genericclioptions.ConfigFlags{
		Namespace:   &env.Namespace,
		Context:     &env.KubeContext,
		BearerToken: &env.KubeToken,
		APIServer:   &env.KubeAPIServer,
		KubeConfig:  &env.KubeConfig,
	}
	return env
}

// AddFlags binds flags to the given flagset.
func (s *Settings) AddFlags(fs *pflag.FlagSet) {
	fs.StringVarP(&s.Namespace, "namespace", "n", s.Namespace, "namespace scope for this request")
	fs.StringVar(&s.KubeConfig, "kubeconfig", "", "path to the kubeconfig file")
	fs.StringVar(&s.KubeContext, "kube-context", s.KubeContext, "name of the kubeconfig context to use")
	fs.StringVar(&s.KubeToken, "kube-token", s.KubeToken, "bearer token used for authentication")
	fs.StringVar(&s.KubeAPIServer, "kube-apiserver", s.KubeAPIServer, "the address and the port for the Kubernetes API server")
	fs.BoolVar(&s.Debug, "debug", s.Debug, "enable verbose output")
	fs.StringVar(&s.RegistryConfig, "registry-config", s.RegistryConfig, "path to the registry config file")
	fs.StringVar(&s.RepositoryConfig, "repository-config", s.RepositoryConfig, "path to the file containing repository names and URLs")
	fs.StringVar(&s.RepositoryCache, "repository-cache", s.RepositoryCache, "path to the file containing cached repository indexes")
}

// GetNamespace gets the namespace from the configuration
func (s *Settings) GetNamespace() string {
	if ns, _, err := s.config.ToRawKubeConfigLoader().Namespace(); err == nil {
		return ns
	}
	return "default"
}

// RESTClientGetter gets the kubeconfig from Settings
func (s *Settings) RESTClientGetter() genericclioptions.RESTClientGetter {
	return s.config
}

func envOr(name, def string) string {
	if v, ok := os.LookupEnv(name); ok {
		return v
	}
	return def
}

func (s *Settings) EnvVars() map[string]string {
	envvars := map[string]string{
		"HELM_BIN":               os.Args[0],
		"HELM_DEBUG":             fmt.Sprint(s.Debug),
		"HELM_PLUGINS":           s.PluginsDirectory,
		"HELM_REGISTRY_CONFIG":   s.RegistryConfig,
		"HELM_REPOSITORY_CACHE":  s.RepositoryCache,
		"HELM_REPOSITORY_CONFIG": s.RepositoryConfig,
		"HELM_NAMESPACE":         s.GetNamespace(),

		// broken, these are populated from helm flags and not kubeconfig.
		"HELM_KUBECONTEXT":   s.KubeContext,
		"HELM_KUBETOKEN":     s.KubeToken,
		"HELM_KUBEAPISERVER": s.KubeAPIServer,
	}
	if s.KubeConfig != "" {
		envvars["KUBECONFIG"] = s.KubeConfig
	}
	return envvars
}
