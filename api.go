package erlcgo

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// GetServer retrieves server data depending on the provided query options.
// This is the primary endpoint for the v2 API.
//
// Example:
//
//	opts := erlcgo.ServerQueryOptions{Players: true, Vehicles: true}
//	resp, err := client.GetServer(ctx, opts)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Printf("Server Name: %s\n", resp.Name)
func (c *Client) GetServer(ctx context.Context, opts ...ServerQueryOptions) (*ERLCServerResponse, error) {
	query := ""
	if len(opts) > 0 {
		opt := opts[0]
		params := []string{}
		if opt.Players {
			params = append(params, "Players=true")
		}
		if opt.Staff {
			params = append(params, "Staff=true")
		}
		if opt.JoinLogs {
			params = append(params, "JoinLogs=true")
		}
		if opt.Queue {
			params = append(params, "Queue=true")
		}
		if opt.KillLogs {
			params = append(params, "KillLogs=true")
		}
		if opt.CommandLogs {
			params = append(params, "CommandLogs=true")
		}
		if opt.ModCalls {
			params = append(params, "ModCalls=true")
		}
		if opt.Vehicles {
			params = append(params, "Vehicles=true")
		}
		if len(params) > 0 {
			query = "?"
			for i, p := range params {
				if i > 0 {
					query += "&"
				}
				query += p
			}
		}
	}

	var resp ERLCServerResponse
	err := c.get(ctx, "/v2/server"+query, &resp)
	return &resp, err
}

// ExecuteCommand executes a server command.
// The command should include the leading slash (e.g., "/announce").
//
// Example:
//
//	err := client.ExecuteCommand(ctx, "/announce Server maintenance in 5 minutes")
//	if err != nil {
//	    if apiErr, ok := err.(*APIError); ok {
//	        fmt.Println(GetFriendlyErrorMessage(apiErr))
//	    }
//	}
func (c *Client) ExecuteCommand(ctx context.Context, command string) error {
	data := map[string]string{"command": command}
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/server/command", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	return c.doRequest(req, nil)
}

// get is an internal helper that executes GET requests and parses responses.
func (c *Client) get(ctx context.Context, path string, v interface{}) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	return c.doRequest(req, v)
}

