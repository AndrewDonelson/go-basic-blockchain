// Package sdk is a software development kit for building blockchain applications.
// File sdk/p2p_auth.go - authenticated, encrypted peer sessions
//
// The old handshake was HELLO / ACK / {"id","address"} / OK. A node simply
// asserted an ID and the peer believed it: any host could claim any identity,
// and the whole connection was plaintext, so an active attacker could read and
// rewrite everything on it.
//
// This replaces it with a mutually authenticated handshake that also establishes
// an encrypted session:
//
//	client -> HELLO <version>
//	server -> ACK   <serverID> <serverPubPEM> <serverNonce> <unixSeconds>
//	client -> AUTH  <clientID> <clientPubPEM> <clientNonce> <signature> <address>
//	server -> OK    <signature>
//	                          ... both sides switch to an encrypted stream ...
//
// Each side signs a transcript covering BOTH nonces and BOTH node IDs, so a
// captured handshake cannot be replayed against a different peer or a later
// session. Node IDs are the SHA-256 of the public key (see identity.go), so the
// claimed ID is recomputed from the presented key before any signature is
// checked -- an ID cannot be borrowed.
//
// The session key is derived by ECDH over the same identity keys, with both
// nonces as the salt. That binds the encryption to the authenticated identities:
// an attacker who cannot produce a valid signature also cannot derive the key,
// so authentication and confidentiality stand or fall together.
package sdk

import (
	"bufio"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)

// hkdfSHA256 implements HKDF (RFC 5869) over SHA-256.
//
// Written out rather than pulled in from golang.org/x/crypto: it is thirty lines
// of standard-library HMAC, and the project deliberately keeps its dependency
// surface small.
func hkdfSHA256(secret, salt, info []byte, length int) ([]byte, error) {
	if length <= 0 {
		return nil, errors.New("key length must be positive")
	}

	// Extract: a pseudo-random key of the hash length.
	if len(salt) == 0 {
		salt = make([]byte, sha256.Size)
	}
	extractor := hmac.New(sha256.New, salt)
	extractor.Write(secret)
	prk := extractor.Sum(nil)

	// Expand: T(n) = HMAC(prk, T(n-1) || info || n)
	maxLength := 255 * sha256.Size
	if length > maxLength {
		return nil, fmt.Errorf("cannot derive %d bytes; HKDF-SHA256 caps at %d", length, maxLength)
	}

	out := make([]byte, 0, length)
	var previous []byte
	for counter := byte(1); len(out) < length; counter++ {
		expander := hmac.New(sha256.New, prk)
		expander.Write(previous)
		expander.Write(info)
		expander.Write([]byte{counter})
		previous = expander.Sum(nil)
		out = append(out, previous...)
	}

	return out[:length], nil
}

const (
	// p2pProtocolVersion is the handshake version. A peer speaking a different
	// version is refused rather than half-understood.
	p2pProtocolVersion = "gbb/3"

	// p2pAuthContext domain-separates handshake signatures, so a signature made
	// here can never be replayed as a transaction signature or vice versa.
	p2pAuthContext = "gbb-p2p-auth-v1"

	// p2pSessionContext domain-separates the session key derivation.
	p2pSessionContext = "gbb-p2p-session-v1"

	// p2pNonceSize is the length of each side's handshake challenge.
	p2pNonceSize = 32

	// p2pMaxClockSkew bounds how stale a handshake may be. Combined with the
	// nonce cache this is what makes replay impractical.
	p2pMaxClockSkew = 2 * time.Minute

	// p2pMaxRecordSize caps one encrypted record.
	p2pMaxRecordSize = 1 << 20 // 1 MiB
)

var (
	// ErrPeerNotAllowed is returned when a peer is not on the configured allowlist.
	ErrPeerNotAllowed = errors.New("peer is not on the allowlist")

	// ErrHandshakeReplay is returned when a handshake nonce has been seen before.
	ErrHandshakeReplay = errors.New("handshake nonce has already been used")
)

// authenticatedPeer is what a completed handshake establishes about the far side.
type authenticatedPeer struct {
	NodeID    string
	PublicPEM string
	Address   string
}

