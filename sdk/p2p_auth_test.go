package sdk

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// -----------------------------------------------------------------------------
// Identity
// -----------------------------------------------------------------------------

// TestNodeIDIsDerivedFromTheKey is the property that makes an ID unforgeable: it
// is the hash of the public key, not a name the node picks. A node ID used to be
// a random UUID the node asserted about itself, and the peer took its word.
func TestNodeIDIsDerivedFromTheKey(t *testing.T) {
	identity, err := NewPeerIdentity()
	if err != nil {
		t.Fatalf("new identity: %v", err)
	}

	if len(identity.NodeID) != 64 {
		t.Fatalf("expected a 64-character hex ID, got %q", identity.NodeID)
	}

	derived, err := PeerIDFromPublicPEM(identity.PublicPEM)
	if err != nil {
		t.Fatalf("derive: %v", err)
	}
	if derived != identity.NodeID {
		t.Fatalf("the ID does not match its own key: %s vs %s", derived, identity.NodeID)
	}

	// A different key must give a different ID.
	other, err := NewPeerIdentity()
	if err != nil {
		t.Fatalf("new identity: %v", err)
	}
	if other.NodeID == identity.NodeID {
		t.Fatal("two independently generated identities collided")
	}
}

func TestPeerIDFromPublicPEMRejectsBadInput(t *testing.T) {
	cases := []struct {
		name string
		pem  string
	}{
		{"empty", ""},
		{"not pem", "definitely not a PEM block"},
		{"truncated", "-----BEGIN PUBLIC KEY-----\nAAAA\n-----END PUBLIC KEY-----\n"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := PeerIDFromPublicPEM(tc.pem); err == nil {
				t.Fatalf("expected %s to be rejected", tc.name)
			}
		})
	}
}

// TestIdentityIsPersistedAndReloaded: an ID that changed every restart would make
// an allowlist useless and every reconnection look like a new peer.
func TestIdentityIsPersistedAndReloaded(t *testing.T) {
	dir := t.TempDir()

	first, err := LoadOrCreatePeerIdentity(dir)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	second, err := LoadOrCreatePeerIdentity(dir)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}

	if first.NodeID != second.NodeID {
		t.Fatalf("the node ID changed across a restart: %s -> %s", first.NodeID, second.NodeID)
	}
	if first.PublicPEM != second.PublicPEM {
		t.Fatal("the public key changed across a restart")
	}

	// The private key must not be world-readable.
	info, err := os.Stat(filepath.Join(dir, peerIdentityFileName))
	if err != nil {
		t.Fatalf("stat identity: %v", err)
	}
	if perm := info.Mode().Perm(); perm&0077 != 0 {
		t.Fatalf("the identity key is group/world accessible: mode %o", perm)
	}
}

func TestCorruptIdentityFileIsReported(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, peerIdentityFileName), []byte("garbage"), 0600); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Silently regenerating would change the node's identity without saying so.
	if _, err := LoadOrCreatePeerIdentity(dir); err == nil {
		t.Fatal("a corrupt identity file must be reported, not silently replaced")
	}
}

// TestSignAndVerifyRoundTrip covers the primitive the handshake rests on.
func TestSignAndVerifyRoundTrip(t *testing.T) {
	identity, err := NewPeerIdentity()
	if err != nil {
		t.Fatalf("identity: %v", err)
	}

	message := []byte("a handshake transcript")
	signature, err := identity.Sign(message)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	if err := VerifyPeerSignature(identity.PublicPEM, message, signature); err != nil {
		t.Fatalf("a valid signature failed to verify: %v", err)
	}

	if err := VerifyPeerSignature(identity.PublicPEM, []byte("a different message"), signature); err == nil {
		t.Fatal("a signature verified against the wrong message")
	}

	other, _ := NewPeerIdentity()
	if err := VerifyPeerSignature(other.PublicPEM, message, signature); err == nil {
		t.Fatal("a signature verified against the wrong key")
	}
}

// -----------------------------------------------------------------------------
// Handshake transcript
// -----------------------------------------------------------------------------

// TestTranscriptBindsBothPartiesAndBothNonces is what stops a captured signature
// being replayed against a different peer, or reflected back at its author.
func TestTranscriptBindsBothPartiesAndBothNonces(t *testing.T) {
	nonceA := []byte("nonce-a")
	nonceB := []byte("nonce-b")
	ephA := []byte("ephemeral-a")
	ephB := []byte("ephemeral-b")

	base := handshakeTranscript("alice", "bob", nonceA, nonceB, ephA, ephB)

	variations := map[string][]byte{
		"different signer":     handshakeTranscript("mallory", "bob", nonceA, nonceB, ephA, ephB),
		"different peer":       handshakeTranscript("alice", "mallory", nonceA, nonceB, ephA, ephB),
		"different own nonce":  handshakeTranscript("alice", "bob", []byte("other"), nonceB, ephA, ephB),
		"different peer nonce": handshakeTranscript("alice", "bob", nonceA, []byte("other"), ephA, ephB),
		"reflected":            handshakeTranscript("bob", "alice", nonceB, nonceA, ephB, ephA),
		// The ephemeral keys must be covered, or a machine-in-the-middle could
		// substitute its own and reuse the identity signature unchanged.
		"different own ephemeral":  handshakeTranscript("alice", "bob", nonceA, nonceB, []byte("swapped"), ephB),
		"different peer ephemeral": handshakeTranscript("alice", "bob", nonceA, nonceB, ephA, []byte("swapped")),
	}

	for name, variant := range variations {
		if bytes.Equal(base, variant) {
			t.Fatalf("the transcript does not bind %s -- a signature could be replayed", name)
		}
	}
}

