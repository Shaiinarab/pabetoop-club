package app

import (
	"sync"
	"time"
)

type loginAttemptWindow struct {
	started time.Time
	count   int
}

type fixedWindowLimiter struct {
	mu      sync.Mutex
	entries map[string]loginAttemptWindow
	limit   int
	window  time.Duration
}

func newFixedWindowLimiter(limit int, window time.Duration) *fixedWindowLimiter {
	return &fixedWindowLimiter{entries: map[string]loginAttemptWindow{}, limit: limit, window: window}
}

func (limiter *fixedWindowLimiter) allow(key string, now time.Time) bool {
	limiter.mu.Lock()
	defer limiter.mu.Unlock()

	entry := limiter.entries[key]
	if entry.started.IsZero() || now.Sub(entry.started) >= limiter.window {
		limiter.entries[key] = loginAttemptWindow{started: now, count: 1}
		return true
	}
	if entry.count >= limiter.limit {
		return false
	}
	entry.count++
	limiter.entries[key] = entry
	return true
}

var loginLimiter = newFixedWindowLimiter(5, 10*time.Minute)
