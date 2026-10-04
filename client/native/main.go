package main

import (
	"crypto/ed25519"
	"crypto/rand"
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

func createWakeRequest(serverURL string, privKey ed25519.PrivateKey, v2Auth bool) (*http.Request, error) {
	req, err := http.NewRequest("GET", serverURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	tsStr := fmt.Sprintf("%d", time.Now().Unix())
	req.Header.Set("X-Timestamp", tsStr)

	if v2Auth {
		nonceBytes := make([]byte, 16)
		if _, err := rand.Read(nonceBytes); err != nil {
			return nil, fmt.Errorf("failed to generate nonce: %w", err)
		}
		nonce := hex.EncodeToString(nonceBytes)
		req.Header.Set("X-Nonce", nonce)

		v2Payload := fmt.Sprintf("v2:%s:%s:%s:%s", req.Method, req.URL.Path, tsStr, nonce)
		sig := ed25519.Sign(privKey, []byte(v2Payload))
		req.Header.Set("X-Signature", hex.EncodeToString(sig))
	} else {
		sig := ed25519.Sign(privKey, []byte(tsStr))
		req.Header.Set("X-Signature", hex.EncodeToString(sig))
	}

	return req, nil
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

	tlsConfig, err := buildTLSConfig(context)
	if err != nil {
		log.Fatalf("TLS configuration error: %v", err)
	}

	transport := &http.Transport{
		TLSClientConfig: tlsConfig,
	}
	client := &http.Client{Timeout: 5 * time.Second, Transport: transport}

	// 1. Attempt v2 authenticated request (nonce + method/path binding)
	reqV2, err := createWakeRequest(serverURL, privKey, true)
	if err != nil {
		log.Fatalf("Failed to create v2 wake request: %v", err)
	}

	resp, err := client.Do(reqV2)
	if err != nil {
		fmt.Println("Connection Error:", err)
		return
	}
	defer resp.Body.Close()

	// If server returned 404 (an older server that does not recognize v2 auth),
	// automatically fallback to legacy v1 auth for backward compatibility.
	if resp.StatusCode == http.StatusNotFound {
		reqV1, err := createWakeRequest(serverURL, privKey, false)
		if err == nil {
			if retryResp, retryErr := client.Do(reqV1); retryErr == nil {
				defer retryResp.Body.Close()
				fmt.Println("Server Response Status:", retryResp.Status)
				return
			}
		}
	}

	fmt.Println("Server Response Status:", resp.Status)
}
