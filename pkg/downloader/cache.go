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

package downloader

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"helm.sh/helm/v4/internal/fileutil"
)

// Cache describes a cache that can get and put chart data.
// The cache key is the sha256 has of the content. sha256 is used in Helm for
// digests in index files providing a common key for checking content.
type Cache interface {
	// Get returns a reader for the given key.
	Get(key [sha256.Size]byte, cacheType string) (string, error)
	// Put stores the given reader for the given key.
	Put(key [sha256.Size]byte, data io.Reader, cacheType string) (string, error)
}

// CacheChart specifies the content is a chart
var CacheChart = ".chart"

// CacheProv specifies the content is a provenance file
var CacheProv = ".prov"

// TODO: The cache assumes files because much of Helm assumes files. Convert
// Helm to pass content around instead of file locations.

// DiskCache is a cache that stores data on disk.
type DiskCache struct {
	Root string
}

// Get returns a reader for the given key.
func (c *DiskCache) Get(key [sha256.Size]byte, cacheType string) (string, error) {
	p := c.fileName(key, cacheType)
	fi, err := os.Stat(p)
	if err != nil {
		return "", err
	}
	// Empty files treated as not exist because there is no content.
	if fi.Size() == 0 {
		return p, os.ErrNotExist
	}
	// directories should never happen unless something outside helm is operating
	// on this content.
	if fi.IsDir() {
		return p, errors.New("is a directory")
	}
	return p, nil
}

// Put stores the given reader for the given key.
// It returns the path to the stored file.
func (c *DiskCache) Put(key [sha256.Size]byte, data io.Reader, cacheType string) (string, error) {
	// TODO: verify the key and digest of the key are the same.
	p := c.fileName(key, cacheType)
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		slog.Error("failed to create cache directory")
		return p, err
	}
	return p, fileutil.AtomicWriteFile(p, data, 0644)
}

// fileName generates the filename in a structured manner where the first part is the
// directory and the full hash is the filename.
func (c *DiskCache) fileName(id [sha256.Size]byte, cacheType string) string {
	return filepath.Join(c.Root, fmt.Sprintf("%02x", id[0]), fmt.Sprintf("%x", id)+cacheType)
}
