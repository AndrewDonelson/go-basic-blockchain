package sdk

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"net"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

// tlsTestCert generates a certificate in a temporary directory.
func tlsTestCert(t *testing.T) (certFile, keyFile string) {
	t.Helper()
	certFile, keyFile, err := GenerateSelfSignedCert(t.TempDir(), []string{"localhost", "127.0.0.1"})
	if err != nil {
		t.Fatalf("generate certificate: %v", err)
	}
	return certFile, keyFile
}

// -----------------------------------------------------------------------------
// Certificate generation
// -----------------------------------------------------------------------------

func TestGeneratedCertificateIsUsable(t *testing.T) {
	certFile, keyFile := tlsTestCert(t)

	pair, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		t.Fatalf("the generated pair does not load: %v", err)
	}

	leaf, err := x509.ParseCertificate(pair.Certificate[0])
	if err != nil {
		t.Fatalf("parse certificate: %v", err)
	}

	if leaf.NotAfter.Before(time.Now()) {
		t.Fatal("the certificate is already expired")
	}
	// Short-lived on purpose: a self-signed certificate that works for a decade
	// is one somebody eventually ships.
	if leaf.NotAfter.After(time.Now().Add(90 * 24 * time.Hour)) {
		t.Fatalf("a development certificate should be short-lived, expires %s",
			leaf.NotAfter)
	}

	var haveLocalhost bool
	for _, name := range leaf.DNSNames {
		if name == "localhost" {
			haveLocalhost = true
		}
	}
	if !haveLocalhost {
		t.Fatalf("certificate is not valid for localhost: %v", leaf.DNSNames)
	}
	if len(leaf.IPAddresses) == 0 {
		t.Fatal("certificate carries no IP SANs, so https://127.0.0.1 will not verify")
	}
}

// TestPrivateKeyIsNotWorldReadable: the certificate is public, the key is the
// whole secret.
func TestPrivateKeyIsNotWorldReadable(t *testing.T) {
	certFile, keyFile := tlsTestCert(t)

	keyInfo, err := os.Stat(keyFile)
	if err != nil {
		t.Fatalf("stat key: %v", err)
	}
	if mode := keyInfo.Mode().Perm(); mode&0o077 != 0 {
		t.Fatalf("the private key is readable beyond its owner (%v)", mode)
	}

	certInfo, err := os.Stat(certFile)
	if err != nil {
		t.Fatalf("stat cert: %v", err)
	}
	if certInfo.Mode().Perm()&0o400 == 0 {
		t.Fatal("the certificate is not readable by its owner")
	}
}

func TestGeneratedKeyIsPKCS8(t *testing.T) {
	_, keyFile := tlsTestCert(t)

	raw, err := os.ReadFile(keyFile)
	if err != nil {
		t.Fatalf("read key: %v", err)
	}
	block, _ := pem.Decode(raw)
	if block == nil {
		t.Fatal("the key is not valid PEM")
	}
	if block.Type != "PRIVATE KEY" {
		t.Fatalf("PEM block is %q, want PRIVATE KEY", block.Type)
	}
	if _, err := x509.ParsePKCS8PrivateKey(block.Bytes); err != nil {
		t.Fatalf("the key is not valid PKCS#8: %v", err)
	}
}

func TestGenerateSelfSignedCertRejectsAnEmptyDirectory(t *testing.T) {
	if _, _, err := GenerateSelfSignedCert("", nil); err == nil {
		t.Fatal("an empty directory was accepted")
	}
}

// -----------------------------------------------------------------------------
// Configuration
// -----------------------------------------------------------------------------

