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

## Case 14: Any block could mint the entire supply

The UTXO set credits a coinbase's `TokenCount` to its recipient and consumes
nothing. Nothing restricted *which block* a coinbase could appear in.

```go
case *Coinbase:
    units := concrete.TokenCount * UnitsPerToken
    emit(recipient, units, true)   // consumes nothing
```

So a peer could mine an ordinary block containing a coinbase for the full
33,554,432 tokens, have every node accept it, and repeat once per block.

This was proven before it was fixed, which is the part worth copying:

```
attacker balance before: 0
AcceptBlock result={true false 0 0 1 0} err=<nil>
attacker balance after:  3355443200000000
```

`err=<nil>`. The chain accepted it.

**The fix.** A coinbase is valid only in block 0 — enforced in `Block.Validate`
(the one path `AcceptBlock`, branch validation and `ValidateChain` all share),
again in the UTXO set, and again at mempool admission.

*Tests:* `sdk/coinbase_supply_test.go`

**Lesson.** When you write code that creates value from nothing, the very next
question is *who is allowed to call this, and how is that enforced?* "Only the
genesis path calls it" is an observation about today's callers, not a rule. A
rule is something the code refuses to break.

---

## Case 15: Fixing one thing exposed another

Difficulty used to be a static config value, and fork choice weighs branches by
the work each block **declares**:

```go
BlockWork(d) = 2^d
```

While difficulty never changed, that was harmless. The moment it varies, an
unchecked declaration lets a miner write `Difficulty: 200` into a cheap block and
outweigh every honest chain in existence, for free.

The feature and its defence had to land together: the required difficulty is
derived from the block's own ancestry, and a mismatch is rejected outright.

There was a second trap inside the same change. `Header.Difficulty` is an
**exponent** (work = `2^d`), while the adjuster works in **linear** difficulty.
Passing the exponent straight in turns a requested doubling into a 16× jump:

```
adjuster wants 2x the work at exponent 4
hands back 8  ->  2^8 / 2^4  =  16x
```

Within a few windows the chain becomes unmineable.

*Tests:* `TestDifficultyExponentRoundTrip`, `TestBlockDeclaringTheWrongDifficultyIsRejected`

**Lesson.** Two things. A value that is safe *because* something else never
changes is a landmine with a date on it — write down the dependency, or enforce
it. And when two components exchange numbers, name the units. "Difficulty" meant
two incompatible things in one codebase and the compiler was happy with both.

---

## Case 16: The crash the fuzzer found in one second

```go
func (t *Tx) GetID() string {
    return t.ID.String()      // t.ID is a *PUID
}
```

A transaction decoded from a peer message or a block file with no `"id"` field
leaves `t.ID` nil. `PUID.String()` dereferenced it. Any malformed transaction
crashed the node that read it — and `GetID` is called constantly: mempool
deduplication, block indexing, logging.

`MarshalJSON` already handled the nil case. `String` did not.

The first run of `FuzzDecodeTransaction` found it immediately:

```
panic: runtime error: invalid memory address or nil pointer dereference
sdk.(*PUID).String(0x0)
sdk.(*Tx).GetID(...)
```

*Tests:* `sdk/fuzz_test.go`

**Lesson.** Every parser that reads bytes from disk or the network deserves a
fuzz target — they are twenty lines each. Note also *where* the bug was: not in
the parser, but in a getter three calls away. The fuzzer found it because the
target did something with the parsed value instead of just checking the error.

---

## Case 17: Authentication attached to the wrong thing

```go
func (api *API) Start() error {
    api.router.Use(apiKeyMiddleware)   // installed here
    ...
}
```

Authentication went on in `Start()`. So `api.router` — on its own, without
`Start()` — was an API with **no authentication at all**.

Anything serving the router directly got an open node: an embedder wiring it into
its own `http.Server`, or a test harness doing exactly that. This project's own
API tests were in the second category, which is why nothing noticed.

**The fix.** Middleware is installed in `NewAPI`, alongside the routes it
protects, so the router is safe to serve as it is.

*Test:* `TestRouterIsAuthenticatedWithoutStart`

**Lesson.** Ask what your security property is attached to. "Requests are
authenticated" was true of the *server*, not the *router* — and the router is the
thing people reuse. A guarantee that depends on the caller taking an extra step
is a guarantee you do not have.

---

## Case 18: A random nonce is not replay protection

`Nonce` was a random 64-bit value. It makes every transaction distinct, so it
*looks* like it does the job.

It does not. Randomness says nothing about **order**, so nothing stops an
already-mined transaction being handed back to the chain — on a fresh branch
after a reorganisation, or by any peer that kept a copy — and applied a second
time, spending the same coins again.

**The fix.** The nonce is the sender's sequence number, and the chain requires it
to strictly increase. The nonce is covered by the signature, so an old
transaction cannot be renumbered to look new.

The interesting part is what this exposed. Nonce state has to be *rolled back*
with a reorganisation, or a rolled-back transaction leaves its nonce recorded and
the sender is locked out of a nonce they never actually spent. And the rollback
record must be taken **once per sender per block**, not per transaction —
otherwise reverting a block containing two transactions from one sender restores
the value from *between* them.

