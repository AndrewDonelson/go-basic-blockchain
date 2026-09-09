# Wallet Guide

Wallets hold an ECDSA P-256 keypair plus a small key/value store, encrypted at
rest with a passphrase-derived key.

> **This file was rewritten.** The previous version documented functions that do
> not exist (`sdk.CreateWallet`, `wallet.SignTransaction`,
> `sdk.OpenWallet(walletData, passphrase)`) and a web interface that is not part
> of this repository. Everything below is checked against `sdk/wallet.go`.

## 🔐 How a wallet is protected

| Layer | Mechanism |
|---|---|
| Key derivation | scrypt (N=2^20, r=8, p=1 in production), 32-byte random salt per encryption |
| Encryption | AES-256-GCM, 12-byte random nonce |
| At rest | `{DATA_PATH}/wallets/{address}.json`, mode `0600` in a `0700` directory |
| Writes | Temp file → `fsync` → atomic `rename`, so an interrupted write cannot truncate a key |

### KDF parameters are recorded per wallet

`EncryptionParams` stores `ScryptN`/`ScryptR`/`ScryptP` alongside the ciphertext,
so a wallet always decrypts with the parameters it was encrypted with. Earlier
code chose the cost at runtime from `testing.Testing()` and never wrote it down —
a wallet created under test could not be opened in production, because the KDF
silently derived a different key and decryption failed with no explanation.

## 💼 Creating a wallet

### Via the API

```bash
curl -X POST http://localhost:8200/blockchain/wallets/new \
  -H "Authorization: Bearer $BLOCKCHAIN_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"name":"my-wallet","passphrase":"Passw0rd!Passw0rd!","tags":["personal"]}'
```

```json
{
  "wallet_id": "1:1:1738000000:1738000000123456789",
  "address": "…64 hex chars…",
  "name": "my-wallet",
  "tags": ["personal"]
}
```

**You supply the passphrase.** The server used to generate one and return it in
the response body — a secret travelling back over a channel the server does not
control, landing in caches, proxy logs and browser history. It no longer does.

### In code

```go
opts := sdk.NewWalletOptions(
    sdk.NewBigInt(1),                      // organizationID
    sdk.NewBigInt(1),                      // appID
    sdk.NewBigInt(1),                      // userID
    sdk.NewBigInt(time.Now().UnixNano()),  // assetID -- make it unique
    "my-wallet",
    "Passw0rd!Passw0rd!",
    []string{"personal"},
)

wallet, err := sdk.NewWallet(opts)
if err != nil {
    return err
}
```

`NewWallet` creates the keypair, derives the address, and **saves the wallet
locked**. To use it you must unlock it.

Two things to know:

- **`assetID` is honoured.** It used to be discarded and replaced with `0`, so
  every wallet in the system shared the identity `<org>:<app>:<user>:0`.
- **A new wallet has a zero balance.** It used to be created holding
  `fundWalletAmount` tokens from nowhere, and `NewBankTransaction` then checked
  affordability against that fabricated number.

## 🔑 Opening and closing

```go
// Load from disk and unlock in one step.
wallet, err := sdk.OpenWallet(address, passphrase)

// Or, on a wallet you already have in memory:
err := wallet.Unlock(passphrase)   // decrypt in place
err  = wallet.Lock(passphrase)     // encrypt in place, no disk write
err  = wallet.Close(passphrase)    // lock and persist
err  = wallet.Open(passphrase)     // load from disk and unlock
```

> ### ⚠️ If you are upgrading, read this
>
> **`Wallet.Open()` used to call `localStorage.Set` — it *wrote* the in-memory
> wallet over the stored one instead of reading it.** `LocalWalletList()` built an
> empty `Wallet` shell per file and called `Open("")` on each, so **listing your
> wallets destroyed every private key on disk**, replacing each file with a
> 169-byte stub. There was no backup path.
>
> `Open` now loads. To enumerate wallets without loading them, use
> `sdk.ListWalletAddresses()`, which reads filenames only.

