package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func setupTestServer(t *testing.T) (*ServerApp, ed25519.PublicKey, ed25519.PrivateKey, *[]string) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate ed25519 key: %v", err)
	}

	var sentPackets []string
	app := &ServerApp{
		pubKey:       pub,
		targetMAC:    "9C:6B:00:3D:25:6D",
		internalAddr: "192.168.1.100",
		replayCache:  NewReplayCache(),
		sendPacket: func(mac string) error {
			sentPackets = append(sentPackets, mac)
			return nil
		},
		checkSSH: func(host string, port int, timeout time.Duration) (bool, error) {
			return true, nil
		},
	}

	return app, pub, priv, &sentPackets
}

func TestServer_V2Auth_SuccessAndReplayBlocked(t *testing.T) {
	app, _, priv, sentPackets := setupTestServer(t)

	tsStr := fmt.Sprintf("%d", time.Now().Unix())
	nonce := "11223344556677889900aabbccddeeff"
	v2Payload := fmt.Sprintf("v2:GET:/wake:%s:%s", tsStr, nonce)
	sig := ed25519.Sign(priv, []byte(v2Payload))
	sigHex := hex.EncodeToString(sig)

	// 1. First request should succeed
	req := httptest.NewRequest("GET", "/wake", nil)
	req.Header.Set("X-Timestamp", tsStr)
	req.Header.Set("X-Nonce", nonce)
	req.Header.Set("X-Signature", sigHex)
	rec := httptest.NewRecorder()

	app.WakeHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for valid v2 request, got %d", rec.Code)
	}
	if len(*sentPackets) != 1 {
		t.Fatalf("Expected 1 magic packet sent, got %d", len(*sentPackets))
	}

	// 2. Replay of the exact same request MUST be blocked by ReplayCache
	rec2 := httptest.NewRecorder()
	app.WakeHandler(rec2, req)

	if rec2.Code != http.StatusNotFound {
		t.Fatalf("Expected 404 Not Found (stealth reject) for replayed request, got %d", rec2.Code)
	}
	if len(*sentPackets) != 1 {
		t.Fatalf("Expected magic packet NOT to be sent on replay, but sentPackets count is %d", len(*sentPackets))
	}
}

func TestServer_V1Auth_SuccessAndReplayBlocked(t *testing.T) {
	app, _, priv, sentPackets := setupTestServer(t)

	tsStr := fmt.Sprintf("%d", time.Now().Unix())
	sig := ed25519.Sign(priv, []byte(tsStr))
	sigHex := hex.EncodeToString(sig)

	// 1. First v1 legacy request should succeed
	req := httptest.NewRequest("GET", "/wake", nil)
	req.Header.Set("X-Timestamp", tsStr)
	req.Header.Set("X-Signature", sigHex)
	rec := httptest.NewRecorder()

	app.WakeHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for valid v1 legacy request, got %d", rec.Code)
	}
	if len(*sentPackets) != 1 {
		t.Fatalf("Expected 1 magic packet sent, got %d", len(*sentPackets))
	}

	// 2. Replay of v1 legacy request MUST be blocked
	rec2 := httptest.NewRecorder()
	app.WakeHandler(rec2, req)

	if rec2.Code != http.StatusNotFound {
		t.Fatalf("Expected 404 Not Found on replay of v1 request, got %d", rec2.Code)
	}
	if len(*sentPackets) != 1 {
		t.Fatalf("Expected magic packet NOT to be sent on replay, got %d", len(*sentPackets))
	}
}

func TestServer_ExpiredTimestamp(t *testing.T) {
	app, _, priv, sentPackets := setupTestServer(t)

	// 65 seconds in the past
	tsStr := fmt.Sprintf("%d", time.Now().Add(-65*time.Second).Unix())
	nonce := "1234567890abcdef"
	v2Payload := fmt.Sprintf("v2:GET:/wake:%s:%s", tsStr, nonce)
	sig := ed25519.Sign(priv, []byte(v2Payload))

	req := httptest.NewRequest("GET", "/wake", nil)
	req.Header.Set("X-Timestamp", tsStr)
	req.Header.Set("X-Nonce", nonce)
	req.Header.Set("X-Signature", hex.EncodeToString(sig))
	rec := httptest.NewRecorder()

	app.WakeHandler(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("Expected 404 for expired timestamp, got %d", rec.Code)
	}
	if len(*sentPackets) != 0 {
		t.Fatalf("Expected no packets sent for expired timestamp, got %d", len(*sentPackets))
	}
}

func TestServer_FutureTimestamp(t *testing.T) {
	app, _, priv, sentPackets := setupTestServer(t)

	// 65 seconds in the future
	tsStr := fmt.Sprintf("%d", time.Now().Add(65*time.Second).Unix())
	nonce := "1234567890abcdef"
	v2Payload := fmt.Sprintf("v2:GET:/wake:%s:%s", tsStr, nonce)
	sig := ed25519.Sign(priv, []byte(v2Payload))

	req := httptest.NewRequest("GET", "/wake", nil)
	req.Header.Set("X-Timestamp", tsStr)
	req.Header.Set("X-Nonce", nonce)
	req.Header.Set("X-Signature", hex.EncodeToString(sig))
	rec := httptest.NewRecorder()

	app.WakeHandler(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("Expected 404 for future timestamp, got %d", rec.Code)
	}
	if len(*sentPackets) != 0 {
		t.Fatalf("Expected no packets sent for future timestamp, got %d", len(*sentPackets))
	}
}

