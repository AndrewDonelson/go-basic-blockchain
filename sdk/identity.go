// Package sdk is a software development kit for building blockchain applications.
// File sdk/identity.go - node identity keys
//
// A node's ID used to be a random UUID it asserted about itself during the
// handshake, and the peer took its word for it. Any host could claim any ID,
// which made peer identity meaningless: there was nothing to bind the name to.
//
// A node ID is now the SHA-256 of its public key. That makes it self-certifying
// -- a peer proves the ID by signing a challenge with the matching private key,
// and an ID cannot be claimed by anyone who does not hold that key.
package sdk

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
	"os"
	"path/filepath"
)

// peerIdentityFileName is where a node's identity key is stored.
const peerIdentityFileName = "node_identity.pem"

// PeerIdentity is a node's long-term identity keypair.
type PeerIdentity struct {
	privateKey *ecdsa.PrivateKey

	// NodeID is the hex SHA-256 of the marshalled public key.
	NodeID string
	// PublicPEM is the PEM-encoded public key, sent to peers during the handshake.
	PublicPEM string
}

// NewPeerIdentity generates a fresh identity.
func NewPeerIdentity() (*PeerIdentity, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate node identity key: %w", err)
	}
	return peerIdentityFromKey(key)
}

// peerIdentityFromKey derives the public material for a private key.
func peerIdentityFromKey(key *ecdsa.PrivateKey) (*PeerIdentity, error) {
	publicDER, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal node public key: %w", err)
	}

	publicPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicDER})
	if publicPEM == nil {
		return nil, errors.New("failed to PEM-encode node public key")
	}

	return &PeerIdentity{
		privateKey: key,
		NodeID:     peerIDFromPublicDER(publicDER),
		PublicPEM:  string(publicPEM),
	}, nil
}

// peerIDFromPublicDER derives a node ID from a marshalled public key.
func peerIDFromPublicDER(publicDER []byte) string {
	sum := sha256.Sum256(publicDER)
	return hex.EncodeToString(sum[:])
}

// PeerIDFromPublicPEM derives the node ID a public key must be presented under.
//
// This is what makes an ID unforgeable: the peer's claimed ID is recomputed from
// the key it presents, so a mismatch is detected before any signature check.
func PeerIDFromPublicPEM(publicPEM string) (string, error) {
	pub, err := parsePeerPublicKey(publicPEM)
	if err != nil {
		return "", err
	}

	publicDER, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		return "", fmt.Errorf("failed to marshal peer public key: %w", err)
	}
	return peerIDFromPublicDER(publicDER), nil
}

// parsePeerPublicKey decodes a PEM-encoded ECDSA public key.
func parsePeerPublicKey(publicPEM string) (*ecdsa.PublicKey, error) {
	block, _ := pem.Decode([]byte(publicPEM))
	if block == nil {
		return nil, errors.New("peer public key is not valid PEM")
	}

	parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse peer public key: %w", err)
	}

	pub, ok := parsed.(*ecdsa.PublicKey)
	if !ok {
		return nil, errors.New("peer public key is not an ECDSA key")
	}
	if pub.Curve != elliptic.P256() {
		// The session key is derived by ECDH over the same keys, so both sides
		// must agree on the curve.
		return nil, errors.New("peer public key is not on the P-256 curve")
	}
	return pub, nil
}

// PublicKey returns the identity's public key.
func (p *PeerIdentity) PublicKey() *ecdsa.PublicKey {
	if p == nil || p.privateKey == nil {
		return nil
	}
	return &p.privateKey.PublicKey
}

// Sign signs a handshake transcript.
func (p *PeerIdentity) Sign(message []byte) ([]byte, error) {
	if p == nil || p.privateKey == nil {
		return nil, errors.New("node identity has no private key")
	}

	digest := sha256.Sum256(message)
	return ecdsa.SignASN1(rand.Reader, p.privateKey, digest[:])
}

// VerifyPeerSignature checks a signature made by a peer's identity key.
func VerifyPeerSignature(publicPEM string, message, signature []byte) error {
	pub, err := parsePeerPublicKey(publicPEM)
	if err != nil {
		return err
	}

	digest := sha256.Sum256(message)
	if !ecdsa.VerifyASN1(pub, digest[:], signature) {
		return errors.New("peer signature does not verify")
	}
	return nil
}

// LoadOrCreatePeerIdentity loads a node's identity from dataPath, creating one on
// first run.
//
// The identity is persisted because a node ID that changed on every restart would
// make an allowlist useless and every reconnection look like a new peer.
func LoadOrCreatePeerIdentity(dataPath string) (*PeerIdentity, error) {
	if dataPath == "" {
		return NewPeerIdentity()
	}

	path := filepath.Join(dataPath, peerIdentityFileName)

	data, err := os.ReadFile(path)
	switch {
	case err == nil:
		identity, err := parsePeerIdentity(data)
		if err != nil {
			return nil, fmt.Errorf("node identity at %s is unusable: %w", path, err)
		}
		LogVerbosef("Loaded node identity %s", identity.NodeID)
		return identity, nil

	case errors.Is(err, os.ErrNotExist):
		// Fall through and create one.

	default:
		return nil, fmt.Errorf("failed to read node identity: %w", err)
	}

	identity, err := NewPeerIdentity()
	if err != nil {
		return nil, err
	}

	encoded, err := identity.encodePrivate()
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(dataPath, 0700); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}
	// 0600: this is a private key.
	if err := writeFileAtomic(path, encoded, 0600); err != nil {
		return nil, fmt.Errorf("failed to persist node identity: %w", err)
	}

	LogInfof("Generated node identity %s", identity.NodeID)
	return identity, nil
}

// encodePrivate PEM-encodes the identity's private key.
func (p *PeerIdentity) encodePrivate() ([]byte, error) {
	der, err := x509.MarshalECPrivateKey(p.privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal node identity key: %w", err)
	}

	encoded := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der})
	if encoded == nil {
		return nil, errors.New("failed to PEM-encode node identity key")
	}
	return encoded, nil
}

// parsePeerIdentity decodes a stored identity.
func parsePeerIdentity(data []byte) (*PeerIdentity, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, errors.New("identity file is not valid PEM")
	}

	key, err := x509.ParseECPrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse identity key: %w", err)
	}
	if key.Curve != elliptic.P256() {
		return nil, errors.New("identity key is not on the P-256 curve")
	}
	return peerIdentityFromKey(key)
}
