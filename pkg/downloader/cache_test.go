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
	"bytes"
	"crypto/sha256"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// compiler check to ensure DiskCache implements the Cache interface.
var _ Cache = (*DiskCache)(nil)

func TestDiskCache_PutAndGet(t *testing.T) {
	// Setup a temporary directory for the cache
	tmpDir := t.TempDir()
	cache := &DiskCache{Root: tmpDir}

	// Test data
	content := []byte("hello world")
	key := sha256.Sum256(content)

	// --- Test case 1: Put and Get a regular file (prov=false) ---
	t.Run("PutAndGetTgz", func(t *testing.T) {
		// Put the data into the cache
		path, err := cache.Put(key, bytes.NewReader(content), CacheChart)
		require.NoError(t, err, "Put should not return an error")

		// Verify the file exists at the returned path
		_, err = os.Stat(path)
		require.NoError(t, err, "File should exist after Put")

		// Get the file from the cache
		retrievedPath, err := cache.Get(key, CacheChart)
		require.NoError(t, err, "Get should not return an error for existing file")
		assert.Equal(t, path, retrievedPath, "Get should return the same path as Put")

		// Verify content
		data, err := os.ReadFile(retrievedPath)
		require.NoError(t, err)
		assert.Equal(t, content, data, "Content of retrieved file should match original content")
	})

	// --- Test case 2: Put and Get a provenance file (prov=true) ---
	t.Run("PutAndGetProv", func(t *testing.T) {
		provContent := []byte("provenance data")
		provKey := sha256.Sum256(provContent)

		path, err := cache.Put(provKey, bytes.NewReader(provContent), CacheProv)
		require.NoError(t, err)

		retrievedPath, err := cache.Get(provKey, CacheProv)
		require.NoError(t, err)
		assert.Equal(t, path, retrievedPath)

		data, err := os.ReadFile(retrievedPath)
		require.NoError(t, err)
		assert.Equal(t, provContent, data)
	})

	// --- Test case 3: Get a non-existent file ---
	t.Run("GetNonExistent", func(t *testing.T) {
		nonExistentKey := sha256.Sum256([]byte("does not exist"))
		_, err := cache.Get(nonExistentKey, CacheChart)
		assert.ErrorIs(t, err, os.ErrNotExist, "Get for a non-existent key should return os.ErrNotExist")
	})

	// --- Test case 4: Put an empty file ---
	t.Run("PutEmptyFile", func(t *testing.T) {
		emptyContent := []byte{}
		emptyKey := sha256.Sum256(emptyContent)

		path, err := cache.Put(emptyKey, bytes.NewReader(emptyContent), CacheChart)
		require.NoError(t, err)

		// Get should return ErrNotExist for empty files
		_, err = cache.Get(emptyKey, CacheChart)
		assert.ErrorIs(t, err, os.ErrNotExist, "Get for an empty file should return os.ErrNotExist")

		// But the file should exist
		_, err = os.Stat(path)
		require.NoError(t, err, "Empty file should still exist on disk")
	})

	// --- Test case 5: Get a directory ---
	t.Run("GetDirectory", func(t *testing.T) {
		dirKey := sha256.Sum256([]byte("i am a directory"))
		dirPath := cache.fileName(dirKey, CacheChart)
		err := os.MkdirAll(dirPath, 0755)
		require.NoError(t, err)

		_, err = cache.Get(dirKey, CacheChart)
		assert.EqualError(t, err, "is a directory")
	})
}

func TestDiskCache_fileName(t *testing.T) {
	cache := &DiskCache{Root: "/tmp/cache"}
	key := sha256.Sum256([]byte("some data"))

	assert.Equal(t, filepath.Join("/tmp/cache", "13", "1307990e6ba5ca145eb35e99182a9bec46531bc54ddf656a602c780fa0240dee.chart"), cache.fileName(key, CacheChart))
	assert.Equal(t, filepath.Join("/tmp/cache", "13", "1307990e6ba5ca145eb35e99182a9bec46531bc54ddf656a602c780fa0240dee.prov"), cache.fileName(key, CacheProv))
}
