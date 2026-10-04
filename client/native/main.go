package main

import (
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"os"
	"slices"
	"strings"
	"time"
	config "wake-on-wan-relay/lib"
)

func main() {
	context := config.GetContext()

	if slices.Contains(os.Args, "--version") {
		log.Printf("wake-on-wan-relay/client v%s\n", context.Version)
		return
	}

	run(context)
}

func buildTLSConfig(context *config.Context) (*tls.Config, error) {
	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
	}

	if context.ServerName != "" {
		tlsConfig.ServerName = context.ServerName
	}

	// 1. Look for CA / self-signed certificate file
	caPath := context.CACertPath
	if caPath == "" && context.CertPath != "" {
		resolved := config.ResolvePath(context.CertPath)
		if _, err := os.Stat(resolved); err == nil {
			caPath = resolved
		}
	} else if caPath != "" {
		caPath = config.ResolvePath(caPath)
	}

	var rootPoolLoaded bool
	if caPath != "" {
		caBytes, err := os.ReadFile(caPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA certificate from %s: %w", caPath, err)
		}
		pool, err := x509.SystemCertPool()
		if err != nil || pool == nil {
			pool = x509.NewCertPool()
		}
		if !pool.AppendCertsFromPEM(caBytes) {
			return nil, fmt.Errorf("failed to parse certificate PEM from %s", caPath)
		}
		tlsConfig.RootCAs = pool
		rootPoolLoaded = true
	}

	// 2. Handle SHA-256 certificate fingerprint pinning if configured
	if context.CertFingerprint != "" {
		cleanExpected := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(context.CertFingerprint, ":", ""), " ", ""))
		if len(cleanExpected) != 64 {
			return nil, fmt.Errorf("invalid certFingerprint: expected 64 hex characters (SHA-256), got %d", len(cleanExpected))
		}

		if !rootPoolLoaded {
			// When only fingerprint is provided (no local cert/CA file), pin the leaf certificate directly
			tlsConfig.InsecureSkipVerify = true
			tlsConfig.VerifyConnection = func(cs tls.ConnectionState) error {
				if len(cs.PeerCertificates) == 0 {
					return fmt.Errorf("no peer certificates presented")
				}
				leaf := cs.PeerCertificates[0]
				hash := sha256.Sum256(leaf.Raw)
				actual := hex.EncodeToString(hash[:])
				if actual != cleanExpected {
					return fmt.Errorf("certificate fingerprint mismatch:\n  expected: %s\n  received: %s", cleanExpected, actual)
				}
				if context.ServerName != "" {
					if err := leaf.VerifyHostname(context.ServerName); err != nil {
						return fmt.Errorf("certificate hostname verification failed: %w", err)
					}
				}
				return nil
			}
		} else {
			// If CA cert pool was also loaded, enforce fingerprint validation on top of root CA verification
			tlsConfig.VerifyPeerCertificate = func(rawCerts [][]byte, _ [][]*x509.Certificate) error {
				if len(rawCerts) == 0 {
					return fmt.Errorf("no peer certificates presented")
				}
				hash := sha256.Sum256(rawCerts[0])
				actual := hex.EncodeToString(hash[:])
				if actual != cleanExpected {
					return fmt.Errorf("certificate fingerprint mismatch:\n  expected: %s\n  received: %s", cleanExpected, actual)
				}
				return nil
			}
		}
	}

	return tlsConfig, nil
}

func run(context *config.Context) {
	serverURL := context.ExternalAddr
	if serverURL == "" {
		log.Fatal("externalAddress is required in config.json")
	}

	privKeyHex := context.PrivateKey
	privBytes, err := hex.DecodeString(privKeyHex)
	if err != nil || len(privBytes) != ed25519.PrivateKeySize {
		log.Fatalf("Invalid private key: expected %d-byte hex string\n", ed25519.PrivateKeySize)
	}
	privKey := ed25519.PrivateKey(privBytes)

	// Sign the current Unix epoch time
	tsStr := fmt.Sprintf("%d", time.Now().Unix())
	sig := ed25519.Sign(privKey, []byte(tsStr))

	req, err := http.NewRequest("GET", serverURL, nil)
	if err != nil {
		log.Fatalf("Failed to create HTTP request: %v", err)
	}
	req.Header.Set("X-Timestamp", tsStr)
	req.Header.Set("X-Signature", hex.EncodeToString(sig))

	tlsConfig, err := buildTLSConfig(context)
	if err != nil {
		log.Fatalf("TLS configuration error: %v", err)
	}

	transport := &http.Transport{
		TLSClientConfig: tlsConfig,
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
