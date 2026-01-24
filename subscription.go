package erlcgo

import (
	"context"
	"sync"
	"time"
)

func (s *Subscription) Close() {
	close(s.done)
}

func newPlayerSetFromSlice(players []ERLCServerPlayer) playerSet {
	set := make(playerSet)
	for _, p := range players {
		set[p.Player] = struct{}{}
	}
	return set
}

// DefaultEventConfig returns the default event configuration
func DefaultEventConfig() *EventConfig {
	return &EventConfig{
		PollInterval:        time.Second * 2,
		BufferSize:          100,
		RetryOnError:        true,
		RetryInterval:       time.Second * 5,
		IncludeInitialState: false,
		BatchEvents:         false,
		BatchWindow:         time.Millisecond * 100,
		LogErrors:           false,
		TimeFormat:          time.RFC3339,
	}
}

func (s *Subscription) Handle(handlers HandlerRegistration) {
	s.handlers = handlers
	go s.processEvents()
}

func (s *Subscription) processEvents() {
	for event := range s.Events {
		func() {
			defer func() {
				if r := recover(); r != nil {
					if s.config != nil && s.config.OnPanic != nil {
						s.config.OnPanic(r)
					}
				}
			}()

			switch event.Type {
			case EventTypePlayers:
				if s.handlers.PlayerHandler != nil {
					s.handlers.PlayerHandler(event.Data.([]PlayerEvent))
				}
			case EventTypeCommands:
				if s.handlers.CommandHandler != nil {
					s.handlers.CommandHandler(event.Data.([]ERLCCommandLog))
				}
			case EventTypeKills:
				if s.handlers.KillHandler != nil {
					s.handlers.KillHandler(event.Data.([]ERLCKillLog))
				}
			case EventTypeModCalls:
				if s.handlers.ModCallHandler != nil {
					s.handlers.ModCallHandler(event.Data.([]ERLCModCallLog))
				}
			case EventTypeJoins:
				if s.handlers.JoinHandler != nil {
					s.handlers.JoinHandler(event.Data.([]ERLCJoinLog))
				}
			case EventTypeVehicles:
				if s.handlers.VehicleHandler != nil {
					s.handlers.VehicleHandler(event.Data.([]ERLCVehicle))
				}
			}
		}()
	}
}

