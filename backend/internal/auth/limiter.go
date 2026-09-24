package auth

import (
	"sync"
	"time"
)

type limitEntry struct {
	count       int
	windowStart time.Time
}

type Limiter struct {
	mu      sync.Mutex
	entries map[string]limitEntry
	window  time.Duration
}

func NewLimiter(window time.Duration) *Limiter {
	return &Limiter{entries: make(map[string]limitEntry), window: window}
}

func (l *Limiter) Allow(key string, limit int, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	entry := l.entries[key]
	if entry.windowStart.IsZero() || now.Sub(entry.windowStart) >= l.window {
		entry = limitEntry{windowStart: now}
	}
	if entry.count >= limit {
		l.entries[key] = entry
		return false
	}
	entry.count++
	l.entries[key] = entry

	if len(l.entries) > 4096 {
		cutoff := now.Add(-2 * l.window)
		for k, v := range l.entries {
			if v.windowStart.Before(cutoff) {
				delete(l.entries, k)
			}
		}
	}
	return true
}

func (l *Limiter) Reset(key string) {
	l.mu.Lock()
	delete(l.entries, key)
	l.mu.Unlock()
}
