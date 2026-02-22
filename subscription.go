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

	opts := ServerQueryOptions{}
	for _, eventType := range types {
		switch eventType {
		case EventTypePlayers:
			opts.Players = true
		case EventTypeVehicles:
			opts.Vehicles = true
		case EventTypeCommands:
			opts.CommandLogs = true
		case EventTypeModCalls:
			opts.ModCalls = true
		case EventTypeKills:
			opts.KillLogs = true
		case EventTypeJoins:
			opts.JoinLogs = true
		}
	}

	if resp, err := c.GetServer(ctx, opts); err == nil {
		if opts.Players {
			state.players = newPlayerSetFromSlice(resp.Players)
		}
		if opts.Vehicles {
			for _, v := range resp.Vehicles {
				state.vehicleSet[v.Owner+":"+v.Name] = struct{}{}
			}
		}
		if opts.CommandLogs && len(resp.CommandLogs) > 0 {
			state.commandTime = resp.CommandLogs[0].Timestamp
		}
		if opts.ModCalls && len(resp.ModCalls) > 0 {
			state.modCallTime = resp.ModCalls[0].Timestamp
		}
		if opts.KillLogs && len(resp.KillLogs) > 0 {
			state.killTime = resp.KillLogs[0].Timestamp
		}
		if opts.JoinLogs && len(resp.JoinLogs) > 0 {
			state.joinTime = resp.JoinLogs[0].Timestamp
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
				if resp, err := c.GetServer(ctx, opts); err == nil {

					if opts.Players && resp.Players != nil {
						newSet := newPlayerSetFromSlice(resp.Players)
						mu.Lock()
						oldSet := state.players
						state.players = newSet
						mu.Unlock()

						changes := make([]PlayerEvent, 0)
						for _, player := range resp.Players {
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
							case sub.Events <- Event{Type: EventTypePlayers, Data: changes}:
							default:
							}
						}
					}

					if opts.CommandLogs && len(resp.CommandLogs) > 0 {
						mu.RLock()
						lastTime := state.commandTime
						mu.RUnlock()

						if resp.CommandLogs[0].Timestamp > lastTime {
							mu.Lock()
							state.commandTime = resp.CommandLogs[0].Timestamp
							mu.Unlock()

							select {
							case sub.Events <- Event{Type: EventTypeCommands, Data: resp.CommandLogs}:
							default:
							}
						}
					}

					if opts.ModCalls && len(resp.ModCalls) > 0 {
						mu.RLock()
						lastTime := state.modCallTime
						mu.RUnlock()

						if resp.ModCalls[0].Timestamp > lastTime {
							mu.Lock()
							state.modCallTime = resp.ModCalls[0].Timestamp
							mu.Unlock()

							select {
							case sub.Events <- Event{Type: EventTypeModCalls, Data: resp.ModCalls}:
							default:
							}
						}
					}

					if opts.KillLogs && len(resp.KillLogs) > 0 {
						mu.RLock()
						lastTime := state.killTime
						mu.RUnlock()

						if resp.KillLogs[0].Timestamp > lastTime {
							mu.Lock()
							state.killTime = resp.KillLogs[0].Timestamp
							mu.Unlock()

							select {
							case sub.Events <- Event{Type: EventTypeKills, Data: resp.KillLogs}:
							default:
							}
						}
					}

					if opts.JoinLogs && len(resp.JoinLogs) > 0 {
						mu.RLock()
						lastTime := state.joinTime
						mu.RUnlock()

						if resp.JoinLogs[0].Timestamp > lastTime {
							mu.Lock()
							state.joinTime = resp.JoinLogs[0].Timestamp
							mu.Unlock()

							select {
							case sub.Events <- Event{Type: EventTypeJoins, Data: resp.JoinLogs}:
							default:
							}
						}
					}

					if opts.Vehicles && resp.Vehicles != nil {
						newSet := make(map[string]struct{})
						for _, v := range resp.Vehicles {
							newSet[v.Owner+":"+v.Name] = struct{}{}
						}

						mu.Lock()
						oldSet := state.vehicleSet
						state.vehicleSet = newSet
						mu.Unlock()

						newVehicles := make([]ERLCVehicle, 0)
						for _, vehicle := range resp.Vehicles {
							key := vehicle.Owner + ":" + vehicle.Name
							if _, exists := oldSet[key]; !exists {
								newVehicles = append(newVehicles, vehicle)
							}
						}

						if len(newVehicles) > 0 {
							select {
							case sub.Events <- Event{Type: EventTypeVehicles, Data: newVehicles}:
							default:
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
