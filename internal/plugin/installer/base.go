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

package installer // import "helm.sh/helm/v4/internal/plugin/installer"

import (
	"path/filepath"

	"helm.sh/helm/v4/pkg/cli"
)

type base struct {
	// Source is the reference to a plugin
	Source string
	// PluginsDirectory is the directory where plugins are installed
	PluginsDirectory string
}

func newBase(source string) base {
	settings := cli.New()
	return base{
		Source:           source,
		PluginsDirectory: pluginInstallDir(settings.PluginsDirectory),
	}
}

// pluginInstallDir returns the directory to install a new plugin into.
// When HELM_PLUGINS contains multiple colon-separated paths, the first
// path is used as the install destination — consistent with how
// FindPlugins() loads from all paths and matching git conventions.
func pluginInstallDir(helmPlugins string) string {
	if helmPlugins == "" {
		return helmPlugins
	}
	dirs := filepath.SplitList(helmPlugins)
	if len(dirs) == 0 {
		return helmPlugins
	}
	return dirs[0]
}

// Path is where the plugin will be installed.
func (b *base) Path() string {
	if b.Source == "" {
		return ""
	}
	return filepath.Join(b.PluginsDirectory, filepath.Base(b.Source))
}
