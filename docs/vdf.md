# Verifiable Delay Function

Helios stage 2 is a **Wesolowski VDF over the class group of an imaginary
quadratic field**. Implemented in [`internal/helios/vdf`](../internal/helios/vdf).

---

## 🎯 What was wrong with the old stage 2

It was a sequential SHA-256 chain:

```go
for i := 0; i < iterations; i++ {
    result = sha256.Sum256(result)
}
```

Sequential, genuinely — each link needs the one before it, so no amount of
parallelism shortens it. But **checking it cost exactly as much as producing
it**: every validating node had to walk all the links itself. That caps the delay
at whatever a validator will spend, which is not much, so the delay could never
be long enough to mean anything.

A VDF's defining property is precisely this asymmetry: slow to compute, fast to
verify.

---

## 🔢 Why a class group, and not RSA

Wesolowski needs a group whose **order nobody knows**. The claim is that
computing `x^(2^T)` takes T sequential squarings — but anyone who knows the group
order `n` computes `e = 2^T mod n` and gets the same answer with a *single*
exponentiation. That is not a speed-up, it is a complete break.

**The obvious choice, `(Z/NZ)*` for an RSA modulus `N = p·q`, has order
`(p−1)(q−1)` — and whoever generated N knows it.** In a proof-of-work chain that
party produces the "delay" instantly and mines as fast as they like while
everyone else waits. Making that safe needs one of:

- a **multi-party ceremony** to generate N with no participant learning the
  factors — there is no ceremony to hold for an SDK someone installs locally; or
- a modulus whose factors are **asserted to have been destroyed** (an RSA
  challenge modulus) — an assertion no user can verify.

The class group of a negative prime discriminant has no such secret. Its order,
the class number `h(D)`, is believed hard to compute for large `|D|` — and
crucially it is **unknown to whoever chose D as well**. Picking a discriminant
reveals nothing about the order, so there is no trapdoor to hold and nothing to
destroy.

The discriminant here is derived deterministically from a public seed:

```go
const defaultVDFSeed = "gbb/helios/vdf/discriminant/v1"
```

Anyone can re-derive it and confirm the parameters were not chosen with a
convenient structure in hand. **No trusted setup, because there is no secret.**

The cost is arithmetic — elements are binary quadratic forms and composition is
much more work than multiplying integers mod N. That is an implementation
expense, not a weakening of the security argument, which is the right way round
for a chain whose entire premise is that no participant is privileged.