// handshakeTranscript builds the bytes both sides sign.
//
// Including both nonces and both IDs prevents a captured signature being replayed
// against a different peer, or reflected back at its own author.
//
// Including both EPHEMERAL PUBLIC KEYS is what makes the ephemeral exchange safe.
// The session key comes from an anonymous Diffie-Hellman between two ephemeral
// keys; on its own that is wide open to a machine-in-the-middle, who can relay
// the identity handshake untouched while substituting its own ephemeral key on
// each side. Signing the ephemeral keys with the long-term identity key binds
// them to an authenticated identity, so a substituted key invalidates the
// signature. See TestEphemeralKeyIsBoundToIdentity.
func handshakeTranscript(signerID, peerID string, signerNonce, peerNonce, signerEphemeral, peerEphemeral []byte) []byte {
	var buf []byte
	buf = append(buf, p2pAuthContext...)
	buf = append(buf, 0)
	buf = append(buf, signerID...)
	buf = append(buf, 0)
	buf = append(buf, peerID...)
	buf = append(buf, 0)
	buf = append(buf, signerNonce...)
	buf = append(buf, 0)
	buf = append(buf, peerNonce...)
	buf = append(buf, 0)
	buf = append(buf, signerEphemeral...)
	buf = append(buf, 0)
	buf = append(buf, peerEphemeral...)
	return buf
}

// ephemeralKeyPair is a single-use ECDH keypair.
//
// A fresh pair is generated for every handshake and the private half is dropped
// as soon as the session key is derived. That is what gives forward secrecy:
// the session key is not recoverable from the long-term identity keys, so an
// attacker who later compromises a node's identity key cannot decrypt sessions
// they recorded earlier.
type ephemeralKeyPair struct {
	private *ecdh.PrivateKey
	public  []byte
}

// newEphemeralKeyPair generates a per-handshake ECDH keypair on P-256.
func newEphemeralKeyPair() (*ephemeralKeyPair, error) {
	private, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate ephemeral key: %w", err)
	}
	return &ephemeralKeyPair{private: private, public: private.PublicKey().Bytes()}, nil
}

// parseEphemeralPublicKey decodes and validates a peer's ephemeral public key.
//
// ecdh.NewPublicKey rejects points that are not on the curve and the identity
// point, which is what closes off invalid-curve and small-subgroup attacks --
// feeding a crafted "public key" to ECDH can otherwise leak private key bits.
func parseEphemeralPublicKey(raw []byte) (*ecdh.PublicKey, error) {
	if len(raw) == 0 {
		return nil, errors.New("peer sent no ephemeral key")
	}

	pub, err := ecdh.P256().NewPublicKey(raw)
	if err != nil {
		return nil, fmt.Errorf("peer ephemeral key is not a valid P-256 point: %w", err)
	}
	return pub, nil
}

// deriveSessionKey performs ECDH between the two EPHEMERAL keys.
//
// The long-term identity keys are deliberately not an input. They authenticate
// the exchange -- each side signs a transcript covering both ephemeral public
// keys -- but they never contribute to the key itself.
//
// That separation is the whole point of forward secrecy. The previous version
// derived the key from the identity keys directly, so anyone who later obtained a
// node's identity key could decrypt every session it had ever had, including ones
// recorded months earlier. Now the only material that can produce the key is a
// pair of private values that existed for the length of one handshake and were
// never transmitted.
//
// The nonces go in as salt, so two handshakes never collide even in the
// vanishingly unlikely event of an ephemeral key repeating.
func deriveSessionKey(local *ephemeralKeyPair, peerEphemeralPublic []byte, clientNonce, serverNonce []byte) ([]byte, error) {
	if local == nil || local.private == nil {
		return nil, errors.New("no ephemeral key for this handshake")
	}

	peerPub, err := parseEphemeralPublicKey(peerEphemeralPublic)
	if err != nil {
		return nil, err
	}

	shared, err := local.private.ECDH(peerPub)
	if err != nil {
		return nil, fmt.Errorf("ephemeral ECDH failed: %w", err)
	}

	// Order the salt consistently on both sides.
	salt := append(append([]byte{}, clientNonce...), serverNonce...)

	key, err := hkdfSHA256(shared, salt, []byte(p2pSessionContext), 32)
	if err != nil {
		return nil, fmt.Errorf("failed to derive session key: %w", err)
	}
	return key, nil
}

// nonceCache remembers recently seen handshake nonces, so a captured handshake
// cannot simply be replayed inside the freshness window.
type nonceCache struct {
	mu    sync.Mutex
	seen  map[string]time.Time
	ttl   time.Duration
	limit int
}

