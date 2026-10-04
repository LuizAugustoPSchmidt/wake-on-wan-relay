package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
	config "wake-on-wan-relay/lib"
)

// generateTestCertificate generates a self-signed certificate and private key in PEM format.
func generateTestCertificate(t *testing.T, host string) ([]byte, []byte, string) {
	t.Helper()

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate ed25519 key: %v", err)
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: host,
		},
		NotBefore:             time.Now().Add(-1 * time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	if ip := net.ParseIP(host); ip != nil {
		template.IPAddresses = []net.IP{ip}
	} else {
		template.DNSNames = []string{host}
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, pub, priv)
	if err != nil {
		t.Fatalf("Failed to create certificate: %v", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	privDER, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		t.Fatalf("Failed to marshal private key: %v", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privDER})

	hash := sha256.Sum256(certDER)
	fingerprint := hex.EncodeToString(hash[:])

	return certPEM, keyPEM, fingerprint
}

func TestBuildTLSConfig_Default(t *testing.T) {
	ctx := &config.Context{}
	tlsConfig, err := buildTLSConfig(ctx)
	if err != nil {
		t.Fatalf("buildTLSConfig failed: %v", err)
	}

	if tlsConfig.InsecureSkipVerify {
		t.Errorf("Expected InsecureSkipVerify to be false by default")
	}
	if tlsConfig.MinVersion != tls.VersionTLS12 {
		t.Errorf("Expected MinVersion to be TLS 1.2 (%x), got %x", tls.VersionTLS12, tlsConfig.MinVersion)
	}
}

func TestTLSIntegration_SelfSignedAndFingerprint(t *testing.T) {
	certPEM, keyPEM, fingerprint := generateTestCertificate(t, "127.0.0.1")

	tempDir := t.TempDir()
	certFile := filepath.Join(tempDir, "cert.pem")
	keyFile := filepath.Join(tempDir, "key.pem")
	if err := os.WriteFile(certFile, certPEM, 0600); err != nil {
		t.Fatalf("Failed to write cert: %v", err)
	}
	if err := os.WriteFile(keyFile, keyPEM, 0600); err != nil {
		t.Fatalf("Failed to write key: %v", err)
	}

	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatalf("Failed to load test cert: %v", err)
	}

	ts := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	}))
	ts.TLS = &tls.Config{
		Certificates: []tls.Certificate{tlsCert},
	}
	ts.StartTLS()
	defer ts.Close()

	// 1. Test that default config fails against self-signed cert (verifies InsecureSkipVerify is disabled)
	defaultCtx := &config.Context{}
	defTLS, err := buildTLSConfig(defaultCtx)
	if err != nil {
		t.Fatalf("buildTLSConfig error: %v", err)
	}
	client := &http.Client{Transport: &http.Transport{TLSClientConfig: defTLS}}
	_, err = client.Get(ts.URL)
	if err == nil {
		t.Errorf("Expected connection to fail with untrusted self-signed cert, but it succeeded")
	}

	// 2. Test that CACertPath allows connection to succeed
	caCtx := &config.Context{CACertPath: certFile}
	caTLS, err := buildTLSConfig(caCtx)
	if err != nil {
		t.Fatalf("buildTLSConfig error: %v", err)
	}
	clientCA := &http.Client{Transport: &http.Transport{TLSClientConfig: caTLS}}
	resp, err := clientCA.Get(ts.URL)
	if err != nil {
		t.Fatalf("Expected connection with CACertPath to succeed, got: %v", err)
	}
	resp.Body.Close()

	// 3. Test that certFingerprint pinning succeeds
	fpCtx := &config.Context{CertFingerprint: fingerprint}
	fpTLS, err := buildTLSConfig(fpCtx)
	if err != nil {
		t.Fatalf("buildTLSConfig error: %v", err)
	}
	clientFP := &http.Client{Transport: &http.Transport{TLSClientConfig: fpTLS}}
	resp, err = clientFP.Get(ts.URL)
	if err != nil {
		t.Fatalf("Expected connection with matching CertFingerprint to succeed, got: %v", err)
	}
	resp.Body.Close()

	// 4. Test that formatted certFingerprint (with colons) also succeeds
	formattedFingerprint := ""
	for i := 0; i < len(fingerprint); i += 2 {
		if i > 0 {
			formattedFingerprint += ":"
		}
		formattedFingerprint += fingerprint[i : i+2]
	}
	colonFpCtx := &config.Context{CertFingerprint: formattedFingerprint}
	colonFpTLS, err := buildTLSConfig(colonFpCtx)
	if err != nil {
		t.Fatalf("buildTLSConfig error: %v", err)
	}
	clientColonFP := &http.Client{Transport: &http.Transport{TLSClientConfig: colonFpTLS}}
	resp, err = clientColonFP.Get(ts.URL)
	if err != nil {
		t.Fatalf("Expected connection with colon-formatted CertFingerprint to succeed, got: %v", err)
	}
	resp.Body.Close()

	// 5. Test that mismatching certFingerprint is rejected
	badFpCtx := &config.Context{CertFingerprint: "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff"}
	badFpTLS, err := buildTLSConfig(badFpCtx)
	if err != nil {
		t.Fatalf("buildTLSConfig error: %v", err)
	}
	clientBadFP := &http.Client{Transport: &http.Transport{TLSClientConfig: badFpTLS}}
	_, err = clientBadFP.Get(ts.URL)
	if err == nil {
		t.Errorf("Expected connection with mismatching fingerprint to fail, but it succeeded")
	}
}
