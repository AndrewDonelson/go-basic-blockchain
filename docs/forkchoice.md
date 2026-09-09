# Fork Choice and Reorganisation

When two nodes mine at the same time the network briefly holds two competing
histories. Fork choice decides which one wins; reorganisation is the act of
switching to it.

Before this existed, `AcceptBlock` took only blocks that extended the current
head. A block on a competing branch was refused outright, so two nodes that mined
simultaneously **diverged permanently** — sync closed gaps, but it could not
resolve competing histories.

## ⚖️ The rule: heaviest chain, not longest

A branch wins when the **cumulative proof-of-work** behind it exceeds the current
chain's.

```go
work(block) = 2^256 / target(difficulty)   // = 2^difficulty
work(chain) = Σ work(block)
```

Length is the wrong measure as soon as difficulty can vary. Ten blocks at
difficulty 4 carry 10 × 2⁴ = 160 units of work; three blocks at difficulty 8 carry
3 × 2⁸ = 768. The shorter branch represents nearly five times the effort, and
picking the longer one would let an attacker win by mining many cheap blocks.

`TestFewerHarderBlocksOutweighManyEasyOnes` pins exactly this.

> Bitcoin computes work as `2^256 / (target+1)`, the `+1` guarding a division by
> zero when the target is `2^256-1`. Here the target is always an exact power of
> two and never zero, so the `+1` would only add truncation error — at difficulty
> 1 it yields 1 instead of 2, and work stops doubling per difficulty step.

### Work has to come from the block

`Header.Difficulty` is stamped when a block is mined and the proof of work is
verified **against that field**. Previously the header carried a constant while
verification used the node's own config, so the field was decorative.

A peer cannot inflate its branch's weight: claiming a higher difficulty means
producing a hash that meets the harder target, which is real work. The other
direction needs a guard, so blocks below `minAcceptableDifficulty` are refused —
otherwise a long branch of trivially mined blocks could outweigh honest work.

That check reads `block.Header.Difficulty` directly rather than through the
`blockDifficulty()` helper. The helper's fallback exists for blocks already on
our chain that predate the field; applying it to incoming blocks would let a peer
send difficulty 0 and have it silently promoted to the chain default, bypassing
the floor entirely.

## 🔀 What happens to an arriving block

```
                    ┌──────────────────────────┐
   block arrives →  │ validate standalone      │  hash, difficulty floor,
                    │ (outside the chain lock) │  transactions, proof of work
                    └────────────┬─────────────┘
                                 ▼
                    ┌──────────────────────────┐
                    │ already known?           │→ ErrKnownBlock
                    └────────────┬─────────────┘
                                 ▼
                    ┌──────────────────────────┐
                    │ extends the head?        │→ append. done.
                    └────────────┬─────────────┘
                                 ▼
                    ┌──────────────────────────┐
                    │ walk back to a fork point│→ parent unknown: ErrOrphanBlock
                    └────────────┬─────────────┘   (block retained)
                                 ▼
                    ┌──────────────────────────┐
                    │ heavier than our chain?  │→ no: ErrWeakerBranch
                    └────────────┬─────────────┘   (block retained)
                                 ▼
                    ┌──────────────────────────┐
                    │ REORGANISE               │
                    └──────────────────────────┘
```

**Nothing valid is ever discarded.** An orphan or a lighter branch stays in the
block index, because a later block may extend it into the heaviest chain —
`TestRetainedBranchWinsWhenExtended` covers precisely that sequence.

**Ties keep the chain you have.** Switching on equal work would make nodes
flip-flop between branches with every arriving block.

## 🔁 The reorganisation

1. Assemble the branch by walking `PreviousHash` back to the main chain.
2. Validate the **whole** branch: linkage, index sequence, per-block validity.
3. Build the replacement chain in a fresh slice.
4. Swap.
5. Return orphaned transactions to the mempool.
6. Persist the new blocks; delete stale block files above the new tip.

Step 2 completes before step 4 begins. **A branch that turns out to be invalid
leaves the current chain untouched** — a half-applied reorganisation would be far
worse than a refused one. `TestFailedReorgLeavesTheChainIntact` asserts the chain
is byte-identical after a broken branch is rejected.

Proof of work is not re-verified during branch assembly: a block only enters the
index after `AcceptBlock` has verified it, so re-running the memory-hard phase for
every block of every candidate branch would be wasted work.

