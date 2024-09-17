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

package cache

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNoOpCache(t *testing.T) {
	cache := NewNoOpCache[string]()

	t.Run("Set", func(_ *testing.T) {
		cache.Set("key", "value")
		// No assertion needed as it's a no-op
	})

	t.Run("Get", func(t *testing.T) {
		value, exists := cache.Get("key")
		assert.False(t, exists)
		assert.Empty(t, value)
	})
}

func TestConcurrentMapCache(t *testing.T) {
	cache := NewConcurrentMapCache[int]()

	t.Run("Set and Get", func(t *testing.T) {
		cache.Set("key1", 42)
		cache.Set("key2", 84)

		value1, exists1 := cache.Get("key1")
		assert.True(t, exists1)
		assert.Equal(t, 42, value1)

		value2, exists2 := cache.Get("key2")
		assert.True(t, exists2)
		assert.Equal(t, 84, value2)
	})

	t.Run("Get non-existent key", func(t *testing.T) {
		value, exists := cache.Get("non-existent")
		assert.False(t, exists)
		assert.Zero(t, value)
	})

	t.Run("Overwrite existing key", func(t *testing.T) {
		cache.Set("key1", 100)
		value, exists := cache.Get("key1")
		assert.True(t, exists)
		assert.Equal(t, 100, value)
	})
}
