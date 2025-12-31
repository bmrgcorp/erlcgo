package erlcgo

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"
)

type prcRateLimitBody struct {
	Message    string  `json:"message"`
	RetryAfter float64 `json:"retry_after"`
	Bucket     string  `json:"bucket"`
}

func parseRetryAfter(body []byte) *time.Duration {
	if len(body) == 0 {
		return nil
	}
	var b prcRateLimitBody
	if err := json.Unmarshal(body, &b); err != nil {
		return nil
	}
	if b.RetryAfter <= 0 {
		return nil
	}
	d := time.Duration(b.RetryAfter * float64(time.Second))
	return &d
}

func parseRetryAfterHeader(h http.Header) *time.Duration {
	value := h.Get("Retry-After")
	if value == "" {
		return nil
	}

	if secs, err := strconv.Atoi(value); err == nil {
		d := time.Duration(secs) * time.Second
		return &d
	}

	if t, err := http.ParseTime(value); err == nil {
		d := time.Until(t)
		if d < 0 {
			d = 0
		}
		return &d
	}

	return nil
}

func parseRateLimitBucket(body []byte) string {
	if len(body) == 0 {
		return ""
	}
	var b prcRateLimitBody
	if err := json.Unmarshal(body, &b); err != nil {
		return ""
	}
	return b.Bucket
}


