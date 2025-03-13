package erlcgo

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
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
	req.Header.Set("Server-Key", c.apiKey)

	execute := func() error {
		// Always check global rate limit
		if wait, shouldWait := c.rateLimiter.ShouldWait("global"); shouldWait {
			time.Sleep(wait)
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("request failed: %w", err)
		}
		defer resp.Body.Close()

		// Update rate limit info from headers
		if remaining := resp.Header.Get("X-RateLimit-Remaining"); remaining != "" {
			rem, _ := strconv.Atoi(remaining)
			limit, _ := strconv.Atoi(resp.Header.Get("X-RateLimit-Limit"))
			reset, _ := strconv.ParseInt(resp.Header.Get("X-RateLimit-Reset"), 10, 64)
			c.rateLimiter.UpdateFromHeaders("global", limit, rem, time.Unix(reset, 0))
		}

		if resp.StatusCode == 429 {
			var rateLimitError APIError
			if err := json.NewDecoder(resp.Body).Decode(&rateLimitError); err == nil {
				// Add extra delay when we hit the rate limit
				c.rateLimiter.UpdateFromHeaders("global", 0, 0, time.Now().Add(time.Second*5))
				return &rateLimitError
			}
		}

		if resp.StatusCode != http.StatusOK {
			var apiError APIError
			if err := json.NewDecoder(resp.Body).Decode(&apiError); err != nil {
				return fmt.Errorf("unknown error (status %d)", resp.StatusCode)
			}
			return &apiError
		}

		if v != nil {
			return json.NewDecoder(resp.Body).Decode(v)
		}
		return nil
	}

	if c.queue != nil {
		return c.queue.Enqueue(req.Context(), execute)
	}
	return execute()
}
