package erlcgo

import (
	"net/http"
	"strconv"
	"time"
)

type RateLimitInfo struct {
	Bucket    string
	Limit     int
	Remaining int
	ResetAt   time.Time 
}

type ResponseMeta struct {
	Route      string 
	StatusCode int
	Headers    http.Header
	Body       []byte 

	RateLimit  *RateLimitInfo
	RetryAfter *time.Duration 
	Err        error
}

type ResponseHook func(meta ResponseMeta)

func parseRateLimitHeaders(h http.Header) *RateLimitInfo {
	bucket := h.Get("X-RateLimit-Bucket")
	limitStr := h.Get("X-RateLimit-Limit")
	remainingStr := h.Get("X-RateLimit-Remaining")
	resetStr := h.Get("X-RateLimit-Reset")

	if bucket == "" && limitStr == "" && remainingStr == "" && resetStr == "" {
		return nil
	}

	rl := &RateLimitInfo{Bucket: bucket}

	if v, err := strconv.Atoi(limitStr); err == nil {
		rl.Limit = v
	}
	if v, err := strconv.Atoi(remainingStr); err == nil {
		rl.Remaining = v
	}

	// reset is epoch timestamp. Sometimes seconds; detect ms by magnitude.
	if resetStr != "" {
		if epoch, err := strconv.ParseInt(resetStr, 10, 64); err == nil {
			if epoch > 1_000_000_000_000 {
				rl.ResetAt = time.UnixMilli(epoch)
			} else {
				rl.ResetAt = time.Unix(epoch, 0)
			}
		}
	}

	return rl
}
