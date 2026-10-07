package main

import (
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"
)

func (s *ServerApp) WakeHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Received /wake call")

	timestampStr := r.Header.Get("X-Timestamp")
	sigHex := r.Header.Get("X-Signature")
	nonce := r.Header.Get("X-Nonce")

	// Replay Protection: Ensure request was generated within the last 60 seconds
	timestamp, err := strconv.ParseInt(timestampStr, 10, 64)
	if err != nil {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	reqTime := time.Unix(timestamp, 0)
	if time.Since(reqTime) > 60*time.Second || time.Since(reqTime) < -60*time.Second {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	// Cryptographic Verification
	sigBytes, err := hex.DecodeString(sigHex)
	if err != nil {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	var authenticated bool
	// 1. Try v2 format if nonce is present: v2:<METHOD>:<PATH>:<TIMESTAMP>:<NONCE>
	if nonce != "" {
		v2Payload := fmt.Sprintf("v2:%s:%s:%s:%s", r.Method, r.URL.Path, timestampStr, nonce)
		if ed25519.Verify(s.pubKey, []byte(v2Payload), sigBytes) {
			authenticated = true
		}
	}
	// 2. Fallback to v1 legacy format (timestamp only) for backward compatibility
	if !authenticated && ed25519.Verify(s.pubKey, []byte(timestampStr), sigBytes) {
		authenticated = true
	}

	if !authenticated {
		// Return uniform 404 for stealth against scanners
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	// Replay Cache: Ensure this exact signature hasn't been executed before
	if s.replayCache.CheckOrAdd(sigHex, time.Now().Add(120*time.Second)) {
		log.Println("Replay attack detected and blocked")
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	if err := s.sendPacket(s.targetMAC); err != nil {
		http.Error(w, "WOL broadcast failed", http.StatusInternalServerError)
		return
	}
	w.Write([]byte("Authenticated. Magic packet broadcasted locally.\n"))
}

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
