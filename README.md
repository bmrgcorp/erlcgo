# erlcgo

A powerful, concurrent-safe Go client for the ERLC API with automatic rate limiting and request queueing.

## Features

- ✨ Full API coverage
- 🚦 Automatic rate limiting
- 📡 Request queueing system
- ⌛ Context support for timeouts and cancellation
- 🔄 Retry mechanism
- 🛡️ Thread-safe
- 🎯 Zero external dependencies

## Installation

```bash
go get github.com/bmrgcorp/erlcgo
```

## Quick Start

```go
package main

import (
    "context"
    "log"
    "time"
    "github.com/bmrgcorp/erlcgo"
)

func main() {
    // Create a new client
    client := erlcgo.NewClient("your-api-key")

    // Create context with timeout
    ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
    defer cancel()

    // Get players
    players, err := client.GetPlayers(ctx)
    if err != nil {
        log.Fatal(err)
    }

    // Execute command
    err = client.ExecuteCommand(ctx, ":pm NoahCxrest Hello, World!")
    if err != nil {
        log.Fatal(err)
    }
}
```

## Advanced Usage

### Request Queueing

Enable request queueing to prevent rate limits when making many requests:

```go
client := erlcgo.NewClient(
    apiKey,
    erlcgo.WithRequestQueue(2, time.Second), // 2 workers, 1 second between requests
)
```

### Custom Timeouts

Set custom timeouts per client:

```go
client := erlcgo.NewClient(
    apiKey,
    erlcgo.WithTimeout(time.Second*15),
)
```

### Custom HTTP Client

Use a custom HTTP client for advanced configurations:

```go
httpClient := &http.Client{
    Timeout: time.Second * 30,
    Transport: &http.Transport{
        MaxIdleConns: 10,
        IdleConnTimeout: time.Second * 30,
    },
}

client := erlcgo.NewClient(
    apiKey,
    erlcgo.WithHTTPClient(httpClient),
)
```

### Error Handling

The client provides detailed error information:

```go
players, err := client.GetPlayers(ctx)
if err != nil {
    if apiErr, ok := err.(*erlcgo.APIError); ok {
        // Access error details
        fmt.Printf("Code: %d\n", apiErr.Code)
        fmt.Printf("Message: %s\n", apiErr.Message)
        // Get friendly error message
        fmt.Println(erlcgo.GetFriendlyErrorMessage(apiErr))
    }
    return
}
```

### Available Methods

```go
// Players
players, err := client.GetPlayers(ctx)

// Command Logs
logs, err := client.GetCommandLogs(ctx)

// Moderation Calls
calls, err := client.GetModCalls(ctx)

// Kill Logs
kills, err := client.GetKillLogs(ctx)

// Join/Leave Logs
joins, err := client.GetJoinLogs(ctx)

// Vehicles
vehicles, err := client.GetVehicles(ctx)

// Execute Commands
err = client.ExecuteCommand(ctx, ":pm NoahCxrest Hello, World!")
```

## Best Practices

1. **Always Use Context**
   - Set appropriate timeouts
   - Enable cancellation when needed
   ```go
   ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
   defer cancel()
   ```

2. **Enable Request Queueing for Bulk Operations**
   - Prevents rate limiting
   - Manages concurrent requests
   ```go
   client := erlcgo.NewClient(
       apiKey,
       erlcgo.WithRequestQueue(1, time.Second),
   )
   ```

3. **Handle Rate Limits Gracefully**
   - Check for APIError type
   - Use friendly error messages
   ```go
   if err != nil {
       if apiErr, ok := err.(*erlcgo.APIError); ok {
           log.Println(erlcgo.GetFriendlyErrorMessage(apiErr))
           // Implement backoff strategy
           time.Sleep(time.Second * 5)
           return
       }
   }
   ```

4. **Clean Up Resources**
   - Use defer for context cancellation
   - Close long-running operations properly

5. **Configure Timeouts Appropriately**
   - Set client-level timeouts
   - Use context timeouts for individual requests
   - Consider network conditions

## Rate Limits

The API has rate limits per endpoint. The client automatically:
- Tracks rate limit headers
- Queues requests when needed
- Provides backoff mechanisms
- Retries failed requests

## Thread Safety

All client methods are safe for concurrent use. The client handles:
- Request queueing
- Rate limit tracking
- Response parsing
- Error handling
