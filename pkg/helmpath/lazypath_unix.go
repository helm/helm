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

//go:build !windows && !darwin
// +build !windows,!darwin

package helmpath

import (
	"path/filepath"

	"k8s.io/client-go/util/homedir"
)

// dataHome defines the base directory relative to which user specific data files should be stored.
//
// If $XDG_DATA_HOME is either not set or empty, a default equal to $HOME/.local/share is used.
func dataHome() string {
	return filepath.Join(homedir.HomeDir(), ".local", "share")
}

// configHome defines the base directory relative to which user specific configuration files should
// be stored.
//
// If $XDG_CONFIG_HOME is either not set or empty, a default equal to $HOME/.config is used.
func configHome() string {
	return filepath.Join(homedir.HomeDir(), ".config")
}

// cacheHome defines the base directory relative to which user specific non-essential data files
// should be stored.
//
// If $XDG_CACHE_HOME is either not set or empty, a default equal to $HOME/.cache is used.
func cacheHome() string {
	return filepath.Join(homedir.HomeDir(), ".cache")
}
