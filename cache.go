package erlcgo

import (
	"fmt"
	"sync"
	"time"
)

// Cache provides a generic interface for caching operations.
// Implementations must be safe for concurrent use.
//
// Example usage:
//
//	cache := NewMemoryCache()
//	cache.Set("key", value, time.Minute)
//	if val, ok := cache.Get("key"); ok {
//	    // Use cached value
//	}
type Cache interface {
	// Get retrieves a value from the cache.
	// Returns the value and true if found, or nil and false if not found.
	// The returned value should be type asserted to the expected type.
	Get(key string) (interface{}, bool)

	// Set stores a value in the cache with the specified TTL.
	// The value must be JSON serializable.
	// If ttl <= 0, the item will be cached indefinitely.
	Set(key string, value interface{}, ttl time.Duration)

	// Delete removes an item from the cache.
	// It is safe to call Delete on non-existent keys.
	Delete(key string)
}

// CacheError represents errors that can occur during cache operations
type CacheError struct {
	Op  string // Operation that failed (e.g., "get", "set")
	Key string // Key that caused the error
	Err error  // Underlying error
}

func (e *CacheError) Error() string {
	return fmt.Sprintf("cache %s failed for key %q: %v", e.Op, e.Key, e.Err)
}

// CacheStats provides statistics about cache usage
type CacheStats struct {
	Hits      int64         // Number of cache hits
	Misses    int64         // Number of cache misses
	ItemCount int           // Current number of items in cache
	Memory    int64         // Approximate memory usage in bytes
	AvgTTL    time.Duration // Average TTL of cached items
}

// MemoryCache implements a simple in-memory cache
type MemoryCache struct {
	mu      sync.RWMutex
	items   map[string]*cacheItem
	stats   CacheStats
	onEvict func(key string, value interface{}) // Called when an item is evicted
}

type cacheItem struct {
	value      interface{}
	expiration time.Time
}

// NewMemoryCache creates a new MemoryCache instance
func NewMemoryCache() *MemoryCache {
	cache := &MemoryCache{
		items: make(map[string]*cacheItem),
	}
	go cache.cleanupLoop()
	return cache
}

func (c *MemoryCache) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	item, exists := c.items[key]
	if !exists {
		return nil, false
	}

	if time.Now().After(item.expiration) {
		delete(c.items, key)
		return nil, false
	}

	return item.value, true
}

func (c *MemoryCache) Set(key string, value interface{}, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items[key] = &cacheItem{
		value:      value,
		expiration: time.Now().Add(ttl),
	}
}

func (c *MemoryCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.items, key)
}

func (c *MemoryCache) cleanupLoop() {
	ticker := time.NewTicker(time.Minute)
	for range ticker.C {
		c.mu.Lock()
		now := time.Now()
		for key, item := range c.items {
			if now.After(item.expiration) {
				delete(c.items, key)
			}
		}
		c.mu.Unlock()
	}
}

// Stats returns current cache statistics
func (c *MemoryCache) Stats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.stats
}

// WithEvictionCallback sets a callback function that is called when items are evicted
func (c *MemoryCache) WithEvictionCallback(fn func(key string, value interface{})) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onEvict = fn
}