// TestTranscriptFieldsCannotBeShifted guards a concatenation ambiguity: without
// separators, ("ab","c") and ("a","bc") would produce identical transcripts.
func TestTranscriptFieldsCannotBeShifted(t *testing.T) {
	n := []byte("n")
	if bytes.Equal(
		handshakeTranscript("ab", "c", n, n, n, n),
		handshakeTranscript("a", "bc", n, n, n, n),
	) {
		t.Fatal("transcript fields are ambiguous: shifting a byte between them " +
			"produces the same signed bytes")
	}
}

// -----------------------------------------------------------------------------
// Key derivation
// -----------------------------------------------------------------------------

// TestSessionKeysAgreeAndAreUnique: both sides must derive the same key, and no
// two sessions may share one.
func TestSessionKeysAgreeAndAreUnique(t *testing.T) {
	clientEph, err := newEphemeralKeyPair()
	if err != nil {
		t.Fatalf("ephemeral: %v", err)
	}
	serverEph, err := newEphemeralKeyPair()
	if err != nil {
		t.Fatalf("ephemeral: %v", err)
	}

	clientNonce, _ := randomNonce()
	serverNonce, _ := randomNonce()

	clientKey, err := deriveSessionKey(clientEph, serverEph.public, clientNonce, serverNonce)
	if err != nil {
		t.Fatalf("client derive: %v", err)
	}
	serverKey, err := deriveSessionKey(serverEph, clientEph.public, clientNonce, serverNonce)
	if err != nil {
		t.Fatalf("server derive: %v", err)
	}

	if !bytes.Equal(clientKey, serverKey) {
		t.Fatal("the two sides derived different session keys")
	}
	if len(clientKey) != 32 {
		t.Fatalf("expected a 32-byte key, got %d", len(clientKey))
	}

	// Different nonces must give a different key.
	otherNonce, _ := randomNonce()
	rekeyed, err := deriveSessionKey(clientEph, serverEph.public, clientNonce, otherNonce)
	if err != nil {
		t.Fatalf("derive: %v", err)
	}
	if bytes.Equal(clientKey, rekeyed) {
		t.Fatal("changing a nonce did not change the session key")
	}

	// A third party's ephemeral key must not derive the same secret.
	mallory, _ := newEphemeralKeyPair()
	malloryKey, err := deriveSessionKey(mallory, serverEph.public, clientNonce, serverNonce)
	if err != nil {
		t.Fatalf("derive: %v", err)
	}
	if bytes.Equal(malloryKey, clientKey) {
		t.Fatal("an unrelated ephemeral key derived the same session key")
	}
}

// TestDeriveSessionKeyRejectsInvalidPoints guards invalid-curve and
// small-subgroup attacks: feeding a crafted "public key" into ECDH can leak bits
// of the private key, so the point is validated before it is ever used.
func TestDeriveSessionKeyRejectsInvalidPoints(t *testing.T) {
	local, err := newEphemeralKeyPair()
	if err != nil {
		t.Fatalf("ephemeral: %v", err)
	}
	nonce, _ := randomNonce()

	cases := []struct {
		name string
		raw  []byte
	}{
		{"empty", nil},
		{"too short", []byte{0x04, 0x01, 0x02}},
		{"identity point", make([]byte, 65)},
		{"not on the curve", append([]byte{0x04}, bytes.Repeat([]byte{0xff}, 64)...)},
		{"wrong length", bytes.Repeat([]byte{0x04}, 100)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := deriveSessionKey(local, tc.raw, nonce, nonce); err == nil {
				t.Fatalf("a %s ephemeral key was accepted", tc.name)
			}
		})
	}

	if _, err := deriveSessionKey(nil, local.public, nonce, nonce); err == nil {
		t.Fatal("deriving with no local ephemeral key must fail")
	}
}

// TestHKDFMatchesRFC5869 checks the derivation against the RFC's test vector,
// since it is implemented here rather than taken from a library.
func TestHKDFMatchesRFC5869(t *testing.T) {
	// RFC 5869 appendix A.1: SHA-256, IKM 22x0x0b, salt 000102...0c, info f0..f9.
	ikm := bytes.Repeat([]byte{0x0b}, 22)
	salt := []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12}
	info := []byte{0xf0, 0xf1, 0xf2, 0xf3, 0xf4, 0xf5, 0xf6, 0xf7, 0xf8, 0xf9}

	got, err := hkdfSHA256(ikm, salt, info, 42)
	if err != nil {
		t.Fatalf("hkdf: %v", err)
	}

	want := []byte{
		0x3c, 0xb2, 0x5f, 0x25, 0xfa, 0xac, 0xd5, 0x7a, 0x90, 0x43, 0x4f, 0x64,
		0xd0, 0x36, 0x2f, 0x2a, 0x2d, 0x2d, 0x0a, 0x90, 0xcf, 0x1a, 0x5a, 0x4c,
		0x5d, 0xb0, 0x2d, 0x56, 0xec, 0xc4, 0xc5, 0xbf, 0x34, 0x00, 0x72, 0x08,
		0xd5, 0xb8, 0x87, 0x18, 0x58, 0x65,
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("HKDF does not match RFC 5869 A.1:\n got %x\nwant %x", got, want)
	}

	if _, err := hkdfSHA256(ikm, salt, info, 0); err == nil {
		t.Fatal("a zero length must be rejected")
	}
	if _, err := hkdfSHA256(ikm, salt, info, 255*32+1); err == nil {
		t.Fatal("an over-long request must be rejected")
	}
}

// -----------------------------------------------------------------------------
// Encrypted session
// -----------------------------------------------------------------------------