### Orphaned transactions

Transactions that were only in the abandoned blocks go **back to the mempool**, so
they can be mined again. Transactions present in both branches do not — they are
already mined.

Without this, every reorganisation would silently destroy the transactions unique
to the branch it replaced. `TestReorgRestoresOrphanedTransactions` and
`TestReorgDoesNotRestoreTransactionsPresentInBothBranches` cover both halves.

### Stale block files

Blocks are stored as `blocks/{index}.json`. Reorganising onto a **shorter** branch
leaves files for indices that no longer exist, and a restart would resurrect them
into the chain. Those files are removed.

### Depth limit

A reorganisation may rewrite at most `maxReorgDepth` (100) blocks. Deeper branches
are refused: rewriting arbitrarily far back is the shape of a long-range attack,
and the bound keeps a hostile peer from making a node discard its whole history.

## 🔗 How sync follows a fork

`Syncer` downloads from `Height()+1`, which is the wrong place to start when the
peer is on a branch that forked below that point — the blocks it sends will be
orphans.

So when `AcceptBlock` returns `ErrOrphanBlock`, the download **rewinds** by
`syncRewindStep` (16) blocks and requests again, up to `maxSyncRewinds` (8) times.
That walks back to the fork point and feeds the branch's earlier blocks in, so
fork choice can weigh the whole thing. Without the rewind a node could never
follow a reorganisation: it would keep asking from its own height and keep
receiving blocks whose parents it was missing.

`ErrKnownBlock` and `ErrWeakerBranch` are treated as progress during that walk —
both are normal while retracing a branch you partly have.

## ⚙️ Constants

| Constant | Value | Meaning |
|---|---|---|
| `minAcceptableDifficulty` | 1 | Blocks below this are refused |
| `maxReorgDepth` | 100 | Deepest history rewrite allowed |
| `syncRewindStep` | 16 | Blocks to step back per rewind |
| `maxSyncRewinds` | 8 | Rewinds per sync pass (128 blocks) |

## 🚧 Limits

**No finality.** Any block within `maxReorgDepth` can still be reorganised away.
Wait for confirmations before treating a transaction as settled.

**Difficulty is static.** `internal/helios/difficulty` is implemented and tested
but nothing calls it, so `Config.Difficulty` never changes. Cumulative work is
therefore proportional to length in practice today. The work-based rule is what
makes variable difficulty safe to introduce later, and the tests already cover
mixed-difficulty branches.

**Side blocks are memory-only.** The block index is not persisted, so competing
branches are forgotten on restart. Sync re-fetches them if they still matter.

**No peer authentication**, so a hostile peer can feed branches freely. The depth
limit and difficulty floor bound the damage; they do not eliminate it.

## 🧪 Tests

`sdk/forkchoice_test.go`:

| Test | Property |
|---|---|
| `TestBlockWorkGrowsWithDifficulty` | Work doubles per difficulty step |
| `TestFewerHarderBlocksOutweighManyEasyOnes` | Work, not length, decides |
| `TestChainWorkSums` | Aggregation, mixed difficulty, nil blocks |
| `TestReorgOntoHeavierBranch` | The headline case |
| `TestReorgOntoShorterButHeavierBranch` | A shorter branch can win |
| `TestLighterBranchIsRefusedButRetained` | Nothing valid is discarded |
| `TestRetainedBranchWinsWhenExtended` | A kept branch can win later |
| `TestEqualWorkKeepsTheCurrentChain` | No flip-flopping on ties |
| `TestReorgRestoresOrphanedTransactions` | Transactions are not destroyed |
| `TestReorgDoesNotRestoreTransactionsPresentInBothBranches` | No mempool duplication |
| `TestFailedReorgLeavesTheChainIntact` | Atomicity |
| `TestReorgRemovesStaleBlockFiles` | No resurrection on restart |
| `TestReorgIsRefusedBeyondMaxDepth` | Depth bound |
| `TestAcceptBlockRejectsBelowMinimumDifficulty` | Difficulty floor |
| `TestAcceptBlockRejectsUnknownParent` | Orphans retained |
| `TestBranchAssemblyRejectsSelfReferentialBlocks` | No infinite walk |
| `TestReorgIsSafeUnderConcurrency` | Consistent under `-race` |

```bash
go test ./sdk/ -run 'Fork|Reorg|Work|Branch' -v
```
