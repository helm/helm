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

package defaultgetters

import (
	"os"

	"k8s.io/helm/pkg/getter"
	"k8s.io/helm/pkg/getter/http"
	"k8s.io/helm/pkg/getter/plugin"
	"k8s.io/helm/pkg/helm/environment"
	"k8s.io/helm/pkg/helm/helmpath"
	"k8s.io/helm/pkg/plugin"
)

// Get gathers the getter constructors for the downloaders.
// Currently the build-in http getter and the discovered
// plugins with downloader notations are collected.
func Get(settings environment.EnvSettings) []getter.Prop {
	result := []getter.Prop{
		{
			Schemes:     getter.Schemes{"http", "https"},
			Constructor: httpgetter.New,
		},
	}
	pluginDownloaders, _ := collectPlugins(settings)
	result = append(result, pluginDownloaders...)
	return result
}

func collectPlugins(settings environment.EnvSettings) ([]getter.Prop, error) {
	plugdirs := os.Getenv(environment.PluginEnvVar)
	if plugdirs == "" {
		home := helmpath.Home(os.Getenv(environment.HomeEnvVar))
		plugdirs = home.Plugins()
	}

	plugins, err := plugin.FindPlugins(plugdirs)
	if err != nil {
		return nil, err
	}
	var result []getter.Prop
	for _, plugin := range plugins {
		for _, downloader := range plugin.Metadata.Downloaders {
			result = append(result, getter.Prop{
				Schemes: downloader.Protocols,
				Constructor: plugingetter.ConstructNew(
					downloader.Command,
					settings,
					plugin.Metadata.Name,
					plugin.Dir,
				),
			})
		}
	}
	return result, nil
}