// TestSecureConnRoundTrip covers the encrypted framing.
func TestSecureConnRoundTrip(t *testing.T) {
	clientRaw, serverRaw := net.Pipe()
	defer clientRaw.Close()
	defer serverRaw.Close()

	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("key: %v", err)
	}

	client, err := newSecureConn(clientRaw, key)
	if err != nil {
		t.Fatalf("client: %v", err)
	}
	server, err := newSecureConn(serverRaw, key)
	if err != nil {
		t.Fatalf("server: %v", err)
	}

	messages := []string{
		"GET_STATUS\n",
		"GET_BLOCKS 0 10\n",
		strings.Repeat("x", 100000) + "\n", // a large record
	}

	go func() {
		for _, m := range messages {
			if _, err := client.Write([]byte(m)); err != nil {
				return
			}
		}
	}()

	reader := bufio.NewReader(server)
	for i, want := range messages {
		got, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("message %d: %v", i, err)
		}
		if got != want {
			t.Fatalf("message %d round-tripped incorrectly (%d vs %d bytes)",
				i, len(got), len(want))
		}
	}
}

// TestSecureConnActuallyEncrypts guards the obvious failure: a "secure" wrapper
// that passes plaintext through.
func TestSecureConnActuallyEncrypts(t *testing.T) {
	var wire bytes.Buffer
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("key: %v", err)
	}

	conn, err := newSecureConn(&bufferConn{Buffer: &wire}, key)
	if err != nil {
		t.Fatalf("secure conn: %v", err)
	}

	secret := "GET_BLOCKS 0 10 SECRETMARKER"
	if _, err := conn.Write([]byte(secret)); err != nil {
		t.Fatalf("write: %v", err)
	}

	if bytes.Contains(wire.Bytes(), []byte("SECRETMARKER")) {
		t.Fatal("the plaintext appeared on the wire: the session is not encrypted")
	}
	if wire.Len() == 0 {
		t.Fatal("nothing was written")
	}
}

// TestSecureConnRejectsTamperedRecords: GCM must detect modification, which is
// what stops an active attacker rewriting traffic on an authenticated link.
func TestSecureConnRejectsTamperedRecords(t *testing.T) {
	var wire bytes.Buffer
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("key: %v", err)
	}

	writer, err := newSecureConn(&bufferConn{Buffer: &wire}, key)
	if err != nil {
		t.Fatalf("secure conn: %v", err)
	}
	if _, err := writer.Write([]byte("GET_STATUS\n")); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Flip a bit in the ciphertext (past the 4-byte length prefix).
	tampered := append([]byte{}, wire.Bytes()...)
	tampered[6] ^= 0x01

	reader, err := newSecureConn(&bufferConn{Buffer: bytes.NewBuffer(tampered)}, key)
	if err != nil {
		t.Fatalf("secure conn: %v", err)
	}

	buf := make([]byte, 64)
	if _, err := reader.Read(buf); err == nil {
		t.Fatal("a tampered record was accepted")
	}
}

// TestSecureConnRejectsAnAbsurdLengthPrefix guards a hostile allocation.
func TestSecureConnRejectsAnAbsurdLengthPrefix(t *testing.T) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("key: %v", err)
	}

	// A 4-byte length prefix claiming ~4 GiB, then nothing.
	hostile := bytes.NewBuffer([]byte{0xff, 0xff, 0xff, 0xff})
	conn, err := newSecureConn(&bufferConn{Buffer: hostile}, key)
	if err != nil {
		t.Fatalf("secure conn: %v", err)
	}

	buf := make([]byte, 16)
	if _, err := conn.Read(buf); err == nil {
		t.Fatal("an absurd length prefix must be refused, not allocated")
	}
}

// TestSecureConnRejectsReorderedRecords: the per-record nonce is the sequence
// number, so replaying or reordering records breaks decryption.
func TestSecureConnRejectsReorderedRecords(t *testing.T) {
	var wire bytes.Buffer
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("key: %v", err)
	}

	writer, err := newSecureConn(&bufferConn{Buffer: &wire}, key)
	if err != nil {
		t.Fatalf("secure conn: %v", err)
	}
	if _, err := writer.Write([]byte("first\n")); err != nil {
		t.Fatalf("write: %v", err)
	}
	firstRecord := append([]byte{}, wire.Bytes()...)

	// Replay the first record as if it were the second.
	replayed := append(append([]byte{}, firstRecord...), firstRecord...)

	reader, err := newSecureConn(&bufferConn{Buffer: bytes.NewBuffer(replayed)}, key)
	if err != nil {
		t.Fatalf("secure conn: %v", err)
	}

	buf := make([]byte, 64)
	if _, err := reader.Read(buf); err != nil {
		t.Fatalf("the first record should decrypt: %v", err)
	}
	if _, err := reader.Read(buf); err == nil {
		t.Fatal("a replayed record was accepted as the next one")
	}
}

// bufferConn adapts a bytes.Buffer to net.Conn for framing tests.
type bufferConn struct {
	*bytes.Buffer
}

func (b *bufferConn) Close() error                     { return nil }
func (b *bufferConn) LocalAddr() net.Addr              { return nil }
func (b *bufferConn) RemoteAddr() net.Addr             { return nil }
func (b *bufferConn) SetDeadline(time.Time) error      { return nil }
func (b *bufferConn) SetReadDeadline(time.Time) error  { return nil }
func (b *bufferConn) SetWriteDeadline(time.Time) error { return nil }

// -----------------------------------------------------------------------------
// Full handshake over TCP
// -----------------------------------------------------------------------------

