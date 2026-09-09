# Chain Synchronisation

Nodes keep their chains aligned two ways: by **pulling** from peers with longer
chains on a timer, and by **pushing** newly mined blocks to peers immediately.

Before this existed, P2P exchanged peer lists and nothing else. Blocks and
transactions were never propagated, so every node mined its own independent chain
and "the network" was a set of strangers who knew each other's addresses.

## 🧭 How it works

```
                 every 10s
   ┌────────────────────────────────────┐
   │  1. ask each peer  GET_STATUS      │
   │  2. drop peers on a different      │
   │     genesis (different network)    │
   │  3. pick the longest chain that    │
   │     is ahead of ours               │
   │  4. GET_BLOCKS in batches of 64    │
   │  5. apply each via AcceptBlock     │
   └────────────────────────────────────┘

   on mining a block
   ┌────────────────────────────────────┐
   │  ANNOUNCE_BLOCK to every peer      │
   └────────────────────────────────────┘
```

Push gets a block to direct peers within milliseconds. Pull is the backstop: it
closes any gap, including for nodes that were offline, were more than one hop
away, or missed an announcement.

**Only mined blocks are announced, never accepted ones.** Re-announcing an
accepted block would echo it straight back to the peer that sent it. Nodes more
than one hop from the miner catch up on the next pull, which fetches the
intervening blocks too. One-hop push plus periodic pull is simpler than gossip
with deduplication, and has no relay loop to get wrong.

## 📡 Wire protocol

Every exchange is one newline-terminated request and one newline-terminated JSON
response, over a connection that has completed the handshake.

### Handshake

```
client → HELLO
server → ACK
client → {"id":"<node id>","address":"<host:port>"}
server → OK
```

Every client path goes through this, including `GET_NODES`. It previously did
not: `requestNodeList` dialled and sent `GET_NODES` immediately while the server
expected `HELLO` first, so the handshake failed and the connection was dropped —
peer discovery could never have worked. The server also used to *fail* the
handshake when the peer was already known, so a peer could connect exactly once.

### Commands

| Request | Response |
|---|---|
| `GET_NODES` | `[{"id","address"}, …]` |
| `GET_STATUS` | `{"node_id","height","head_hash","genesis_hash"}` |
| `GET_BLOCKS <start> <count>` | `[block, …]` — by block **index**, capped at 512 |
| `ANNOUNCE_BLOCK <json>` | `{"accepted":bool,"reason":string}` |
| `ANNOUNCE_TX <json>` | `{"accepted":bool,"reason":string}` |

`height` is the head block's index, so a chain holding only genesis reports `0`
and an empty chain reports `-1`.

## ✅ What is checked

**Genesis hash.** A peer whose genesis block differs is on a different network and
is skipped. Pulling across that boundary would append blocks that can never
validate.

**Every block, on the way in.** Downloaded and announced blocks go through
`Blockchain.AcceptBlock`, which verifies the block extends the current head,
validates its structure and transactions, and verifies its Helios proof of work.
Nothing bypasses that path.

**Every announced transaction's signature.** A relayed transaction is
attacker-controlled input, so `ANNOUNCE_TX` verifies it against the sender's
public key — which travels with the transaction — before it reaches the mempool.

**Request bounds.** A peer may ask for at most 512 blocks per request, and no
message may exceed 1 MiB. Reads have deadlines.

## 🚧 What this is not

**There is still no fork choice.** `AcceptBlock` only accepts blocks that extend
the current head. A peer on a competing fork of equal or greater length is
recorded in `SyncResult.Skipped` and otherwise ignored; the node does **not**
compare cumulative work, keep an orphan pool, or roll back.

The practical consequence: two nodes that mine simultaneously diverge, and the
divergence is permanent. Sync closes gaps; it does not resolve competing
histories. That is the next piece of work.

**There is no peer authentication.** Any host may claim any node ID, and the
transport is plaintext. Do not expose a node to an untrusted network.

## 🔧 Configuration

| Setting | Value | Meaning |
|---|---|---|
| Sync interval | 10s | How often peers are polled |
| Batch size | 64 | Blocks per `GET_BLOCKS` request |
| Server batch cap | 512 | Most a peer may request at once |
| Max message | 1 MiB | Per-message size cap |
| Dial timeout | 10s | |
| Request timeout | 30s | |

Start a node against a seed with `--seed-address host:port`, or run one as a
seed with `--seed`. Both are now carried onto the node's `Config`; they used to
be set on `NodeOptions` and dropped, so neither flag had any effect.

## 🧪 Tests

The logic is deliberately free of I/O — `Syncer` talks to interfaces — so the
download/apply loop is tested exhaustively against fakes, and the wire is tested
separately with real sockets.

**`sdk/sync_test.go`** (engine, against fakes):

| Test | Property |
|---|---|
| `TestSyncCatchesUpToLongerPeer` | The core case: a trailing node reaches the peer's height |
| `TestSyncPicksTheLongestChain` | Peer selection across several candidates |
| `TestSyncBatchesLargeGaps` | Downloads are chunked, not one huge request |
| `TestSyncIsIdempotentWhenLevel` | No work, and no requests, when already level |
| `TestSyncSkipsPeersOnADifferentNetwork` | Genesis mismatch is refused |
| `TestSyncSurvivesAnUnreachablePeer` | One dead peer does not block another |
| `TestSyncStopsWhenAPeerLiesAboutItsHeight` | No infinite request loop |
| `TestSyncStopsOnARejectedBlock` | Valid prefix kept, download halted |
| `TestSyncRejectsANilBlock` | Hostile input errors instead of panicking |
| `TestSyncHonoursContextCancellation` | Clean shutdown |
| `TestSyncIsSafeUnderConcurrency` | Correct under concurrent passes, `-race` clean |

**`sdk/p2p_sync_test.go`** (the wire, over real TCP):

| Test | Property |
|---|---|
| `TestTwoNodesConvergeOverTCP` | End to end: a node 5 blocks behind converges |
| `TestP2PChainStatusOverTheWire` | Handshake + `GET_STATUS` |
| `TestP2PFetchBlocksOverTheWire` | `GET_BLOCKS` ranges, past-the-end, over-large |
| `TestP2PServerCapsBlockRequests` | The server enforces its own cap |
| `TestP2PRequiresHandshake` | A client skipping `HELLO` is not served |
| `TestP2PServerRejectsMalformedRequests` | Bad input does not take the server down |
| `TestNodeDiscoveryOverTheWire` | `GET_NODES` works now that it handshakes |
| `TestBlockAnnouncementOverTCP` | Push propagation reaches a peer |
| `TestAnnouncedBlockThatDoesNotExtendIsRefused` | Orphans rejected with a reason |
| `TestAnnouncedTransactionIsVerifiedBeforeAcceptance` | Unsigned and tampered transactions refused |
| `TestRelayedTransactionIsNotReAnnounced` | No relay loop |

```bash
go test ./sdk/ -run 'Sync|P2P|TwoNodes|Announce' -v
```
