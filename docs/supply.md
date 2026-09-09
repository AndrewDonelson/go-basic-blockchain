# Supply & Miner Incentives

This chain has a **fixed supply**, minted once in the genesis block. Miners are
paid a **block subsidy drawn from a finite emission reserve**, plus transaction
fees. Nothing after genesis mints: the subsidy moves coins that already exist.

---

## 🔒 The rule

**A coinbase transaction is only valid in block 0.**

A coinbase mints: the UTXO set credits its `TokenCount` to the recipient and
consumes nothing. Nothing used to restrict which block one could appear in, so a
peer could mine an ordinary block containing a coinbase for the entire
`TokenCount`, have it accepted by every node, and mint the whole supply to itself
out of nothing — repeatably, once per block.

The rule is enforced in three places:

| Where | Why there |
|---|---|
| `Block.Validate` | The one path `AcceptBlock`, branch validation and `ValidateChain` all share — so it covers mined blocks, relayed blocks, side branches and blocks reloaded from disk |
| `UTXOSet.applyTransactionLocked` | Second line of defence at the point that actually mints, so a future path that skips block validation still cannot inflate supply |
| `AddTransactionLocal` | Keeps a coinbase out of the mempool, so one can never reach a block this node mines itself |

Guarded by `TestCoinbaseOutsideGenesisIsRejected` and the rest of
[`sdk/coinbase_supply_test.go`](../sdk/coinbase_supply_test.go).

---

## 💵 How miners are paid

Every transaction carries a fee, and `UTXOSet.emitFees` splits it between the
miner and developer addresses on the configured percentages:

```
MINER_REWARD_PCT=50.00
DEV_REWARD_PCT=50.00
```

That is the whole incentive model. Fees are paid out of the sender's inputs, so
they move existing coins rather than creating new ones — the supply is unchanged
by any block after genesis, which is what `TestTotalSupplyReflectsCirculation`
checks.

---

## 💰 The block subsidy

Miners are paid a **block subsidy out of a finite emission reserve**, plus their
share of transaction fees.

### It is paid from a reserve, not minted

The obvious way to pay miners is to let each block create coins. This chain does
not, for two reasons.

**The supply is fixed and publishers hold it.** A platform token whose quantity
changes under its holders is a different product from one whose quantity does not,
and predictability is worth more here than an elegant emission curve.

**And it bounds the damage from a mistake.** A minting subsidy is only as safe as
the check on its amount — get that wrong and the chain inflates without limit,
which is exactly the hole that once let any block mint the entire supply. A
subsidy that *moves* coins out of a reserve cannot create any, whatever the
schedule says. The amount check is still there; it is simply no longer the only
thing between the chain and unbounded inflation.

### The reserve has no private key

`DeriveReserveAddress` is the hash of a domain string, not of a public key, so no
keypair produces it and nobody can sign a transfer out of it. The only way coins
leave is the subsidy, whose amount consensus fixes by height and whose recipient
is the block's miner.

A reserve behind a real wallet would put the entire remaining emission behind one
secret. On a chain publishers depend on, that is not a risk worth carrying for the
convenience of a signature nobody needs.

### The schedule

| | |
|---|---|
| `INITIAL_BLOCK_SUBSIDY` | 50 tokens |
| `SUBSIDY_HALVING_INTERVAL` | 210,000 blocks |
| `RESERVE_ALLOCATION_PCT` | 63% of `TOKEN_COUNT` |

The series `50 × 210,000 × (1 + ½ + ¼ + …)` converges to **21,000,000** tokens
over the life of the chain. The reserve holds 21,139,292 — the schedule fits, with
a little spare. `TestTotalEmissionFitsTheReserve` checks that relationship rather
than trusting the arithmetic in this table.

Genesis splits `TOKEN_COUNT` between the reserve and the founder allocation. Total
supply never changes afterwards.

### The rules a subsidy must satisfy

- **At most one per block, and it must be first.** Position is a consensus rule:
  it makes "at most one" a check on a single element rather than a scan, and gives
  every block an unambiguous shape.
- **The amount must equal `BlockSubsidyUnits(height)` exactly** — a pure function
  of height, so every node computes the same answer and can check rather than
  trust.
- **It must be drawn from the reserve**, and it must declare its own height.

When the reserve empties the subsidy becomes zero and miners are on transaction
fees alone. That is the steady state the schedule exists to bridge to, not a
failure.

## 🚫 Why the subsidy does not mint

`Block.CalculateBlockReward` exists and implements Bitcoin-style halving:

```go
halvings := currentBlockHeight / BlockRewardHalvingInterval
return InitialBlockReward * math.Pow(0.5, float64(halvings))
```

It is superseded by `Blockchain.BlockSubsidyUnits`, which is the value consensus
actually enforces. The older helper returns a float and knows nothing about the
reserve; it is kept because the course refers to it.

The concern that left it unwired still holds, and is why the current subsidy is
built the way it is: an unvalidated per-block amount is the same hole described at
the top of this page. The answer was a consensus rule fixing the amount by height
**and** a finite reserve behind it, so that even a flawed rule cannot inflate.

---

## 📊 Reading supply

`CalculateTotalSupply` sums **unspent outputs**, not everything ever minted, so
it reflects what is actually held.

---

## 🔗 Related

- [UTXO Set](utxo.md) — how outputs, fees and balances work
- [Mempool Policy](mempool.md) — what may be submitted
