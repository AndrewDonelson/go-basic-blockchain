# Dynamic Difficulty

Difficulty is a **function of chain history**, not a configuration value. Every
node computes the same required difficulty for the same ancestry, which is what
makes it verifiable rather than merely advisory.

Implemented in [`sdk/difficulty.go`](../sdk/difficulty.go).

---

## 🎯 Why it has to be derived, not declared

Fork choice picks the branch with the most cumulative work
([forkchoice.md](forkchoice.md)), and work is computed from each block's declared
`Header.Difficulty`:

```
BlockWork(d) = 2^d
```

If a block could declare whatever difficulty it liked, a miner would simply write
`Difficulty: 200` into a cheap block and its branch would outweigh every honest
chain in existence — without doing any of the work. The declaration must
therefore be checked against something the miner does not control.

That something is the ancestry. Given a chain, the required difficulty of the
next block is fully determined:

```go
want := bc.ExpectedDifficulty(ancestry)
if int(block.Header.Difficulty) != want {
    return fmt.Errorf("block %s declares difficulty %d but its history requires %d", ...)
}
```

A block declaring anything else is rejected — not down-weighted, rejected. This
check runs both on the fast path in `AcceptBlock` and, per branch, inside
`validateBranchLocked`, so a side branch is judged against **its own** ancestry
rather than the main chain's.

---

## 🔁 The retargeting rule

Difficulty holds steady and moves only on a window boundary:

```go
if height % window != 0 {
    return previous          // most blocks simply inherit
}
```

On a boundary, the node measures how long the last `window` blocks actually took
and compares that with the target:

```go
elapsed := recent[last].Timestamp.Sub(recent[0].Timestamp)
average := elapsed / window
next    := bc.retarget(previous, average)
```

- Blocks arriving **faster** than the target ⟶ difficulty **rises**
- Blocks arriving **slower** than the target ⟶ difficulty **falls**

The measurement is the *only* input. There is no wall clock, no node-local state,
and no randomness, so two nodes holding the same blocks always agree.

---

## ⚠️ Exponent vs. linear scale

This is the subtlety worth understanding, and the one place the implementation is
easy to get wrong.

`Header.Difficulty` is an **exponent**: work is `2^d`. The adjuster in
`internal/helios/difficulty` works in **linear** difficulty, where doubling the
number means doubling the work.

Passing the exponent straight into the adjuster silently corrupts the feedback
loop. Suppose the adjuster wants to double the difficulty of a block at exponent
4. Handed `4`, it returns `8` — but exponent 8 is `2^8 / 2^4` = **16 times** the
work, not twice. The chain would spiral out of reach within a few windows.

So the conversion is explicit in both directions:

```go
currentWork := BlockWork(uint32(previous))   // exponent -> linear work
newWork     := adjuster.CalculateNewDifficulty(currentWork, metrics)
next        := difficultyFromWork(newWork)   // linear work -> exponent
```

where `difficultyFromWork` recovers the exponent as `work.BitLen() - 1`.
`TestDifficultyExponentRoundTrip` pins this down across the whole usable range.

---

## 🛡️ Bounds

Three limits keep a single anomalous window from wrecking the chain:

| Bound | Value | Purpose |
|---|---|---|
| `maxDifficultyStep` | 2 | Cap per retarget — one window can change work by at most 4× |
| `minAcceptableDifficulty` | 1 | A chain that fell to trivial difficulty could be overtaken cheaply |
| `maxAcceptableDifficulty` | 240 | Above this the chain would never produce another block |

The step cap matters most. Because each exponent step doubles the work, an
unbounded retarget on a window with, say, a one-nanosecond spread would push
difficulty far beyond any achievable hashrate and stall the chain permanently.

Degenerate measurements are treated as *no information* rather than as extreme
information: identical timestamps, backwards timestamps, or a history shorter
than the window all leave difficulty unchanged.

---

## ⚙️ Configuration

| Variable | Default | Meaning |
|---|---|---|
| `DIFFICULTY_WINDOW` | 10 | Blocks between retargets |
| `BLOCK_TIME` | 20 | Target seconds per block |

`Config.Difficulty` still seeds the genesis block, but from block 1 onward the
chain's own history decides. Setting a window of `0` falls back to the default
rather than retargeting on every block.

---

## 🧪 Tests

[`sdk/difficulty_test.go`](../sdk/difficulty_test.go) covers the scale
conversion, retargeting up and down, holding between boundaries, step clamping,
the floor, determinism, degenerate and backwards timestamps, rejection of a
wrongly-declared difficulty, per-branch divergence, and the miner stamping what
validation demands.

```bash
go test ./sdk/ -run 'Difficulty|Retarget|FastBlocks|SlowBlocks' -v
```

---

## 🔗 Related

- [Fork Choice & Reorganisation](forkchoice.md) — how declared work decides branches
- [Helios Consensus](helios.md) — the proof of work difficulty parameterises
