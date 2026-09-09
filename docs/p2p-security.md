# P2P Authentication & Encryption

Every peer connection is **mutually authenticated** and **encrypted**. A peer
proves it holds the key its node ID is derived from, and the same exchange
establishes the session key.

## 🎯 What this replaced

The old handshake was:

```
client → HELLO
server → ACK
client → {"id":"whatever-i-say","address":"..."}
server → OK
```

A node simply asserted an ID and the peer believed it. **Any host could claim any
identity** — including one already in use. The whole connection was plaintext, so
an active attacker could read and rewrite everything on it, and a passive one
could read every block, transaction and address in transit.

`TestSpoofedNodeIDIsRefused` is the direct regression test for that.

## 🔑 Node identity

A node ID is the **SHA-256 of its public key**:

```go
identity, _ := LoadOrCreatePeerIdentity(dataPath)
identity.NodeID   // 64 hex chars, = sha256(marshalled public key)
```

This makes IDs **self-certifying**. A peer's claimed ID is recomputed from the key
it presents, *before* any signature is checked, so an ID cannot be borrowed at
all — a mismatch is refused outright.

The key is an ECDSA P-256 keypair stored at `{DATA_PATH}/node_identity.pem`,
mode `0600`. It is persisted because an ID that changed on every restart would
make an allowlist useless and every reconnection look like a new peer. A corrupt
identity file is **reported, not silently replaced** — quietly regenerating would
change the node's identity without telling anyone.

## 🤝 The handshake

```
client → HELLO <version>
server → ACK   <serverID> <serverPubPEM> <serverNonce> <unixSeconds>
client → AUTH  <clientID> <clientPubPEM> <clientNonce> <signature> <address>
server → OK    <signature>
                        ... both sides switch to an encrypted stream ...
```

Both sides sign a transcript covering **both nonces and both node IDs**:

```
"gbb-p2p-auth-v1" ‖ 0 ‖ signerID ‖ 0 ‖ peerID ‖ 0 ‖ signerNonce ‖ 0 ‖ peerNonce
```

Every part of that earns its place:

| Element | Prevents |
|---|---|
| Context string | A handshake signature being replayed as a transaction signature |
| Both node IDs | A captured signature being replayed against a *different* peer |
| Both nonces | Replay of an earlier session between the same peers |
| Asymmetric ordering | A signature being reflected back at its own author |
| `0` separators | Field-shifting: `("ab","c")` and `("a","bc")` producing the same bytes |

`TestTranscriptBindsBothPartiesAndBothNonces` and
`TestTranscriptFieldsCannotBeShifted` pin all of these.

### Replay protection

Two layers: a **nonce cache** rejects any nonce seen before, and a **freshness
window** (±2 minutes) rejects stale handshakes. The cache is bounded and evicts,
so a peer opening endless connections cannot exhaust memory.

### Protocol version

`HELLO gbb/2`. A peer speaking anything else is refused rather than
half-understood. **This is a breaking change** — nodes running the previous
protocol cannot connect.

## 🔒 Session encryption

Authentication alone would not be enough. On a plaintext channel an active
attacker can let a valid handshake through and then rewrite everything after it,
so authentication and confidentiality have to stand or fall together.

The session key is derived by **ECDH over the same identity keys**, with both
nonces as salt:

```
shared = ECDH(myIdentityKey, peerIdentityKey)
key    = HKDF-SHA256(shared, salt = clientNonce ‖ serverNonce,
                     info = "gbb-p2p-session-v1", 32 bytes)
```

Binding the key to the authenticated identities is the point: an attacker who
cannot produce a valid signature also cannot derive the key. And because both
nonces are salt, every session gets a distinct key — without that, a recorded
session could be decrypted later by replaying the handshake.

Traffic is then **AES-256-GCM** in length-prefixed records, with the record
sequence number as the nonce. That is safe because the key is fresh per session,
and it means a replayed or reordered record fails to decrypt
(`TestSecureConnRejectsReorderedRecords`).

`secureConn` implements `net.Conn`, so the line-oriented protocol above it is
unchanged — the framing swap is invisible to the command handlers.

