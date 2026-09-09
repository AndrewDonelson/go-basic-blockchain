# Sidechains

A sidechain is **its own block space**, owned by one publisher, identified by the
pair `(PublisherID, GameID)`. It is not a namespace on the main chain and not a
rollup of main-chain transactions — it is a separate chain of blocks with its own
heights, its own genesis, and its own tip.

The main chain's role is narrower and more important: it holds the **anchors**
that make a sidechain's history binding.

---

## 🧩 Why a separate block space

A game generates far more events than a payment network: every match result,
achievement unlock, leaderboard submission and session. Putting those on the main
chain would make every node in the network pay — in bandwidth, storage and
validation — for one publisher's traffic.

Separate block space inverts that. Each publisher's node writes its own chain at
whatever rate its game demands. The main chain carries one small commitment per
publisher per anchoring interval, so its cost scales with **the number of
publishers**, not with how much their players do.

| | Sidechain | Main chain |
|---|---|---|
| Written by | The publisher's node, alone | Every miner, by consensus |
| Contains | Game payloads (opaque at this layer) | Anchors, transfers, subsidies |
| Grows with | That game's activity | The number of publishers |
| Guarantees | Ordering, contiguity | Immutability, timestamping |

---

## 🆔 Identity

```go
type SidechainID struct {
    PublisherID uint64
    GameID      uint64
}
```

A publisher registers once and receives a `PublisherID`; each game they register
receives a `GameID`. Together they name exactly one block space.

**Zero is not a legal value for either field.** This is a protobuf decision made
early on purpose: an unset protobuf scalar decodes to zero, so if zero were a
legal id, a message that simply omitted the field would address a real sidechain.
`NewSidechainID` rejects it, and `Validate` rejects it again on every path that
takes an id from the wire.

The canonical encoding is 16 bytes, publisher then game, big-endian —
`SidechainID.Bytes()`.

---

## 🧱 Blocks

```go
type SidechainBlockHeader struct {
    Version           uint32
    PublisherID       uint64
    GameID            uint64
    Height            uint64
    PreviousHash      []byte // 32 bytes; empty only at height 0
    PayloadRoot       []byte // 32 bytes, Merkle root over the payloads
    TimestampUnixNano int64
    PayloadCount      uint32
}
```

Payloads are `[][]byte` and **opaque at this layer**. What a match result or a
leaderboard entry means is the microservice's business; the block space's job is
to order them and make the ordering permanent.

Three properties are worth calling out:

**The hash derives from the header alone.** Payloads are committed through
`PayloadRoot`, so a verifier can check a chain of headers without holding any of
the data behind them.

**The header serialisation is length-prefixed and fixed-width throughout.**
Concatenating variable-length fields without prefixes is how two distinct headers
end up sharing one preimage. `TestSidechainBlockHashCoversEveryHeaderField`
mutates each field in turn and requires the hash to change.

**The Merkle leaves and nodes are domain-separated**, and order is significant —
`TestSidechainPayloadRootDetectsReordering` fixes that. A leaderboard that awards
first place to whoever appears first cannot have its payload order silently
swapped.

### Isolation

`Sidechain.Append` refuses any block whose header names a different
`(PublisherID, GameID)`. This matters most for an **empty** chain: a foreign block
at height 0 has no tip to contradict, so contiguity has nothing to say about it,
and the identity check is the only thing standing between one publisher and the
first block of another's history. That case is
`TestSidechainRejectsAnotherPublishersBlock`.

---

## ✍️ Single writer, by design

A sidechain has **one writer**: the publisher's own node. There is no consensus
within a sidechain and none is wanted — a publisher does not need permission from
other publishers to record their own game's events, and making them wait for it
would be the wrong trade entirely.

The honest description of the trust model is therefore:

- Within the **unanchored window**, the publisher can reorder or drop entries.
- Once a range is **anchored**, they cannot change it without changing a mined
  main-chain block.

Anchoring more often narrows the window at the cost of more main-chain traffic.
That is the dial an operator actually has.

---

## ⚓ Anchoring

An anchor commits a contiguous range of a sidechain's history to the main chain:

```go
type SidechainAnchor struct {
    PublisherID  uint64
    GameID       uint64
    FromHeight   uint64  // inclusive
    ToHeight     uint64  // inclusive
    TipHash      []byte  // the hash of the block at ToHeight
    PayloadCount uint64  // advisory
}
```

Because sidechain blocks chain by `PreviousHash`, **committing the tip commits the
entire prefix**. Changing anything at any earlier height changes every hash after
it, and the anchor no longer matches.