*(This is the same reasoning Chia's VDF uses class groups for.)*

---

## ⚙️ How Wesolowski works

Given input `x`, delay `T`, and output `y = x^(2^T)`:

1. Derive a challenge prime `l` by Fiat-Shamir over `(D, x, y, T)`.
2. The prover computes the witness `π = x^⌊2^T / l⌋`.
3. The verifier checks **`π^l · x^r == y`**, where `r = 2^T mod l`.

`⌊2^T/l⌋` has about T bits, so it can never be materialised. The witness is
accumulated one bit at a time by long division, alongside a running remainder:

```
π = 1, r = 1
repeat T times:  b = ⌊2r/l⌋;  r = 2r mod l;  π = π² · x^b
```

Verification is two exponentiations by values at most `l` (128 bits) — a few
hundred group operations, **independent of T**.

| | cost |
|---|---|
| Evaluate | T squarings |
| Prove | ~T more |
| **Verify** | **~2·log(l), fixed** |

Measured (`TestVerificationCostDoesNotGrowWithDelay`): verification takes 8.0 ms
after 500 iterations and 8.4 ms after 4,000. Evaluation grows with T; verification
does not. End to end through Helios, validation is ~16× cheaper than mining, and
that ratio widens as the delay rises.

Soundness is roughly `1/l`: a cheating prover must guess the challenge before
committing to `y`, and 128 bits puts that out of reach.

---

## ⛓️ The delay is sequential across blocks, not just within one

A per-block delay is not much use on its own: if every block's delay could be
computed independently, a miner with enough cores would compute a hundred of them
at once and the chain would gain no elapsed-time guarantee at all.

It cannot, because **block N−1's delay output is fed into block N's delay
input**:

```go
seed = H( "helios/stage2/vdf/input/v2"
        , len(parentOutput), parentOutput
        , len(blockHeader),  blockHeader )
```

Block N's VDF cannot begin until block N−1's has finished, so a chain of k blocks
costs at least k delays of wall-clock time no matter how much hardware is aimed
at it. `TestProofDoesNotVerifyAgainstTheWrongParent` confirms the binding is real:
a proof computed on one parent does not verify against another, nor against none.

The length prefixes are not decoration. Without them, a parent output ending in
some bytes and a header beginning with them would be indistinguishable from a
different split of the same concatenation, and two distinct blocks could share a
delay input.

The mining header also commits to `PreviousHash`, so this dependency existed
implicitly before. The explicit version does not rely on that: it is a property of
this function rather than of what happens to be in the header, so it cannot
disappear if the header changes.

### The validation ordering this required

Verification now needs the parent, but it must **not** hold the chain lock — it
re-runs the memory-hard phase and checks a delay proof, around a hundred
milliseconds, and holding the lock across that would stall mining and every reader
behind each arriving block. That is exactly what an attacker sending junk blocks
would exploit.

The resolution is that verification needs the parent's *data*, not the lock:

| Step | Lock |
|---|---|
| `validateStandalone` — structure, hashes, transactions | none |
| Resolve the parent by hash (`blockByHash`) | briefly, for a map lookup |
| `verifyProofOfWorkAgainstParent` — memory phase + delay proof | **none** |
| `acceptBlockLocked` — mutate the chain | held |

A parent is immutable: the child names it by hash, so which block it is cannot
change underneath the verifier, and reading it early is safe.

A block whose parent has not arrived is an **orphan** — there is nothing to chain
its delay onto, so its proof is left unchecked at acceptance and verified in
`validateBranchLocked`, against its real ancestry, before it can join the chain.
It cannot be connected unverified.

`TestExpensiveVerificationNeedsNoChainLock` holds the chain lock and requires a
full verification to complete anyway.

## 🧱 The input is the block header, never the nonce

This is the design point that is easy to get wrong, and the old code did.

A delay function **inside a nonce search provides no delay**. Each attempt would
begin its own independent chain, so a miner with *n* cores runs *n* of them at
once and the wall-clock cost of the block is one chain, not *n*. The sequentiality
is real per attempt and worthless per block.

So stage 2's input is derived from the block header alone, and it runs **once**,
before the search:

```go
seed := sha256.Sum256(append([]byte("helios/stage2/vdf/input"), blockHeader...))
input, _ := vdf.HashToForm(seed[:], discriminant)
```

There is now exactly one chain per block, and it must be walked end to end before
any nonce can be tried. As a side effect the search stops paying T sequential
squarings per candidate, which is what made the naive placement unusable anyway.

The input is *hashed into* the group rather than being a fixed generator: an
element whose discrete logarithm to a published base were known would let a
prover shortcut the delay.

---

## ⚙️ Configuration

| Field | Default | Meaning |
|---|---|---|
| `VDFDiscriminantSeed` | `gbb/helios/vdf/discriminant/v1` | Public seed the group is derived from |
| `VDFDiscriminantBits` | **2048** | Discriminant size |
| `VDFIterations` | 2000 | T, the delay in sequential squarings |

A validator rejects a proof whose iteration count is not the one it requires, so
a miner cannot present a cheap delay as an expensive one.

### Discriminant size

**2048 bits.** The security of a class group rests on the class number being hard
to compute, and the best known algorithms for that are subexponential in the size
of the discriminant — the same shape as factoring, so the sizing intuition
carries over from RSA. 1024 is defensible and is what Chia runs; 2048 costs about
4.7× per group operation and buys a margin that does not need revisiting.

Measured at 2048 bits:

| T | Evaluate | Verify | Ratio | Proof |
|---|---|---|---|---|
| 1,000 | 464 ms | 88 ms | 5.3× | 546 bytes |
| 2,000 | 1.02 s | 83 ms | **12.3×** | 546 bytes |

Verification is flat while evaluation doubles, so the advantage grows with the
delay rather than shrinking — at T=10,000 evaluation is around five seconds and
verification is still ~85 ms.

### The discriminant is precomputed

Deriving a 2048-bit discriminant means searching for a prime — roughly half a
second, far too long to repeat at every startup for a value that can never
change. It is embedded as a constant in
[`parameters.go`](../internal/helios/vdf/parameters.go).

That would ordinarily weaken the no-trusted-setup argument, since an embedded
constant is exactly where a chosen value could hide. So
`TestPublishedDiscriminantMatchesItsSeed` re-derives it from the published seed
and fails if it differs. The claim "this was produced by public derivation, not
selected" is therefore **checked on every test run** rather than asserted in a
comment.

---

## 🧪 Tests

The group is checked by its **axioms**, which is the real safety net: Gauss
composition is intricate, and an error in it would not crash — it would produce
something that is not a group. `TestCompositionIsAssociative` runs 125
combinations; associativity is very hard to satisfy by accident.

The VDF is checked for forged outputs, forged and identity witnesses, proofs
replayed at a different iteration count or against a different input, challenge
dependence on every field of the statement, and the cost asymmetry itself.

```bash
go test ./internal/helios/vdf/ -v
go test ./internal/helios/algorithm/ -run VDF -v
```

---

## 🚧 Limits

- **Squaring is specialised, not NUDUPL.** Composing a form with itself collapses
  the general algorithm — h is zero and s equals t, so one of the two modular
  congruences disappears — which is worth about 19% at 2048 bits, where the
  bottleneck is big-integer reduction rather than the congruence solve. NUDUPL
  would go further by keeping intermediate values smaller. It was not taken:
  correctness here is worth more than the constant factor, and the specialised
  square is already a second implementation of an operation the group axioms were
  verified against, so it is pinned to `Compose(f, f)` by
  `TestSquareMatchesComposition` and falls back to it for any case the derivation
  does not cover.
- **The memory phase is still not Argon2id.** Stage 1 is unchanged by this work.

---

## 🔗 Related

- [Helios Consensus](helios.md) — the three-stage proof of work
- [P2P Security](p2p-security.md) — where ephemeral ECDH provides a related property