func newNonceCache(ttl time.Duration) *nonceCache {
	return &nonceCache{seen: map[string]time.Time{}, ttl: ttl, limit: 10000}
}

// use records a nonce, reporting false if it has been seen before.
func (c *nonceCache) use(nonce string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	if seenAt, ok := c.seen[nonce]; ok && now.Sub(seenAt) < c.ttl {
		return false
	}

	// Opportunistic eviction, so the cache cannot grow without bound.
	if len(c.seen) >= c.limit {
		for key, seenAt := range c.seen {
			if now.Sub(seenAt) >= c.ttl {
				delete(c.seen, key)
			}
		}
		if len(c.seen) >= c.limit {
			// Still full of live entries: drop the whole set rather than leak.
			c.seen = map[string]time.Time{}
		}
	}

	c.seen[nonce] = now
	return true
}

// secureConn wraps a connection in an authenticated encrypted stream.
//
// It satisfies net.Conn, so the line-oriented protocol above it is unchanged: the
// framing swap is invisible to the command handlers.
type secureConn struct {
	net.Conn

	aead    cipher.AEAD
	sendKey []byte

	writeMu sync.Mutex
	sendSeq uint64

	readMu  sync.Mutex
	recvSeq uint64
	pending []byte
	header  [4]byte
}

// newSecureConn wraps conn with AES-256-GCM using the derived session key.
func newSecureConn(conn net.Conn, key []byte) (*secureConn, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create session cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create session AEAD: %w", err)
	}
	return &secureConn{Conn: conn, aead: aead, sendKey: key}, nil
}

// recordNonce derives a per-record nonce from the sequence number.
//
// The session key is fresh per handshake, so a counter is safe and never repeats
// within a session -- which is the requirement GCM actually has.
func recordNonce(aead cipher.AEAD, seq uint64) []byte {
	nonce := make([]byte, aead.NonceSize())
	for i := 0; i < 8 && i < len(nonce); i++ {
		nonce[len(nonce)-1-i] = byte(seq >> (8 * i))
	}
	return nonce
}

// Write encrypts p as a single length-prefixed record.
func (c *secureConn) Write(p []byte) (int, error) {
	if len(p) > p2pMaxRecordSize {
		return 0, fmt.Errorf("record of %d bytes exceeds the %d byte limit", len(p), p2pMaxRecordSize)
	}

	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	nonce := recordNonce(c.aead, c.sendSeq)
	c.sendSeq++

	sealed := c.aead.Seal(nil, nonce, p, nil)

	var header [4]byte
	header[0] = byte(len(sealed) >> 24)
	header[1] = byte(len(sealed) >> 16)
	header[2] = byte(len(sealed) >> 8)
	header[3] = byte(len(sealed))

	if _, err := c.Conn.Write(header[:]); err != nil {
		return 0, err
	}
	if _, err := c.Conn.Write(sealed); err != nil {
		return 0, err
	}
	return len(p), nil
}

// Read decrypts records, buffering so callers can read at any granularity.
func (c *secureConn) Read(p []byte) (int, error) {
	c.readMu.Lock()
	defer c.readMu.Unlock()

	if len(c.pending) == 0 {
		if err := c.readRecordLocked(); err != nil {
			return 0, err
		}
	}

	n := copy(p, c.pending)
	c.pending = c.pending[n:]
	return n, nil
}

// readRecordLocked reads and decrypts exactly one record.
func (c *secureConn) readRecordLocked() error {
	if _, err := io.ReadFull(c.Conn, c.header[:]); err != nil {
		return err
	}

	length := int(c.header[0])<<24 | int(c.header[1])<<16 | int(c.header[2])<<8 | int(c.header[3])
	if length <= 0 || length > p2pMaxRecordSize+64 {
		// A hostile length prefix must not become a huge allocation.
		return fmt.Errorf("peer announced an invalid record length of %d bytes", length)
	}

	sealed := make([]byte, length)
	if _, err := io.ReadFull(c.Conn, sealed); err != nil {
		return err
	}

	nonce := recordNonce(c.aead, c.recvSeq)
	plaintext, err := c.aead.Open(nil, nonce, sealed, nil)
	if err != nil {
		// Either the record was tampered with, or records arrived out of order.
		// Both are fatal for the session.
		return fmt.Errorf("failed to decrypt peer record %d: %w", c.recvSeq, err)
	}
	c.recvSeq++

	c.pending = plaintext
	return nil
}