// doRequest executes HTTP requests, handling authorization, rate limiting, and errors.
func (c *Client) doRequest(req *http.Request, v interface{}) error {
	if req == nil {
		return fmt.Errorf("request cannot be nil")
	}

	if c.httpClient == nil {
		return fmt.Errorf("http client is nil - was NewClient() used to create the client?")
	}

	req.Header.Set("Server-Key", c.apiKey)

	if c.apiKey == "" {
		return fmt.Errorf("API key is empty")
	}

	// Set global API key in Authorization header if provided
	if c.globalAPIKey != "" {
		req.Header.Set("Authorization", c.globalAPIKey)
	}

	if c.cache != nil && c.cache.Enabled {
		if c.cache.Cache == nil {
			c.cache.Cache = NewMemoryCache()
		}

		if req.Method == http.MethodGet && c.cache.Cache != nil {
			cacheKey := c.cache.Prefix + req.URL.String()
			if cached, ok := c.cache.Cache.Get(cacheKey); ok {
				c.metricsMu.Lock()
				c.metrics.CacheHits++
				c.metricsMu.Unlock()
				if v != nil {
					data, err := json.Marshal(cached)
					if err != nil {
						return fmt.Errorf("failed to marshal cached data: %w", err)
					}
					return json.Unmarshal(data, v)
				}
				return nil
			}
			c.metricsMu.Lock()
			c.metrics.CacheMisses++
			c.metricsMu.Unlock()
		}
	}

	// execute performs the HTTP request and returns the raw JSON body bytes
	execute := func() ([]byte, error) {
		if c.rateLimiter != nil {
			if wait, shouldWait := c.rateLimiter.ShouldWait("global"); shouldWait {
				time.Sleep(wait)
			}
		}

		if c.httpClient == nil {
			return nil, fmt.Errorf("http client not initialized")
		}

		start := time.Now()
		resp, err := c.httpClient.Do(req)
		duration := time.Since(start)

		c.metricsMu.Lock()
		c.metrics.TotalRequests++
		// Simple moving average for response time
		if c.metrics.AvgResponseTime == 0 {
			c.metrics.AvgResponseTime = duration
		} else {
			c.metrics.AvgResponseTime = (c.metrics.AvgResponseTime + duration) / 2
		}
		c.metricsMu.Unlock()

		if err != nil {
			return nil, fmt.Errorf("request failed: %w", err)
		}
		if resp == nil {
			return nil, fmt.Errorf("received nil response")
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read response: %w", err)
		}

		rl := parseRateLimitHeaders(resp.Header)
		ra := (*time.Duration)(nil)
		if resp.StatusCode == http.StatusTooManyRequests {
			ra = parseRetryAfter(body)
			if ra == nil {
				ra = parseRetryAfterHeader(resp.Header)
			}
			if rl == nil || rl.Bucket == "" {
				if bucket := parseRateLimitBucket(body); bucket != "" {
					if rl == nil {
						rl = &RateLimitInfo{Bucket: bucket}
					} else {
						rl.Bucket = bucket
					}
				}
			}
		}

		if c.rateLimiter != nil {
			if rl != nil {
				c.rateLimiter.UpdateFromHeaders("global", rl.Limit, rl.Remaining, rl.ResetAt)
			} else if resp.StatusCode == http.StatusTooManyRequests {
				if ra != nil {
					c.rateLimiter.UpdateFromHeaders("global", 0, 0, time.Now().Add(*ra))
				} else {
					c.rateLimiter.UpdateFromHeaders("global", 0, 0, time.Now().Add(time.Second*5))
				}
			}
		}

		routeName := req.Method + " " + req.URL.Path

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			apiErr := &APIError{
				StatusCode: resp.StatusCode,
				Body:       body,
				Headers:    resp.Header.Clone(),
				RateLimit:  rl,
				RetryAfter: ra,
			}
			if len(body) > 0 {
				if err := json.Unmarshal(body, apiErr); err != nil {
					apiErr.Message = string(body)
				}
			} else {
				apiErr.Message = fmt.Sprintf("unknown error (status %d)", resp.StatusCode)
			}

			c.metricsMu.Lock()
			c.metrics.TotalErrors++
			if resp.StatusCode == http.StatusTooManyRequests {
				c.metrics.TotalRateLimits++
			}
			c.metricsMu.Unlock()

			if c.responseHook != nil {
				c.responseHook(ResponseMeta{
					Route:      routeName,
					StatusCode: resp.StatusCode,
					Headers:    resp.Header.Clone(),
					Body:       body,
					RateLimit:  rl,
					RetryAfter: ra,
					Err:        apiErr,
				})
			}

			if c.cache != nil && c.cache.StaleIfError && c.cache.Cache != nil {
				// Stale data fallback is handled locally by the caller upon receiving this error
			}
			return nil, apiErr
		}

		if resp.StatusCode == http.StatusOK {
			// Populate cache
			var rawData interface{}
			if err := json.Unmarshal(body, &rawData); err == nil {
				if c.cache != nil && c.cache.Enabled && c.cache.Cache != nil && req.Method == http.MethodGet {
					c.cache.Cache.Set(c.cache.Prefix+req.URL.String(), rawData, c.cache.TTL)
				}
			}
		}

		if c.responseHook != nil {
			c.responseHook(ResponseMeta{
				Route:      routeName,
				StatusCode: resp.StatusCode,
				Headers:    resp.Header.Clone(),
				Body:       nil,
				RateLimit:  rl,
				RetryAfter: nil,
				Err:        nil,
			})
		}

		return body, nil
	}

	var body []byte
	var err error

	runWithQueue := func() ([]byte, error) {
		if c.queue != nil {
			var b []byte
			var e error
			qErr := c.queue.Enqueue(req.Context(), func() error {
				b, e = execute()
				return e
			})
			if qErr != nil {
				return nil, qErr
			}
			return b, e
		}
		return execute()
	}

	// Request Coalescing for GET requests
	if req.Method == http.MethodGet {
		key := req.URL.String()
		res, doErr := c.requestGroup.Do(key, func() (interface{}, error) {
			return runWithQueue()
		})
		if doErr != nil {
			err = doErr
		} else if res != nil {
			body = res.([]byte)
		}
	} else {
		body, err = runWithQueue()
	}

	if err != nil {
		// Try stale cache if enabled
		if c.cache != nil && c.cache.StaleIfError && c.cache.Cache != nil {
			if cached, ok := c.cache.Cache.Get(c.cache.Prefix + req.URL.String()); ok {
				if v != nil {
					data, _ := json.Marshal(cached)
					return json.Unmarshal(data, v)
				}
				return nil
			}
		}
		return err
	}

	if v != nil && body != nil {
		return json.Unmarshal(body, v)
	}

	return nil
}

type call struct {
	wg  sync.WaitGroup
	val interface{}
	err error
}

type group struct {
	mu sync.Mutex
	m  map[string]*call
}

func (g *group) Do(key string, fn func() (interface{}, error)) (interface{}, error) {
	g.mu.Lock()
	if g.m == nil {
		g.m = make(map[string]*call)
	}
	if c, ok := g.m[key]; ok {
		g.mu.Unlock()
		c.wg.Wait()
		return c.val, c.err
	}
	c := new(call)
	c.wg.Add(1)
	g.m[key] = c
	g.mu.Unlock()

	c.val, c.err = fn()
	c.wg.Done()

	g.mu.Lock()
	delete(g.m, key)
	g.mu.Unlock()

	return c.val, c.err
}




