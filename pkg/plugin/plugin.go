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

package plugin // import "helm.sh/helm/pkg/plugin"

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	cli "helm.sh/helm/pkg/cli"
	"helm.sh/helm/pkg/osutil"
)

// PluginNamePrefix is the prefix for all helm plugins.
const PluginNamePrefix = "helm-"

// Plugin represents a plugin.
type Plugin struct {
	// Name is the name of the plugin.
	Name string `json:"name"`
	// Dir is the string path to the directory that holds the plugin.
	Dir string `json:"dir"`
}

// FindAll returns a list of executables that can be used as helm plugins.
func FindAll(plugdirs string) ([]*Plugin, error) {
	found := []*Plugin{}

	// Let's get all UNIXy and allow path separators
	for _, dir := range filepath.SplitList(plugdirs) {
		matches, err := LoadAll(dir)
		if err != nil {
			return found, err
		}
		found = append(found, matches...)
	}
	return found, nil
}

// LoadAll loads all plugins found beneath the base directory that are executable.
//
// This scans only one directory level deep.
func LoadAll(basedir string) ([]*Plugin, error) {
	plugins := []*Plugin{}
	// We want basedir/helm-*
	scanpath := filepath.Join(basedir, PluginNamePrefix+"*")
	matches, err := filepath.Glob(scanpath)
	if err != nil {
		return plugins, err
	}

	for i := range matches {
		if isExec, err := osutil.IsExecutable(matches[i]); err == nil && !isExec {
			// swap the element to delete with the one at the end of the slice, then return n-1 elements
			matches[len(matches)-1], matches[i] = matches[i], matches[len(matches)-1]
			matches = matches[:len(matches)-1]
		} else if err != nil {
			return nil, fmt.Errorf("error: unable to identify %s as an executable file: %v", matches[i], err)
		}

		plugins = append(plugins, &Plugin{
			Name: strings.TrimPrefix(filepath.Base(matches[i]), PluginNamePrefix),
			Dir:  filepath.Dir(matches[i]),
		})
	}

	return plugins, nil
}

// SetupPluginEnv prepares os.Env for plugins. It operates on os.Env because
// the plugin subsystem itself needs access to the environment variables
// created here.
func SetupPluginEnv(settings cli.EnvSettings, shortName, base string) {
	for key, val := range map[string]string{
		"HELM_PLUGIN_NAME": shortName,
		"HELM_PLUGIN_DIR":  base,
		"HELM_BIN":         os.Args[0],

		// Set vars that may not have been set, and save client the
		// trouble of re-parsing.
		"HELM_PLUGIN": settings.PluginDirs(),
		"HELM_HOME":   settings.Home.String(),

		// Set vars that convey common information.
		"HELM_PATH_REPOSITORY":      settings.Home.Repository(),
		"HELM_PATH_REPOSITORY_FILE": settings.Home.RepositoryFile(),
		"HELM_PATH_CACHE":           settings.Home.Cache(),
		"HELM_PATH_STARTER":         settings.Home.Starters(),
	} {
		os.Setenv(key, val)
	}

	if settings.Debug {
		os.Setenv("HELM_DEBUG", "1")
	}
}
