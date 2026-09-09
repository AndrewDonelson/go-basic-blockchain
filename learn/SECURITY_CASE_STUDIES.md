# Security Case Studies — Real Bugs From This Codebase

This repository was audited and every defect found was fixed. The findings are a
better teaching resource than the working code is, so they are collected here.

Every bug below **shipped in this project**. None are invented. Each was
reproduced at runtime before being fixed, and each now has a regression test
named at the end of its section.

> **How to use this.** Read the "Symptom" and try to explain the cause before
> reading on. Then open the named test in `sdk/regression_test.go` — those tests
> are written to be readable as specifications of the correct behaviour, so they
> double as the answer key.

**Full audit:** `_design/CODE_REVIEW_AND_IMPROVEMENT_PLAN.md` (not tracked in git;
generate your own with a review pass, or ask the maintainer).

---

## Case 1: The signature that did not sign anything

**Symptom.** A signed transfer of 1 token also verifies as a transfer of
1,000,000 tokens.

**The code.**

```go
type Bank struct {
    Tx              // embedded base transaction
    Amount float64  // the value being moved
}

func (t *Tx) Sign(privPEM []byte) (string, error) {
    txBytes, err := json.Marshal(t)   // marshals *only* the embedded Tx
    ...
}

func (b *Bank) Sign(privPEM []byte) (string, error) {
    return b.Tx.Sign(privPEM)         // delegates -- Amount never reaches the signer
}
```

**The cause.** Go has **no virtual dispatch on embedded structs**. `b.Tx.Sign`
sees a `*Tx`; it has no idea a `Bank` exists, so `Amount` is outside the signed
bytes. Anyone who observes a signed transfer can rewrite its value and the
signature still verifies.

**Why it hid so well.** Each protocol had ~20 hand-written pass-through methods
(`func (b *Bank) GetFee() float64 { return b.Tx.GetFee() }`). Embedding already
provides those, so they were pure noise — and the one method in the pile that
genuinely *needed* to differ looked exactly like the nineteen that did not.

**The fix.** `SigningBytes()` on the `Transaction` interface; each protocol
includes its own fields. The pass-through noise was deleted so the real overrides
stand out.

*Test:* `TestSignatureCoversProtocolFields`

**Lesson.** Delete boilerplate. It is not harmless — it is camouflage.

---

## Case 2: The proof of work that proved nothing

**Symptom.** A miner can skip all three expensive stages and still produce a
valid-looking proof.

**The code.**

```go
func (h *HeliosAlgorithm) executeCryptographicPhase(stage2 []byte) ([]byte, error) {
    key   := make([]byte, h.config.CryptoKeySize)
    nonce := make([]byte, 12)
    rand.Read(key)      // ← random
    rand.Read(nonce)    // ← random
    ...
}
```

**The cause.** Stage 3 was non-deterministic, so a verifier could not recompute
it. `ValidateProof` had no choice but to re-hash the stage outputs *stored in the
proof*. That means the stages constrained nothing: supply any bytes for all three,
then brute-force the single cheap final SHA-256. The memory-hard phase, the
sequential phase and the cipher phase were all free to skip.

Stage 2 was worse: its "work" was `time.Sleep`. Sleeping consumes no resources,
costs nothing to fake, and a miner running N goroutines pays the same wall-clock
cost as one.

**The fix.** Derive the key and nonce from the stage-2 result with domain
separation; recompute every stage from the block header during validation;
replace the sleep with a real sequential hash chain.

*Test:* `TestValidateProofRejectsFabricatedStages` — including the strongest form
of the attack, where all three stages are invented *and* the final hash is
recomputed to match, so the proof is internally consistent.

**Lesson.** **A proof is only as strong as the verifier's ability to recompute
it.** If a verifier must trust any part of what the prover supplies, that part
provides no security. Ask of every proof system: *what exactly can the verifier
check independently?*

---

## Case 3: The check that could never fail

```go
if proof.Difficulty.Cmp(targetDifficulty) > 0 {
    return fmt.Errorf("proof difficulty does not meet target")
}
```

