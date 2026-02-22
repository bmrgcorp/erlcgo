package erlcgo

import (
	"fmt"
	"net/http"
	"sync"
	"time"
)

// ERLCLocation represents a player's physical in-game location.
type ERLCLocation struct {
	LocationX      float64 `json:"LocationX"`
	LocationZ      float64 `json:"LocationZ"`
	PostalCode     string  `json:"PostalCode"`
	StreetName     string  `json:"StreetName"`
	BuildingNumber string  `json:"BuildingNumber"`
}

// ERLCServerPlayer represents a player currently in the server.
type ERLCServerPlayer struct {
	Player      string       `json:"Player"`
	Permission  string       `json:"Permission"`
	Callsign    string       `json:"Callsign"`
	Team        string       `json:"Team"`
	Location    ERLCLocation `json:"Location"`
	WantedStars int          `json:"WantedStars"`
}

// ClientMetrics tracks performance and health data for the client.
type ClientMetrics struct {
	TotalRequests    int64
	TotalErrors      int64
	TotalRateLimits  int64
	CacheHits        int64
	CacheMisses      int64
	AvgResponseTime  time.Duration
}

// ERLCStaff contains mapping lists for the current staff in the server.
type ERLCStaff struct {
	Admins  map[string]string `json:"Admins"`
	Mods    map[string]string `json:"Mods"`
	Helpers map[string]string `json:"Helpers"`
}

// ERLCServerResponse represents the full payload returned from the /v2/server endpoint.
type ERLCServerResponse struct {
	Name           string             `json:"Name"`
	OwnerId        int64              `json:"OwnerId"`
	CoOwnerIds     []int64            `json:"CoOwnerIds"`
	CurrentPlayers int                `json:"CurrentPlayers"`
	MaxPlayers     int                `json:"MaxPlayers"`
	JoinKey        string             `json:"JoinKey"`
	AccVerifiedReq string             `json:"AccVerifiedReq"`
	TeamBalance    bool               `json:"TeamBalance"`
	Players        []ERLCServerPlayer `json:"Players,omitempty"`
	Staff          *ERLCStaff         `json:"Staff,omitempty"`
	JoinLogs       []ERLCJoinLog      `json:"JoinLogs,omitempty"`
	Queue          []int64            `json:"Queue,omitempty"`
	KillLogs       []ERLCKillLog      `json:"KillLogs,omitempty"`
	CommandLogs    []ERLCCommandLog   `json:"CommandLogs,omitempty"`
	ModCalls       []ERLCModCallLog   `json:"ModCalls,omitempty"`
	Vehicles       []ERLCVehicle      `json:"Vehicles,omitempty"`
}

// ServerQueryOptions specifies which optional data sets to fetch via the /v2/server endpoint.
type ServerQueryOptions struct {
	Players     bool `url:"Players,omitempty"`
	Staff       bool `url:"Staff,omitempty"`
	JoinLogs    bool `url:"JoinLogs,omitempty"`
	Queue       bool `url:"Queue,omitempty"`
	KillLogs    bool `url:"KillLogs,omitempty"`
	CommandLogs bool `url:"CommandLogs,omitempty"`
	ModCalls    bool `url:"ModCalls,omitempty"`
	Vehicles    bool `url:"Vehicles,omitempty"`
}

// ERLCServerInfo represents metadata about the server.
type ERLCServerInfo struct {
	Name           string  `json:"Name"`
	OwnerId        int64   `json:"OwnerId"`
	CoOwnerIds     []int64 `json:"CoOwnerIds"`
	CurrentPlayers int     `json:"CurrentPlayers"`
	MaxPlayers     int     `json:"MaxPlayers"`
	JoinKey        string  `json:"JoinKey"`
	AccVerifiedReq string  `json:"AccVerifiedReq"`
	TeamBalance    bool    `json:"TeamBalance"`
}

// ERLCCommandLog represents a command executed in the server.
type ERLCCommandLog struct {
	Player    string `json:"Player"`
	Timestamp int64  `json:"Timestamp"`
	Command   string `json:"Command"`
}

// ERLCModCallLog represents a moderation call log entry.
type ERLCModCallLog struct {
	Caller    string `json:"Caller"`
	Moderator string `json:"Moderator"`
	Timestamp int64  `json:"Timestamp"`
}

// ERLCKillLog represents a kill log entry.
type ERLCKillLog struct {
	Killed    string `json:"Killed"`
	Timestamp int64  `json:"Timestamp"`
	Killer    string `json:"Killer"`
}

// ERLCJoinLog represents a join or leave log entry.
type ERLCJoinLog struct {
	Join      bool   `json:"Join"`
	Timestamp int64  `json:"Timestamp"`
	Player    string `json:"Player"`
}

// ERLCVehicle represents a vehicle in the server.
type ERLCVehicle struct {
	Texture string `json:"Texture"`
	Name    string `json:"Name"`
	Owner   string `json:"Owner"`
}

// APIError represents an error returned by the ERLC API.
// It implements the standard Go error interface.
type APIError struct {
	Code       int            `json:"code"`
	Message    string         `json:"message"`
	CommandID  string         `json:"commandId,omitempty"`
	StatusCode int            `json:"-"`
	Body       []byte         `json:"-"`
	Headers    http.Header    `json:"-"`
	RateLimit  *RateLimitInfo `json:"-"`
	RetryAfter *time.Duration `json:"-"`
}

