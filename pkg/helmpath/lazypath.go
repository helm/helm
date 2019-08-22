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
	"os"
	"path/filepath"

	"helm.sh/helm/pkg/helmpath/xdg"
)

// lazypath is an lazy-loaded path buffer for the XDG base directory specification.
type lazypath string

func (l lazypath) path(envVar string, defaultFn func() string, file string) string {
	base := os.Getenv(envVar)
	if base == "" {
		base = defaultFn()
	}
	path := filepath.Join(base, string(l), file)
	// Create directory if not exist.
	l.ensurePathExist(path)
	return path
}

// cachePath defines the base directory relative to which user specific non-essential data files
// should be stored.
func (l lazypath) cachePath(file string) string {
	return l.path(xdg.CacheHomeEnvVar, cacheHome, file)
}

// configPath defines the base directory relative to which user specific configuration files should
// be stored.
func (l lazypath) configPath(file string) string {
	return l.path(xdg.ConfigHomeEnvVar, configHome, file)
}

// dataPath defines the base directory relative to which user specific data files should be stored.
func (l lazypath) dataPath(file string) string {
	return l.path(xdg.DataHomeEnvVar, dataHome, file)
}

func (l lazypath) ensurePathExist(path string) {
	if fi, err := os.Stat(path); err != nil {
		if err := os.MkdirAll(path, 0755); err != nil {
			// FIXME - Need to discuss best way to log error message in this package,
			//  otherwise would need to refactor path() method. That would be huge refactor :(
			fmt.Printf("creation of directory %s failed with error %s \n", path, err.Error())
			// FIXME - os.Exit() seems like anti-pattern to exit, any suggestion here ?
			os.Exit(1)
		}
	} else if !fi.IsDir() {
		fmt.Printf("%s must be a directory. \n", path)
		os.Exit(1)
	}
}
