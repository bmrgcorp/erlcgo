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
- [Support](#support)

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
    client := erlcgo.NewClient("your-api-key",
        erlcgo.WithTimeout(time.Second*15),
        erlcgo.WithRequestQueue(2, time.Second),
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

## Client Configuration

```go
client := erlcgo.NewClient("your-api-key",
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

## Best Practices

1. **Use Contexts for Control**
   ```go
   ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
   defer cancel()
   ```

2. **Handle Rate Limits**
   ```go
   client := erlcgo.NewClient("your-api-key",
       erlcgo.WithRequestQueue(1, time.Second),
   )
   ```

3. **Enable Caching**
   ```go
   client := erlcgo.NewClient("your-api-key",
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
We welcome contributions! Make sure to read our Code of Conduct before contributing.

## Support
For support, bug reports, or feature requests, please:
1. Check existing [GitHub Issues](https://github.com/bmrgcorp/erlcgo/issues)
2. Open a new issue with detailed information

## License
APACHE 2.0 License - see [LICENSE](LICENSE) for details
