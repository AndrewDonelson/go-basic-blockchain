# Mempool Policy & Block Size

The mempool is bounded, admission is fee-competitive, and blocks are capped at
`MaxBlockSize`. Implemented in [`sdk/mempool.go`](../sdk/mempool.go).

---

## 🎯 Why a policy is needed at all

The mempool used to be a plain unbounded slice, and `createNewBlock` drained all
of it into a block. Two consequences:

- **Unbounded memory.** Anyone could post cheap transactions faster than blocks
  are mined and grow the queue until the node ran out of memory.
- **Unbounded blocks.** `MaxBlockSize` and `Block.CanAddTransaction` both existed
  and neither was ever called, so a block could grow past anything a peer would
  relay or reload.

Both are now enforced on the paths that actually create the problem.

---

## 💰 Ranking: fee per byte, not fee

Transactions are ranked by **fee rate** — fee divided by encoded size:

```go
func txFeeRate(tx Transaction) float64 {
    size := tx.Size()
    if size <= 0 { size = 1 }
    return tx.GetFee() / float64(size)
}
```

Raw fee is the wrong metric. A 20,000-byte transaction paying 5 coins looks
generous next to a 200-byte one paying 1 — but per unit of the scarce resource
(block space) the small one pays five times more, and a hundred like it pay far
better than the large one. Ranking by raw fee lets a single large transaction
crowd out a set of small ones worth more to the miner.

The `size <= 0` guard is not decorative: without it a zero-size transaction
divides by zero and ranks infinitely valuable, which is precisely what a crafted
transaction would aim for.

**Priority wins over fee rate.** `SetPriority` was stored on every transaction and
read by nothing; `outranks` consults it first, which is what finally makes the
field load-bearing.

---

## 🚪 Admission

Below capacity, a transaction is simply appended. At capacity, the cheapest
resident is evicted — **but only if the newcomer outbids it**:

| Policy | Failure it causes |
|---|---|
| Always evict | An attacker flushes every paying transaction out with a stream of zero-fee ones |
| Never evict | Early cheap transactions hold the mempool hostage against later paying ones |
| **Evict only for a higher bid** | Neither |

Capacity comes from `MAX_MEMPOOL_TXS` (default 5000). A transaction refused this
way gets `ErrMempoolFull`.

---

## 📦 Block selection

`selectBlockTransactionsLocked` ranks the mempool, fills the block up to
`MaxBlockSize`, and **removes exactly what it selected**. Whatever did not fit
stays queued — it is not this block's, but it is not dropped either.

One detail worth noting: selection does not stop at the first transaction that
does not fit. It keeps scanning, because a smaller transaction further down the
ranking may still use the space the large one could not:

```go
if size+txSize > MaxBlockSize {
    continue   // not break -- something smaller may still fit
}
```

Ties keep arrival order (`sort.SliceStable`), so two nodes with the same mempool
build the same block.

---

## ♻️ Requeue paths

Two paths put transactions back without going through admission: a failed mining
attempt, and a reorg returning transactions from orphaned blocks. These bypass
admission deliberately — those transactions were already accepted once, and
dropping them would lose user funds in flight — but they must not leave the
mempool permanently over its bound. Both call `trimMempoolLocked`, which drops
the **lowest**-ranked entries back down to capacity.

---

## ⚙️ Configuration

| Variable | Default | Meaning |
|---|---|---|
| `MAX_MEMPOOL_TXS` | 5000 | Transactions held in the mempool |
| `MAX_BLOCK_SIZE` | 1000000 | Maximum block size in bytes |

---

## 🚧 Still missing

- **Replace-by-fee.** A sender cannot bump a stuck transaction's fee.
- **Per-sender nonce sequencing.** `Nonce` is `SecureRandomInt(8)` — a random
  value in 0..7 — so it is not a replay guard.
- **Time-based expiry.** A transaction that never gets mined is only removed by
  eviction pressure, not by age.

---

## 🧪 Tests

[`sdk/mempool_test.go`](../sdk/mempool_test.go) covers fee-rate ranking and its
degenerate inputs, priority precedence, the capacity bound, eviction in both
directions, size-bounded selection, the fill-remaining-space behaviour,
determinism, nil entries, trimming, and the end-to-end path through
`createNewBlock`.

```bash
go test ./sdk/ -run 'Mempool|FeeRate|BlockSelection|Trim' -v
```

---

## 🔗 Related

- [UTXO Set](utxo.md) — the funds check that runs before admission
- [Fork Choice & Reorganisation](forkchoice.md) — where reorg requeue comes from