func TestServer_BadSignature(t *testing.T) {
	app, _, _, sentPackets := setupTestServer(t)

	tsStr := fmt.Sprintf("%d", time.Now().Unix())
	nonce := "1234567890abcdef"

	req := httptest.NewRequest("GET", "/wake", nil)
	req.Header.Set("X-Timestamp", tsStr)
	req.Header.Set("X-Nonce", nonce)
	req.Header.Set("X-Signature", hex.EncodeToString(make([]byte, 64)))
	rec := httptest.NewRecorder()

	app.WakeHandler(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("Expected 404 for bad signature, got %d", rec.Code)
	}
	if len(*sentPackets) != 0 {
		t.Fatalf("Expected no packets sent for bad signature, got %d", len(*sentPackets))
	}
}

func TestServer_WakeHandler_Unauthenticated(t *testing.T) {
	app, _, _, sentPackets := setupTestServer(t)

	req := httptest.NewRequest("GET", "/wake", nil)
	rec := httptest.NewRecorder()
	app.WakeHandler(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("Expected 404 for unauthenticated /wake request, got %d", rec.Code)
	}
	if len(*sentPackets) != 0 {
		t.Fatalf("Expected no packets sent, got %d", len(*sentPackets))
	}
}

func TestServer_MultipleRequestsSameSecond_DifferentNonces(t *testing.T) {
	app, _, priv, sentPackets := setupTestServer(t)

	tsStr := fmt.Sprintf("%d", time.Now().Unix())

	// Request 1 with nonce A
	nonce1 := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	v2Payload1 := fmt.Sprintf("v2:GET:/wake:%s:%s", tsStr, nonce1)
	sig1 := ed25519.Sign(priv, []byte(v2Payload1))

	req1 := httptest.NewRequest("GET", "/wake", nil)
	req1.Header.Set("X-Timestamp", tsStr)
	req1.Header.Set("X-Nonce", nonce1)
	req1.Header.Set("X-Signature", hex.EncodeToString(sig1))
	rec1 := httptest.NewRecorder()
	app.WakeHandler(rec1, req1)

	if rec1.Code != http.StatusOK {
		t.Fatalf("Expected req1 to succeed, got %d", rec1.Code)
	}

	// Request 2 with same timestamp but nonce B
	nonce2 := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	v2Payload2 := fmt.Sprintf("v2:GET:/wake:%s:%s", tsStr, nonce2)
	sig2 := ed25519.Sign(priv, []byte(v2Payload2))

	req2 := httptest.NewRequest("GET", "/wake", nil)
	req2.Header.Set("X-Timestamp", tsStr)
	req2.Header.Set("X-Nonce", nonce2)
	req2.Header.Set("X-Signature", hex.EncodeToString(sig2))
	rec2 := httptest.NewRecorder()
	app.WakeHandler(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Fatalf("Expected req2 in the same second with distinct nonce to succeed, got %d", rec2.Code)
	}

	if len(*sentPackets) != 2 {
		t.Fatalf("Expected 2 packets sent, got %d", len(*sentPackets))
	}
}

func TestServer_StatusHandler_Unauthenticated(t *testing.T) {
	app, _, _, _ := setupTestServer(t)

	req := httptest.NewRequest("GET", "/status", nil)
	rec := httptest.NewRecorder()
	app.StatusHandler(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("Expected 404 Not Found for unauthenticated /status request, got %d", rec.Code)
	}
}

func TestServer_StatusHandler_Authenticated(t *testing.T) {
	app, _, priv, _ := setupTestServer(t)

	tsStr := fmt.Sprintf("%d", time.Now().Unix())
	nonce := "99887766554433221100aabbccddeeff"
	v2Payload := fmt.Sprintf("v2:GET:/status:%s:%s", tsStr, nonce)
	sig := ed25519.Sign(priv, []byte(v2Payload))

	req := httptest.NewRequest("GET", "/status", nil)
	req.Header.Set("X-Timestamp", tsStr)
	req.Header.Set("X-Nonce", nonce)
	req.Header.Set("X-Signature", hex.EncodeToString(sig))
	rec := httptest.NewRecorder()

	app.StatusHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for authenticated /status request, got %d", rec.Code)
	}
	if rec.Body.String() != "Server is up and SSH is enabled" {
		t.Fatalf("Unexpected response body: %q", rec.Body.String())
	}
}

func TestServer_StatusHandler_SSHDown(t *testing.T) {
	app, _, priv, _ := setupTestServer(t)
	app.checkSSH = func(host string, port int, timeout time.Duration) (bool, error) {
		return false, fmt.Errorf("connection refused")
	}

	tsStr := fmt.Sprintf("%d", time.Now().Unix())
	nonce := "12345678123456781234567812345678"
	v2Payload := fmt.Sprintf("v2:GET:/status:%s:%s", tsStr, nonce)
	sig := ed25519.Sign(priv, []byte(v2Payload))

	req := httptest.NewRequest("GET", "/status", nil)
	req.Header.Set("X-Timestamp", tsStr)
	req.Header.Set("X-Nonce", nonce)
	req.Header.Set("X-Signature", hex.EncodeToString(sig))
	rec := httptest.NewRecorder()

	app.StatusHandler(rec, req)

	if rec.Code != http.StatusExpectationFailed {
		t.Fatalf("Expected 417 Expectation Failed when SSH is down, got %d", rec.Code)
	}
}