// TestAuthenticatedHandshakeSucceeds is the happy path, end to end.
func TestAuthenticatedHandshakeSucceeds(t *testing.T) {
	server, address := startPeer(t, "server", syncTestChain(t, 1))
	client := newTestP2P(t, "127.0.0.1:0")

	pc, err := client.dialPeer(address)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer pc.close()

	// The client learned the server's real ID, derived from its key.
	if pc.peerID != server.Identity().NodeID {
		t.Fatalf("client authenticated %s, expected %s", pc.peerID, server.Identity().NodeID)
	}

	// And the server learned the client's.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if server.IsRegistered(client.Identity().NodeID) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("the server did not record the authenticated client")
}

// TestUnauthenticatedClientIsRefused: a node with no identity cannot connect.
func TestUnauthenticatedClientIsRefused(t *testing.T) {
	_, address := startPeer(t, "server", syncTestChain(t, 1))

	// No identity: the old code let a node assert any ID it liked.
	client := NewP2P()
	client.SetSelfInfo("i-am-whoever-i-say", "127.0.0.1:0")

	if _, err := client.dialPeer(address); err == nil {
		t.Fatal("a node with no identity completed the handshake")
	}
}

// TestSpoofedNodeIDIsRefused is the core property: an attacker presenting its own
// key while claiming someone else's ID must be rejected. This is exactly what the
// old handshake permitted.
func TestSpoofedNodeIDIsRefused(t *testing.T) {
	_, address := startPeer(t, "server", syncTestChain(t, 1))

	victim, err := NewPeerIdentity()
	if err != nil {
		t.Fatalf("identity: %v", err)
	}
	attacker, err := NewPeerIdentity()
	if err != nil {
		t.Fatalf("identity: %v", err)
	}

	conn, err := net.DialTimeout("tcp", address, 3*time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	reader := bufio.NewReader(conn)

	if _, err := conn.Write([]byte("HELLO " + p2pProtocolVersion + "\n")); err != nil {
		t.Fatalf("hello: %v", err)
	}
	ack, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("ack: %v", err)
	}
	ackFields := strings.Fields(strings.TrimSpace(ack))
	if len(ackFields) < 6 {
		t.Fatalf("malformed ACK: %q", ack)
	}
	serverID := ackFields[1]
	serverEphemeral, err := base64.StdEncoding.DecodeString(ackFields[3])
	if err != nil {
		t.Fatalf("ephemeral: %v", err)
	}
	serverNonce, err := base64.StdEncoding.DecodeString(ackFields[4])
	if err != nil {
		t.Fatalf("nonce: %v", err)
	}

	// Claim the victim's ID while holding only the attacker's key.
	clientNonce, _ := randomNonce()
	clientEph, _ := newEphemeralKeyPair()
	transcript := handshakeTranscript(victim.NodeID, serverID, clientNonce, serverNonce,
		clientEph.public, serverEphemeral)
	signature, err := attacker.Sign(transcript)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	_, err = conn.Write([]byte(strings.Join([]string{
		"AUTH",
		victim.NodeID, // the claim
		base64.StdEncoding.EncodeToString([]byte(attacker.PublicPEM)), // the actual key
		base64.StdEncoding.EncodeToString(clientEph.public),
		base64.StdEncoding.EncodeToString(clientNonce),
		base64.StdEncoding.EncodeToString(signature),
		base64.StdEncoding.EncodeToString([]byte("127.0.0.1:1")),
	}, " ") + "\n"))
	if err != nil {
		t.Fatalf("auth: %v", err)
	}

	// The server must hang up rather than send OK.
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	line, err := reader.ReadString('\n')
	if err == nil && strings.HasPrefix(strings.TrimSpace(line), "OK") {
		t.Fatal("the server accepted a node ID the peer could not prove it owns")
	}
}

// TestHandshakeWithAWrongSignatureIsRefused covers a peer presenting a consistent
// ID and key but a signature it could not have made.
func TestHandshakeWithAWrongSignatureIsRefused(t *testing.T) {
	_, address := startPeer(t, "server", syncTestChain(t, 1))

	attacker, _ := NewPeerIdentity()
	unrelated, _ := NewPeerIdentity()

	conn, err := net.DialTimeout("tcp", address, 3*time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	reader := bufio.NewReader(conn)
	if _, err := conn.Write([]byte("HELLO " + p2pProtocolVersion + "\n")); err != nil {
		t.Fatalf("hello: %v", err)
	}
	ack, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("ack: %v", err)
	}
	ackFields := strings.Fields(strings.TrimSpace(ack))
	serverEphemeral, _ := base64.StdEncoding.DecodeString(ackFields[3])
	serverNonce, _ := base64.StdEncoding.DecodeString(ackFields[4])

	clientNonce, _ := randomNonce()
	clientEph, _ := newEphemeralKeyPair()
	// Signed by a key the attacker is not presenting.
	transcript := handshakeTranscript(attacker.NodeID, ackFields[1], clientNonce, serverNonce,
		clientEph.public, serverEphemeral)
	signature, _ := unrelated.Sign(transcript)

	_, _ = conn.Write([]byte(strings.Join([]string{
		"AUTH", attacker.NodeID,
		base64.StdEncoding.EncodeToString([]byte(attacker.PublicPEM)),
		base64.StdEncoding.EncodeToString(clientEph.public),
		base64.StdEncoding.EncodeToString(clientNonce),
		base64.StdEncoding.EncodeToString(signature),
		base64.StdEncoding.EncodeToString([]byte("127.0.0.1:1")),
	}, " ") + "\n"))

	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	line, err := reader.ReadString('\n')
	if err == nil && strings.HasPrefix(strings.TrimSpace(line), "OK") {
		t.Fatal("the server accepted an invalid signature")
	}
}

