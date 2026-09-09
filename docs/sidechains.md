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

**Ids are allocated by consensus, never chosen.** If a registrant picked their
own, ids would be squattable — register publisher 1 and every game id under it,
then sell them back. The same dynamic that makes a domain registry adversarial
applies here. Publisher ids come from a chain-wide counter, game ids from a
counter inside each publisher, both starting at 1, both advanced in the order
registrations are applied. The consequence is that a registrant does not learn
their id until the registration confirms; that is the price of not having a land
grab.

Registration is a `REGISTER` transaction, not a local call, because an id has to
mean the same thing on every node. A registry each node filled in for itself
would let two nodes disagree about who owns publisher 7, and an anchor accepted
by one would be refused by the other.

| Registration | Signed by | Proves |
|---|---|---|
| Publisher | The key being registered | Possession — otherwise anyone could register a key they had merely seen |
| Game | The owning publisher's registered key | The registrant owns the namespace |

A publisher also locks a **bond** (`MinPublisherBondUnits`) at registration. It
is not a fee — it comes back when nothing goes wrong — and it exists so the
availability rules below have something behind them. It moves to an address
derived from a domain string, so no private key for it exists and only a
consensus rule can move it again.

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
That is the dial an operator has — and "Bounding the mutable window" below turns
it from a convention into a consensus rule.

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
    HeaderRoot   []byte  // Merkle root over the block hashes in the range
    PayloadCount uint64  // advisory
}
```

`TipHash` alone already commits the range. `HeaderRoot` is what makes anything in
it **provable**: without it, proving one block belongs to anchored history means
producing every header between that block and the tip — O(range). With it, any
block is provable in O(log range), and each block's own `PayloadRoot` then makes
any single payload provable the same way. That is what makes the availability
challenges below cheap enough to actually issue.

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

Anchors are signed by the publisher's identity key. `AuthoriseSidechain` checks
**both halves** of the id: the publisher must be registered, and the game must be
one they registered. The game half is not a formality — without it a registered
publisher could anchor any game id at all, including ids another publisher's SDK
is already writing, and the registry would describe ownership it did not enforce.

One key may hold only one publisher identity. Sharing a key across identities
would mean compromising it compromises both, and it would make "which publisher
signed this" stop having a single answer.

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

## 🪟 Bounding the mutable window

Between anchors, a publisher can reorder or drop. That is inherent to a
single-writer chain — but "until they get around to anchoring" is not a bound
anyone can rely on, so two consensus rules make it one.

| Rule | Bounds | Enforcement |
|---|---|---|
| `MaxAnchorSpan` (4096 blocks) | How much history one anchor may commit | Anchors exceeding it are **rejected** |
| `MaxAnchorGap` (1000 main-chain blocks) | How long history may stay uncommitted | Exceeding it marks the sidechain **delinquent** |

Together they say: at any moment, at most `MaxAnchorSpan` blocks, written within
the last `MaxAnchorGap` main-chain blocks, are still rewritable.

The two are enforced differently on purpose. An oversized span is trivially
avoidable — submit several anchors instead of one, each contiguous with the last —
so rejecting it costs an honest publisher nothing. A late anchor is not: refusing
it would mean a publisher whose node was down for an afternoon could never catch
up, and their entire history would become permanently uncommittable. So lateness
is **recorded rather than punished**, and `IsDelinquent` makes it visible to
anyone who asks. That is the proportionate response to an outage. Withholding
data is a different matter, and is handled below.

---

## 📡 Data availability

An anchor proves a history existed. It does not make the publisher hand it over.
A publisher who withholds their blocks leaves everyone able to prove that
*something* was committed and unable to say *what* — so a player's achievement is
committed, unforgeable, and unreadable.

### Why not the usual answers

**Erasure coding with data-availability sampling** is what a general-purpose
rollup chain reaches for, and it is right when anyone may publish and nobody is
accountable. Neither holds here: every sidechain has exactly one writer, that
writer is a registered publisher with a bond, and they are commercially motivated
to serve their own game's data. Sampling would impose a sampler network and an
erasure-coding scheme on every node to solve a problem that identified, bonded
writers already mostly solve.

**Putting payloads on the main chain** solves it completely and defeats the
purpose — that cost is the reason sidechains exist.

### What this does instead: bonded challenge and response

1. Anyone may **challenge** a publisher to produce one payload at one height.
2. The publisher **answers** with an inclusion proof against the anchor that
   already commits that height — O(log n), whatever the range size.
3. An unanswered challenge **expires** and slashes the bond.

The asymmetry is the whole design. Answering is trivial for a publisher who has
their data and impossible for one who does not, and it costs the chain a few
hundred bytes rather than the payload set. Honest operation is free: no challenge
is ever issued against a publisher nobody suspects.

A proof carries **two** inclusion proofs, and the pair is what makes it binding:

```
payload → PayloadRoot (block header) → HeaderRoot (anchor) → main chain
```

Either alone would prove nothing about anchored history. The block hash is
**recomputed from the supplied header** rather than taken on trust, so a
respondent cannot present a hash that is in the anchor alongside header fields
that are not.

### The incentives

| Outcome | Challenger's stake | Publisher's bond |
|---|---|---|
| Publisher answers | Goes to the publisher | Untouched |
| Challenge expires | Returned, plus the slash | `AvailabilitySlashUnits` taken |

A challenge is not free, because answering one costs the publisher work and an
unpriced challenge is a way to make a competitor do that work forever. The stake
sits in an escrow address with no private key while the question is open, so
neither party can withdraw it unilaterally. A well-founded challenge is close to
free; a frivolous one pays the person it inconvenienced.

Expiry is swept **automatically on every applied block**, not by someone
submitting a "you failed" claim. Making a slash depend on a third party bothering
to claim it would mean a publisher who withheld data escaped whenever nobody was
watching — which is exactly the case the mechanism exists for. The sweep runs
*after* the block's transactions, so a proof included in the very block that
would expire it still counts: a publisher who answers on the deadline must not be
slashed by an ordering accident.

### What this gives, and what it does not

One answered challenge proves **one payload was available at one moment**. This
is sampling, not proof of total availability, and confidence grows with the
number of challenges issued. What it does change is the incentive: withholding
stops being free and becomes a bonded, detectable fault that anyone can trigger
for the price of a fee.

`PayloadCount` on an anchor remains **advisory** — nothing on the main chain can
check it without the data. It is carried so an auditor holding the blocks can
compare.

---

## 📁 Where the code is

| File | Contents |
|---|---|
| [`sdk/sidechain_id.go`](../sdk/sidechain_id.go) | `SidechainID`, encoding, zero-rejection |
| [`sdk/sidechain_block.go`](../sdk/sidechain_block.go) | Headers, hashing, Merkle root, `Follows` |
| [`sdk/sidechain_store.go`](../sdk/sidechain_store.go) | `Sidechain`, `SidechainSet`, anchored watermark |
| [`sdk/sidechain_anchor.go`](../sdk/sidechain_anchor.go) | `SidechainAnchor`, `PublisherRegistry`, `AnchorLedger` |
| [`sdk/anchortx.go`](../sdk/anchortx.go) | The `ANCHOR` main-chain transaction |
| [`sdk/publisher.go`](../sdk/publisher.go) | `PublisherRegistry`, id allocation, bonds |
| [`sdk/registrationtx.go`](../sdk/registrationtx.go) | The `REGISTER` transaction |
| [`sdk/availability.go`](../sdk/availability.go) | Challenges, proofs, expiry, slashing |
| [`sdk/availabilitytx.go`](../sdk/availabilitytx.go) | The `AVAILABILITY` transaction |
| [`sdk/merkle.go`](../sdk/merkle.go) | Merkle trees with inclusion proofs |
| `sdk/sidechain_test.go`, `sdk/anchortx_test.go`, `sdk/availability_test.go`, `sdk/registrationtx_test.go`, `sdk/merkle_test.go` | The tests named above |

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
