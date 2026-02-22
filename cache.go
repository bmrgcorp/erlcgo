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

// MemoryCache implements a simple in-memory cache with automatic expiration.
// It runs a background goroutine that periodically removes expired items.
// MemoryCache is safe for concurrent use by multiple goroutines.
type MemoryCache struct {
	mu       sync.RWMutex                    // Protects access to items and stats
	items    map[string]*cacheItem           // The actual cache storage
	stats    CacheStats                      // Cache statistics
	onEvict  func(key string, value interface{}) // Optional callback invoked when items are evicted
	stopCh   chan struct{}                   // Used to signal the cleanup goroutine to stop
	stopOnce sync.Once                       // Ensures Close() only closes stopCh once, preventing panic
}

type cacheItem struct {
	value      interface{}
	expiration time.Time
}

// NewMemoryCache creates a new MemoryCache instance.
// The cache starts a background goroutine that periodically removes expired items
// (every minute). To prevent goroutine leaks, call Close() when the cache is no
// longer needed, especially in long-running applications or when creating many
// cache instances.
//
// Example:
//
//	cache := NewMemoryCache()
//	defer cache.Close() // Always close to clean up the background goroutine
func NewMemoryCache() *MemoryCache {
	cache := &MemoryCache{
		items:  make(map[string]*cacheItem),
		stopCh: make(chan struct{}),
	}
	go cache.cleanupLoop() // Start background cleanup goroutine
	return cache
}

func (c *MemoryCache) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	item, exists := c.items[key]
	if !exists {
		c.mu.RUnlock()
		return nil, false
	}

	// If the item has expired, upgrade to a write lock and delete it.
	expired := time.Now().After(item.expiration)
	if expired {
		// Upgrade to write lock to delete the expired item
		c.mu.RUnlock()
		c.mu.Lock()
		// Double-check expiration after acquiring write lock
		if item, exists := c.items[key]; exists && time.Now().After(item.expiration) {
			delete(c.items, key)
			if c.onEvict != nil {
				c.onEvict(key, item.value)
			}
		}
		c.mu.Unlock()
		return nil, false
	}

	value := item.value
	c.mu.RUnlock()
	return value, true
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

// cleanupLoop runs in a background goroutine and periodically removes expired items.
// It runs every minute until the cache is closed via Close(). When stopCh is closed,
// the loop exits and the goroutine terminates, preventing goroutine leaks.
func (c *MemoryCache) cleanupLoop() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop() // Ensure ticker is stopped when goroutine exits

	for {
		select {
		case <-ticker.C:
			// Periodic cleanup: remove all expired items
			c.mu.Lock()
			now := time.Now()
			for key, item := range c.items {
				if now.After(item.expiration) {
					delete(c.items, key)
					// Call eviction callback if set
					if c.onEvict != nil {
						c.onEvict(key, item.value)
					}
				}
			}
			c.mu.Unlock()
		case <-c.stopCh:
			// Signal received to stop the cleanup loop
			return
		}
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

// Close stops the background cleanup goroutine and releases resources.
// After Close() is called, the cache will no longer automatically remove expired items,
// though expired items will still be removed during Get() operations.
//
// It is safe to call Close() multiple times; subsequent calls are no-ops.
// Once Close() is called, the cache should not be used further, though this is
// not enforced.
//
// It is recommended to call Close() when the cache is no longer needed, especially
// in long-running applications or when creating many cache instances, to prevent
// goroutine leaks.
//
// Example:
//
//	cache := NewMemoryCache()
//	// ... use cache ...
//	cache.Close() // Clean up when done
func (c *MemoryCache) Close() {
	c.stopOnce.Do(func() {
		// Close the channel safely
		close(c.stopCh)
	})
}