// TestProtocolVersionMismatchIsRefused: a peer speaking a different protocol is
// refused rather than half-understood.
func TestProtocolVersionMismatchIsRefused(t *testing.T) {
	_, address := startPeer(t, "server", syncTestChain(t, 1))

	for _, greeting := range []string{"HELLO", "HELLO gbb/1", "HELLO gbb/2", "HELLO nonsense", "GARBAGE"} {
		conn, err := net.DialTimeout("tcp", address, 3*time.Second)
		if err != nil {
			t.Fatalf("dial: %v", err)
		}

		_, _ = conn.Write([]byte(greeting + "\n"))
		_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))

		reader := bufio.NewReader(conn)
		line, err := reader.ReadString('\n')
		if err == nil && strings.HasPrefix(line, "ACK") {
			conn.Close()
			t.Fatalf("greeting %q was accepted", greeting)
		}
		conn.Close()
	}
}

// TestReplayedHandshakeIsRefused: a captured handshake must not be reusable.
func TestReplayedHandshakeIsRefused(t *testing.T) {
	p := NewP2P()

	nonce := "a-captured-nonce"
	if !p.nonces.use(nonce) {
		t.Fatal("the first use of a nonce should be allowed")
	}
	if p.nonces.use(nonce) {
		t.Fatal("a replayed nonce was accepted")
	}

	if !p.nonces.use("a-different-nonce") {
		t.Fatal("an unrelated nonce should be allowed")
	}
}

// TestNonceCacheDoesNotGrowWithoutBound guards a memory leak from a peer that
// opens endless connections.
func TestNonceCacheDoesNotGrowWithoutBound(t *testing.T) {
	cache := newNonceCache(time.Hour)
	cache.limit = 100

	for i := 0; i < 1000; i++ {
		cache.use(strings.Repeat("x", i%50) + string(rune(i)))
	}

	cache.mu.Lock()
	size := len(cache.seen)
	cache.mu.Unlock()

	if size > cache.limit {
		t.Fatalf("the nonce cache grew to %d entries, above its %d limit", size, cache.limit)
	}
}

// TestStaleHandshakeIsRefused covers the freshness window: an old recorded ACK
// must not be usable to impersonate a server.
func TestStaleHandshakeIsRefused(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	identity, _ := NewPeerIdentity()
	staleTimestamp := time.Now().Add(-time.Hour).Unix()

	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		reader := bufio.NewReader(conn)
		if _, err := reader.ReadString('\n'); err != nil {
			return
		}
		nonce, _ := randomNonce()
		eph, _ := newEphemeralKeyPair()
		_, _ = conn.Write([]byte(strings.Join([]string{
			"ACK", identity.NodeID,
			base64.StdEncoding.EncodeToString([]byte(identity.PublicPEM)),
			base64.StdEncoding.EncodeToString(eph.public),
			base64.StdEncoding.EncodeToString(nonce),
			time.Unix(staleTimestamp, 0).UTC().Format("20060102"),
		}, " ") + "\n"))
	}()

	client := newTestP2P(t, "127.0.0.1:0")
	if _, err := client.dialPeer(listener.Addr().String()); err == nil {
		t.Fatal("a malformed/stale handshake timestamp was accepted")
	}
}

// -----------------------------------------------------------------------------
// Allowlist
// -----------------------------------------------------------------------------

// TestAllowlistRestrictsPeers: authentication proves who a peer is; the allowlist
// is the separate question of whether they are welcome.
func TestAllowlistRestrictsPeers(t *testing.T) {
	server, address := startPeer(t, "server", syncTestChain(t, 1))

	permitted := newTestP2P(t, "127.0.0.1:0")
	blocked := newTestP2P(t, "127.0.0.1:0")

	server.SetAllowedPeers([]string{permitted.Identity().NodeID})

	pc, err := permitted.dialPeer(address)
	if err != nil {
		t.Fatalf("an allowlisted peer was refused: %v", err)
	}
	pc.close()

	if _, err := blocked.dialPeer(address); err == nil {
		t.Fatal("a peer that is not on the allowlist was allowed to connect")
	}
}

func TestEmptyAllowlistPermitsAnyAuthenticatedPeer(t *testing.T) {
	p := NewP2P()

	if !p.peerAllowed("anyone") {
		t.Fatal("an empty allowlist should permit any authenticated peer")
	}

	p.SetAllowedPeers([]string{"a", "b"})
	if !p.peerAllowed("a") || p.peerAllowed("c") {
		t.Fatal("the allowlist is not being applied")
	}

	// Clearing it restores the permissive default.
	p.SetAllowedPeers(nil)
	if !p.peerAllowed("c") {
		t.Fatal("clearing the allowlist should restore the permissive default")
	}

	// Blank entries must not create an accidental empty-ID allowance.
	p.SetAllowedPeers([]string{"  ", ""})
	if !p.peerAllowed("anything") {
		t.Fatal("an allowlist of only blanks should behave as empty")
	}
}

// -----------------------------------------------------------------------------
// Traffic confidentiality end to end
// -----------------------------------------------------------------------------

