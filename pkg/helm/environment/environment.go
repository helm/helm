/*
Copyright 2016 The Kubernetes Authors All rights reserved.

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

/*Package environment describes the operating environment for Tiller.

Tiller's environment encapsulates all of the service dependencies Tiller has.
These dependencies are expressed as interfaces so that alternate implementations
(mocks, etc.) can be easily generated.
*/
package environment

import (
	"os"
	"path/filepath"

	"github.com/spf13/pflag"

	"k8s.io/helm/pkg/helm/helmpath"
)

// DefaultHelmHome is the default HELM_HOME.
var DefaultHelmHome = filepath.Join("$HOME", ".helm")

// EnvSettings describes all of the environment settings.
type EnvSettings struct {
	// TillerHost is the host and port of Tiller.
	TillerHost string
	// TillerNamespace is the namespace in which Tiller runs.
	TillerNamespace string
	// Home is the local path to the Helm home directory.
	Home helmpath.Home
	// Debug indicates whether or not Helm is running in Debug mode.
	Debug bool
	// KubeContext is the name of the kubeconfig context.
	KubeContext string
}

// AddFlags binds flags to the given flagset.
func (s *EnvSettings) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar((*string)(&s.Home), "home", DefaultHelmHome, "location of your Helm config. Overrides $HELM_HOME")
	fs.StringVar(&s.TillerHost, "host", "", "address of Tiller. Overrides $HELM_HOST")
	fs.StringVar(&s.KubeContext, "kube-context", "", "name of the kubeconfig context to use")
	fs.BoolVar(&s.Debug, "debug", false, "enable verbose output")
	fs.StringVar(&s.TillerNamespace, "tiller-namespace", "kube-system", "namespace of Tiller")
}

// Init sets values from the environment.
func (s *EnvSettings) Init(fs *pflag.FlagSet) {
	for name, envar := range envMap {
		setFlagFromEnv(name, envar, fs)
	}
}

// PluginDirs is the path to the plugin directories.
func (s EnvSettings) PluginDirs() string {
	if d, ok := os.LookupEnv("HELM_PLUGIN"); ok {
		return d
	}
	return s.Home.Plugins()
}

// envMap maps flag names to envvars
var envMap = map[string]string{
	"debug":            "HELM_DEBUG",
	"home":             "HELM_HOME",
	"host":             "HELM_HOST",
	"tiller-namespace": "TILLER_NAMESPACE",
}

func setFlagFromEnv(name, envar string, fs *pflag.FlagSet) {
	if fs.Changed(name) {
		return
	}
	if v, ok := os.LookupEnv(envar); ok {
		fs.Set(name, v)
	}
}

// Deprecated
const (
	HomeEnvVar          = "HELM_HOME"
	PluginEnvVar        = "HELM_PLUGIN"
	PluginDisableEnvVar = "HELM_NO_PLUGINS"
	HostEnvVar          = "HELM_HOST"
	DebugEnvVar         = "HELM_DEBUG"
)
