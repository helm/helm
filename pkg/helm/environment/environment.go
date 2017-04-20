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

	"k8s.io/helm/pkg/helm/helmpath"
)

const (
	HomeEnvVar          = "HELM_HOME"
	PluginEnvVar        = "HELM_PLUGIN"
	PluginDisableEnvVar = "HELM_NO_PLUGINS"
	HostEnvVar          = "HELM_HOST"
)

func DefaultHelmHome() string {
	if home := os.Getenv(HomeEnvVar); home != "" {
		return home
	}
	return filepath.Join(os.Getenv("HOME"), ".helm")
}

func DefaultHelmHost() string {
	return os.Getenv(HostEnvVar)
}

type EnvSettings struct {
	TillerHost      string
	TillerNamespace string
	Home            helmpath.Home
	PlugDirs        string
	Debug           bool
}
