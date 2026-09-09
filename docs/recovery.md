# Wallet Recovery

Every wallet is derived from a BIP-39 recovery phrase, so a lost wallet file is
recoverable. Implemented in [`sdk/recovery.go`](../sdk/recovery.go).

---

## 🔑 The promise

The wallet file holds **no secret the phrase cannot regenerate**. Given the
phrase, the same seed passphrase and the same wallet identity, you get the same
private key and therefore the same address.

```go
wallet, phrase, err := sdk.NewRecoverableWallet(opts)
// ... the file is lost ...
recovered, err := sdk.NewWalletFromMnemonic(opts, phrase, "")
// recovered.GetAddress() == wallet.GetAddress()
```

`NewWallet` does this too — every wallet gets a phrase, readable via
`wallet.Mnemonic()`. Recovery that is opt-in is recovery nobody has when they
need it, which is exactly how the previous code failed: `sdk/mnemonic.go` existed
and nothing called it.

---

## ⚠️ Why `DeriveKeyPair` could not simply be wired up

The existing `DeriveKeyPair` returns **BIP-32 secp256k1** material. Every wallet,
signature and address in this project is **ECDSA P-256**. Handing secp256k1 bytes
to a wallet produces a key the chain cannot verify — which is a large part of why
nothing ever called it.

So derivation goes: phrase → BIP-39 seed → HKDF-SHA256 → P-256 scalar.

```go
d := new(big.Int).SetBytes(material)
if d.Sign() <= 0 || d.Cmp(n) >= 0 {
    continue   // resample, do not reduce
}
```

**The scalar is resampled, not reduced.** Taking a uniform value modulo `n` — the
obvious shortcut — biases the low end of the range, and biased ECDSA keys and
nonces are a well-worn route to leaking a private key. Rejection happens with
negligible probability for P-256, so the loop is a safety valve rather than an
expected path.

The HKDF `info` string (`gbb/wallet/p256/v1`) domain-separates this from every
other HKDF use in the codebase, so one seed can never produce the same bytes for
two purposes.

---

## 🧊 Two different passphrases

This trips people up and costs them funds, so it is worth stating plainly:

| Passphrase | What it does | Can it change? |
|---|---|---|
| `options.Passphrase` | Encrypts the wallet **file** at rest | Freely — choose a new one on recovery |
| `seedPassphrase` | BIP-39's "25th word", part of **key derivation** | No — a different one yields a different wallet |

The dangerous property of the second one: getting it wrong produces a valid,
empty wallet at a different address rather than an error.

---

## ✅ Phrase validation

BIP-39 phrases carry a checksum, so `ValidateMnemonic` catches a mistyped word
instead of silently deriving a different wallet. `SeedFromMnemonic` returns
`ErrInvalidMnemonic` for anything malformed.

A *wrong but valid* phrase cannot be detected — it is simply someone else's
wallet — which is why the checksum on the phrase you typed is the last line of
defence.

---

## 🚫 The phrase is never persisted

`Wallet.mnemonic` is unexported and excluded from serialisation. A recovery
phrase stored beside the wallet it recovers protects nothing.

Consequently `Mnemonic()` returns `""` for a wallet loaded from disk. **Show the
phrase to the user at creation time** — there is no way to get it back afterwards.

---

## 🚧 Still missing

- **BIP-32 derivation paths.** One phrase yields one key, not a tree of accounts.
- **Key import/export** in a standard interchange format.
- **Encrypted seed backup** files.

---

## 🧪 Tests

[`sdk/recovery_test.go`](../sdk/recovery_test.go) covers determinism, curve and
range of the derived scalar, phrase uniqueness, the seed passphrase's effect,
checksum validation, end-to-end recovery to the same address, signing with a
recovered key, and that the phrase is never serialised.

```bash
go test ./sdk/ -run 'Recover|Mnemonic|Derivation' -v
```

---

## 🔗 Related

- [Wallets & Key Storage](wallet.md) — encryption at rest
