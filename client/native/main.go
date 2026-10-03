package main

import (
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"os"
	"slices"
	"time"
)

// Replace with the generated Private key hex string
const serverURL = "http://lapschmidt.duckdns.org/wake"

func main() {
	if slices.Contains(os.Args, "--version") {
		log.Println("wake-on-wan-relay/client v0.0.1")
		return
	}
	run()
}

func run() {
	privKeyHex := "" // Read from ./.private-key

	privBytes, _ := hex.DecodeString(privKeyHex)
	privKey := ed25519.PrivateKey(privBytes)

	// Sign the current Unix epoch time
	tsStr := fmt.Sprintf("%d", time.Now().Unix())
	sig := ed25519.Sign(privKey, []byte(tsStr))

	req, _ := http.NewRequest("GET", serverURL, nil)
	req.Header.Set("X-Timestamp", tsStr)
	req.Header.Set("X-Signature", hex.EncodeToString(sig))

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Connection Error:", err)
		return
	}
	defer resp.Body.Close()
	fmt.Println("Server Status:", resp.Status)
}