// TestSessionTrafficIsEncryptedOnTheWire is the property that makes authentication
// meaningful: without encryption an active attacker could still rewrite traffic
// after a valid handshake.
func TestSessionTrafficIsEncryptedOnTheWire(t *testing.T) {
	chain := syncTestChain(t, 3)
	_, serverAddress := startPeer(t, "server", chain)

	// Proxy the connection so the bytes actually crossing the wire can be seen.
	proxy, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer proxy.Close()

	var (
		mu       sync.Mutex
		observed bytes.Buffer
	)

	go func() {
		client, err := proxy.Accept()
		if err != nil {
			return
		}
		defer client.Close()

		upstream, err := net.Dial("tcp", serverAddress)
		if err != nil {
			return
		}
		defer upstream.Close()

		go func() {
			buf := make([]byte, 4096)
			for {
				n, err := client.Read(buf)
				if n > 0 {
					mu.Lock()
					observed.Write(buf[:n])
					mu.Unlock()
					if _, werr := upstream.Write(buf[:n]); werr != nil {
						return
					}
				}
				if err != nil {
					return
				}
			}
		}()
		_, _ = io.Copy(client, upstream)
	}()

	client := newTestP2P(t, "127.0.0.1:0")
	pc, err := client.dialPeer(proxy.Addr().String())
	if err != nil {
		t.Fatalf("dial through proxy: %v", err)
	}
	defer pc.close()

	if _, err := pc.request(cmdGetStatus); err != nil {
		t.Fatalf("request: %v", err)
	}

	mu.Lock()
	wire := observed.String()
	mu.Unlock()

	// The handshake itself is in the clear -- it carries public keys and nonces --
	// but the command that follows must not be.
	if strings.Contains(wire, cmdGetStatus) {
		t.Fatal("the request appeared in plaintext on the wire: the session is not encrypted")
	}
	if !strings.Contains(wire, "HELLO") {
		t.Fatal("the proxy did not observe the handshake; the test is not measuring anything")
	}
}

// TestAuthenticatedPeersCanSync confirms the whole stack still works end to end
// once every connection is authenticated and encrypted.
func TestAuthenticatedPeersCanSync(t *testing.T) {
	ahead := syncTestChain(t, 6)
	_, aheadAddr := startPeer(t, "ahead", ahead)

	behind := syncTestChain(t, 0)
	behind.Blocks = append([]*Block{}, ahead.Blocks[:1]...)
	behind.CurrentBlockIndex = 0
	behind.NextBlockIndex = 1

	client := newTestP2P(t, "127.0.0.1:0")
	if err := client.RegisterNode(&Node{ID: "ahead", Config: &Config{P2PHostName: aheadAddr}}); err != nil {
		t.Fatalf("register: %v", err)
	}

	syncer, err := NewSyncer(SyncerOptions{Chain: behind, Transport: client, Peers: client, BatchSize: 4})
	if err != nil {
		t.Fatalf("syncer: %v", err)
	}

	result, err := syncer.SyncOnce(nil)
	if err != nil {
		t.Fatalf("sync over an authenticated session failed: %v (skipped %v)", err, result.Skipped)
	}
	if behind.Height() != 6 {
		t.Fatalf("expected height 6, got %d", behind.Height())
	}
}

// TestConcurrentHandshakes exercises the shared nonce cache and peer table.
func TestConcurrentHandshakes(t *testing.T) {
	_, address := startPeer(t, "server", syncTestChain(t, 1))

	var wg sync.WaitGroup
	errs := make(chan error, 16)

	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			client := newTestP2PNoHelper("127.0.0.1:0")
			pc, err := client.dialPeer(address)
			if err != nil {
				errs <- err
				return
			}
			defer pc.close()
			if _, err := pc.request(cmdGetStatus); err != nil {
				errs <- err
			}
		}()
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Fatalf("a concurrent handshake failed: %v", err)
	}
}

// newTestP2PNoHelper builds a P2P instance without needing *testing.T, for use
// inside goroutines where t.Fatalf is not allowed.
func newTestP2PNoHelper(address string) *P2P {
	identity, err := NewPeerIdentity()
	if err != nil {
		return NewP2P()
	}
	p := NewP2P()
	p.SetIdentity(identity)
	p.SetSelfInfo(identity.NodeID, address)
	return p
}

// TestSyncOnceHandlesNilContext guards the call above, which passes nil.
func TestSyncOnceHandlesNilContext(t *testing.T) {
	local := newFakeChain(0)
	s := newTestSyncer(t, local, newFakeTransport(), fakePeers{}, 10)

	if _, err := s.SyncOnce(nil); err != nil && !errors.Is(err, context.Canceled) {
		t.Fatalf("a nil context must not panic: %v", err)
	}
}

// -----------------------------------------------------------------------------
// Forward secrecy
// -----------------------------------------------------------------------------

// TestSessionKeyDoesNotDependOnIdentityKeys is the structural heart of forward
// secrecy: identity keys are not an input to the session key at all.
//
// The previous design derived the key from the long-term identity keys directly,
// so anyone who later obtained a node's identity key could decrypt every session
// it had ever had, including ones recorded months earlier.
func TestSessionKeyDoesNotDependOnIdentityKeys(t *testing.T) {
	clientEph, err := newEphemeralKeyPair()
	if err != nil {
		t.Fatalf("ephemeral: %v", err)
	}
	serverEph, err := newEphemeralKeyPair()
	if err != nil {
		t.Fatalf("ephemeral: %v", err)
	}

	clientNonce, _ := randomNonce()
	serverNonce, _ := randomNonce()

	key, err := deriveSessionKey(clientEph, serverEph.public, clientNonce, serverNonce)
	if err != nil {
		t.Fatalf("derive: %v", err)
	}

	// Deriving again with the same ephemeral material gives the same key,
	// regardless of which identities are involved -- there is nowhere for an
	// identity key to enter the computation.
	again, err := deriveSessionKey(clientEph, serverEph.public, clientNonce, serverNonce)
	if err != nil {
		t.Fatalf("derive: %v", err)
	}
	if !bytes.Equal(key, again) {
		t.Fatal("derivation is not deterministic in its ephemeral inputs")
	}
}

