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
	"fmt"
	"path/filepath"
)

// This helper builds paths to Helm's configuration, cache and data paths.
const lp = lazypath("helm")

// ConfigPath returns the path where Helm stores configuration.
func ConfigPath() string {
	return lp.configPath("")
}

// CachePath returns the path where Helm stores cached objects.
func CachePath() string {
	return lp.cachePath("")
}

// DataPath returns the path where Helm stores data.
func DataPath() string {
	return lp.dataPath("")
}

// Registry returns the path to the local registry cache.
func Registry() string {
	return lp.cachePath("registry")
}

// RepositoryFile returns the path to the repositories.yaml file.
func RepositoryFile() string {
	return lp.configPath("repositories.yaml")
}

// RepositoryCache returns the cache path for repository metadata.
func RepositoryCache() string {
	return lp.cachePath("repository")
}

// CacheIndex returns the path to an index for the given named repository.
func CacheIndex(name string) string {
	target := fmt.Sprintf("%s-index.yaml", name)
	if name == "" {
		target = "index.yaml"
	}
	return filepath.Join(RepositoryCache(), target)
}

// Starters returns the path to the Helm starter packs.
func Starters() string {
	return lp.dataPath("starters")
}

// PluginCache returns the cache path for plugins.
func PluginCache() string {
	return lp.cachePath("plugins")
}

// Plugins returns the path to the plugins directory.
func Plugins() string {
	return lp.dataPath("plugins")
}

// Archive returns the path to download chart archives.
func Archive() string {
	return lp.cachePath("archive")
}