> **HKDF is implemented locally** (RFC 5869, ~30 lines of standard-library HMAC)
> rather than pulled from `golang.org/x/crypto`, which is not vendored and would
> have added a dependency to a project that deliberately keeps few.
> `TestHKDFMatchesRFC5869` checks it against the RFC's own test vector.

## 📋 Allowlist

Authentication answers *who* a peer is. Whether they are **welcome** is separate:

```bash
P2P_ALLOWED_PEERS=<nodeID>,<nodeID>
```

Empty (the default) admits any authenticated peer. Blank entries are ignored, so
a misconfigured list cannot accidentally create an empty-ID allowance.

## ⚙️ Configuration

| Setting | Value |
|---|---|
| Identity key | `{DATA_PATH}/node_identity.pem`, mode `0600` |
| Curve | ECDSA P-256 (signatures and ECDH) |
| KDF | HKDF-SHA256 |
| Cipher | AES-256-GCM |
| Nonce size | 32 bytes |
| Clock skew allowed | ±2 minutes |
| Max record | 1 MiB |
| Allowlist | `P2P_ALLOWED_PEERS` (comma-separated) |

Your node ID is logged at startup:

```
Generated node identity 9f2c...
```

## 🚧 Limits

**Trust on first use.** Identity is verified, but nothing says *which* identity
you should expect at an address. A first connection to a new address takes the
peer's word for its key. Use `P2P_ALLOWED_PEERS` where the peer set is known.

**No revocation.** A compromised key is only excluded by removing it from an
allowlist; there is no revocation list or key rotation protocol.

**No forward secrecy.** The session key comes from long-term identity keys, so
someone who later obtains a node's identity key can decrypt recorded sessions. An
ephemeral ECDH exchange would fix this and is the natural next step.

**The handshake is in the clear.** It carries public keys and nonces, which is
harmless for confidentiality, but a passive observer can see which node IDs are
talking to each other.

**No rate limiting on handshakes.** ECDH and signature verification cost CPU, so
a flood of connections is a plausible denial-of-service vector.

## 🧪 Tests

`sdk/p2p_auth_test.go`:

| Test | Property |
|---|---|
| `TestNodeIDIsDerivedFromTheKey` | IDs are self-certifying |
| `TestIdentityIsPersistedAndReloaded` | Stable across restarts, key is `0600` |
| `TestCorruptIdentityFileIsReported` | No silent identity change |
| `TestSpoofedNodeIDIsRefused` | **Claiming another node's ID is refused** |
| `TestHandshakeWithAWrongSignatureIsRefused` | Signature is actually checked |
| `TestUnauthenticatedClientIsRefused` | No identity, no connection |
| `TestTranscriptBindsBothPartiesAndBothNonces` | No signature replay |
| `TestTranscriptFieldsCannotBeShifted` | No concatenation ambiguity |
| `TestReplayedHandshakeIsRefused` | Nonce cache works |
| `TestNonceCacheDoesNotGrowWithoutBound` | No memory leak |
| `TestStaleHandshakeIsRefused` | Freshness window |
| `TestProtocolVersionMismatchIsRefused` | No half-understood peers |
| `TestSessionKeysAgreeAndAreUnique` | Both sides agree; third parties cannot |
| `TestHKDFMatchesRFC5869` | The local HKDF is correct |
| `TestSecureConnActuallyEncrypts` | Plaintext never hits the wire |
| `TestSecureConnRejectsTamperedRecords` | GCM detects modification |
| `TestSecureConnRejectsReorderedRecords` | No record replay |
| `TestSecureConnRejectsAnAbsurdLengthPrefix` | No hostile allocation |
| `TestAllowlistRestrictsPeers` | Allowlist is enforced |
| `TestSessionTrafficIsEncryptedOnTheWire` | **End to end, through a proxy that reads the wire** |
| `TestAuthenticatedPeersCanSync` | Sync still works over authenticated sessions |
| `TestConcurrentHandshakes` | 16 simultaneous handshakes, `-race` clean |

```bash
go test ./sdk/ -run 'Auth|Identity|Handshake|SecureConn|Allowlist|Nonce|HKDF' -v
```
