# Performance

Two lookups sat on the path that **peer traffic drives**, both linear in the
length of the chain, both holding the chain lock. This documents what changed and
why it mattered.

---

## 🐌 The problem

A lock held during an O(chain) scan is not just slow — it blocks mining and every
reader for the duration, and the work is chosen by whoever is sending you
messages.

### `HasTransactionID`

Called for every transaction arriving from the network; the relay path uses it to
avoid re-announcing what it already has. It walked every transaction in every
block:

```go
for _, block := range bc.Blocks {
    for _, tx := range block.Transactions {
        if tx.GetID() == id { return true }
    }
}
```

So a peer sending transactions made each node re-scan its whole history **per
message**.

### `indexMainChainLocked`

Called once per accepted block, and re-inserted *every* block into the index each
time. Syncing `n` blocks therefore did `n(n+1)/2` map writes — quadratic in the
length of the chain, under the lock.

### `mainChainHeightOfLocked`

Called once per step while walking a candidate branch back to a fork point, up to
`maxReorgDepth` times per block, scanning the whole chain each step. One block
from a peer whose parent is unknown cost `maxReorgDepth × len(chain)`
comparisons — and an attacker picks how deeply their blocks fork.

---

## ⚡ The fix

All three now use indexes maintained **incrementally**, doing work proportional
only to what has arrived since the last call.

Measured with `go test ./sdk/ -bench . -benchtime 2000x`, on a chain of one
transaction per block. Both the old and new implementations are in
[`sdk/performance_test.go`](../sdk/performance_test.go), so these numbers are
reproducible rather than claimed:

| Chain length | `HasTransactionID` before | after | |
|---|---|---|---|
| 100 | 45 ns | 19 ns | |
| 1,000 | 1,228 ns | 14 ns | |
| 5,000 | 3,223 ns | 19 ns | **~166×** |

| Chain length | Block indexing before | after | |
|---|---|---|---|
| 100 | 812 ns | 18 ns | |
| 1,000 | 9,548 ns | 55 ns | |
| 5,000 | 66,360 ns | 170 ns | **~390×** |

The absolute numbers matter less than the shape: the "before" column grows with
the chain and the "after" column does not.

---

## ⚠️ The hazard in incremental indexes

An index that is only ever *extended* is wrong the moment a **reorganisation
replaces blocks that are already indexed**.

A naive prefix count fails here. If a 10-block chain reorganises to 11 blocks
forking at height 5, the count says "10 indexed" and the new blocks 5–9 are
silently skipped — their children then cannot find their parent, and valid blocks
get rejected for no visible reason.

So each index records the **hash** that was at the indexed position, not just how
many blocks it covered:

```go
valid := bc.indexedBlocks <= len(bc.Blocks)
if valid && bc.indexedBlocks > 0 {
    valid = bc.Blocks[bc.indexedBlocks-1].Hash == bc.indexedTipHash
}
if !valid { /* rebuild */ }
```

The three indexes differ in what a rebuild means, and the difference is not
cosmetic:

| Index | On reorganisation |
|---|---|
| `blockIndex` (hash → block) | **Extended.** It deliberately holds every block ever seen, on the main chain or not, so a later block can still find its parent on an abandoned branch |
| `txIDIndex` (mined transaction IDs) | **Discarded.** A stale entry here is *wrong*: a rolled-back transaction must become unknown again, or it can never be mined onto the new branch |
| `mainChainHeight` (hash → height) | **Discarded.** Heights shift when blocks are replaced |

Guarded by `TestTransactionIndexIsInvalidatedByAReorg` and
`TestBlockIndexIsRebuiltWhenBlocksAreReplaced`.

---

## 📏 Other bounded work

These are linear but bounded by configuration rather than by chain length, so
they do not grow over time:

- **Mempool admission** scans the queue for a duplicate ID, a matching
  `(sender, nonce)` for [replace-by-fee](mempool.md), and the cheapest resident
  when full — all bounded by `MAX_MEMPOOL_TXS` (default 5,000).
- **Block selection** sorts the mempool by fee rate.
- **UTXO input selection** is bounded by the number of outputs an address holds.

---

## 🚧 Still linear

`GetBlockByHash`, `GetBlockByIndex` and `GetTransactionHistory` walk the chain.
They are public query APIs called by operators and the REST layer, not by the
network hot path, so they have not been indexed. If you expose them to untrusted
callers at scale, index them first.

---

## 🔗 Related

- [Fork Choice & Reorganisation](forkchoice.md) — the reorganisations the indexes must survive
- [Mempool Policy](mempool.md) — the bounded queue
