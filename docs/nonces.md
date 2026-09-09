# Transaction Nonces & Replay Protection

Every transaction carries its sender's sequence number, and the chain requires it
to **strictly increase**. That is what makes a transaction unrepeatable.

---

## 🎯 The problem a random nonce does not solve

`Nonce` used to be `SecureRandomUint64()`. A random value makes each transaction
distinct — so two otherwise identical payments get different IDs and signatures —
but it says nothing about **order**. Two consequences:

- **Replay.** A transaction already mined could be handed back to the chain — on
  a fresh branch after a reorganisation, or by any peer that kept a copy — and
  applied a second time, spending the sender's coins again.
- **No replacement key.** [Replace-by-fee](mempool.md) needs a stable identifier
  for "the same intended payment". Without one, a transaction whose fee was
  priced too low is simply stuck.

The nonce is covered by the signature, so it cannot be edited to make an
already-mined transaction look new.

---

## 📈 The rule: strictly increasing, not contiguous

```go
if last, seen := s.nonces[sender]; seen && nonce <= last {
    return ErrNonceNotIncreasing
}
```

A sender may skip nonces. Requiring contiguity would mean one lost transaction
blocks every later one until it is resubmitted — real value for a public chain,
but a large amount of queueing machinery for an educational one. Monotonicity
alone is enough for replay protection and for a total order per sender.

A **coinbase is exempt**: it has no sender and mints rather than spends.

---

## 🔄 Nonce state and reorganisations

Nonce state lives in the UTXO set, beside the outputs, because it is derived from
the blocks in exactly the same way and has to move with them.

`BlockUndo` records each sender's nonce **as it was before the block**, so a
rollback restores it. Without that, a rolled-back transaction would leave its
nonce recorded and the sender could never resubmit it on the new branch — locked
out of a nonce they never actually spent.

The record is taken **once per sender per block**, not per transaction. A block
containing two transactions from one sender must restore the value from before
the block, not the value from between them. `nonceRestore` carries an `Existed`
flag so "had no nonce" is distinguishable from "had nonce zero" — a sender's
first transaction may legitimately use 0.

---

## 👛 Where the number comes from

The sender's wallet holds the counter, and `NewTransaction` reserves from it. The
nonce is part of the signing payload, so it has to be settled **before** signing —
which is why it is assigned at construction rather than at submission.

The counter is held **in memory**, not in the vault, because the vault cannot be
written while the wallet is encrypted and a transaction may be built from a
wallet that is only being read.

**This has a consequence worth knowing:** a wallet loaded from disk starts at
zero, and its transactions will be refused as replays of nonces it already used.
Call `Blockchain.SyncWalletNonce(w)` after loading a wallet you intend to spend
from. `NextNonceFor` counts queued transactions too, so two built back to back do
not claim the same number.

---

## 🧪 Tests

[`sdk/nonce_test.go`](../sdk/nonce_test.go) covers sequential assignment,
signature coverage, replay refusal, backwards nonces, reorg restore (including
the per-block-not-per-transaction subtlety), coinbase exemption, mempool
fail-fast, wallet resynchronisation, and that block selection cannot invert a
sender's order.

---

## 🔗 Related

- [Mempool Policy](mempool.md) — replace-by-fee, which keys on the nonce
- [UTXO Set](utxo.md) — where nonce state lives
- [Fork Choice](forkchoice.md) — the reorganisations the undo record exists for