`VerifyAgainstChain` is the audit this makes possible: given the blocks, confirm
the history is the one that was committed. It checks the tip hash and then walks
the range confirming the links are intact — one hash only speaks for the blocks
below it if the chain between them holds.
`TestAnchorDetectsRewrittenHistory` rewrites a payload in the middle of anchored
history and requires the check to catch it.

### Contiguity

| Case | Rule |
|---|---|
| First anchor | Must start at height 0 |
| Every later anchor | `FromHeight == previous.ToHeight + 1` |

A **gap** would leave a stretch of history permanently uncommitted. An **overlap**
would let a publisher re-anchor a range with different contents — which is
precisely the rewrite the mechanism exists to prevent. Both are refused.

### Authorisation

Anchors are signed by the publisher's identity key, registered once in
`PublisherRegistry`. A `PublisherID` may be **claimed once**: allowing
re-registration would let whoever registered last take over an existing
publisher's sidechains.

The signature covers **every field the anchor asserts**, the range included.
Leaving the range outside the signature would let one captured signature be
replayed to commit a different span of history under the same authorisation —
`TestAnchorSignatureIsBoundToItsRange` mutates each field and requires the
signature to fail.

---

## 🔗 Reaching the main chain

An anchor becomes binding by riding in a main-chain transaction:

```
ANCHOR protocol → AnchorTx → mined block → UTXOSet.ApplyBlock → AnchorLedger
```

`AnchorTx` carries **two signatures**, and they are deliberately separate:

| Signature | Says |
|---|---|
| `Tx.Signature` | This account authorised this transaction and its fee |
| `AnchorTx.AnchorSignature` | The publisher who owns this sidechain vouches for this history |

They can be different keys. Conflating them would force a publisher to spend from
the same wallet that holds their identity key.

`AnchorTx.Validate` checks shape only. Contiguity and authorisation are questions
about **chain state** — what has already been anchored, and which key owns this
publisher — and a transaction cannot answer them alone. They are enforced where
that state lives, when the block is applied.

### Atomicity

The anchor is recorded **before** the fee is taken. If the commitment is refused —
wrong publisher, or a range that does not follow what is already anchored — the
whole transaction has no effect, fee included. Charging for a commitment that was
not made would be a way to drain a publisher by replaying bad anchors.
`TestRejectedAnchorLeavesNoFeeCharged` holds that line.

### Reorganisation

The ledger lives inside `UTXOSet` because that is where block application and
rollback already happen. `BlockUndo` records each sidechain's prior anchor
alongside the outputs and nonces it already tracked.

An anchor that survived a reorg would be a commitment to a history the chain no
longer contains — and every later honest anchor would fail contiguity against it,
**forever**. `TestRevertingABlockRollsBackItsAnchor` applies two anchors, reverts
one, and requires the ledger to fall back to the first and accept a replacement on
the new branch.

---

## ⚠️ What anchoring does not give you

**Data availability.** An anchor proves a history existed; it does not make the
publisher hand it over. A publisher who loses or withholds their blocks leaves
everyone able to prove that *something* was committed and unable to say *what*.

**Anything about the unanchored window.** Between anchors, reordering and dropping
remain possible. See "Single writer, by design" above.

**Payload semantics.** `PayloadCount` is advisory: nothing on the main chain can
check it without the data. It is carried so an auditor holding the blocks can
compare.

These are real limits, and stating them is more useful than implying they are not
there.

---

## 📁 Where the code is

| File | Contents |
|---|---|
| [`sdk/sidechain_id.go`](../sdk/sidechain_id.go) | `SidechainID`, encoding, zero-rejection |
| [`sdk/sidechain_block.go`](../sdk/sidechain_block.go) | Headers, hashing, Merkle root, `Follows` |
| [`sdk/sidechain_store.go`](../sdk/sidechain_store.go) | `Sidechain`, `SidechainSet`, anchored watermark |
| [`sdk/sidechain_anchor.go`](../sdk/sidechain_anchor.go) | `SidechainAnchor`, `PublisherRegistry`, `AnchorLedger` |
| [`sdk/anchortx.go`](../sdk/anchortx.go) | The `ANCHOR` main-chain transaction |
| [`sdk/sidechain_test.go`](../sdk/sidechain_test.go), [`sdk/anchortx_test.go`](../sdk/anchortx_test.go) | The tests named above |

Not to be confused with [`internal/helios/sidechain`](../internal/helios/sidechain),
which is a protocol-keyed **rollup batcher** used for main-chain transaction
accounting. Different mechanism, unfortunate name collision.

---

## 🧬 A note on shape

Every type here is **protobuf-shaped** on purpose: fixed-width integers, `[]byte`
rather than strings for hashes, `TimestampUnixNano int64` rather than `time.Time`,
and no interfaces anywhere on the wire. The JSON encoding is what exists today;
the schema is built so the move to protobuf is a serialisation change and not a
redesign.
