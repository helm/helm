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

package helmpath

import (
	"fmt"
	"path/filepath"
)

// Home describes the location of a CLI configuration.
//
// This helper builds paths relative to a Helm Home directory.
type Home string

// String returns Home as a string.
//
// Implements fmt.Stringer.
func (h Home) String() string {
	return string(h)
}

// Repository returns the path to the local repository.
func (h Home) Repository() string {
	return filepath.Join(string(h), "repository")
}

// RepositoryFile returns the path to the repositories.yaml file.
func (h Home) RepositoryFile() string {
	return filepath.Join(string(h), "repository/repositories.yaml")
}

// Cache returns the path to the local cache.
func (h Home) Cache() string {
	return filepath.Join(string(h), "repository/cache")
}

// CacheIndex returns the path to an index for the given named repository.
func (h Home) CacheIndex(name string) string {
	target := fmt.Sprintf("repository/cache/%s-index.yaml", name)
	return filepath.Join(string(h), target)
}

// Starters returns the path to the Helm starter packs.
func (h Home) Starters() string {
	return filepath.Join(string(h), "starters")
}

// LocalRepository returns the location to the local repo.
//
// The local repo is the one used by 'helm serve'
//
// If additional path elements are passed, they are appended to the returned path.
func (h Home) LocalRepository(paths ...string) string {
	frag := append([]string{string(h), "repository/local"}, paths...)
	return filepath.Join(frag...)
}

// Plugins returns the path to the plugins directory.
func (h Home) Plugins() string {
	return filepath.Join(string(h), "plugins")
}
