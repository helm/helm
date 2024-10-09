// Copyright The Helm Authors.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at

// http://www.apache.org/licenses/LICENSE-2.0

// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package helmpath

import (
	"os"
	"path/filepath"

	"helm.sh/helm/v3/pkg/helmpath/xdg"
)

const (
	// CacheHomeEnvVar is the environment variable used by Helm
	// for the cache directory. When no value is set a default is used.
	CacheHomeEnvVar = "HELM_CACHE_HOME"

	// ConfigHomeEnvVar is the environment variable used by Helm
	// for the config directory. When no value is set a default is used.
	ConfigHomeEnvVar = "HELM_CONFIG_HOME"

	// DataHomeEnvVar is the environment variable used by Helm
	// for the data directory. When no value is set a default is used.
	DataHomeEnvVar = "HELM_DATA_HOME"
)

// lazypath is a lazy-loaded path buffer for the XDG base directory specification.
type lazypath string

func (l lazypath) path(helmEnvVar, xdgEnvVar string, defaultFn func() string, elem ...string) string {

	// There is an order to checking for a path.
	// 1. See if a Helm specific environment variable has been set.
	// 2. Check if an XDG environment variable is set
	// 3. Fall back to a default
	base := os.Getenv(helmEnvVar)
	if base != "" {
		return filepath.Join(base, filepath.Join(elem...))
	}
	base = os.Getenv(xdgEnvVar)
	if base == "" {
		base = defaultFn()
	}
	return filepath.Join(base, string(l), filepath.Join(elem...))
}

// cachePath defines the base directory relative to which user specific non-essential data files
// should be stored.
func (l lazypath) cachePath(elem ...string) string {
	return l.path(CacheHomeEnvVar, xdg.CacheHomeEnvVar, cacheHome, filepath.Join(elem...))
}

// configPath defines the base directory relative to which user specific configuration files should
// be stored.
func (l lazypath) configPath(elem ...string) string {
	return l.path(ConfigHomeEnvVar, xdg.ConfigHomeEnvVar, configHome, filepath.Join(elem...))
}

// dataPath defines the base directory relative to which user specific data files should be stored.
func (l lazypath) dataPath(elem ...string) string {
	return l.path(DataHomeEnvVar, xdg.DataHomeEnvVar, dataHome, filepath.Join(elem...))
}
