package cache

import (
	"fmt"
	"sync"
)

// Cache interface defines the methods for a cache
type Cache[V any] interface {
	// Set adds an item to the cache
	Set(key string, value V)

	// Get retrieves an item from the cache
	// The boolean return value indicates whether the key was found
	Get(key string) (V, bool)
}

// NoOpCache implements Cache interface with no-op operations
type NoOpCache[V any] struct{}

func NewNoOpCache[V any]() *NoOpCache[V] {
	return &NoOpCache[V]{}
}

func (c *NoOpCache[V]) Set(key string, value V) {}

func (c *NoOpCache[V]) Get(key string) (V, bool) {
	var zero V
	return zero, false
}

// ConcurrentMapCache implements Cache interface using a concurrent map
type ConcurrentMapCache[V any] struct {
	items map[string]V
	mu    sync.RWMutex
}

func NewConcurrentMapCache[V any]() *ConcurrentMapCache[V] {
	return &ConcurrentMapCache[V]{
		items: make(map[string]V),
	}
}

func (c *ConcurrentMapCache[V]) Set(key string, value V) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items[key] = value
	fmt.Println("set", key)
}

func (c *ConcurrentMapCache[V]) Get(key string) (V, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	value, exists := c.items[key]
	fmt.Println("get", key, "exists", exists)
	return value, exists
}