// -----------------------------------------------------------------------------
// Handshake
// -----------------------------------------------------------------------------

// writeLine writes one newline-terminated handshake line.
func writeLine(conn net.Conn, format string, args ...interface{}) error {
	_, err := fmt.Fprintf(conn, format+"\n", args...)
	return err
}

// b64 encodes handshake fields, which are binary and must survive a line protocol.
func b64(data []byte) string { return base64.StdEncoding.EncodeToString(data) }

// randomNonce returns a fresh handshake challenge.
func randomNonce() ([]byte, error) {
	nonce := make([]byte, p2pNonceSize)
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("failed to generate handshake nonce: %w", err)
	}
	return nonce, nil
}

// serverHandshake authenticates an inbound peer and establishes a session.
func (p *P2P) serverHandshake(conn net.Conn) (*secureConn, *authenticatedPeer, error) {
	identity := p.Identity()
	if identity == nil {
		return nil, nil, errors.New("this node has no identity; cannot authenticate peers")
	}

	if err := conn.SetDeadline(time.Now().Add(p2pHandshakeTimeout)); err != nil {
		return nil, nil, fmt.Errorf("set handshake deadline: %w", err)
	}
	//nolint:errcheck // clearing a deadline on a connection being closed
	defer func() { _ = conn.SetDeadline(time.Time{}) }()

	reader := bufio.NewReader(io.LimitReader(conn, maxP2PMessageSize))

	// 1. HELLO <version>
	hello, err := readLimitedLine(reader)
	if err != nil {
		return nil, nil, fmt.Errorf("receive HELLO: %w", err)
	}
	fields := strings.Fields(strings.TrimSpace(hello))
	if len(fields) < 1 || fields[0] != "HELLO" {
		return nil, nil, fmt.Errorf("unexpected greeting: %q", strings.TrimSpace(hello))
	}
	if len(fields) < 2 || fields[1] != p2pProtocolVersion {
		version := "(none)"
		if len(fields) > 1 {
			version = fields[1]
		}
		return nil, nil, fmt.Errorf("peer speaks protocol %s, this node speaks %s", version, p2pProtocolVersion)
	}

	// 2. ACK with our identity, a fresh ephemeral key, and a challenge.
	serverNonce, err := randomNonce()
	if err != nil {
		return nil, nil, err
	}
	serverEphemeral, err := newEphemeralKeyPair()
	if err != nil {
		return nil, nil, err
	}
	if err := writeLine(conn, "ACK %s %s %s %s %d",
		identity.NodeID, b64([]byte(identity.PublicPEM)), b64(serverEphemeral.public),
		b64(serverNonce), time.Now().Unix()); err != nil {
		return nil, nil, fmt.Errorf("send ACK: %w", err)
	}

	// 3. AUTH from the client.
	authLine, err := readLimitedLine(reader)
	if err != nil {
		return nil, nil, fmt.Errorf("receive AUTH: %w", err)
	}
	authFields := strings.Fields(strings.TrimSpace(authLine))
	if len(authFields) < 6 || authFields[0] != "AUTH" {
		return nil, nil, fmt.Errorf("malformed AUTH from peer")
	}

	peerID := authFields[1]
	peerPublicPEM, err := decodeB64Field(authFields[2], "public key")
	if err != nil {
		return nil, nil, err
	}
	clientEphemeral, err := decodeB64Bytes(authFields[3], "ephemeral key")
	if err != nil {
		return nil, nil, err
	}
	clientNonce, err := decodeB64Bytes(authFields[4], "nonce")
	if err != nil {
		return nil, nil, err
	}
	signature, err := decodeB64Bytes(authFields[5], "signature")
	if err != nil {
		return nil, nil, err
	}
	peerAddress := ""
	if len(authFields) > 6 {
		if decoded, err := decodeB64Field(authFields[6], "address"); err == nil {
			peerAddress = decoded
		}
	}

	// Validate the point before it reaches ECDH, and before spending a signature
	// verification on it.
	if _, err := parseEphemeralPublicKey(clientEphemeral); err != nil {
		return nil, nil, err
	}

	if len(clientNonce) != p2pNonceSize {
		return nil, nil, fmt.Errorf("peer nonce is %d bytes, expected %d", len(clientNonce), p2pNonceSize)
	}
	if !p.nonces.use(b64(clientNonce)) {
		return nil, nil, ErrHandshakeReplay
	}

	if err := p.verifyPeerClaim(peerID, peerPublicPEM); err != nil {
		return nil, nil, err
	}

	// The transcript covers the ephemeral keys, so a machine-in-the-middle cannot
	// substitute its own without invalidating this signature.
	transcript := handshakeTranscript(peerID, identity.NodeID, clientNonce, serverNonce,
		clientEphemeral, serverEphemeral.public)
	if err := VerifyPeerSignature(peerPublicPEM, transcript, signature); err != nil {
		if p.chain != nil {
			p.chain.Metrics().Inc("peer_auth_failed")
		}
		return nil, nil, fmt.Errorf("peer %s failed authentication: %w", peerID, err)
	}

	// 4. Prove our own identity in return, so the client knows who it reached.
	ourTranscript := handshakeTranscript(identity.NodeID, peerID, serverNonce, clientNonce,
		serverEphemeral.public, clientEphemeral)
	ourSignature, err := identity.Sign(ourTranscript)
	if err != nil {
		return nil, nil, err
	}
	if err := writeLine(conn, "OK %s", b64(ourSignature)); err != nil {
		return nil, nil, fmt.Errorf("send OK: %w", err)
	}

	key, err := deriveSessionKey(serverEphemeral, clientEphemeral, clientNonce, serverNonce)
	if err != nil {
		return nil, nil, err
	}
	// Drop the ephemeral private key. Nothing else may derive this session key
	// from here on, which is what forward secrecy means in practice.
	serverEphemeral.private = nil

	secure, err := newSecureConn(conn, key)
	if err != nil {
		return nil, nil, err
	}

	// Anything the client buffered after AUTH would be lost by switching readers,
	// but the protocol has the client wait for OK before sending, so there is
	// nothing in flight.
	if reader.Buffered() > 0 {
		return nil, nil, errors.New("peer sent data before the handshake completed")
	}

	if p.chain != nil {
		p.chain.Metrics().Inc("peers_connected")
	}
	return secure, &authenticatedPeer{NodeID: peerID, PublicPEM: peerPublicPEM, Address: peerAddress}, nil
}

