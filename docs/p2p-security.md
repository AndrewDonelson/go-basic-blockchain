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
server → ACK   <serverID> <serverPubPEM> <serverEphPub> <serverNonce> <unixSeconds>
client → AUTH  <clientID> <clientPubPEM> <clientEphPub> <clientNonce> <signature> <address>
server → OK    <signature>
                        ... both sides switch to an encrypted stream ...
```

Both sides sign a transcript covering **both nonces, both node IDs, and both
ephemeral public keys**:

```
"gbb-p2p-auth-v1" ‖ 0 ‖ signerID ‖ 0 ‖ peerID
                  ‖ 0 ‖ signerNonce ‖ 0 ‖ peerNonce
                  ‖ 0 ‖ signerEphemeral ‖ 0 ‖ peerEphemeral
```

Every part of that earns its place:

| Element | Prevents |
|---|---|
| Context string | A handshake signature being replayed as a transaction signature |
| Both node IDs | A captured signature being replayed against a *different* peer |
| Both nonces | Replay of an earlier session between the same peers |
| **Both ephemeral keys** | **A machine-in-the-middle substituting its own key** |
| Asymmetric ordering | A signature being reflected back at its own author |
| `0` separators | Field-shifting: `("ab","c")` and `("a","bc")` producing the same bytes |

`TestTranscriptBindsBothPartiesAndBothNonces`,
`TestTranscriptFieldsCannotBeShifted` and `TestEphemeralKeyIsBoundToIdentity` pin
all of these.

### Replay protection

Two layers: a **nonce cache** rejects any nonce seen before, and a **freshness
window** (±2 minutes) rejects stale handshakes. The cache is bounded and evicts,
so a peer opening endless connections cannot exhaust memory.

### Protocol version

`HELLO gbb/3`. A peer speaking anything else is refused rather than
half-understood. **This is a breaking change** — nodes running `gbb/2` or earlier
cannot connect, because the handshake now carries ephemeral keys.

## 🔒 Session encryption and forward secrecy

Authentication alone would not be enough. On a plaintext channel an active
attacker can let a valid handshake through and then rewrite everything after it,
so authentication and confidentiality have to stand or fall together.

The session key comes from **ephemeral ECDH**. Each side generates a fresh P-256
keypair per handshake, exchanges the public half, and discards the private half
as soon as the key is derived:

```
shared = ECDH(myEphemeralKey, peerEphemeralKey)     ← single-use, never transmitted
key    = HKDF-SHA256(shared, salt = clientNonce ‖ serverNonce,
                     info = "gbb-p2p-session-v1", 32 bytes)
```

**Long-term identity keys are deliberately not an input.** They authenticate the
exchange — each side signs a transcript covering both ephemeral public keys — but
they never contribute to the key itself.

That separation is what forward secrecy means. An earlier version derived the key
from the identity keys directly (`ECDH(myIdentityKey, peerIdentityKey)`), so
anyone who later obtained a node's identity key could decrypt **every session it
had ever had**, including traffic recorded months earlier. Now the only material
that can produce the key is a pair of private values that existed for the length
of one handshake and were never sent anywhere.

`TestCompromisedIdentityKeysCannotDecryptARecordedSession` demonstrates this
operationally: it records a real session's wire bytes, hands an attacker **both**
nodes' identity private keys, and confirms the traffic still cannot be decrypted.

### Why the ephemeral keys must be signed

An anonymous Diffie-Hellman exchange is wide open to a machine-in-the-middle:
relay the identity handshake untouched, substitute your own ephemeral key on each
side, and you hold both session keys while both peers believe they authenticated
each other.

Signing a transcript that covers the ephemeral keys closes that — a substituted
key invalidates the signature. `TestEphemeralKeyIsBoundToIdentity` performs
exactly that substitution and asserts the server refuses it.

### Point validation

`ecdh.P256().NewPublicKey()` rejects points that are not on the curve and the
identity point, and the check runs **before** the point reaches ECDH. Feeding a
crafted "public key" into a Diffie-Hellman computation can otherwise leak bits of
the private key — the invalid-curve and small-subgroup attacks.
`TestDeriveSessionKeyRejectsInvalidPoints` covers the cases.

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
| Identity key | ECDSA P-256 (signatures) |
| Session key exchange | Ephemeral ECDH P-256, fresh per handshake |
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

**Ephemeral private keys are dropped, not wiped.** The reference is released once
the session key is derived, but Go gives no way to zero the underlying memory, so
a key could survive in a heap dump or swap file until it is collected. This
weakens forward secrecy against an attacker with live memory access, though not
against one who compromises the identity key later.

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
| `TestSessionKeyDoesNotDependOnIdentityKeys` | Identity keys are not an input |
| `TestCompromisedIdentityKeysCannotDecryptARecordedSession` | **Forward secrecy, with both identity keys handed to the attacker** |
| `TestEphemeralKeyIsBoundToIdentity` | **A substituted ephemeral key is refused** |
| `TestEveryHandshakeUsesFreshEphemeralKeys` | No key reuse across sessions |
| `TestTwoSessionsBetweenTheSamePeersUseDifferentKeys` | Breaking one session does not break another |
| `TestDeriveSessionKeyRejectsInvalidPoints` | No invalid-curve attack |
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