*Tests:* `sdk/nonce_test.go`

**Lesson.** "It is unique" and "it prevents replay" are different claims. Write
down the property you actually need — here, *a total order per sender that
survives a reorganisation* — and check the mechanism against that sentence, not
against a vague sense that random values are safe.

---

## Case 19: The wallet address that read other files

`/blockchain/wallets/{id}` takes an address from the URL and hands it to storage,
which built a path from it:

```go
filePath = filepath.Join(ls.dataPath, "wallets", tt.Address+".json")
```

Nothing checked the address. `filepath.Join` *cleans* the result, which is
precisely what makes this work: `wallets/../node` collapses to `node`.

Proven before fixing:

```
address "../node" resolved to path: /tmp/.../001/node.json
relative to wallets dir: ../node.json
TRAVERSAL: path escapes the wallets directory
```

An authenticated caller could read any `.json` file relative to the data
directory — and the update handler writes through the same resolver.

**The fix, in two layers.** Identifiers are validated to a strict character set
(addresses are hex, block indices decimal), so a separator is **refused rather
than escaped**. And the resolved path is checked to be inside the data directory,
so a future caller that builds a name some other way still cannot escape.

*Tests:* `sdk/pathsafety_test.go`

**Lesson.** `filepath.Join` is not a security boundary — it is a string
operation that happens to normalise `..` for you. Any time user input becomes
part of a path, validate the *component* against what a legitimate one looks
like, then verify the *result* is where you meant it to be. And prefer refusing
odd input to sanitising it: escaping invites the next encoding bug, refusing does
not.

This one was found by a linter, which is worth noting after eighteen cases that
were not. Newer gosec added taint analysis, and it followed the address from the
HTTP handler to the file path — a data-flow question, which is exactly the kind
of mechanical fault tools are good at. It is still not a substitute for the
adversarial reading that found the other eighteen.

---

## Case 20: A delay function that delayed nothing

Helios stage 2 was a sequential SHA-256 chain, and it had a real problem and a
subtler one.

**The real problem:** verifying cost the same as computing. Every node walked all
T links itself, so the delay could never be set higher than a validator would
spend — which is not much. A "delay" nobody can afford to make long is decoration.

That is what a VDF fixes, and Wesolowski's construction verifies in a few hundred
group operations *whatever T is*. Measured end to end: validation is ~16x cheaper
than mining, and the ratio widens as the delay rises.

**The subtler problem** is the one worth carrying away. The chain's input was the
stage-1 result, which depends on the nonce — so a fresh chain started on every
attempt. Picture a miner with 32 cores: it runs 32 attempts at once, each walking
its own chain. The wall-clock cost of producing a block is **one chain**, not 32.

The sequentiality was real per attempt and worth nothing per block, which is the
only level anyone cares about.

The fix is not cryptographic. The input now derives from the block header alone,
so there is exactly one chain per block and it must be walked before any nonce can
be tried:

```go
seed := sha256.Sum256(append([]byte("helios/stage2/vdf/input"), blockHeader...))
```

**Choosing the group.** Wesolowski needs a group whose order nobody knows: anyone
who knows the order `n` computes `2^T mod n` and gets the answer in one
exponentiation instead of T. The natural choice, an RSA group, has order
`(p−1)(q−1)` — known to whoever generated the modulus, who could then mine
arbitrarily fast while everyone else waited. A class group of a negative prime
discriminant has no such secret: its order is unknown even to whoever picked the
discriminant, so there is no trapdoor to hold and no setup to trust.

*Tests:* `internal/helios/vdf/`, `internal/helios/algorithm/vdf_integration_test.go`

**Lessons.** Two.

*Verify the property you actually want.* "Sequential" was true of the old chain
and irrelevant, because the property that matters is sequential **per block**, and
nobody had written that sentence down to check it against. When you claim a
security property, say precisely what it is a property *of*.

*And when a primitive needs a parameter nobody may know, ask who generates it.*
"Trusted setup" sounds procedural. Here it means one identified party can mine
infinitely fast forever, and no one else can tell. Sometimes the harder
implementation is the only honest one.

---

## Case 21: Two bugs that only appear when the machine is busy

Chaining the VDF across blocks turned up two defects that had nothing to do with
VDFs, and both were invisible on an idle machine.

**The block that invalidated itself.** Recording a mined proof overwrote the
block's timestamp with the proof's:

```go
b.Header.Nonce = uint32(proof.Nonce)
b.Header.Timestamp = proof.Timestamp   // <-- part of what was mined
```

The timestamp is in the header the miner mined against. Replacing it afterwards
left the stored proof describing a header the block no longer had, so every peer
recomputing stage 1 got a different answer and rejected the block.

It survived because the mining header records the timestamp **to the second**, and
test mining finishes inside one. At production difficulty, where mining takes many
seconds, the timestamp would move nearly every time — so nearly every mined block
would have been refused by the network, and only ever at real difficulty.

