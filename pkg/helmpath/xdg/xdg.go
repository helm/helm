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

// Package xdg holds constants pertaining to XDG Base Directory Specification.
//
// The XDG Base Directory Specification https://specifications.freedesktop.org/basedir-spec/basedir-spec-latest.html
// specifies the environment variables that define user-specific base directories for various categories of files.
package xdg

const (
	// CacheHomeEnvVar is the environment variable used by the
	// XDG base directory specification for the cache directory.
	CacheHomeEnvVar = "XDG_CACHE_HOME"

	// ConfigHomeEnvVar is the environment variable used by the
	// XDG base directory specification for the config directory.
	ConfigHomeEnvVar = "XDG_CONFIG_HOME"

	// DataHomeEnvVar is the environment variable used by the
	// XDG base directory specification for the data directory.
	DataHomeEnvVar = "XDG_DATA_HOME"
)
