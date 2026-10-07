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

// ServerApp holds the dependencies for handling relay requests.
type ServerApp struct {
	pubKey       ed25519.PublicKey
	targetMAC    string
	internalAddr string
	replayCache  *ReplayCache
	sendPacket   func(mac string) error
	checkSSH     func(host string, port int, timeout time.Duration) (bool, error)
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
		pubKey:       pubKey,
		targetMAC:    targetMAC,
		internalAddr: context.InternalAddr,
		replayCache:  NewReplayCache(),
		sendPacket:   sendMagicPacket,
		checkSSH:     CheckSSH,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/wake", app.WakeHandler)
	mux.HandleFunc("/status", app.StatusHandler)

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
