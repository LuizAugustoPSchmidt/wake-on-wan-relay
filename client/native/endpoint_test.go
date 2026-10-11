package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"testing"
)

func TestResolveEndpointURL(t *testing.T) {
	tests := []struct {
		input    string
		endpoint string
		expected string
	}{
		{"https://lapschmidt.duckdns.org:2411", "/status", "https://lapschmidt.duckdns.org:2411/status"},
		{"https://lapschmidt.duckdns.org:2411", "/wake", "https://lapschmidt.duckdns.org:2411/wake"},
		{"https://lapschmidt.duckdns.org:2411/", "status", "https://lapschmidt.duckdns.org:2411/status"},
		{"https://lapschmidt.duckdns.org:2411/wake", "/status", "https://lapschmidt.duckdns.org:2411/status"},
		{"https://lapschmidt.duckdns.org:2411/status", "/wake", "https://lapschmidt.duckdns.org:2411/wake"},
		{"lapschmidt.duckdns.org:2411", "/status", "https://lapschmidt.duckdns.org:2411/status"},
		{"http://192.168.1.100:8443/wake", "/status", "http://192.168.1.100:8443/status"},
	}

	for _, tc := range tests {
		actual, err := resolveEndpointURL(tc.input, tc.endpoint)
		if err != nil {
			t.Errorf("resolveEndpointURL(%q, %q) returned unexpected error: %v", tc.input, tc.endpoint, err)
			continue
		}
		if actual != tc.expected {
			t.Errorf("resolveEndpointURL(%q, %q) = %q, expected %q", tc.input, tc.endpoint, actual, tc.expected)
		}
	}
}

func TestCreateRequest_V2Verification(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	for _, endpoint := range []string{"/wake", "/status"} {
		targetURL := "https://example.com:8443" + endpoint
		req, err := createRequest(targetURL, priv, true)
		if err != nil {
			t.Fatalf("createRequest failed for %s: %v", endpoint, err)
		}

		tsStr := req.Header.Get("X-Timestamp")
		nonce := req.Header.Get("X-Nonce")
		sigHex := req.Header.Get("X-Signature")

		if tsStr == "" || nonce == "" || sigHex == "" {
			t.Fatalf("missing required headers for %s", endpoint)
		}

		sigBytes, err := hex.DecodeString(sigHex)
		if err != nil {
			t.Fatalf("invalid hex signature: %v", err)
		}

		expectedPayload := fmt.Sprintf("v2:GET:%s:%s:%s", endpoint, tsStr, nonce)
		if !ed25519.Verify(pub, []byte(expectedPayload), sigBytes) {
			t.Errorf("v2 signature verification failed for endpoint %s", endpoint)
		}
	}
}