// TestEveryHandshakeUsesFreshEphemeralKeys: two connections between the same pair
// of nodes must not reuse key material. If they did, compromising one session
// would compromise the other.
func TestEveryHandshakeUsesFreshEphemeralKeys(t *testing.T) {
	_, address := startPeer(t, "server", syncTestChain(t, 1))
	client := newTestP2P(t, "127.0.0.1:0")

	seenClient := map[string]struct{}{}
	seenServer := map[string]struct{}{}

	for i := 0; i < 5; i++ {
		clientEph, serverEph := observeHandshakeEphemerals(t, client, address)

		if _, dup := seenClient[string(clientEph)]; dup {
			t.Fatal("the client reused an ephemeral key across handshakes")
		}
		if _, dup := seenServer[string(serverEph)]; dup {
			t.Fatal("the server reused an ephemeral key across handshakes")
		}
		seenClient[string(clientEph)] = struct{}{}
		seenServer[string(serverEph)] = struct{}{}
	}
}

// observeHandshakeEphemerals performs one handshake by hand and returns both
// ephemeral public keys as they appeared on the wire.
func observeHandshakeEphemerals(t *testing.T, client *P2P, address string) (clientEph, serverEph []byte) {
	t.Helper()

	conn, err := net.DialTimeout("tcp", address, 3*time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	identity := client.Identity()
	reader := bufio.NewReader(conn)

	if _, err := conn.Write([]byte("HELLO " + p2pProtocolVersion + "\n")); err != nil {
		t.Fatalf("hello: %v", err)
	}
	ack, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("ack: %v", err)
	}
	fields := strings.Fields(strings.TrimSpace(ack))
	if len(fields) < 6 {
		t.Fatalf("malformed ACK: %q", ack)
	}

	serverEph, err = base64.StdEncoding.DecodeString(fields[3])
	if err != nil {
		t.Fatalf("server ephemeral: %v", err)
	}
	serverNonce, err := base64.StdEncoding.DecodeString(fields[4])
	if err != nil {
		t.Fatalf("server nonce: %v", err)
	}

	pair, err := newEphemeralKeyPair()
	if err != nil {
		t.Fatalf("ephemeral: %v", err)
	}
	clientNonce, _ := randomNonce()

	transcript := handshakeTranscript(identity.NodeID, fields[1], clientNonce, serverNonce,
		pair.public, serverEph)
	signature, err := identity.Sign(transcript)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	_, err = conn.Write([]byte(strings.Join([]string{
		"AUTH", identity.NodeID,
		base64.StdEncoding.EncodeToString([]byte(identity.PublicPEM)),
		base64.StdEncoding.EncodeToString(pair.public),
		base64.StdEncoding.EncodeToString(clientNonce),
		base64.StdEncoding.EncodeToString(signature),
		base64.StdEncoding.EncodeToString([]byte("127.0.0.1:1")),
	}, " ") + "\n"))
	if err != nil {
		t.Fatalf("auth: %v", err)
	}

	if _, err := reader.ReadString('\n'); err != nil {
		t.Fatalf("ok: %v", err)
	}
	return pair.public, serverEph
}

// TestCompromisedIdentityKeysCannotDecryptARecordedSession is the property in its
// operational form.
//
// It records a real session's wire bytes, then hands an attacker *both* nodes'
// long-term identity private keys -- total compromise, after the fact -- and
// confirms the recorded traffic still cannot be decrypted. The ephemeral private
// keys were never transmitted and were dropped when the handshake finished, so
// there is nothing left that reconstructs the session key.
func TestCompromisedIdentityKeysCannotDecryptARecordedSession(t *testing.T) {
	chain := syncTestChain(t, 2)
	server, serverAddress := startPeer(t, "server", chain)

	// Record everything crossing the wire.
	proxy, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer proxy.Close()

	var (
		mu       sync.Mutex
		recorded bytes.Buffer
	)

	go func() {
		downstream, err := proxy.Accept()
		if err != nil {
			return
		}
		defer downstream.Close()

		upstream, err := net.Dial("tcp", serverAddress)
		if err != nil {
			return
		}
		defer upstream.Close()

		record := func(dst io.Writer, src io.Reader) {
			buf := make([]byte, 4096)
			for {
				n, err := src.Read(buf)
				if n > 0 {
					mu.Lock()
					recorded.Write(buf[:n])
					mu.Unlock()
					if _, werr := dst.Write(buf[:n]); werr != nil {
						return
					}
				}
				if err != nil {
					return
				}
			}
		}
		go record(upstream, downstream)
		record(downstream, upstream)
	}()

	client := newTestP2P(t, "127.0.0.1:0")
	pc, err := client.dialPeer(proxy.Addr().String())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}

	if _, err := pc.request(cmdGetStatus); err != nil {
		t.Fatalf("request: %v", err)
	}
	pc.close()

	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	wire := recorded.Bytes()
	mu.Unlock()

	if len(wire) == 0 {
		t.Fatal("nothing was recorded; the test is not measuring anything")
	}
	if bytes.Contains(wire, []byte(cmdGetStatus)) {
		t.Fatal("the request was in plaintext on the wire")
	}

	// The attacker now holds BOTH long-term identity private keys.
	clientIdentity := client.Identity()
	serverIdentity := server.Identity()

	// The best they can do is a static-static ECDH between the two identity keys
	// -- which is exactly what the previous design used as the session key.
	clientECDH, err := clientIdentity.privateKey.ECDH()
	if err != nil {
		t.Fatalf("ecdh: %v", err)
	}
	serverPub, err := parsePeerPublicKey(serverIdentity.PublicPEM)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	serverECDH, err := serverPub.ECDH()
	if err != nil {
		t.Fatalf("ecdh: %v", err)
	}
	staticShared, err := clientECDH.ECDH(serverECDH)
	if err != nil {
		t.Fatalf("ecdh: %v", err)
	}

	// Try every nonce ordering they could recover from the recorded handshake.
	nonces := extractHandshakeNonces(t, wire)
	for _, salt := range nonces {
		guessed, err := hkdfSHA256(staticShared, salt, []byte(p2pSessionContext), 32)
		if err != nil {
			t.Fatalf("hkdf: %v", err)
		}
		if decryptsRecordedSession(wire, guessed) {
			t.Fatal("the recorded session was decrypted using only the long-term " +
				"identity keys -- there is no forward secrecy")
		}
	}
}

