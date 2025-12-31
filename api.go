package erlcgo

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// GetPlayers retrieves a list of players currently on the server.
//
// Example:
//
//	players, err := client.GetPlayers(ctx)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	for _, player := range players {
//	    fmt.Printf("Player: %s, Team: %s\n", player.Player, player.Team)
//	}
func (c *Client) GetPlayers(ctx context.Context) ([]ERLCServerPlayer, error) {
	var players []ERLCServerPlayer
	err := c.get(ctx, "/server/players", &players)
	return players, err
}

// GetCommandLogs retrieves the command execution history.
// The logs are ordered from newest to oldest.
//
// Example:
//
//	logs, err := client.GetCommandLogs(ctx)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	for _, log := range logs {
//	    fmt.Printf("%s executed: %s\n", log.Player, log.Command)
//	}
func (c *Client) GetCommandLogs(ctx context.Context) ([]ERLCCommandLog, error) {
	var logs []ERLCCommandLog
	err := c.get(ctx, "/server/commandlogs", &logs)
	return logs, err
}

// GetModCalls retrieves the moderation call history.
// Returns a list of moderation calls ordered from newest to oldest.
//
// Example:
//
//	calls, err := client.GetModCalls(ctx)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	for _, call := range calls {
//	    fmt.Printf("Call by %s handled by %s\n", call.Caller, call.Moderator)
//	}
func (c *Client) GetModCalls(ctx context.Context) ([]ERLCModCallLog, error) {
	var logs []ERLCModCallLog
	err := c.get(ctx, "/server/modcalls", &logs)
	return logs, err
}

// GetKillLogs retrieves the kill log history.
// Returns a list of kills ordered from newest to oldest.
//
// Example:
//
//	kills, err := client.GetKillLogs(ctx)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	for _, kill := range kills {
//	    fmt.Printf("%s was killed by %s\n", kill.Killed, kill.Killer)
//	}
func (c *Client) GetKillLogs(ctx context.Context) ([]ERLCKillLog, error) {
	var logs []ERLCKillLog
	err := c.get(ctx, "/server/killlogs", &logs)
	return logs, err
}

// GetJoinLogs retrieves the server join/leave history.
// Returns a list of join/leave events ordered from newest to oldest.
//
// Example:
//
//	joins, err := client.GetJoinLogs(ctx)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	for _, join := range joins {
//	    action := "joined"
//	    if !join.Join {
//	        action = "left"
//	    }
//	    fmt.Printf("%s %s the server\n", join.Player, action)
//	}
func (c *Client) GetJoinLogs(ctx context.Context) ([]ERLCJoinLog, error) {
	var logs []ERLCJoinLog
	err := c.get(ctx, "/server/joinlogs", &logs)
	return logs, err
}

// GetVehicles retrieves a list of all vehicles on the server.
// Returns information about each vehicle including its owner and texture.
//
// Example:
//
//	vehicles, err := client.GetVehicles(ctx)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	for _, vehicle := range vehicles {
//	    fmt.Printf("%s owns a %s\n", vehicle.Owner, vehicle.Name)
//	}
func (c *Client) GetVehicles(ctx context.Context) ([]ERLCVehicle, error) {
	var vehicles []ERLCVehicle
	err := c.get(ctx, "/server/vehicles", &vehicles)
	return vehicles, err
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

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/server/command", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	return c.doRequest(req, nil)
}

// get is an internal helper method for making GET requests.
// It handles the creation of the request and response parsing.
func (c *Client) get(ctx context.Context, path string, v interface{}) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	return c.doRequest(req, v)
}

// doRequest is an internal helper method that executes HTTP requests.
// It handles authorization, rate limiting, and error parsing.
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
				if v != nil {
					data, err := json.Marshal(cached)
					if err != nil {
						return fmt.Errorf("failed to marshal cached data: %w", err)
					}
					return json.Unmarshal(data, v)
				}
				return nil
			}
		}
	}

	execute := func() error {
		if c.rateLimiter != nil {
			if wait, shouldWait := c.rateLimiter.ShouldWait("global"); shouldWait {
				time.Sleep(wait)
			}
		}

		if c.httpClient == nil {
			return fmt.Errorf("http client not initialized")
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("request failed: %w", err)
		}
		if resp == nil {
			return fmt.Errorf("received nil response")
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("failed to read response: %w", err)
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
				// Try to return stale data on error
				if cached, ok := c.cache.Cache.Get(c.cache.Prefix + req.URL.String()); ok {
					if v != nil {
						data, _ := json.Marshal(cached)
						return json.Unmarshal(data, v)
					}
					return nil
				}
			}
			return apiErr
		}

		if resp.StatusCode == http.StatusOK && v != nil {
			var rawData interface{}
			if err := json.Unmarshal(body, &rawData); err != nil {
				return fmt.Errorf("failed to decode response: %w", err)
			}

			if c.cache != nil && c.cache.Enabled && c.cache.Cache != nil && req.Method == http.MethodGet {
				c.cache.Cache.Set(c.cache.Prefix+req.URL.String(), rawData, c.cache.TTL)
			}

			data, err := json.Marshal(rawData)
			if err != nil {
				return fmt.Errorf("failed to marshal data: %w", err)
			}
			if err := json.Unmarshal(data, v); err != nil {
				return fmt.Errorf("failed to unmarshal data: %w", err)
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

		return nil
	}

	if c.queue != nil {
		return c.queue.Enqueue(req.Context(), execute)
	}
	return execute()
}