`proof.Difficulty` is written by the miner — and `Mine` sets it to
`targetDifficulty` itself. The comparison is `x <= x`. It never fired.

This code had **96.4% test coverage**. High coverage on a check that cannot fail
is worth exactly nothing.

**The fix.** Compare the decoded `FinalHash` value against the target.

*Test:* `TestValidateProofRejectsHashAboveTarget`

**Lesson.** Coverage measures which lines ran, not whether the assertions mean
anything. For every validation you write, ask: *what input makes this fail?* If
you cannot construct one, the check is decoration.

---

## Case 4: `Open()` that deleted your keys

```go
func (w *Wallet) Open(passphrase string) error {
    err := localStorage.Set("wallet", w)   // Set. Not Get.
    ...
}
```

`Open` **wrote** the in-memory wallet over the stored one. And `LocalWalletList()`
built an empty `Wallet` shell per file and called `Open("")` on each:

```
wallet file before Open(): 1323 bytes
wallet file after  Open():  169 bytes
```

**Listing your wallets destroyed every private key on disk.** No backup, no
recovery.

**The fix.** `Open` calls `Get`. Listing reads filenames only. All writes go
through temp-file → `fsync` → atomic `rename`, so an interrupted write cannot
truncate a key file.

*Tests:* `TestWalletOpenDoesNotDestroyTheWalletFile`, `TestLocalStorageWritesAtomically`

**Lesson.** Test the *observable effect*, not just the return value. The old code
returned `nil`. Every test passed.

---

## Case 5: The identity that changed underneath you

```go
toWalletPUID := to.ID              // pointer to the recipient wallet's own ID
toWalletPUID.SetAssetID(assetID)   // mutates the wallet
tx.ID = toWalletPUID               // tx.ID aliases it
```

Observed:

```
tx1 ID at creation : 2:1:1:8624972568939678
tx1 ID read again  : 2:1:1:-5240661355808963400   ← changed
tx2 ID             : 2:1:1:-5240661355808963400   ← identical
```

Creating a second transaction to the same wallet **retroactively rewrote the
first transaction's ID**, and the wallet's identity with it. Every ID-keyed
structure in the project was unsound.

**The fix.** Build a fresh `PUID` from copies.

*Test:* `TestTransactionIDsAreIndependent`

**Lesson.** In Go, `x := someStruct.Ptr` shares, it does not copy. An identity
should be immutable; if it can be mutated in place, it is not an identity.

---

## Case 6: Three deadlocks, one mistake

```go
func (bc *Blockchain) UpdateConfig(cfg *Config) error {
    bc.mux.Lock()
    defer bc.mux.Unlock()
    ...
    return bc.Save()   // Save() takes bc.mux again
}
```

`sync.Mutex` is **not reentrant**. This never returned. The same shape appeared
in `P2P.ProcessQueue` and `P2P.discoverNodes`.

The tell was already in the repository:

```go
t.Skip("Skipping TestProcessQueue due to mutex contention issues …")
```

Someone hit the deadlock, and skipped the test.

**The fix.** Public method takes the lock; private `…Locked` method assumes it.

*Test:* `TestP2PProcessQueueDoesNotDeadlock`, `TestUpdateConfigDoesNotDeadlock`

**Lesson.** A skipped test is a bug report. When you write `t.Skip`, write down
*why* — and treat it as work, not as a resolution.

---

## Case 7: Fields that vanished on the wire

```go
type P2PTransaction struct {
    Tx                    // has MarshalJSON on *Tx
    Target string
    Action string
    Data   interface{}
}
```

`json.Marshal(&p2pTx)` emitted **only the base transaction**. `Action`, `Target`,
`State` and `Data` were gone, because Go promoted `Tx.MarshalJSON` to
`*P2PTransaction`. Every P2P message on the network was undispatchable — the
receiver's switch hit `default: unknown action ""`.

The P2P protocol had never worked over a network.

A companion bug: `Data interface{}` was read back with `tx.Data.([]byte)`. After
a JSON round trip an `interface{}` is never `[]byte` — it is `nil`, a string, or
a map. In `p2p.go` the assertion was unchecked, so **any peer could crash the node
by sending a message**.

