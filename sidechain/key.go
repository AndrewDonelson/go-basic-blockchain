// File sidechain/key.go - The ECDSA P-256 keys that sign anchors, registrations and transactions.
package sidechain

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
)

// Key is an ECDSA P-256 key pair, the only kind the chain accepts.
//
// One type serves both roles an application needs: a publisher identity (it
// signs anchors and game registrations, and is what the chain registers) and a
// wallet (it signs transactions and pays their fees and bonds). They may be the
// same key or two different ones.
type Key struct {
	private *ecdsa.PrivateKey
}

// GenerateKey creates a new key.
func GenerateKey() (*Key, error) {
	k, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate key: %w", err)
	}
	return &Key{private: k}, nil
}

// ParseKeyPEM reads a private key in SEC 1 ("EC PRIVATE KEY") or PKCS #8
// ("PRIVATE KEY") PEM form.
func ParseKeyPEM(privatePEM []byte) (*Key, error) {
	block, _ := pem.Decode(privatePEM)
	if block == nil {
		return nil, errors.New("private key is not valid PEM")
	}
	var key *ecdsa.PrivateKey
	switch block.Type {
	case "EC PRIVATE KEY":
		k, err := x509.ParseECPrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parse private key: %w", err)
		}
		key = k
	case "PRIVATE KEY":
		parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parse private key: %w", err)
		}
		k, ok := parsed.(*ecdsa.PrivateKey)
		if !ok {
			return nil, errors.New("private key is not an ECDSA key")
		}
		key = k
	default:
		return nil, fmt.Errorf("unexpected PEM block %q", block.Type)
	}
	if key.Curve != elliptic.P256() {
		return nil, errors.New("private key is not on the P-256 curve")
	}
	return &Key{private: key}, nil
}

// PrivatePEM returns the private key in SEC 1 PEM form (the form the node's
// wallets use). Keep it secret.
func (k *Key) PrivatePEM() []byte {
	der, err := x509.MarshalECPrivateKey(k.private)
	if err != nil {
		return nil
	}
	return pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der})
}

// PublicPEM returns the public key in PKIX PEM form.
func (k *Key) PublicPEM() string {
	der, err := x509.MarshalPKIXPublicKey(&k.private.PublicKey)
	if err != nil {
		return ""
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}))
}

// Address is the key's account address: the hex SHA-256 of its PKIX DER
// encoding. It is also a publisher key's fingerprint.
func (k *Key) Address() string {
	der, err := x509.MarshalPKIXPublicKey(&k.private.PublicKey)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(der)
	return hex.EncodeToString(sum[:])
}

// Sign signs SHA-256(message) and returns the ASN.1 signature.
func (k *Key) Sign(message []byte) ([]byte, error) {
	if k == nil || k.private == nil {
		return nil, errors.New("key has no private half")
	}
	digest := sha256.Sum256(message)
	return ecdsa.SignASN1(rand.Reader, k.private, digest[:])
}

// AddressOf returns the address (fingerprint) of a PKIX PEM public key.
func AddressOf(publicKeyPEM string) (string, error) {
	pub, err := parsePublicKey(publicKeyPEM)
	if err != nil {
		return "", err
	}
	der, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(der)
	return hex.EncodeToString(sum[:]), nil
}

// VerifySignature checks an ASN.1 signature over SHA-256(message).
func VerifySignature(publicKeyPEM string, message, signature []byte) error {
	pub, err := parsePublicKey(publicKeyPEM)
	if err != nil {
		return err
	}
	digest := sha256.Sum256(message)
	if !ecdsa.VerifyASN1(pub, digest[:], signature) {
		return errors.New("signature does not verify")
	}
	return nil
}

func parsePublicKey(publicKeyPEM string) (*ecdsa.PublicKey, error) {
	block, _ := pem.Decode([]byte(publicKeyPEM))
	if block == nil {
		return nil, errors.New("public key is not valid PEM")
	}
	parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse public key: %w", err)
	}
	pub, ok := parsed.(*ecdsa.PublicKey)
	if !ok || pub.Curve != elliptic.P256() {
		return nil, errors.New("public key is not an ECDSA P-256 key")
	}
	return pub, nil
}
