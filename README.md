# erlcgo

A powerful, concurrent-safe Go client for the ERLC API with automatic rate limiting, real time event, and caching support.

## Features

- ✨ Full API coverage
- 📣 Real-time event system
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

## Event Subscriptions

Subscribe to real-time updates from the ERLC server with type-safe handlers:

```go
config := &erlcgo.EventConfig{
    PollInterval:        time.Second * 1,
    BufferSize:         200,
    RetryOnError:       true,
    RetryInterval:      time.Second * 3,
    IncludeInitialState: false,
    BatchEvents:        true,
    BatchWindow:        time.Millisecond * 100,
    LogErrors:          true,
    ErrorHandler: func(err error) {
        log.Printf("Error: %v", err)
    },
}

// Subscribe to events
sub, err := client.SubscribeWithConfig(ctx, config,
    erlcgo.EventTypePlayers,
    erlcgo.EventTypeCommands,
    erlcgo.EventTypeKills,
    erlcgo.EventTypeModCalls,
    erlcgo.EventTypeJoins,
    erlcgo.EventTypeVehicles,
)
if err != nil {
    log.Fatal(err)
}
defer sub.Close()

// Register type-safe event handlers
sub.Handle(erlcgo.HandlerRegistration{
    PlayerHandler: func(changes []erlcgo.PlayerEvent) {
        for _, change := range changes {
            fmt.Printf("[Player] %s %s\n", change.Player.Player, change.Type)
        }
    },
    CommandHandler: func(logs []erlcgo.ERLCCommandLog) {
        if len(logs) > 0 {
            fmt.Printf("[Command] %s: %s\n", logs[0].Player, logs[0].Command)
        }
    },
    KillHandler: func(kills []erlcgo.ERLCKillLog) {
        if len(kills) > 0 {
            fmt.Printf("[Kill] %s -> %s\n", kills[0].Killer, kills[0].Killed)
        }
    },
    ModCallHandler: func(calls []erlcgo.ERLCModCallLog) {
        if len(calls) > 0 {
            fmt.Printf("[ModCall] %s called mod\n", calls[0].Caller)
        }
    },
    JoinHandler: func(logs []erlcgo.ERLCJoinLog) {
        if len(logs) > 0 {
            action := "joined"
            if !logs[0].Join {
                action = "left"
            }
            fmt.Printf("[Join] %s %s\n", logs[0].Player, action)
        }
    },
    VehicleHandler: func(vehicles []erlcgo.ERLCVehicle) {
        if len(vehicles) > 0 {
            fmt.Printf("[Vehicle] %s got %s\n", vehicles[0].Owner, vehicles[0].Name)
        }
    },
})

// Wait for context cancellation
<-ctx.Done()
```

### Event Handler Types

Type-safe handlers are available for each event type:

```go
type PlayerEventHandler func([]PlayerEvent)
type CommandEventHandler func([]ERLCCommandLog)
type KillEventHandler func([]ERLCKillLog)
type ModCallEventHandler func([]ERLCModCallLog)
type JoinEventHandler func([]ERLCJoinLog)
type VehicleEventHandler func([]ERLCVehicle)
```

### Handler Registration

Register handlers using the `HandlerRegistration` struct:

```go
type HandlerRegistration struct {
    PlayerHandler  PlayerEventHandler
    CommandHandler CommandEventHandler
    KillHandler    KillEventHandler
    ModCallHandler ModCallEventHandler
    JoinHandler    JoinEventHandler
    VehicleHandler VehicleEventHandler
}
```

### Event Filtering Example

Filter specific teams with type safety:

```go
FilterFunc: func(e erlcgo.Event) bool {
    if e.Type == erlcgo.EventTypePlayers {
        changes := e.Data.([]erlcgo.PlayerEvent)
        for _, change := range changes {
            if change.Player.Team == "Sheriff" {
                return true
            }
        }
        return false
    }
    return true
},
```

### Configuration Options

| Option              | Type             | Default | Description                     |
| ------------------- | ---------------- | ------- | ------------------------------- |
| PollInterval        | time.Duration    | 2s      | How often to check for updates  |
| BufferSize          | int              | 100     | Event channel buffer size       |
| RetryOnError        | bool             | true    | Automatically retry on errors   |
| RetryInterval       | time.Duration    | 5s      | Time between retry attempts     |
| IncludeInitialState | bool             | false   | Include initial state in events |
| BatchEvents         | bool             | false   | Combine similar events          |
| BatchWindow         | time.Duration    | 100ms   | Time window for batching        |
| LogErrors           | bool             | false   | Enable error logging            |
| TimeFormat          | string           | RFC3339 | Timestamp format for logs       |
| ErrorHandler        | func(error)      | nil     | Custom error handling           |
| FilterFunc          | func(Event) bool | nil     | Event filtering function        |

### Best Practices

1. **Configure Poll Interval**

   - Balance between responsiveness and API load
   - Consider rate limits

   ```go
   PollInterval: time.Second * 2,
   ```

2. **Handle Errors**

   - Enable error logging
   - Use custom error handlers
   - Configure retry behavior

   ```go
   LogErrors: true,
   ErrorHandler: func(err error) {
       log.Printf("Error: %v", err)
   },
   ```

3. **Optimize Performance**

   - Use event filtering
   - Configure appropriate buffer sizes
   - Enable event batching for high-volume events

   ```go
   BatchEvents: true,
   BatchWindow: time.Millisecond * 100,
   ```

4. **Clean Up**
   - Always close subscriptions
   - Use context for cancellation
   ```go
   defer sub.Close()
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

### Contributing

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

### Bug Reports

If you encounter a bug, you have two options for reporting it:

1. **Submit a Fix** – If you're able to resolve the issue, open a pull request with your fix following the contributing guidelines above.
2. **Request Support** – If you're unable to fix the issue yourself, report it in our [support server](https://discord.gg/boomerang) via the support forum. Please include:
   - A clear description of the issue
   - Steps to reproduce it
   - Expected vs actual behaviour
   - Any relevant error messages or logs

By keeping reports detailed, we can resolve issues more efficiently.