**The fix.** Explicit `MarshalJSON`/`UnmarshalJSON`; `Data` typed as
`json.RawMessage`.

*Tests:* `TestP2PTransactionRoundTrip`, `TestP2PHandlersSurviveHostilePayloads`

**Lesson.** Method promotion is invisible. Round-trip test every type that
crosses a process boundary — `go vet` will not catch this, and neither will a
unit test that never serialises.

---

## Case 8: The chain that never persisted

```go
type Block struct {
    Transactions []Transaction  // an INTERFACE slice
}
```

`encoding/json` cannot decode into an interface — it does not know which concrete
type to build:

```
json: cannot unmarshal object into Go struct field
      Block.transactions of type sdk.Transaction
```

`LoadExistingBlocks` caught that error, logged it, and `continue`d. **The node
silently discarded its entire history on every restart** and started from a fresh
genesis block.

Two more bugs sat underneath, waiting:

- Block files were ordered with `sort.Strings`, so `10.json` sorted before
  `2.json`.
- `CalculateHash` interpolated `Timestamp.String()`, which renders Go's monotonic
  clock reading (`m=+0.057091985`). JSON drops it, so a block's hash **changed the
  moment it touched disk** and every `Hash != CalculateHash()` check failed.

**The fix.** A protocol-discriminated codec (`sdk/txcodec.go`), numeric sort with
hash-chain verification at load, and a hash over a canonical fixed-width encoding
of `Timestamp.UTC().UnixNano()`.

*Tests:* `TestBlockPersistenceRoundTrip`, `TestLoadExistingBlocksOrdersNumerically`,
`TestBlockHashIsStableAcrossSerialization`

**Lesson.** "Log and continue" turns a loud failure into a silent one. And a hash
input must be a *canonical* encoding — never a `String()` intended for humans.

---

## Case 9: One constant, two meanings

```go
maxNonce = 12 // bytes
```

Used as the AES-GCM nonce **size**… and as the mining loop's nonce **ceiling**:

```go
for i := 0; i < maxNonce; i++ {   // gives up after 12 attempts
```

Simple proof-of-work tried twelve nonces, gave up, and returned an **unmined
block** — which the caller appended and persisted anyway.

**The fix.** `gcmNonceSize = 12` and `maxMiningNonce = 1 << 32`.

*Test:* `TestSimplePoWDoesNotGiveUpAfterTwelveNonces`

**Lesson.** A name that fits two concepts fits neither. Related: `BigInt` in this
codebase wraps an `int64`.

---

## Case 10: The Merkle tree that panicked on six transactions

```go
if len(data)%2 != 0 {
    data = append(data, data[len(data)-1])   // pads ONLY the leaf level
}
for len(nodes) > 1 {
    for i := 0; i < len(nodes); i += 2 {
        node := NewMerkleNode(nodes[i], nodes[i+1], nil)   // ← nodes[i+1]
```

6 leaves → 3 internal nodes → `nodes[3]` on a slice of length 3:

```
panic: runtime error: index out of range [3] with length 3
```

A block containing six transactions crashed the node. Also 10, 14, 22, …

**The fix.** Pad inside the level loop. And domain-separate leaf hashes (`0x00`)
from internal hashes (`0x01`) — without that, duplicating the last node to pad an
odd level lets two different transaction lists produce the same root
(**CVE-2012-2459**).

*Tests:* `TestMerkleTreeHandlesEveryLeafCount` (1–64 leaves),
`TestMerkleTreeIsDomainSeparated`

> ⚠️ **The Merkle example in [Section 4](phase1/section4/README.md) is recursive,
> so it does not have the panic. But it duplicates the last leaf without domain
> separation, so it *does* have the malleability.** Fixing that is a good
> exercise.

**Lesson.** Test the boundaries, not the happy path. `1..64` takes one loop.

---

## Case 11: Money that evaporated