// clientHandshakeAuthenticated authenticates an outbound connection.
func (p *P2P) clientHandshakeAuthenticated(conn net.Conn) (*secureConn, *authenticatedPeer, error) {
	identity := p.Identity()
	if identity == nil {
		return nil, nil, errors.New("this node has no identity; cannot authenticate to peers")
	}

	if err := conn.SetDeadline(time.Now().Add(p2pHandshakeTimeout)); err != nil {
		return nil, nil, fmt.Errorf("set handshake deadline: %w", err)
	}
	//nolint:errcheck // clearing a deadline on a connection being closed
	defer func() { _ = conn.SetDeadline(time.Time{}) }()

	reader := bufio.NewReader(io.LimitReader(conn, maxP2PMessageSize))

	// 1. HELLO
	if err := writeLine(conn, "HELLO %s", p2pProtocolVersion); err != nil {
		return nil, nil, fmt.Errorf("send HELLO: %w", err)
	}

	// 2. ACK
	ackLine, err := readLimitedLine(reader)
	if err != nil {
		return nil, nil, fmt.Errorf("receive ACK: %w", err)
	}
	ackFields := strings.Fields(strings.TrimSpace(ackLine))
	if len(ackFields) < 6 || ackFields[0] != "ACK" {
		return nil, nil, fmt.Errorf("malformed ACK from peer")
	}

	serverID := ackFields[1]
	serverPublicPEM, err := decodeB64Field(ackFields[2], "public key")
	if err != nil {
		return nil, nil, err
	}
	serverEphemeral, err := decodeB64Bytes(ackFields[3], "ephemeral key")
	if err != nil {
		return nil, nil, err
	}
	serverNonce, err := decodeB64Bytes(ackFields[4], "nonce")
	if err != nil {
		return nil, nil, err
	}
	timestamp, err := strconv.ParseInt(ackFields[5], 10, 64)
	if err != nil {
		return nil, nil, fmt.Errorf("malformed handshake timestamp: %w", err)
	}

	if _, err := parseEphemeralPublicKey(serverEphemeral); err != nil {
		return nil, nil, err
	}

	// Freshness: an old recorded ACK must not be usable to impersonate a server.
	if skew := time.Since(time.Unix(timestamp, 0)); skew > p2pMaxClockSkew || skew < -p2pMaxClockSkew {
		return nil, nil, fmt.Errorf("peer handshake is %s out of date", skew.Round(time.Second))
	}
	if len(serverNonce) != p2pNonceSize {
		return nil, nil, fmt.Errorf("peer nonce is %d bytes, expected %d", len(serverNonce), p2pNonceSize)
	}
	if !p.nonces.use(b64(serverNonce)) {
		return nil, nil, ErrHandshakeReplay
	}

	if err := p.verifyPeerClaim(serverID, serverPublicPEM); err != nil {
		return nil, nil, err
	}

	// 3. AUTH: prove who we are, and bind our ephemeral key to that identity.
	clientNonce, err := randomNonce()
	if err != nil {
		return nil, nil, err
	}
	clientEphemeral, err := newEphemeralKeyPair()
	if err != nil {
		return nil, nil, err
	}

	transcript := handshakeTranscript(identity.NodeID, serverID, clientNonce, serverNonce,
		clientEphemeral.public, serverEphemeral)
	signature, err := identity.Sign(transcript)
	if err != nil {
		return nil, nil, err
	}

	if err := writeLine(conn, "AUTH %s %s %s %s %s %s",
		identity.NodeID, b64([]byte(identity.PublicPEM)), b64(clientEphemeral.public),
		b64(clientNonce), b64(signature), b64([]byte(p.selfAddressSnapshot()))); err != nil {
		return nil, nil, fmt.Errorf("send AUTH: %w", err)
	}

	// 4. OK: check the server proved itself too.
	okLine, err := readLimitedLine(reader)
	if err != nil {
		return nil, nil, fmt.Errorf("receive OK: %w", err)
	}
	okFields := strings.Fields(strings.TrimSpace(okLine))
	if len(okFields) < 2 || okFields[0] != "OK" {
		return nil, nil, fmt.Errorf("peer did not complete the handshake: %q", strings.TrimSpace(okLine))
	}
	serverSignature, err := decodeB64Bytes(okFields[1], "signature")
	if err != nil {
		return nil, nil, err
	}

	serverTranscript := handshakeTranscript(serverID, identity.NodeID, serverNonce, clientNonce,
		serverEphemeral, clientEphemeral.public)
	if err := VerifyPeerSignature(serverPublicPEM, serverTranscript, serverSignature); err != nil {
		return nil, nil, fmt.Errorf("peer %s failed authentication: %w", serverID, err)
	}

	key, err := deriveSessionKey(clientEphemeral, serverEphemeral, clientNonce, serverNonce)
	if err != nil {
		return nil, nil, err
	}
	// Drop the ephemeral private key: from here on nothing can re-derive this
	// session key, including this node's own identity key.
	clientEphemeral.private = nil

	secure, err := newSecureConn(conn, key)
	if err != nil {
		return nil, nil, err
	}

	if reader.Buffered() > 0 {
		return nil, nil, errors.New("peer sent data before the handshake completed")
	}

	return secure, &authenticatedPeer{NodeID: serverID, PublicPEM: serverPublicPEM}, nil
}

// verifyPeerClaim checks a peer's claimed ID against the key it presented, and
// against the allowlist when one is configured.
func (p *P2P) verifyPeerClaim(claimedID, publicPEM string) error {
	derived, err := PeerIDFromPublicPEM(publicPEM)
	if err != nil {
		return fmt.Errorf("peer public key is unusable: %w", err)
	}

	// The ID is the hash of the key, so this is what stops a peer borrowing
	// someone else's identity.
	if derived != claimedID {
		return fmt.Errorf("peer claims ID %s but its key hashes to %s", claimedID, derived)
	}

	if !p.peerAllowed(claimedID) {
		return fmt.Errorf("%w: %s", ErrPeerNotAllowed, claimedID)
	}
	return nil
}

// decodeB64Field decodes a base64 handshake field as a string.
func decodeB64Field(encoded, what string) (string, error) {
	decoded, err := decodeB64Bytes(encoded, what)
	if err != nil {
		return "", err
	}
	return string(decoded), nil
}

// decodeB64Bytes decodes a base64 handshake field.
func decodeB64Bytes(encoded, what string) ([]byte, error) {
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("malformed %s in handshake: %w", what, err)
	}
	return decoded, nil
}
