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

// This helper builds paths to Helm's configuration, cache and data paths.
const lp = lazypath("helm")

// ConfigPath returns the path where Helm stores configuration.
func ConfigPath(elem ...string) string {
	return lp.configPath(elem...)
}

// CachePath returns the path where Helm stores cached objects.
func CachePath(elem ...string) string {
	return lp.cachePath(elem...)
}

// DataPath returns the path where Helm stores data.
func DataPath(elem ...string) string {
	return lp.dataPath(elem...)
}

// Registry returns the path to the local registry cache.
// func Registry() string { return CachePath("registry") }

// RepositoryFile returns the path to the repositories.yaml file.
// func RepositoryFile() string { return ConfigPath("repositories.yaml") }

// RepositoryCache returns the cache path for repository metadata.
// func RepositoryCache() string { return CachePath("repository") }

// CacheIndex returns the path to an index for the given named repository.
func CacheIndexFile(name string) string {
	if name != "" {
		name += "-"
	}
	return name + "index.yaml"
}

func CacheIndex(name string) string {
	if name != "" {
		name += "-"
	}
	name += "index.yaml"
	return CachePath("repository", name)
}

// Starters returns the path to the Helm starter packs.
// func Starters() string { return DataPath("starters") }

// PluginCache returns the cache path for plugins.
// func PluginCache() string { return CachePath("plugins") }

// Plugins returns the path to the plugins directory.
// func Plugins() string { return DataPath("plugins") }