// Error implements the error interface for APIError.
// It returns a formatted error message containing both the error code and message.
func (e *APIError) Error() string {
	return fmt.Sprintf("API error %d: %s", e.Code, e.Message)
}

// GetFriendlyErrorMessage returns a human-readable error message based on the error code
func GetFriendlyErrorMessage(err error) string {
	if apiErr, ok := err.(*APIError); ok {
		switch apiErr.Code {
		case 0:
			return "An unknown error occurred. If this persists, please contact PRC support."
		case 1001:
			return "Failed to communicate with the game server. Please try again in a few minutes."
		case 1002:
			return "An internal system error occurred. Please try again later."
		case 2000:
			return "No server key provided. Please configure your server key."
		case 2001, 2002:
			return "Invalid server key. Please check your configuration."
		case 2003:
			return "Invalid API key. Please check your configuration."
		case 2004:
			return "This server key has been banned from accessing the API."
		case 3001:
			return "Invalid command format. Please check your input."
		case 3002:
			return "The server is currently offline (no players). Please try again when players are in the server."
		case 4001:
			return "You are being rate limited. Please wait a moment and try again."
		case 4002:
			return "This command is restricted and cannot be executed."
		case 4003:
			return "The message you're trying to send contains prohibited content."
		case 9998:
			return "Access to this resource is restricted."
		case 9999:
			return "The server module is out of date. Please kick all players and try again."
		default:
			return apiErr.Message
		}
	}
	return err.Error()
}

// RateLimit represents the rate limit information for a specific bucket.
type RateLimit struct {
	// Bucket is the identifier for the rate limit bucket
	Bucket string
	// Limit is the maximum number of requests allowed in the bucket
	Limit int
	// Remaining is the number of requests remaining in the current rate limit window
	Remaining int
	// Reset is the time when the rate limit will reset
	Reset time.Time
}

// RateLimiter manages rate limits for different buckets.
type RateLimiter struct {
	mu     sync.RWMutex
	limits map[string]*RateLimit
}

// CacheConfig represents cache configuration for different endpoints
type CacheConfig struct {
	// Enabled determines if caching is enabled
	Enabled bool

	// TTL is the time-to-live for cached items
	// Items older than TTL are considered stale and will be refetched
	TTL time.Duration

	// StaleIfError determines if stale items should be returned when errors occur
	// This can help maintain availability during API outages
	StaleIfError bool

	// Cache is the cache implementation to use
	Cache Cache

	// Prefix is prepended to all cache keys
	Prefix string

	// MaxItems is the maximum number of items to store in the cache
	MaxItems int
}

// DefaultCacheConfig returns a default cache configuration.
// Note: The Cache field is set to nil by default. A MemoryCache instance will be
// created automatically when caching is enabled and a cache instance is needed.
// This prevents creating unnecessary goroutines when caching is disabled.
func DefaultCacheConfig() *CacheConfig {
	return &CacheConfig{
		Enabled:      true,
		TTL:          time.Minute * 5,
		StaleIfError: true,
		Cache:        nil, // Will be created lazily when needed
		Prefix:       "erlcgo:",
		MaxItems:     1000,
	}
}

// Event-related types
type EventType string

const (
	EventTypePlayers  EventType = "players"
	EventTypeCommands EventType = "commands"
	EventTypeModCalls EventType = "modcalls"
	EventTypeKills    EventType = "kills"
	EventTypeJoins    EventType = "joins"
	EventTypeVehicles EventType = "vehicles"
)

type Event struct {
	Type EventType
	Data interface{}
}

// Event handler types for type-safety
type PlayerEventHandler func([]PlayerEvent)
type CommandEventHandler func([]ERLCCommandLog)
type KillEventHandler func([]ERLCKillLog)
type ModCallEventHandler func([]ERLCModCallLog)
type JoinEventHandler func([]ERLCJoinLog)
type VehicleEventHandler func([]ERLCVehicle)

type HandlerRegistration struct {
	PlayerHandler  PlayerEventHandler
	CommandHandler CommandEventHandler
	KillHandler    KillEventHandler
	ModCallHandler ModCallEventHandler
	JoinHandler    JoinEventHandler
	VehicleHandler VehicleEventHandler
}

type PlayerEvent struct {
	Player ERLCServerPlayer
	Type   string // "join" or "leave"
}

// EventConfig provides configuration options for event subscriptions
type EventConfig struct {
	PollInterval        time.Duration
	BufferSize          int
	RetryOnError        bool
	RetryInterval       time.Duration
	FilterFunc          func(Event) bool
	IncludeInitialState bool
	BatchEvents         bool
	BatchWindow         time.Duration
	LogErrors           bool
	ErrorHandler        func(error)
	// OnPanic is called if an event handler panics. 
	// If nil, the panic is recovered but not reported.
	OnPanic             func(interface{})
	TimeFormat          string
}

// Internal types for subscription handling
type playerSet map[string]struct{}

type lastState struct {
	players     playerSet
	commandTime int64
	modCallTime int64
	killTime    int64
	joinTime    int64
	vehicleSet  map[string]struct{}
	initialized bool
}

type Subscription struct {
	Events   chan Event
	done     chan struct{}
	handlers HandlerRegistration
	config   *EventConfig
}




