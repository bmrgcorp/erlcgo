package erlcgo

import (
	"time"
)

func NewRateLimiter() *RateLimiter {
	return &RateLimiter{
		limits: make(map[string]*RateLimit),
	}
}

func (rl *RateLimiter) UpdateFromHeaders(bucket string, limit, remaining int, reset time.Time) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	rl.limits[bucket] = &RateLimit{
		Bucket:    bucket,
		Limit:     limit,
		Remaining: remaining,
		Reset:     reset,
	}
}

func (rl *RateLimiter) ShouldWait(bucket string) (time.Duration, bool) {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	limit, exists := rl.limits[bucket]
	if !exists {
		return 0, false
	}

	if limit.Remaining <= 0 {
		wait := time.Until(limit.Reset)
		if wait > 0 {
			return wait, true
		}
	}
	return 0, false
}
