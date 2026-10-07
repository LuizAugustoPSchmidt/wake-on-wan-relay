package main

import (
	"sync"
	"time"
)

// ReplayCache tracks seen signatures with thread-safe expiration to prevent replay attacks.
type ReplayCache struct {
	mu   sync.Mutex
	seen map[string]time.Time
}

func NewReplayCache() *ReplayCache {
	return &ReplayCache{
		seen: make(map[string]time.Time),
	}
}

// CheckOrAdd returns true if sig was already seen and has not expired (replay detected).
// Returns false if sig is new, and records it with the given expiry.
func (rc *ReplayCache) CheckOrAdd(sig string, expiresAt time.Time) bool {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	now := time.Now()
	// Lazy cleanup of expired entries
	for k, exp := range rc.seen {
		if now.After(exp) {
			delete(rc.seen, k)
		}
	}

	if exp, exists := rc.seen[sig]; exists && now.Before(exp) {
		return true // Replay detected
	}

	rc.seen[sig] = expiresAt
	return false
}