**The deadlock that needed a busy CPU.** The full test suite began hanging, but
only when run as `./...` and never when the package was run alone. Two goroutines,
both blocked on a mutex:

```
Stop()          mutex.Lock  ->  outputMu.Lock
renderStatus()  outputMu.Lock  ->  mutex.Lock
```

A textbook lock-order inversion. Stop holds `mutex` waiting for `outputMu`; the
render loop holds `outputMu` waiting for `mutex`. The window is a few instructions
wide, so it needed a shutdown to land exactly between the render loop's two
acquisitions — which is why adding CPU-heavy tests elsewhere in the repo was what
finally exposed it. It would have hung node shutdown in production for the same
reason.

The fix is to advance the spinner in the same critical section that reads it,
before the output lock is taken, so every path acquires the two locks in the same
order.

**Lessons.**

*A test that passes in isolation and fails in a suite is telling you something
about timing, not about the suite.* The instinct is to re-run it or mark it flaky.
Both of these were real, and both would have appeared in production — one on every
block at real difficulty, one at shutdown.

*Two locks are an ordering, whether or not anyone wrote it down.* The moment a
second mutex appears, every function that takes both is making a claim about the
order, and the claim has to be checked. Grepping for "which functions acquire B
while holding A" takes a minute and is the whole audit.

---

## Case 22: The memory-hard function that needed 64 bytes

Helios stage 1 was the memory-hard phase. It allocated a buffer — 1 MB by default,
64 MB originally — filled it, mixed it, and hashed the result. The whole point of
the stage is that memory is what denies an attacker the GPU and ASIC advantage: a
device can fit thousands of hash cores on a die, but not thousands of 64 MB
memories beside them.

Except the fill was a sequential chain:

```go
for j := 32; j < len(memory); j += 32 {
    hash := sha256.Sum256(memory[j-32 : j])   // depends only on the block before
    copy(memory[j:j+32], hash[:])
}
```

and the mix only combined adjacent blocks. Neither step needs the buffer to
exist. Each block regenerates from its predecessor on demand and can be fed
straight into the final hash.

Rather than argue this, it was demonstrated. A streaming implementation using **64
bytes of state** reproduced the exact output of a **131,072-byte** buffer:

```
buffer allocated by the real implementation: 131072 bytes
working set of the streaming equivalent:     64 bytes
NOT MEMORY-HARD: a 131072-byte buffer reproduced with 64 bytes of state
```

So the stage cost honest miners the full buffer and cost an attacker with custom
hardware nothing — the exact inversion of what it was for.

**The fix** is Argon2id (RFC 9106), whose data dependencies are designed so that
computing it with less memory than configured costs disproportionately more time.
The replacement test measures allocation through `runtime.MemStats` and fails if
the phase does not actually use the memory it asks for, because "it is
memory-hard" is precisely the claim that had gone unchecked.

A smaller bug fell out alongside it: the old input was
`append(blockHeader, fmt.Sprintf("%d", nonce))`, so a header ending in digits and
a nonce could collide with a different header and a different nonce — two distinct
candidates sharing stage-1 work. The nonce is now eight fixed bytes behind a
length prefix.

**Lesson.** *A name is not a property.* The function was called the memory phase,
it allocated memory, it was described in comments as "Argon2-inspired" — and none
of that made it memory-hard. The question that settles it is not "does this look
expensive?" but **"what is the cheapest way to produce this output?"** For a
sequential chain the answer is always a constant working set, and that is
determined by the data dependencies, which you can read off the loop in a minute.

Where a standard primitive exists for the property you need, the burden of proof
for rolling your own is that you can state what an attacker's cheapest strategy
costs. If you cannot, you have not designed a hard function — you have designed a
slow one, and only for yourself.

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

---

## Postscript: what the tooling caught, and what it did not

After the defects above were fixed by hand, the static analysers were turned on
properly. They found real things:

- an unchecked `tx.(*Bank)` asserted on the strength of a protocol *string*, so a
  transaction claiming to be a BANK without being one panicked the node reading
  it — reachable from disk and from the network
- PEM encoding that discarded both marshalling errors, so a wallet whose key
  could not be encoded produced a valid-looking PEM block containing nothing, was
  written to disk, and failed much later at signing with nothing connecting the
  failure to its cause
- a difficulty ceiling of `math.MaxUint32` on a value that is an *exponent* —
  `2^4294967295` is not a hard target, it is an unreachable one

And it found a great deal of noise, which is the other half of the lesson. Roughly
half the findings were correct-but-intentional: a deliberate nil context in a test
*about* nil contexts, a bit reinterpretation for hashing, a count from `len()`
that cannot be negative. Those are now annotated with the reason rather than
silenced, so the next reader learns why instead of wondering.

**What tooling will not find.** Every defect in Cases 1–18 was invisible to the
linters. No analyser knows that a signature should cover the amount, that a
coinbase belongs only in block 0, or that difficulty is an exponent on one side
of an interface and a scalar on the other. Static analysis finds *mechanical*
faults. Consensus bugs are semantic, and the only tools for those are adversarial
tests and reading the code while asking what an attacker would send.