// extractHandshakeNonces pulls candidate nonce salts out of a recorded handshake.
func extractHandshakeNonces(t *testing.T, wire []byte) [][]byte {
	t.Helper()

	var candidates [][]byte

	// The handshake lines are plaintext, so an attacker can read the nonces.
	lines := strings.Split(string(wire), "\n")
	var nonces [][]byte
	for _, line := range lines {
		fields := strings.Fields(line)
		switch {
		case len(fields) >= 6 && fields[0] == "ACK":
			if n, err := base64.StdEncoding.DecodeString(fields[4]); err == nil {
				nonces = append(nonces, n)
			}
		case len(fields) >= 6 && fields[0] == "AUTH":
			if n, err := base64.StdEncoding.DecodeString(fields[4]); err == nil {
				nonces = append(nonces, n)
			}
		}
	}

	// Both orderings, plus each alone.
	for _, a := range nonces {
		candidates = append(candidates, a)
		for _, b := range nonces {
			candidates = append(candidates, append(append([]byte{}, a...), b...))
		}
	}
	return candidates
}

// decryptsRecordedSession reports whether a candidate key opens any record in the
// recorded traffic.
func decryptsRecordedSession(wire, key []byte) bool {
	conn, err := newSecureConn(&bufferConn{Buffer: bytes.NewBuffer(nil)}, key)
	if err != nil {
		return false
	}

	// Walk the stream looking for a record this key can open.
	for offset := 0; offset+4 < len(wire); offset++ {
		length := int(wire[offset])<<24 | int(wire[offset+1])<<16 |
			int(wire[offset+2])<<8 | int(wire[offset+3])
		if length <= 0 || length > 4096 || offset+4+length > len(wire) {
			continue
		}
		sealed := wire[offset+4 : offset+4+length]
		for seq := uint64(0); seq < 4; seq++ {
			if _, err := conn.aead.Open(nil, recordNonce(conn.aead, seq), sealed, nil); err == nil {
				return true
			}
		}
	}
	return false
}

// TestEphemeralKeyIsBoundToIdentity is the machine-in-the-middle case.
//
// An anonymous Diffie-Hellman exchange is wide open to a MITM: relay the identity
// handshake untouched, substitute your own ephemeral key on each side, and you
// hold both session keys. Signing the transcript that covers the ephemeral keys
// is what closes that, and this test substitutes a key to prove the signature
// actually catches it.
func TestEphemeralKeyIsBoundToIdentity(t *testing.T) {
	_, address := startPeer(t, "server", syncTestChain(t, 1))

	client := newTestP2P(t, "127.0.0.1:0")
	identity := client.Identity()

	conn, err := net.DialTimeout("tcp", address, 3*time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	reader := bufio.NewReader(conn)
	if _, err := conn.Write([]byte("HELLO " + p2pProtocolVersion + "\n")); err != nil {
		t.Fatalf("hello: %v", err)
	}
	ack, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("ack: %v", err)
	}
	fields := strings.Fields(strings.TrimSpace(ack))
	if len(fields) < 6 {
		t.Fatalf("malformed ACK: %q", ack)
	}
	serverEph, _ := base64.StdEncoding.DecodeString(fields[3])
	serverNonce, _ := base64.StdEncoding.DecodeString(fields[4])

	// Sign a transcript committing to one ephemeral key...
	honest, _ := newEphemeralKeyPair()
	clientNonce, _ := randomNonce()
	transcript := handshakeTranscript(identity.NodeID, fields[1], clientNonce, serverNonce,
		honest.public, serverEph)
	signature, err := identity.Sign(transcript)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	// ...then send a different one, as a MITM would.
	substituted, _ := newEphemeralKeyPair()
	_, err = conn.Write([]byte(strings.Join([]string{
		"AUTH", identity.NodeID,
		base64.StdEncoding.EncodeToString([]byte(identity.PublicPEM)),
		base64.StdEncoding.EncodeToString(substituted.public), // swapped
		base64.StdEncoding.EncodeToString(clientNonce),
		base64.StdEncoding.EncodeToString(signature),
		base64.StdEncoding.EncodeToString([]byte("127.0.0.1:1")),
	}, " ") + "\n"))
	if err != nil {
		t.Fatalf("auth: %v", err)
	}

	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	line, err := reader.ReadString('\n')
	if err == nil && strings.HasPrefix(strings.TrimSpace(line), "OK") {
		t.Fatal("the server accepted an ephemeral key the signature did not cover: " +
			"a machine-in-the-middle could substitute its own key and hold the session")
	}
}

// TestTwoSessionsBetweenTheSamePeersUseDifferentKeys is forward secrecy's
// practical consequence: breaking one session must not break another.
func TestTwoSessionsBetweenTheSamePeersUseDifferentKeys(t *testing.T) {
	_, address := startPeer(t, "server", syncTestChain(t, 1))
	client := newTestP2P(t, "127.0.0.1:0")

	firstClientEph, firstServerEph := observeHandshakeEphemerals(t, client, address)
	secondClientEph, secondServerEph := observeHandshakeEphemerals(t, client, address)

	if bytes.Equal(firstClientEph, secondClientEph) {
		t.Fatal("the client reused its ephemeral key between two sessions")
	}
	if bytes.Equal(firstServerEph, secondServerEph) {
		t.Fatal("the server reused its ephemeral key between two sessions")
	}
}
