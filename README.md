# erlcgo

A powerful, feature-rich Go client for the Emergency Response: Liberty County (ER:LC) API with built-in concurrency safety, automatic rate limiting, real-time events, and caching support.

## Table of Contents

- [Features](#features)
- [Installation](#installation)
- [Basic Usage](#basic-usage)
- [Client Configuration](#client-configuration)
- [API Methods](#api-methods)
- [Real-time Events](#real-time-events)
- [Event Filtering](#event-filtering)
- [Error Handling](#error-handling)
- [Rate Limiting](#rate-limiting)
- [Request Queueing](#request-queueing)
- [Caching](#caching)
- [Best Practices](#best-practices)
- [Contributing](#contributing)
- [Bug Reports](#bug-reports)
- [License](#license)

## Features

- 🌟 Complete ERLC API coverage with type safety
- 📡 Real-time event system with type-safe handlers
- 🚦 Smart rate limiting with automatic backoff
- 📱 Automatic request queueing and batching
- 🔄 Built-in retry mechanism with configurable policies
- ⚡ High-performance caching system
- 💪 Fully concurrent-safe
- ⌛ Context support for timeouts and cancellation
- 🎯 Zero external dependencies

## Installation

```bash
go get github.com/bmrgcorp/erlcgo
```

## Basic Usage

```go
package main

import (
    "context"
    "fmt"
    "time"
    "github.com/bmrgcorp/erlcgo"
)

func main() {
    // Initialize client with options
    client := erlcgo.NewClient("your-server-key",
        erlcgo.WithTimeout(time.Second*15),
        erlcgo.WithRequestQueue(2, time.Second),
        erlcgo.WithGlobalAPIKey("your-global-key"), // Optional global API key
        erlcgo.WithCache(&erlcgo.CacheConfig{
            Enabled: true,
            TTL:     time.Minute,
        }),
    )

    // Create context with timeout
    ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
    defer cancel()

    // Get current players
    players, err := client.GetPlayers(ctx)
    if err != nil {
        fmt.Printf("Error: %v\n", erlcgo.GetFriendlyErrorMessage(err))
        return
    }

    // Real-time event subscription
    sub, err := client.SubscribeWithConfig(ctx, &erlcgo.EventConfig{
        PollInterval: time.Second,
        BatchEvents:  true,
        LogErrors:    true,
    }, erlcgo.EventTypePlayers, erlcgo.EventTypeCommands)

    if err != nil {
        fmt.Printf("Failed to subscribe: %v\n", err)
        return
    }
    defer sub.Close()

    // Register type-safe event handlers
    sub.Handle(erlcgo.HandlerRegistration{
        PlayerHandler: func(changes []erlcgo.PlayerEvent) {
            for _, change := range changes {
                fmt.Printf("Player %s: %s\n", change.Player.Player, change.Type)
            }
        },
    })

    // Wait for events or context cancellation
    <-ctx.Done()
}
```

## Authentication

The ERLC API uses two types of API keys:

- **Server Key** (required): Identifies your specific server and is sent in the `Server-Key` header
- **Global API Key** (optional): Provides higher rate limits for large applications and is sent in the `Authorization` header


```go
// Without global API key
client := erlcgo.NewClient("your-server-key")

// With global API key (for large applications serving 150+ servers)
client := erlcgo.NewClient("your-server-key",
    erlcgo.WithGlobalAPIKey("your-global-key"),
)
```

## Client Configuration

```go
client := erlcgo.NewClient("your-server-key",
    // Global API key (optional)
    erlcgo.WithGlobalAPIKey("your-global-key"),

    // Custom HTTP client
    erlcgo.WithHTTPClient(&http.Client{
        Timeout: time.Second * 30,
        Transport: &http.Transport{
            MaxIdleConns:    10,
            IdleConnTimeout: time.Second * 30,
        },
    }),

    // Request queueing
    erlcgo.WithRequestQueue(2, time.Second),

    // Caching
    erlcgo.WithCache(&erlcgo.CacheConfig{
        Enabled:      true,
        TTL:          time.Minute,
        StaleIfError: true,
    }),

    // Custom base URL (if you want to do this for some reason..)
    erlcgo.WithBaseURL("https://api.bmrg.app"),
)
```

## API Methods

```go
// Player Management
players, err := client.GetPlayers(ctx)

// Server Commands
err = client.ExecuteCommand(ctx, ":pm NoahCxrest Hello!")

// Logs
commandLogs, err := client.GetCommandLogs(ctx)
modCalls, err := client.GetModCalls(ctx)
killLogs, err := client.GetKillLogs(ctx)
joinLogs, err := client.GetJoinLogs(ctx)

// Vehicle Management
vehicles, err := client.GetVehicles(ctx)
```

## Real-time Events

```go
config := &erlcgo.EventConfig{
    PollInterval:        time.Second,
    BufferSize:         200,
    RetryOnError:       true,
    BatchEvents:        true,
    BatchWindow:        time.Millisecond * 100,
    LogErrors:          true,
}

sub, err := client.SubscribeWithConfig(ctx, config,
    erlcgo.EventTypePlayers,
    erlcgo.EventTypeCommands,
    erlcgo.EventTypeKills,
)
if err != nil {
    log.Fatal(err)
}
defer sub.Close()

sub.Handle(erlcgo.HandlerRegistration{
    PlayerHandler: func(changes []erlcgo.PlayerEvent) {
        for _, change := range changes {
            fmt.Printf("Player %s: %s\n", change.Player.Player, change.Type)
        }
    },
    KillHandler: func(kills []erlcgo.ERLCKillLog) {
        for _, kill := range kills {
            fmt.Printf("Kill: %s -> %s\n", kill.Killer, kill.Killed)
        }
    },
})
```

## Event Filtering

```go
config := &erlcgo.EventConfig{
    FilterFunc: func(e erlcgo.Event) bool {
        switch e.Type {
        case erlcgo.EventTypePlayers:
            changes := e.Data.([]erlcgo.PlayerEvent)
            for _, change := range changes {
                if change.Player.Team == "Sheriff" {
                    return true
                }
            }
        case erlcgo.EventTypeKills:
            kills := e.Data.([]erlcgo.ERLCKillLog)
            return len(kills) > 0 && kills[0].Killer != ""
        }
        return false
    },
}
```

## Error Handling

```go
if err != nil {
    switch apiErr := err.(*erlcgo.APIError); apiErr.Code {
    case 1001:
        // Server communication error
        time.Sleep(time.Second * 5)
        retry()
    case 4001:
        // Rate limit hit
        handleRateLimit()
    default:
        log.Printf("Error: %s\n", erlcgo.GetFriendlyErrorMessage(err))
    }
}
```

## Rate Limiting

The client automatically handles rate limits by:

- Tracking rate limit headers
- Implementing exponential backoff
- Queuing requests when limits are hit
- Providing real-time rate limit status

## Request Queueing

Enable automatic request queueing to prevent rate limits:

```go
client := erlcgo.NewClient("your-api-key",
    erlcgo.WithRequestQueue(2, time.Second),
)
```

## Caching

Configure caching to improve performance and reduce API calls:

```go
client := erlcgo.NewClient("your-api-key",
    erlcgo.WithCache(&erlcgo.CacheConfig{
        Enabled:      true,
        TTL:          time.Minute,
        StaleIfError: true,
        MaxItems:     1000,
    }),
)
```


## Metrics 

For large-scale bots, you can track internal client health metrics:

```go
// Get real-time stats
stats := client.Metrics()

fmt.Printf("Requests: %d | Errors: %d\n", stats.TotalRequests, stats.TotalErrors)
fmt.Printf("Cache Efficiency: %.2f%%\n", float64(stats.CacheHits)/float64(stats.TotalRequests)*100)
fmt.Printf("Avg Latency: %s\n", stats.AvgResponseTime)
```

## Best Practices

1. **Use Contexts for Control**

   ```go
   ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
   defer cancel()
   ```

2. **Handle Rate Limits**

   ```go
   client := erlcgo.NewClient("your-server-key",
       erlcgo.WithRequestQueue(1, time.Second),
   )
   ```

3. **Enable Caching**

   ```go
   client := erlcgo.NewClient("your-server-key",
       erlcgo.WithCache(&erlcgo.CacheConfig{
           Enabled: true,
           TTL:    time.Minute * 5,
       }),
   )
   ```

4. **Clean Up Resources**

   ```go
   defer sub.Close()
   defer cancel()
   ```

5. **Use Type-Safe Event Handlers**
   ```go
   sub.Handle(erlcgo.HandlerRegistration{
       PlayerHandler: func(changes []erlcgo.PlayerEvent) {
           // this is type safe. u can put things in here that use the player event type
       },
   })
   ```

## Contributing

We welcome contributions of all kinds, whether it's bug fixes, new features, or documentation improvements. To contribute, please follow these steps:

1. **Fork the Repository** – Create your own copy of the project.
2. **Create a Branch** – Work on a separate branch for your changes:
   ```bash
   git checkout -b feature-or-fix-name
   ```
3. **Make Your Changes** – Ensure your code follows the project's style and guidelines.
4. **Test Thoroughly** – If applicable, add tests to verify your changes.
5. **Commit and Push** – Keep commits clear and concise:
   ```bash
   git commit -m "Brief description of changes"
   git push origin feature-or-fix-name
   ```
6. **Open a Pull Request** – Submit your changes for review. Ensure your PR includes a clear description of the changes and any relevant issue references.

We appreciate all contributions and will review PRs as soon as possible.

## Bug Reports

If you encounter a bug, you have two options for reporting it:

1. **Submit a Fix** – If you're able to resolve the issue, open a pull request with your fix following the contributing guidelines above.
2. **Request Support** – If you're unable to fix the issue yourself, report it in our [support server](https://discord.gg/boomerang) via the support forum. Please include:
   - A clear description of the issue
   - Steps to reproduce it
   - Expected vs actual behaviour
   - Any relevant error messages or logs

By keeping reports detailed, we can resolve issues more efficiently.

## License

APACHE 2.0 License - see [LICENSE](LICENSE) for details
