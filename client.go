// Package erlcgo provides a Go client for interacting with the Emergency Response: Liberty County (ER:LC) API.
//
// Basic usage:
//
//	client := erlcgo.NewClient("your-server-key")
//
//	// With optional global API key:
//	client := erlcgo.NewClient("your-server-key",
//	    erlcgo.WithGlobalAPIKey("your-global-key"),
//	)
//
//	// Fetch server data
//	opts := erlcgo.ServerQueryOptions{Players: true, Vehicles: true}
//	resp, err := client.GetServer(context.Background(), opts)
//
//	// Execute command
//	err = client.ExecuteCommand(context.Background(), ":pm NoahCxrest Hello, World!")
package erlcgo

import (
	"net/http"
	"sync"
	"time"
)

// Client represents an ERLC API client.
// It handles authentication, rate limiting, and request execution.
// Create a new client using NewClient().
type Client struct {
	httpClient   *http.Client
	baseURL      string
	apiKey       string
	globalAPIKey string
	rateLimiter  *RateLimiter
	queue        *RequestQueue
	ownsQueue    bool
	cache        *CacheConfig
	responseHook ResponseHook
	metrics      *ClientMetrics
	metricsMu    sync.RWMutex
	requestGroup group
}

// ClientOption allows customizing the client's behavior.
// Use the With* functions to create options.
type ClientOption func(*Client)

// NewClient creates a new ERLC API client with the given server key and options.
//
// Example:
//
//	client := NewClient("your-server-key",
//	    WithTimeout(time.Second*15),
//	    WithBaseURL("https://custom-url.com"),
//	    WithGlobalAPIKey("your-global-key"),
//	)
func NewClient(apiKey string, opts ...ClientOption) *Client {
	if apiKey == "" {
		panic("server key is required")
	}

	// Create default cache config but disable it by default
	defaultCache := DefaultCacheConfig()
	defaultCache.Enabled = false

	c := &Client{
		httpClient: &http.Client{
			Timeout: time.Second * 10,
		},
		baseURL:     "https://api.policeroleplay.community",
		apiKey:      apiKey,
		rateLimiter: NewRateLimiter(),
		cache:       defaultCache,
		metrics:     &ClientMetrics{},
	}

	// Apply custom options
	for _, opt := range opts {
		opt(c)
	}

	// Initialize cache if enabled
	if c.cache != nil && c.cache.Enabled && c.cache.Cache == nil {
		c.cache.Cache = NewMemoryCache()
	}

	return c
}

// WithTimeout sets a custom timeout for all requests.
// The default timeout is 10 seconds.
func WithTimeout(timeout time.Duration) ClientOption {
	return func(c *Client) {
		c.httpClient.Timeout = timeout
	}
}

// WithBaseURL sets a custom base URL for the API.
// This is useful for testing or using a different API endpoint.
func WithBaseURL(url string) ClientOption {
	return func(c *Client) {
		c.baseURL = url
	}
}

// WithHTTPClient allows using a custom HTTP client.
func WithHTTPClient(httpClient *http.Client) ClientOption {
	return func(c *Client) {
		c.httpClient = httpClient
	}
}

// WithResponseHook registers a hook to observe response metadata.
func WithResponseHook(h ResponseHook) ClientOption {
	return func(c *Client) {
		c.responseHook = h
	}
}

// WithCache enables caching with the specified configuration.
//
// Example:
//
//	cacheConfig := &CacheConfig{
//	    Enabled:      true,
//	    StaleIfError: true,
//	    TTL:          time.Second * 1,
//	    Cache:        NewMemoryCache(),
//	}
func WithCache(config *CacheConfig) ClientOption {
	return func(c *Client) {
		c.cache = config
	}
}

// WithGlobalAPIKey sets a global API key for higher rate limits.
// The global API key is sent in the Authorization header.
//
// Example:
//
//	client := NewClient("your-server-key",
//	    WithGlobalAPIKey("your-global-key"),
//	)
func WithGlobalAPIKey(globalAPIKey string) ClientOption {
	return func(c *Client) {
		c.globalAPIKey = globalAPIKey
	}
}

// WithRateLimiter allows using an existing RateLimiter.
// This is essential for large bots using a Global API Key to ensure all clients
// respect the account-wide rate limits.
func WithRateLimiter(rl *RateLimiter) ClientOption {
	return func(c *Client) {
		c.rateLimiter = rl
	}
}

// WithQueue allows using an existing RequestQueue. 
// This is recommended for large bots to share a single worker pool across all server clients.
func WithQueue(q *RequestQueue) ClientOption {
	return func(c *Client) {
		if c.queue != nil && c.ownsQueue {
			c.queue.Stop()
		}
		c.queue = q
		c.ownsQueue = false
	}
}

// WithRequestQueue enables automatic request queueing with the specified
// number of workers and interval between requests.
//
// Example:
//
//	client := NewClient("your-server-key",
//	    WithRequestQueue(2, time.Second),
//	)
func WithRequestQueue(workers int, interval time.Duration) ClientOption {
	return func(c *Client) {
		if c.queue != nil && c.ownsQueue {
			c.queue.Stop()
		}
		q := NewRequestQueue(workers, interval)
		q.Start()
		c.queue = q
		c.ownsQueue = true
	}
}

// Close stops background goroutines and releases resources associated with the client.
// This includes closing the cache cleanup goroutine if caching is enabled, and stopping
// the request queue if one was configured.
//
// It is safe to call Close() multiple times; subsequent calls are no-ops.
// Once Close() is called, the client should not be used further, though this is
// not enforced.
//
// It is recommended to call Close() when the client is no longer needed, especially
// in long-running applications, to prevent goroutine leaks.
//
// Example:
//
//	client := NewClient("your-api-key")
//	defer client.Close() // Clean up when done
func (c *Client) Close() {
	if c.cache != nil && c.cache.Cache != nil {
		// Close the cache if it's a MemoryCache instance
		if mc, ok := c.cache.Cache.(*MemoryCache); ok {
			mc.Close()
		}
	}
	if c.queue != nil && c.ownsQueue {
		// Stop the request queue only if it was created by this client
		c.queue.Stop()
	}
}

// Metrics returns a copy of the current client metrics.
func (c *Client) Metrics() ClientMetrics {
	c.metricsMu.RLock()
	defer c.metricsMu.RUnlock()
	return *c.metrics
}


