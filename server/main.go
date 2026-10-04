package main

import (
	"crypto/ed25519"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"sync"
	"time"
	config "wake-on-wan-relay/lib"
)

func main() {
	context := config.GetContext()

	if slices.Contains(os.Args, "--version") {
		log.Printf("wake-on-wan-relay/server v%s\n", context.Version)
		return
	}
	run(context)
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

// ServerApp holds the dependencies for handling relay requests.
type ServerApp struct {
	pubKey      ed25519.PublicKey
	targetMAC   string
	replayCache *ReplayCache
	sendPacket  func(mac string) error
}

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

func run(context *config.Context) {
	targetMAC := context.MacAddress
	if _, err := net.ParseMAC(targetMAC); err != nil {
		log.Fatalf("Invalid macAddress in config: %v\n", err)
	}

	pubKeyHex := context.PublicKey
	pubBytes, err := hex.DecodeString(pubKeyHex)
	if err != nil || len(pubBytes) != ed25519.PublicKeySize {
		log.Fatalf("Invalid publicKey in config: expected %d-byte hex string\n", ed25519.PublicKeySize)
	}
	pubKey := ed25519.PublicKey(pubBytes)

	certPath := config.ResolvePath(context.CertPath)
	if certPath == "" {
		certPath = filepath.Join(config.GetProjectRoot(), "cert.pem")
	}

	keyPath := config.ResolvePath(context.KeyPath)
	if keyPath == "" {
		keyPath = filepath.Join(config.GetProjectRoot(), "key.pem")
	}

	log.Printf("Starting TLS server on :8443 (cert: %s, key: %s)\n", certPath, keyPath)

	app := &ServerApp{
		pubKey:      pubKey,
		targetMAC:   targetMAC,
		replayCache: NewReplayCache(),
		sendPacket:  sendMagicPacket,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/wake", app.WakeHandler)

	server := &http.Server{
		Addr:    ":8443",
		Handler: mux,
		TLSConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
		},
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	if err := server.ListenAndServeTLS(certPath, keyPath); err != nil {
		log.Fatalf("%s\n", err.Error())
	}
}

func sendMagicPacket(mac string) error {
	hw, err := net.ParseMAC(mac)
	if err != nil {
		return fmt.Errorf("invalid MAC address: %w", err)
	}
	packet := make([]byte, 6+16*len(hw))
	for i := range 6 {
		packet[i] = 0xFF
	}
	for i := 1; i <= 16; i++ {
		copy(packet[6+(i-1)*len(hw):], hw)
	}
	addr, err := net.ResolveUDPAddr("udp", "255.255.255.255:9")
	if err != nil {
		return err
	}
	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		return err
	}
	defer conn.Close()
	_, err = conn.Write(packet)
	return err
}