// TestTLSConfigIsValidatedAtStartup: a certificate that cannot be loaded should
// fail with the rest of the configuration, not when the listener comes up.
func TestTLSConfigIsValidatedAtStartup(t *testing.T) {
	t.Run("missing paths", func(t *testing.T) {
		cfg := NewConfig()
		cfg.APITLSEnabled = true
		cfg.APITLSCertFile = ""
		cfg.APITLSKeyFile = ""

		err := cfg.Validate()
		if err == nil {
			t.Fatal("TLS was enabled with no certificate configured")
		}
		if !strings.Contains(err.Error(), "API_TLS_CERT_FILE") {
			t.Fatalf("the error does not name the missing setting: %v", err)
		}
	})

	t.Run("unreadable certificate", func(t *testing.T) {
		cfg := NewConfig()
		cfg.APITLSEnabled = true
		cfg.APITLSCertFile = "/nonexistent/cert.pem"
		cfg.APITLSKeyFile = "/nonexistent/key.pem"

		err := cfg.Validate()
		if err == nil {
			t.Fatal("an unloadable certificate validated successfully")
		}
		if !strings.Contains(err.Error(), "cannot be loaded") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("valid certificate", func(t *testing.T) {
		certFile, keyFile := tlsTestCert(t)

		cfg := NewConfig()
		cfg.APITLSEnabled = true
		cfg.APITLSCertFile = certFile
		cfg.APITLSKeyFile = keyFile

		if err := cfg.Validate(); err != nil {
			t.Fatalf("a valid TLS configuration was rejected: %v", err)
		}
	})

	t.Run("TLS off needs no certificate", func(t *testing.T) {
		cfg := NewConfig()
		cfg.APITLSEnabled = false
		if err := cfg.Validate(); err != nil {
			t.Fatalf("TLS disabled should not require a certificate: %v", err)
		}
	})
}

// TestTLSProfileRefusesObsoleteVersions.
//
// Go permits TLS 1.0 and 1.1 unless a minimum is set, and both have been
// deprecated for years.
func TestTLSProfileRefusesObsoleteVersions(t *testing.T) {
	profile := apiTLSConfig()

	if profile.MinVersion != tls.VersionTLS12 {
		t.Fatalf("minimum TLS version is %x, want TLS 1.2 (%x)",
			profile.MinVersion, tls.VersionTLS12)
	}

	// Every named suite must be an ECDHE (forward-secret) AEAD suite. A recorded
	// session must not become readable later because the server key leaked.
	for _, suite := range profile.CipherSuites {
		name := tls.CipherSuiteName(suite)
		if !strings.Contains(name, "ECDHE") {
			t.Fatalf("%s does not provide forward secrecy", name)
		}
		if !strings.Contains(name, "GCM") && !strings.Contains(name, "CHACHA20") {
			t.Fatalf("%s is not an AEAD suite", name)
		}
	}

	// And none of the insecure suites Go still knows about.
	for _, insecure := range tls.InsecureCipherSuites() {
		for _, suite := range profile.CipherSuites {
			if suite == insecure.ID {
				t.Fatalf("%s is in the list and Go considers it insecure",
					tls.CipherSuiteName(suite))
			}
		}
	}
}

// -----------------------------------------------------------------------------
// End to end
// -----------------------------------------------------------------------------

// serveTLS starts the API over TLS on a free port and returns its base URL.
func serveTLS(t *testing.T, certFile, keyFile string) (string, *x509.CertPool) {
	t.Helper()

	bc := forkTestChain(t, 1, uint32(genesisDifficulty))
	api := NewAPI(bc)
	if api == nil {
		t.Fatal("failed to create the API")
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	server := &http.Server{
		Handler:           api.router,
		TLSConfig:         apiTLSConfig(),
		ReadHeaderTimeout: apiReadHeaderTimeout,
	}
	go func() { _ = server.ServeTLS(listener, certFile, keyFile) }()
	t.Cleanup(func() { _ = server.Close() })

	// Trust exactly the certificate we generated, so this verifies rather than
	// skipping verification.
	pemBytes, err := os.ReadFile(certFile)
	if err != nil {
		t.Fatalf("read certificate: %v", err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(pemBytes) {
		t.Fatal("could not add the certificate to a pool")
	}

	return "https://" + listener.Addr().String(), pool
}

// TestAPIServesOverTLS is the end-to-end check: a real handshake, a verified
// certificate, and a response.
func TestAPIServesOverTLS(t *testing.T) {
	certFile, keyFile := tlsTestCert(t)
	baseURL, pool := serveTLS(t, certFile, keyFile)

	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12},
		},
	}

	resp, err := client.Get(baseURL + "/v1/health")
	if err != nil {
		t.Fatalf("HTTPS request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("/v1/health returned %d over TLS", resp.StatusCode)
	}
	if resp.TLS == nil {
		t.Fatal("the response did not come over TLS")
	}
	if resp.TLS.Version < tls.VersionTLS12 {
		t.Fatalf("negotiated TLS %x, below the 1.2 floor", resp.TLS.Version)
	}
}

// TestTLSRejectsAnUntrustedClientChain: verification must actually be happening,
// or the test above proves nothing.
func TestTLSRejectsAnUntrustedClientChain(t *testing.T) {
	certFile, keyFile := tlsTestCert(t)
	baseURL, _ := serveTLS(t, certFile, keyFile)

	// An empty pool trusts nothing, so the handshake must fail.
	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{RootCAs: x509.NewCertPool(), MinVersion: tls.VersionTLS12},
		},
	}

	resp, err := client.Get(baseURL + "/v1/health")
	if err == nil {
		resp.Body.Close()
		t.Fatal("a client trusting no certificate authority completed the handshake")
	}
}

// TestAuthenticationStillAppliesOverTLS: encryption is not authorisation.
func TestAuthenticationStillAppliesOverTLS(t *testing.T) {
	certFile, keyFile := tlsTestCert(t)
	baseURL, pool := serveTLS(t, certFile, keyFile)

	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12},
		},
	}

	resp, err := client.Get(baseURL + "/v1/blockchain")
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		t.Fatal("a protected endpoint answered an unauthenticated request over TLS")
	}
}
