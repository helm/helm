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
package pluginloader // import "helm.sh/helm/v4/internal/plugins/loader"

import (
	"helm.sh/helm/v4/internal/plugins"
	"helm.sh/helm/v4/internal/plugins/runtimes/subprocess"
)

// FindPlugins returns a list of YAML files that describe plugins
func FindPlugins(pluginsDirs []string, descriptor plugins.PluginDescriptor) ([]plugins.Plugin, error) {
	return findPlugins(pluginsDirs, subprocessFindPlugins, makeDescriptorFilter(descriptor))
}

type findFunc func(pluginsDirs string) ([]plugins.Plugin, error)

type filterFunc func(plugins.Plugin) bool

func findPlugins(pluginsDirs []string, findFunc findFunc, filterFunc filterFunc) ([]plugins.Plugin, error) {

	found := []plugins.Plugin{}
	for _, pluginsDir := range pluginsDirs {
		ps, err := findFunc(pluginsDir)
		if err != nil {
			return nil, err
		}

		for _, p := range ps {
			if filterFunc(p) {
				found = append(found, p)
			}
		}
	}

	return found, nil
}

func convertSubprocess(subs []*subprocess.Plugin) []plugins.Plugin {
	ps := make([]plugins.Plugin, len(subs))
	for i, r := range subs {
		ps[i] = r
	}
	return ps
}

func subprocessFindPlugins(pluginsDir string) ([]plugins.Plugin, error) {

	ps, err := subprocess.FindPlugins(pluginsDir)
	if err != nil {
		return nil, err
	}

	return convertSubprocess(ps), nil
}

func makeDescriptorFilter(descriptor plugins.PluginDescriptor) filterFunc {

	return func(p plugins.Plugin) bool {
		manifest := p.Manifest()
		return manifest.TypeVersion == descriptor.TypeVersion
	}
}
