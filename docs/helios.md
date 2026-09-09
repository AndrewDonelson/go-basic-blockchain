# Helios Consensus Algorithm

Helios is the proof-of-work algorithm used by this blockchain. It is a
**three-phase mining function**: each nonce attempt runs a memory-hard phase, a
sequential phase and a cipher phase, and the three results are folded into a
final SHA-256 hash that must fall below the target.

> **This file was rewritten.** The previous version described a design that was
> never implemented — a `HeliosConsensus` type generating per-transaction proofs
> across "Proof Generation / Sidechain Routing / Block Finalization" stages. The
> real implementation is in `internal/helios/algorithm/helios.go`, and what it
> does is documented below.

## 🎯 What Helios actually is

A nonce search, like Bitcoin's, with a deliberately expensive per-attempt
function instead of a single hash:

```
for nonce = 0, 1, 2, …
    stage1 = MemoryPhase(blockHeader, nonce)      # memory-hard
    stage2 = TimeLockPhase(stage1)                # sequential, un-parallelisable
    stage3 = CryptoPhase(stage2)                  # iterated AES-GCM
    hash   = SHA256(blockHeader ‖ nonce ‖ stage1 ‖ stage2 ‖ stage3)
    if hash <= target: done
```

The intent is to make mining costly along three different axes (memory
bandwidth, sequential latency, cipher throughput) rather than raw hash rate.

## 🔑 The property that makes it work: determinism

**Every phase must be reproducible from the block header and the nonce alone.**
This is not a stylistic preference — it is the entire basis of the algorithm's
security, and it is worth understanding why.

A verifier is given a block and a proof. To check that work was done, it must be
able to **recompute** each phase and compare. If any phase depends on something
the verifier cannot reproduce, it has no choice but to accept the miner's claimed
output — and at that point the phase constrains nothing.

An earlier version of this code derived the stage-3 AES key and nonce from
`crypto/rand`:

```go
// WRONG -- this is what the code used to do
key   := make([]byte, h.config.CryptoKeySize)
nonce := make([]byte, 12)
rand.Read(key)      // non-deterministic
rand.Read(nonce)    // non-deterministic
```

Because stage 3 could not be recomputed, `ValidateProof` re-hashed the stage
outputs *stored in the proof* rather than recomputing them. The consequence:

> A miner could invent arbitrary bytes for all three stage results and
> brute-force only the final SHA-256. The memory-hard phase, the sequential phase
> and the cipher phase were all skippable at zero cost. The "three-stage proof of
> work" reduced to a plain SHA-256 grind over attacker-chosen input.

The key and nonce are now derived from the stage-2 result with domain separation:

```go
keyDigest   := sha256.Sum256(append([]byte("helios/stage3/key"), stage2Result...))
nonceDigest := sha256.Sum256(append([]byte("helios/stage3/nonce"), stage2Result...))
```

and `ValidateProof` recomputes every phase from the header. `TestValidateProofRejectsFabricatedStages`
covers this directly, including the strongest form of the attack: all three
stages invented *and* the final hash recomputed to match them, so the proof is
internally consistent. Only recomputation from the header catches that.

## 🏗️ The three phases

### Phase 1 — Memory (weight 40)

Allocates `MemoryBaseSize` bytes, seeds the first 32 bytes with
`SHA256(blockHeader ‖ nonce)`, then fills and mixes the buffer. Returns
`SHA256(memory)`.

The cost is memory bandwidth, which is harder to scale with special-purpose
hardware than raw hashing.

### Phase 2 — Time-lock (weight 30)

A sequential hash chain of `TimeLockIterations` rounds. Each round depends on the
previous one, so the work cannot be split across cores.

**This phase used to call `time.Sleep` on every iteration of every nonce
attempt.** Sleeping is not computation: it consumes no resources, costs nothing
to skip if the output is fabricated, and a miner running N goroutines pays the
same wall-clock cost as one. The chain length is the work; the sleep is gone.

### Phase 3 — Cryptographic (weight 30)

Iterated AES-GCM sealing, with the key and nonce derived deterministically from
phase 2 as described above.

The weights must sum to 100 or `Mine` returns an error. They are currently
descriptive — they document the intended cost split rather than scaling it.

## ✅ Validation

`HeliosAlgorithm.ValidateProof(proof, blockHeader, target)`:

1. Recompute phase 1 from the header and the proof's nonce; compare to
   `Stage1Result`.
2. Recompute phase 2 from that; compare to `Stage2Result`.
3. Recompute phase 3 from that; compare to `Stage3Result`.
4. Recompute the final hash; compare to `FinalHash`.
5. Compare the recomputed hash against the target.

`validation.ProofValidator` adds structural checks (stage presence, hash format,
nonce sanity) and its own target comparison — against the **decoded hash value**,
not the proof's self-declared `Difficulty` field. That field is written by the
miner, and `Mine` sets it to the target itself, so comparing the two was `x <= x`
and could never fail.

