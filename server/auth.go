package main

import (
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"
)

// Authenticate verifies the cryptographic signature, timestamp window, and replay protection
// of an incoming HTTP request. It returns true if valid, false otherwise.
func (s *ServerApp) Authenticate(r *http.Request) bool {
	timestampStr := r.Header.Get("X-Timestamp")
	sigHex := r.Header.Get("X-Signature")
	nonce := r.Header.Get("X-Nonce")

	// 1. Replay Protection: Ensure request was generated within the last 60 seconds
	timestamp, err := strconv.ParseInt(timestampStr, 10, 64)
	if err != nil {
		return false
	}

	reqTime := time.Unix(timestamp, 0)
	if time.Since(reqTime) > 60*time.Second || time.Since(reqTime) < -60*time.Second {
		return false
	}

	// 2. Decode signature bytes
	sigBytes, err := hex.DecodeString(sigHex)
	if err != nil {
		return false
	}

	var authenticated bool
	// 3. Try v2 format if nonce is present: v2:<METHOD>:<PATH>:<TIMESTAMP>:<NONCE>
	if nonce != "" {
		v2Payload := fmt.Sprintf("v2:%s:%s:%s:%s", r.Method, r.URL.Path, timestampStr, nonce)
		if ed25519.Verify(s.pubKey, []byte(v2Payload), sigBytes) {
			authenticated = true
		}
	}
	// Fallback to v1 legacy format (timestamp only) for backward compatibility
	if !authenticated && ed25519.Verify(s.pubKey, []byte(timestampStr), sigBytes) {
		authenticated = true
	}

	if !authenticated {
		return false
	}

	// 4. Replay Cache: Ensure this exact signature hasn't been executed before
	if s.replayCache.CheckOrAdd(sigHex, time.Now().Add(120*time.Second)) {
		log.Println("Replay attack detected and blocked")
		return false
	}

	return true
}