```go
func (b *Bank) Process() string {
    newFromBalance := b.From.GetBalance() - (b.Amount + transactionFee)
    b.From.SetData("balance", newFromBalance)
    // ...and nothing credits b.To
    return fmt.Sprintf("Transferred %f from %s to %s", ...)   // reports success
}
```

The sender was debited. The recipient was never credited. The fee went nowhere.
The function reported success.

`Send` had the same shape: `Bank.Send` delegated to `Tx.Send`, which enqueued the
**embedded base transaction** — so the `Amount` never reached the mempool at all.

**The fix.** Credit the recipient, pay the fee out per the configured split, and
have each protocol enqueue itself.

*Tests:* `TestBankProcessConservesValue`, `TestBankSendPreservesAmount`

**Lesson.** For anything that moves value, assert **conservation**: total before
== total after. A test that only checks the sender's balance passes here.

---

## Case 12: Credentials in the source code

```go
legacyDemoAPIKey = "69a082ff3996745bd4b48bcc92d5bb40ff97115896183f1cb53a3409f818b15f"
legacyServerSeed = "0ebe1955e527d0a3f354315d0af97e88be3d4a499c9dacd0d947bf1bd5c71bca"
```

Used whenever the environment was unconfigured — the default. Every unconfigured
deployment accepted a key published in this repository. The seed was worse: API
keys were `SHA256(seed + email)`, so anyone with the repo could mint a valid key
for any address.

Alongside it:

- `rawKey == hex.EncodeToString(key)` — string comparison short-circuits on the
  first differing byte, leaking the key by timing.
- The API accepted a client-supplied `password_hash` and stored it verbatim, so
  the stored value **was** the credential.
- `/account/register` returned the verification token **in the response body**,
  making email verification a formality — and because re-registering a verified
  address overwrote it, anyone could take over any account.

**The fix.** No fallback (authentication fails closed), `crypto/subtle` for every
comparison, server-side scrypt with a per-account salt, tokens delivered out of
band, re-registration of a verified address refused, and rate limiting throughout.

*Tests:* `TestAPIKeyAuthenticationFailsClosed`, `TestAPIKeyMatchingIsConstantTime`,
`TestAccountPasswordHashingIsSalted`, `testAccountTakeoverIsRefused`

**Lesson.** **Fail closed.** A security default that makes the unconfigured case
*work* is a security default that makes the unconfigured case *insecure*.

---

## Case 13: Endpoints that lied

```go
func (api *API) handleConsensusTx(w http.ResponseWriter, r *http.Request) {
    var payload map[string]interface{}
    json.Unmarshal(body, &payload)   // parsed into a variable that is never read
    respondJSON(w, 200, map[string]any{"status": "ok", "accepted": true})
}
```

Three endpoints reported success for work they never did. Any client built
against that API was built on a fiction.

**The fix.** Implement them, and return a real status — `202` when accepted,
`422` with a reason when not.

*Tests:* `testTransactionCreation`, `testConsensusEndpoints`

**Lesson.** If you cannot implement it yet, return `501 Not Implemented`. A stub
that returns success is worse than no endpoint at all, because it is
indistinguishable from a working one until something depends on it.

---

## The meta-lesson

Before the audit:

```
go build ./...      ✅ clean
go vet ./...        ✅ clean
go test ./... -race ✅ all packages pass
```

All of it green, with signature forgery, key destruction, a fake proof of work, a
chain that did not persist, three deadlocks, a remotely triggerable panic and a
published API key in the tree.

**The test suite exercised constructors and getters.** It did not exercise
signing, persisting, reloading, validating or gossiping — the paths where the
system does its job.

Ask of your own test suite: *if this feature were completely broken, would any
test fail?* For most of the bugs above, the honest answer was no. Several tests
actively asserted the buggy behaviour:

```go
// Current queue semantics dequeue on full-capacity enqueue before duplicate
// check, so enqueuing duplicate "b" at capacity shrinks queue from [a,b] to [b].
assert.Equal(t, 1, q.Len())
```

That comment describes silent data loss, written down as intended behaviour.

**Green is not the goal. Correct is.**