// SubscribeWithConfig creates a new subscription with custom configuration
func (c *Client) SubscribeWithConfig(ctx context.Context, config *EventConfig, types ...EventType) (*Subscription, error) {
	if config == nil {
		config = DefaultEventConfig()
	}

	sub := &Subscription{
		Events: make(chan Event, config.BufferSize),
		done:   make(chan struct{}),
		config: config,
	}

	state := &lastState{
		players:     make(playerSet),
		vehicleSet:  make(map[string]struct{}),
		commandTime: 0,
		modCallTime: 0,
		killTime:    0,
		joinTime:    0,
		initialized: false,
	}

	for _, eventType := range types {
		switch eventType {
		case EventTypePlayers:
			if players, err := c.GetPlayers(ctx); err == nil {
				state.players = newPlayerSetFromSlice(players)
			}
		case EventTypeVehicles:
			if vehicles, err := c.GetVehicles(ctx); err == nil {
				for _, v := range vehicles {
					state.vehicleSet[v.Owner+":"+v.Name] = struct{}{}
				}
			}
		case EventTypeCommands:
			if logs, err := c.GetCommandLogs(ctx); err == nil && len(logs) > 0 {
				state.commandTime = logs[0].Timestamp
			}
		case EventTypeModCalls:
			if logs, err := c.GetModCalls(ctx); err == nil && len(logs) > 0 {
				state.modCallTime = logs[0].Timestamp
			}
		case EventTypeKills:
			if logs, err := c.GetKillLogs(ctx); err == nil && len(logs) > 0 {
				state.killTime = logs[0].Timestamp
			}
		case EventTypeJoins:
			if logs, err := c.GetJoinLogs(ctx); err == nil && len(logs) > 0 {
				state.joinTime = logs[0].Timestamp
			}
		}
	}

	state.initialized = true

	go func() {
		defer close(sub.Events)

		ticker := time.NewTicker(config.PollInterval)
		defer ticker.Stop()

		var mu sync.RWMutex

		for {
			select {
			case <-ctx.Done():
				return
			case <-sub.done:
				return
			case <-ticker.C:
				for _, eventType := range types {
					switch eventType {
					case EventTypePlayers:
						if players, err := c.GetPlayers(ctx); err == nil {
							newSet := newPlayerSetFromSlice(players)
							mu.Lock()
							oldSet := state.players
							state.players = newSet
							mu.Unlock()

							changes := make([]PlayerEvent, 0)

							for _, player := range players {
								if _, exists := oldSet[player.Player]; !exists {
									changes = append(changes, PlayerEvent{
										Player: player,
										Type:   "join",
									})
								}
							}

							for player := range oldSet {
								if _, exists := newSet[player]; !exists {
									changes = append(changes, PlayerEvent{
										Player: ERLCServerPlayer{Player: player},
										Type:   "leave",
									})
								}
							}

							if len(changes) > 0 {
								select {
								case sub.Events <- Event{Type: eventType, Data: changes}:
								default:
								}
							}
						}

					case EventTypeCommands:
						if logs, err := c.GetCommandLogs(ctx); err == nil && len(logs) > 0 {
							mu.RLock()
							lastTime := state.commandTime
							mu.RUnlock()

							if logs[0].Timestamp > lastTime {
								mu.Lock()
								state.commandTime = logs[0].Timestamp
								mu.Unlock()

								select {
								case sub.Events <- Event{Type: eventType, Data: logs}:
								default:
								}
							}
						}

					case EventTypeModCalls:
						if logs, err := c.GetModCalls(ctx); err == nil && len(logs) > 0 {
							mu.RLock()
							lastTime := state.modCallTime
							mu.RUnlock()

							if logs[0].Timestamp > lastTime {
								mu.Lock()
								state.modCallTime = logs[0].Timestamp
								mu.Unlock()

								select {
								case sub.Events <- Event{Type: eventType, Data: logs}:
								default:
								}
							}
						}

					case EventTypeKills:
						if logs, err := c.GetKillLogs(ctx); err == nil && len(logs) > 0 {
							mu.RLock()
							lastTime := state.killTime
							mu.RUnlock()

							if logs[0].Timestamp > lastTime {
								mu.Lock()
								state.killTime = logs[0].Timestamp
								mu.Unlock()

								select {
								case sub.Events <- Event{Type: eventType, Data: logs}:
								default:
								}
							}
						}

					case EventTypeJoins:
						if logs, err := c.GetJoinLogs(ctx); err == nil && len(logs) > 0 {
							mu.RLock()
							lastTime := state.joinTime
							mu.RUnlock()

							if logs[0].Timestamp > lastTime {
								mu.Lock()
								state.joinTime = logs[0].Timestamp
								mu.Unlock()

								select {
								case sub.Events <- Event{Type: eventType, Data: logs}:
								default:
								}
							}
						}

					case EventTypeVehicles:
						if vehicles, err := c.GetVehicles(ctx); err == nil {
							newSet := make(map[string]struct{})
							for _, v := range vehicles {
								newSet[v.Owner+":"+v.Name] = struct{}{}
							}

							mu.Lock()
							oldSet := state.vehicleSet
							state.vehicleSet = newSet
							mu.Unlock()

							newVehicles := make([]ERLCVehicle, 0)
							for _, vehicle := range vehicles {
								key := vehicle.Owner + ":" + vehicle.Name
								if _, exists := oldSet[key]; !exists {
									newVehicles = append(newVehicles, vehicle)
								}
							}

							if len(newVehicles) > 0 {
								select {
								case sub.Events <- Event{Type: eventType, Data: newVehicles}:
								default:
								}
							}
						}
					}
				}
			}
		}
	}()

	return sub, nil
}

func (c *Client) Subscribe(ctx context.Context, types ...EventType) (*Subscription, error) {
	return c.SubscribeWithConfig(ctx, DefaultEventConfig(), types...)
}