### Where validation runs

The proof is stored on the block (`Block.HeliosProof`) and verified:

- at mining time, before a block is published (self-check);
- in `Blockchain.AcceptBlock`, for blocks received from a peer;
- in `Blockchain.ValidateChain`, for every block in the chain.

Previously `bc.heliosValidator` was constructed and then never called anywhere,
and the proof was discarded at mining time — so no block was ever verifiable
after the fact.

## 🔧 Configuration

```go
type HeliosConfig struct {
    MemoryWeight   int  // 40   — must sum to 100
    TimeLockWeight int  // 30
    CryptoWeight   int  // 30

    MemoryBaseSize    int     // bytes; floored at 32
    MemoryScaleFactor float64
    MemoryIterations  int

    TimeLockBaseDuration time.Duration // retained for config compatibility
    TimeLockScaleFactor  float64
    TimeLockIterations   int           // the actual work: chain length

    CryptoKeySize    int // 32 (AES-256)
    CryptoBlockSize  int
    CryptoIterations int

    EnableEnergyTracking bool
    MiningTimeout        time.Duration // 0 = 18s default
}
```

`DefaultHeliosConfig()` is the production profile; `TestHeliosConfig()` is a much
cheaper one for tests.

> **Known gap:** `sdk.NewBlockchain` currently constructs the algorithm with
> `TestHeliosConfig()`, with the comment *"Use test config for faster mining"*.
> That is fine for an educational single node, but it means the shipped chain runs
> test-grade mining parameters. Switching it to `DefaultHeliosConfig()` is a
> one-line change when the node is meant to do real work.

### Mining timeout

`Mine` gives up after `MiningTimeout` and returns an **error**. The caller
(`Blockchain.createNewBlock`) treats that as a hard failure: the block is
abandoned and its transactions are returned to the mempool.

Previously the timeout was logged and the *unmined* block was returned, appended
to the chain and persisted — so blocks with no valid proof entered the chain
whenever mining was slow.

## 🔐 Security notes

Honest assessment of what this does and does not give you:

**It does:**
- Make each nonce attempt expensive along three axes.
- Produce a proof that any node can independently verify from the block header.
- Bind the proof to a specific block header and nonce, so proofs cannot be reused
  across blocks (`TestValidateProofRejectsWrongHeaderOrNonce`).

**It does not:**
- Provide network consensus. There is no chain sync and no fork choice, so
  "longest chain wins" is not implemented at all. A single node mines its own
  chain.
- Provide ASIC resistance in any rigorous sense. The memory phase is
  Argon2-*inspired*, not Argon2, and has not been analysed.
- Provide a verifiable delay function. Phase 2 is a sequential hash chain, which
  is un-parallelisable but is not a VDF — verification costs the same as
  computation. A real VDF (repeated squaring in a group of unknown order) would
  give fast verification of slow work.

## 🧪 Testing

`internal/helios/algorithm/determinism_test.go` covers the properties above:

| Test | Property |
|---|---|
| `TestStagesAreDeterministic` | Every phase reproduces from header + nonce |
| `TestMinedProofValidates` | Round trip: what `Mine` produces, `ValidateProof` accepts |
| `TestValidateProofRejectsFabricatedStages` | Invented stages rejected, including a self-consistent fabrication |
| `TestValidateProofRejectsWrongHeaderOrNonce` | Proofs cannot be reused across blocks |
| `TestValidateProofRejectsHashAboveTarget` | Difficulty is enforced against the real hash |
| `TestTimeLockPhaseDoesNotSleep` | Phase 2 does real work rather than sleeping |
| `TestMemoryPhaseDoesNotMutateTheHeader` | No `append` aliasing into the caller's slice |
| `TestMemoryPhaseHandlesUnalignedSizes` | No out-of-range slicing for any buffer size |
| `TestMiningTimeoutIsReported` | A timeout is an error, never a block |

Run them with:

```bash
go test ./internal/helios/... -v
```

## ⏳ Stage 2 is a verifiable delay function

Stage 2 was a sequential SHA-256 chain. It is now a **Wesolowski VDF over a class
group** — see [vdf.md](vdf.md).

The chain was genuinely sequential, but verifying it cost as much as producing
it, so the delay could never be set higher than a validator would spend. A
Wesolowski proof is checked in a few hundred group operations whatever the delay
was: validation is ~16x cheaper than mining, and the ratio widens as the delay
rises.

Its input is the **block header, not the nonce**. A delay function inside a nonce
search provides no delay — each attempt starts an independent chain, so a miner
with n cores runs n at once and the block still costs one chain of wall-clock
time. Stage 2 therefore runs once, before the search.

## 📈 Future work

- Scale the phase parameters by the weights, so the weights are load-bearing
  rather than descriptive.
- Analyse the memory phase properly, or replace it with real Argon2id.
