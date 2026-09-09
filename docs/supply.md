# Supply & Miner Incentives

This chain has a **fixed supply**, minted once in the genesis block. Miners are
paid from transaction fees, not from a block subsidy.

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

## 🚫 Why there is no block subsidy

`Block.CalculateBlockReward` exists and implements Bitcoin-style halving:

```go
halvings := currentBlockHeight / BlockRewardHalvingInterval
return InitialBlockReward * math.Pow(0.5, float64(halvings))
```

**It is deliberately not wired into consensus.** Paying it would mint new coins
per block, which contradicts the two things the configuration actually says:

- `TOKEN_COUNT` is a fixed supply (33,554,432), minted in full at genesis
- `AllowNewTokens` is `false`

Adding a subsidy would mean choosing a different monetary policy, not filling in
a gap. The function is kept because it is a clear worked example of halving for
readers of the course, and its tests document the schedule — but nothing in block
production or validation calls it.

If you *did* want a subsidy, the work is not "call this function": you would need
a consensus rule fixing the exact permitted amount per height, or every node
would disagree about which blocks are valid — and an unvalidated amount is the
same hole described at the top of this page.

---

## 📊 Reading supply

`CalculateTotalSupply` sums **unspent outputs**, not everything ever minted, so
it reflects what is actually held.

---

## 🔗 Related

- [UTXO Set](utxo.md) — how outputs, fees and balances work
- [Mempool Policy](mempool.md) — what may be submitted
