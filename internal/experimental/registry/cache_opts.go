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

package registry // import "helm.sh/helm/v3/internal/experimental/registry"

import (
	"io"
)

type (
	// CacheOption allows specifying various settings configurable by the user for overriding the defaults
	// used when creating a new default cache
	CacheOption func(*Cache)
)

// CacheOptDebug returns a function that sets the debug setting on cache options set
func CacheOptDebug(debug bool) CacheOption {
	return func(cache *Cache) {
		cache.debug = debug
	}
}

// CacheOptWriter returns a function that sets the writer setting on cache options set
func CacheOptWriter(out io.Writer) CacheOption {
	return func(cache *Cache) {
		cache.out = out
	}
}

// CacheOptRoot returns a function that sets the root directory setting on cache options set
func CacheOptRoot(rootDir string) CacheOption {
	return func(cache *Cache) {
		cache.rootDir = rootDir
	}
}
