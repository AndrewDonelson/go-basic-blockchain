# The UTXO Set

The UTXO set is the authoritative record of who owns what. It is a set of
**unspent transaction outputs** derived deterministically from the blocks: every
node that applies the same blocks arrives at the same set.

## 🎯 What this replaced

Balances used to have **two sources of truth that never agreed**:

1. `Blockchain.GetBalance(address)` rescanned every block on every call — O(chain)
   per query.
2. Each wallet also carried a number in its own encrypted store, and
   `NewBankTransaction` checked affordability against *that*. Nothing kept it in
   step with the chain.

Neither prevented a double spend, because neither knew what had already been
spent. The same funds could be committed any number of times.

## 🧱 Model

```go
type Outpoint struct { TxID string; Index uint32 }   // "txid:index"

type UTXO struct {
    Outpoint   Outpoint
    Address    string
    Units      int64   // base units, not float64
    BlockIndex int64
    Coinbase   bool
}
```

Balance is the sum of the outputs an address holds. Supply is the sum of every
output in the set — *circulating* supply, not the sum of everything ever minted,
which is what the old `CalculateTotalSupply` reported.

### Implicit inputs

A Bank transaction here is `{From, To, Amount}`. It does **not** carry an explicit
list of inputs the way a Bitcoin transaction does, and rewriting the transaction
format would have taken the wire codec, the P2P protocol and the course material
with it.

So inputs are selected **deterministically at apply time**: the sender's outputs
ordered by `(block index, transaction ID, output index)`, consumed until the
amount plus fee is covered.

That ordering is the load-bearing part. Go randomises map iteration, so selecting
without the sort would give different nodes different UTXO sets from identical
blocks — a consensus failure. `TestInputSelectionIsDeterministic` applies the same
blocks to 25 independent sets and compares every resulting output.

**The tradeoff:** a transaction cannot be validated in isolation — you need the
set as of its block. Explicit inputs would remove that constraint, and are the
natural next step.

### Integer amounts

Balances are `int64` base units, `UnitsPerToken = 100_000_000`.

Float64 cannot represent `0.1`, so repeatedly crediting and debiting float
balances accumulates error. That is unacceptable in the component that is
supposed to be authoritative about ownership.
`TestIntegerBalancesDoNotDrift` pins it: ten additions of `0.1` sum to exactly
1.00000000 in units, and to something else in float.

`GetBalance` still returns `float64` for display. **Prefer `GetBalanceUnits`
wherever a value is compared or accumulated.**

## 🔁 Applying a block

Per transaction, in order:

| Protocol | Effect |
|---|---|
| `COINBASE` | Creates an output to the recipient. Consumes nothing — this is newly minted supply. |
| `BANK` | Consumes the sender's outputs covering `amount + fee`; creates an output to the recipient, fee outputs to the miner and developer, and change back to the sender. |
| `MESSAGE`, `PERSIST`, `CHAIN`, `P2P` | Consumes the sender's outputs covering the fee; creates fee outputs and change. |

**Application is all-or-nothing.** Changes are staged and only committed once
every transaction in the block has been processed. A partially applied block
would leave the set describing a state no chain ever had —
`TestApplyBlockIsAllOrNothing` covers exactly that.

A transaction can spend an output created **earlier in the same block**, which is
what the genesis block does: it mints to the developer and funds the miner from
that same coinbase.

### Fees are paid out

Fees become real outputs to `MinerAddress` and `DevAddress`, split by
`MINER_REWARD_PCT` / `DEV_REWARD_PCT`. Any rounding remainder goes to the miner,
so the units balance exactly.

Fees used to be deducted from the sender and credited to nobody — every
transaction quietly destroyed value.
`TestBankTransferMovesValueAndPaysFees` asserts conservation: supply before ==
supply after.

## ⏪ Reverting

Each applied block records a `BlockUndo`: the outputs it spent (restored on
revert) and the outputs it created (removed on revert).

This is what makes reorganisation cheap. Without it, switching branches would
mean replaying the chain from genesis to rebuild the set.

**Reverts must be in reverse application order.** Reverting an earlier block
first could resurrect an output that a later block already spent, so out-of-order
reverts are refused.

### Across a reorganisation

`AcceptBlock` builds the candidate set on a **clone** — revert the disconnected
blocks, apply the branch — and only swaps it in once the whole branch applies
cleanly. A branch containing a double spend is refused and the live set is
untouched, matching how a failed reorganisation leaves the chain untouched.

