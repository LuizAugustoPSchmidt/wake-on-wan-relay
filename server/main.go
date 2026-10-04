package main

import (
	"crypto/ed25519"
	"crypto/tls"
	"encoding/hex"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
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

func run(context *config.Context) {
	targetMAC := context.MacAddress

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

	mux := http.NewServeMux()
	mux.HandleFunc("/wake", func(w http.ResponseWriter, r *http.Request) {
		log.Println("Received /wake call")
		timestampStr := r.Header.Get("X-Timestamp")
		sigHex := r.Header.Get("X-Signature")

		// Replay Protection: Ensure request was generated within the last 60 seconds
		timestamp, err := strconv.ParseInt(timestampStr, 10, 64)
		if err != nil {
			http.Error(w, "Invalid timestamp", http.StatusBadRequest)
			return
		}

		reqTime := time.Unix(timestamp, 0)
		if time.Since(reqTime) > 60*time.Second || time.Since(reqTime) < -60*time.Second {
			http.Error(w, "Request expired (Replay protection)", http.StatusUnauthorized)
			return
		}

		// Cryptographic Verification
		sigBytes, err := hex.DecodeString(sigHex)
		if err != nil || !ed25519.Verify(pubKey, []byte(timestampStr), sigBytes) {
			// Return 404 to try and lose potential attackers
			http.Error(w, "Not Found", http.StatusNotFound)
			return
		}

		if err := sendMagicPacket(targetMAC); err != nil {
			http.Error(w, "WOL broadcast failed", http.StatusInternalServerError)
			return
		}
		w.Write([]byte("Authenticated. Magic packet broadcasted locally.\n"))
	})

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
	mac = strings.ReplaceAll(mac, ":", "")
	hw, _ := hex.DecodeString(mac)
	packet := make([]byte, 6+16*len(hw))
	for i := range 6 {
		packet[i] = 0xFF
	}
	for i := 1; i <= 16; i++ {
		copy(packet[6+(i-1)*len(hw):], hw)
	}
	addr, _ := net.ResolveUDPAddr("udp", "255.255.255.255:9")
	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		return err
	}
	defer conn.Close()
	_, err = conn.Write(packet)
	return err
}
