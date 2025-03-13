package erlcgo

import (
	"fmt"
	"sync"
	"time"
)

// ERLCServerPlayer represents a player currently on the server.
// It contains information about their permissions, team, and callsign.
type ERLCServerPlayer struct {
	// Player is the username of the player
	Player string `json:"Player"`
	// Permission represents the player's permission level (e.g., "Admin", "Moderator", "Player")
	Permission string `json:"Permission"`
	// Callsign is the player's in-game identifier (e.g., "PC-31")
	Callsign string `json:"Callsign"`
	// Team represents the player's current team or department
	Team string `json:"Team"`
}

// ERLCCommandLog represents a command executed on the server.
type ERLCCommandLog struct {
	// Player who executed the command
	Player string `json:"Player"`
	// Timestamp of when the command was executed (Unix timestamp)
	Timestamp int64 `json:"Timestamp"`
	// Command that was executed
	Command string `json:"Command"`
}

// ERLCModCallLog represents a moderation call log entry.
type ERLCModCallLog struct {
	// Caller is the player who initiated the call
	Caller string `json:"Caller"`
	// Moderator is the moderator who responded to the call
	Moderator string `json:"Moderator"`
	// Timestamp of when the call was made (Unix timestamp)
	Timestamp int64 `json:"Timestamp"`
}

// ERLCKillLog represents a kill log entry.
type ERLCKillLog struct {
	// Killed is the player who was killed
	Killed string `json:"Killed"`
	// Timestamp of when the kill occurred (Unix timestamp)
	Timestamp int64 `json:"Timestamp"`
	// Killer is the player who made the kill
	Killer string `json:"Killer"`
}

// ERLCJoinLog represents a join log entry.
type ERLCJoinLog struct {
	// Join indicates whether the player joined (true) or left (false) the server
	Join bool `json:"Join"`
	// Timestamp of when the join/leave occurred (Unix timestamp)
	Timestamp int64 `json:"Timestamp"`
	// Player is the player who joined or left the server
	Player string `json:"Player"`
}

// ERLCVehicle represents a vehicle in the game.
type ERLCVehicle struct {
	// Texture is the texture applied to the vehicle
	Texture string `json:"Texture"`
	// Name is the name of the vehicle
	Name string `json:"Name"`
	// Owner is the player who owns the vehicle
	Owner string `json:"Owner"`
}

// APIError represents an error returned by the ERLC API.
// It implements the error interface and provides detailed error information.
type APIError struct {
	// Code is the numeric error code
	Code int `json:"code"`
	// Message is the human-readable error description
	Message string `json:"message"`
	// CommandID is the ID of the command that caused the error (if applicable)
	CommandID string `json:"commandId,omitempty"`
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