Behaviour of `Open`:

- An empty passphrase loads metadata and leaves the wallet locked.
- A non-empty passphrase **must** unlock successfully or `Open` returns an error.
  It used to skip unlocking for any passphrase under 12 characters and return
  `nil`, so callers could not tell success from a silent no-op.

## 💰 Balance and data

```go
balance := wallet.GetBalance()        // from the wallet's own key/value store
name    := wallet.GetWalletName()
tags    := wallet.GetTags()

err := wallet.SetData("balance", 100.0)
v, err := wallet.GetData("some-key")  // errors if the key is absent
```

**There are two notions of balance, and they are not the same thing:**

1. `wallet.GetBalance()` — the value in the wallet's own encrypted store.
2. `blockchain.GetBalance(address)` — derived by scanning the chain: coinbase
   output credited to its recipient, bank transfers moving value between parties,
   and fees paid out to the miner and developer per `MINER_REWARD_PCT` /
   `DEV_REWARD_PCT`.

The chain is authoritative. Unifying these behind a single state model is
outstanding work — see `_design/` and the roadmap in `docs/intro.md`.

## 💸 Signing and sending

```go
tx, err := sdk.NewBankTransaction(from, to, 10.0)
if err != nil {
    return err
}

tx.Signature, err = tx.Sign([]byte(from.PrivatePEM()))
if err != nil {
    return err
}

_, err = from.SendTransaction(tx, blockchain)
```

Note `SendTransaction(tx, bc)` — the old signature took an unused leading `to
string` parameter.

> ### ⚠️ Signatures cover the whole transaction
>
> Signing operates on `SigningBytes()`, which each protocol implements over **all
> of its own fields**. Previously `Sign` marshalled only the embedded base `Tx`,
> so `Bank.Amount`, `Message.Message`, `Persist.Data` and `Coinbase.TokenCount`
> sat outside the signed bytes: a signature for 1 token verified just as happily
> for 1,000,000. If you are implementing a new protocol, **you must override
> `SigningBytes`, `Sign`, `Verify`, `Hash` and `Send`** — Go has no virtual
> dispatch on embedded structs, so inheriting them silently leaves your fields
> unsigned. See `sdk/banktx.go` for the pattern.

## 🔒 Practical guidance

**Passphrases** must be 12–24 characters with at least two each of uppercase,
lowercase, digits and specials (`testPasswordStrength` in `sdk/common.go`).

**Back up `{DATA_PATH}/wallets/`.** A lost wallet file is a lost key: there is no
mnemonic recovery wired up. `sdk/mnemonic.go` wraps BIP-39 but nothing uses it,
and there is no BIP-32 derivation or key export. This is a known gap.

**The node's own wallet** is created at startup from `NODE_WALLET_PASSPHRASE`. If
that is unset, one is generated and logged **once**, prominently — save it. It
used to be generated and immediately discarded, which meant the node wallet was
encrypted with a key nobody had and every restart orphaned another file on disk.

**Genesis wallets** no longer log their passphrases in plaintext.

## 🧪 Related tests

| Test | Guards |
|---|---|
| `TestWalletOpenDoesNotDestroyTheWalletFile` | `Open` reads, never writes |
| `TestWalletRoundTripThroughDisk` | Save → load → unlock preserves keys and data |
| `TestWalletDecryptRejectsShortCiphertext` | Truncated ciphertext errors instead of panicking |
| `TestNewWalletHonoursAssetID` | `assetID` is not discarded |
| `TestNewWalletDoesNotMintBalance` | A new wallet starts at zero |
| `TestWalletFilesAreNotWorldReadable` | Mode `0600` |
| `TestLocalStorageWritesAtomically` | Temp file + rename, correct mode, no leftovers |
| `TestSignatureCoversProtocolFields` | Tampering with a protocol field invalidates the signature |