## 🚫 Double-spend prevention

Two layers:

1. **Mempool.** `AddTransactionLocal` calls `ValidateTransactionFunds`, which
   checks the sender's unspent balance against the amount plus fee — **and
   subtracts what they have already committed in the mempool**, so a sender
   cannot queue the same coins repeatedly (`TestMempoolCountsAlreadyQueuedSpends`).
2. **Block application.** `ApplyBlock` fails if any transaction tries to consume
   outputs that are not there, so a block containing a double spend never joins
   the chain (`TestBlockWithADoubleSpendIsRefusedByTheChain`).

The mempool check is advisory-in-principle — it can be raced — and the block
check is authoritative.

## 🔄 Derived, never loaded

The set is **not persisted**. It is rebuilt by replaying the blocks, on startup
and on demand via `RebuildUTXOSet()`.

Loading a snapshot would let the set and the blocks drift, which is the class of
bug this whole component exists to remove.
`TestRebuildUTXOSetMatchesIncrementalApplication` asserts a full replay reproduces
the incrementally built set exactly.

The cost is a replay on startup, proportional to chain length.

## 📐 Wallet balances are advisory

`wallet.GetBalance()` still exists and still returns the wallet's own cached
number. It is **not authoritative** and is not kept in step with the chain.

`NewBankTransaction` checks it as a client-side convenience so a caller gets a
quick error when building a transaction locally. The real check happens on
submission, against the set.

If you are writing code that needs a balance, use
`blockchain.GetBalanceUnits(address)`.

## 🚧 Limits

- **Implicit inputs**, as above: transactions are not independently validatable.
- **No explicit scripts or locking conditions.** Ownership is a plain address
  match; there is no scripting system.
- **The set is memory-only**, so startup replays the chain.
- **No UTXO pruning or commitment.** The whole set lives in memory, and there is
  no commitment to it in the block header, so a syncing node cannot verify state
  without replaying every block.
- **Coin selection is oldest-first**, chosen for determinism, not for minimising
  change outputs or fees.

## 🧪 Tests

`sdk/utxo_test.go`:

| Test | Property |
|---|---|
| `TestAmountUnitConversion` | Conversion, round trip, non-finite input |
| `TestIntegerBalancesDoNotDrift` | Why balances are integers |
| `TestCoinbaseCreatesSpendableOutput` | Minting credits someone |
| `TestBankTransferMovesValueAndPaysFees` | Full accounting, and conservation |
| `TestDoubleSpendIsRejected` | The property the chain had no defence against |
| `TestSpendingMoreThanHeldIsRejected` | Oversized single transfer |
| `TestSpendingAnOutputCreatedInTheSameBlock` | The genesis pattern |
| `TestFeeOnlyTransactionsChargeTheSender` | MESSAGE / PERSIST |
| `TestApplyBlockIsAllOrNothing` | No partial application |
| `TestApplyingTheSameBlockTwiceIsRejected` | No double counting |
| `TestInputSelectionIsDeterministic` | 25 runs, identical sets |
| `TestOutputsAreReturnedInDeterministicOrder` | The sort itself |
| `TestRevertRestoresThePreviousState` | Undo correctness |
| `TestRevertMustBeInReverseOrder` | No resurrecting spent outputs |
| `TestApplyRevertApplyIsStable` | The reorganisation cycle |
| `TestCloneIsIndependent` | Branch testing does not touch the live set |
| `TestChainBalanceComesFromTheUTXOSet` | The chain reads from the set |
| `TestMempoolRejectsUnfundedTransactions` | Submission-time enforcement |
| `TestMempoolCountsAlreadyQueuedSpends` | No repeated queueing |
| `TestRebuildUTXOSetMatchesIncrementalApplication` | Replay == incremental |
| `TestReorgRebuildsTheUTXOSet` | Balances move with the chain |
| `TestBlockWithADoubleSpendIsRefusedByTheChain` | Chain-level guard |
| `TestTotalSupplyReflectsCirculation` | Supply is unspent, not minted |
| `TestUTXOSetIsSafeUnderConcurrency` | `-race` clean |

```bash
go test ./sdk/ -run 'UTXO|Balance|DoubleSpend|Coinbase|Revert|Mempool|Supply' -v
```
