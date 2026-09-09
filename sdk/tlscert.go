// Package sdk is a software development kit for building blockchain applications.
// File sdk/tlscert.go - Self-signed certificate generation for local development.
package sdk

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"
)

// selfSignedValidity is how long a generated development certificate lasts.
//
// Deliberately short. A self-signed certificate that quietly works for a decade
// is one somebody eventually ships; one that expires in a month is one they
// replace before it matters.
const selfSignedValidity = 30 * 24 * time.Hour

// GenerateSelfSignedCert writes a certificate and key for local development.
//
// FOR DEVELOPMENT ONLY. A self-signed certificate encrypts the connection but
// authenticates nobody: any client that trusts it will equally trust an attacker
// who presents their own, so it stops passive eavesdropping and not an active
// machine-in-the-middle. Use a certificate from a CA your clients already trust
// for anything reachable by someone else.
//
// hosts are the names and IPs the certificate is valid for; "localhost" and
// 127.0.0.1 are included when none are given.
func GenerateSelfSignedCert(dir string, hosts []string) (certFile, keyFile string, err error) {
	if dir == "" {
		return "", "", fmt.Errorf("certificate directory is empty")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", "", fmt.Errorf("create certificate directory: %w", err)
	}

	if len(hosts) == 0 {
		hosts = []string{"localhost", "127.0.0.1", "::1"}
	}

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return "", "", fmt.Errorf("generate key: %w", err)
	}

	// A random serial: a fixed one makes two certificates indistinguishable to
	// anything that caches by serial.
	serialLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serial, err := rand.Int(rand.Reader, serialLimit)
	if err != nil {
		return "", "", fmt.Errorf("generate serial: %w", err)
	}

	template := x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			Organization: []string{"go-basic-blockchain (development)"},
			CommonName:   hosts[0],
		},
		NotBefore:             time.Now().Add(-time.Hour), // tolerate modest clock skew
		NotAfter:              time.Now().Add(selfSignedValidity),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	for _, host := range hosts {
		if ip := net.ParseIP(host); ip != nil {
			template.IPAddresses = append(template.IPAddresses, ip)
			continue
		}
		template.DNSNames = append(template.DNSNames, host)
	}

	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		return "", "", fmt.Errorf("create certificate: %w", err)
	}

	certFile = filepath.Join(dir, "api-cert.pem")
	keyFile = filepath.Join(dir, "api-key.pem")

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	if certPEM == nil {
		return "", "", fmt.Errorf("encode certificate")
	}
	// 0644: a certificate is public by definition; it is presented to every
	// client that connects.
	if err := writeFileAtomic(certFile, certPEM, 0o644); err != nil {
		return "", "", fmt.Errorf("write certificate: %w", err)
	}

	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return "", "", fmt.Errorf("marshal key: %w", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	if keyPEM == nil {
		return "", "", fmt.Errorf("encode key")
	}
	// 0600: the private key is the whole secret.
	if err := writeFileAtomic(keyFile, keyPEM, 0o600); err != nil {
		return "", "", fmt.Errorf("write key: %w", err)
	}

	LogInfof("Generated a SELF-SIGNED certificate for %v, valid %d days: %s",
		hosts, int(selfSignedValidity.Hours()/24), certFile)
	LogInfof("This encrypts the connection but authenticates nobody. Use a CA-issued " +
		"certificate for anything another machine can reach.")

	return certFile, keyFile, nil
}
