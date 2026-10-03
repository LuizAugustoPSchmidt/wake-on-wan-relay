package main

import (
	"crypto/ed25519"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"os"
	"slices"
	"time"
	config "wake-on-wan-relay/lib"
)

// Replace with the generated Private key hex string
const serverURL = "https://lapschmidt.duckdns.org/wake"

func main() {
	context := config.GetContext()
	if slices.Contains(os.Args, "--version") {
		log.Printf("wake-on-wan-relay/client v%s\n", context.Version)
		return
	}

	run(context)
}

func run(context *config.Context) {
	privKeyHex := context.PrivateKey

	privBytes, _ := hex.DecodeString(privKeyHex)
	privKey := ed25519.PrivateKey(privBytes)

	// Sign the current Unix epoch time
	tsStr := fmt.Sprintf("%d", time.Now().Unix())
	sig := ed25519.Sign(privKey, []byte(tsStr))

	req, _ := http.NewRequest("GET", serverURL, nil)
	req.Header.Set("X-Timestamp", tsStr)
	req.Header.Set("X-Signature", hex.EncodeToString(sig))

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	client := &http.Client{Timeout: 5 * time.Second, Transport: transport}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Connection Error:", err)
		return
	}
	defer resp.Body.Close()

	fmt.Println("Server Response Status:", resp.Status)
}
