# Stage 1: Argon2id

Helios stage 1 is **Argon2id** (RFC 9106), the standard memory-hard function.
It replaced a hand-rolled buffer fill that provided **no memory hardness at all**.

---

## 🔓 What was wrong

The previous phase allocated a buffer, filled it, mixed it, and hashed the
result. The fill was a sequential chain:

```go
for j := 32; j < len(memory); j += 32 {
    hash := sha256.Sum256(memory[j-32 : j])   // depends only on the block before
    copy(memory[j:j+32], hash[:])
}
```

and the mix only combined **adjacent** blocks. Neither step needs the buffer to
exist. Each block can be regenerated from its predecessor on demand and fed
straight into the final hash, so the whole phase computes in a constant working
set.

That was demonstrated before the replacement, not argued: a streaming
implementation using **64 bytes** reproduced the exact output of a **131,072-byte**
buffer.

This matters because memory is the *entire point* of the stage. Memory is what
denies an attacker the GPU and ASIC advantage — a device can fit thousands of
hash cores on a die, but it cannot fit thousands of 64 MiB memories beside them.
A phase that streams in 64 bytes hands that advantage straight back, while
costing honest miners the full buffer.

---

## 🔐 Argon2id

Argon2id won the Password Hashing Competition and is specified in RFC 9106. Its
data dependencies are designed so that computing it with less memory than
configured costs disproportionately more time — which is the property the old
phase merely asserted.

```go
password := memoryPhaseInput("helios/stage1/argon2/password", blockHeader, nonce)
salt     := memoryPhaseInput("helios/stage1/argon2/salt",     blockHeader, nonce)
argon2.IDKey(password, salt, timeCost, memoryKiB, parallelism, keyLength)
```

**The salt is derived, not random.** A verifier has to reproduce this exactly, and
a random salt would make the stage unverifiable — the defect the whole Helios
rewrite began from. It varies per candidate so work cannot be amortised across
nonces, and it is domain-separated from the password so the two are never equal.

The nonce is encoded as **eight fixed bytes**, not decimal text. The old code
appended `fmt.Sprintf("%d", nonce)`, so a header ending in digits and a nonce
could collide with a different header and a different nonce — two distinct
candidates sharing stage-1 work.

---

## ⚙️ Parameters — all four are consensus-critical

| Field | Default | |
|---|---|---|
| `Argon2MemoryKiB` | 65536 (64 MiB) | RFC 9106 second recommendation |
| `Argon2TimeCost` | 1 | |
| `Argon2Parallelism` | 1 | |
| `Argon2KeyLength` | 32 | |

Argon2's output depends on **every one** of these, so two nodes configured
differently compute different stage-1 results and reject each other's blocks. They
are not tuning knobs. `TestParametersAreConsensusCritical` confirms that changing
any one changes the digest.

Parallelism is 1 deliberately: raising it changes the digest, so it cannot be set
per machine, and 1 keeps a single attempt costing one core rather than several.

Values are validated rather than trusted — memory below `8 × parallelism` KiB (the
RFC minimum) or a key shorter than 16 bytes is refused. A misconfiguration
otherwise presents as blocks being mysteriously refused by the network.

---

## 📊 What this costs

Measured on this machine, per nonce attempt:

| Memory | Per attempt | Attempts/sec/core |
|---|---|---|
| 8 MiB | 4 ms | 253 |
| 32 MiB | 15 ms | 65 |
| **64 MiB** | **29 ms** | **34** |

That is the intended shape for a memory-hard proof of work: a few dozen expensive
attempts per second rather than billions of cheap ones. **Difficulty must be set
to match**, which the chain-derived retargeting does on its own — but do not
expect hash-rate figures that look like SHA-256 mining, because the work is a
different thing.

The test configuration uses 8 MiB. The suite mines blocks, and 64 MiB per attempt
would dominate its runtime; 8 MiB still exercises the real KDF rather than a stub.

---

## 🧪 Tests

[`internal/helios/algorithm/argon2_test.go`](../internal/helios/algorithm/argon2_test.go)
checks the stage against a direct `argon2.IDKey` call, that it **actually
allocates** the memory it claims (the property the old phase lacked, measured
through `runtime.MemStats`), determinism, per-candidate separation, input
unambiguity, parameter validation, consensus-criticality of all four parameters,
and that a fabricated stage-1 result is refused end to end.

---

## 🔗 Related

- [Helios Consensus](helios.md) — the three stages
- [Verifiable Delay Function](vdf.md) — stage 2
