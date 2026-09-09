# API Reference

This document describes the API **as implemented**. Every route below is
registered in `sdk/api.go:registerRoutes`; nothing here is aspirational.

> **This file was rewritten.** The previous version documented a completely
> different API — `/api/blockchain/status`, `/api/wallet/create`,
> `/api/mining/start`, `/api/network/peers` and so on. None of those routes ever
> existed in the code. If you built a client against the old document, none of it
> will work.

## 🔗 Base URL

```
http://localhost:8200
```

Set by `API_HOSTNAME` (default `:8100`; the shipped `.env.example` uses `:8200`).

## 🔐 Authentication

Every route except the public ones below requires a bearer token:

```bash
curl -H "Authorization: Bearer $BLOCKCHAIN_API_KEY" http://localhost:8200/blockchain
```

**Authentication fails closed.** If `BLOCKCHAIN_API_KEY` is not configured, the
server refuses to start the API middleware and every authenticated request is
rejected. There is deliberately no built-in fallback key — an earlier version
shipped one as a constant in the source, which meant any unconfigured deployment
accepted a credential published in this repository.

Two kinds of key are accepted:

1. **The configured static key** — `BLOCKCHAIN_API_KEY`, attributed to the
   principal in `BLOCKCHAIN_API_EMAIL`.
2. **A key issued to a verified account** — returned by `/account/login` and
   `/account/verify`. These are random per issuance and stored only as a SHA-256
   hash.

All key comparisons are constant-time, and authentication attempts are rate
limited per source address (10 per minute; a success clears the counter).

**Public paths** (no key required): `/`, `/version`, `/info`, `/health`,
`/metrics`, `/account/register`, `/account/login`, `/account/verify`.

Authentication is attached to the **router**, not to the server. It used to be
installed in `Start()`, so `api.router` on its own was an unauthenticated API and
anything serving it directly — an embedder, or a test harness wiring it into its
own `http.Server` — silently got no authentication at all.

### Versioning

Every endpoint is served under **`/v1`**. The same endpoints are also served at
their historic unprefixed paths, so nothing that works today stops working.

Versioning is additive on purpose: moving the endpoints outright would break
every existing client the day it shipped, to buy nothing until there is a second
version to distinguish from. The unprefixed surface is frozen — it will not gain
new endpoints — and new clients should use `/v1`.

`TestLegacyPathsMirrorTheVersionedMount` checks the two have not drifted apart,
and `TestVersionedAndLegacyMountsBehaveIdentically` that they answer the same.

### Machine-readable specification

[`api/openapi.yaml`](../api/openapi.yaml) is the contract — 25 paths, schemas and
auth. Before it, the only description of the API was a Postman collection, so a
client author had to read the handlers.

`TestOpenAPISpecCoversEveryRoute` and its counterparts check the document against
the real router in both directions, so an undocumented endpoint or a documented
one that is not served fails the build. Writing them immediately caught five
paths in a draft spec that the code has commented out.

### Error shape

**Every** error is this JSON envelope. Handlers previously mixed it with
`http.Error`'s plain text, and which one you got depended on the handler rather
than the kind of error, so a client could not parse a failure without guessing.

```json
{ "message": "invalid api key", "status": 401 }
```

`status` is repeated in the body because a client holding only the payload — a
log line, a queued webhook — otherwise cannot tell a 400 from a 500.

| Status | Meaning |
|---|---|
| `400` | Malformed request (bad JSON, missing required field) |
| `401` | Missing, malformed or unrecognised credential |
| `404` | Resource does not exist |
| `422` | Well-formed but semantically rejected (bad signature, insufficient funds, block does not extend the head) |
| `429` | Rate limited |
| `503` | Subsystem unavailable (e.g. consensus auth not configured) |

---

## 📋 Public Endpoints

### `GET /` — Home

HTML summary of the chain parameters.

### `GET /version`

```json
{ "version": "0.1.0" }
```

### `GET /info`

```json
{
  "version": "0.1.0",
  "name": "Go Basic Blockchain",
  "symbol": "GBB",
  "block_time": 20,
  "difficulty": 2,
  "fee": 0.05
}
```

### `GET /health`

```json
{ "status": "ok", "block_count": 42, "mempool_size": 3 }
```

---

## 👤 Account Endpoints

Registration and login are **`POST` with a JSON body**. They were previously
`GET` with the credential in the query string, which put it into access logs,
proxy logs and browser history.

### `POST /account/register`

```bash
curl -X POST http://localhost:8200/account/register \
  -H "Content-Type: application/json" \
  -d '{"email":"user@example.com","password":"a sufficiently long password"}'
```

```json
{
  "status": "ok",
  "message": "If the address is eligible, a verification link has been sent."
}
```
`202 Accepted`.

Notes, all of them deliberate:

- **The verification token is not in the response.** It is delivered out of band —
  by email when `GMAIL_EMAIL`/`GMAIL_PASSWORD` are configured, otherwise written
  to the node log. Returning it made email verification a formality anyone could
  skip.
- **The response is identical whether or not the address is already registered**,
  so this endpoint cannot be used to enumerate accounts.
- **Re-registering an already-verified address does nothing.** Previously it
  created a pending record, handed back the token, and let the caller replace the
  victim's stored credential.
- The password must be 12–128 characters. It is hashed server-side with scrypt
  and a per-account random salt; the server never stores or sees a client-supplied
  hash.

### `GET /account/verify?email=…&token=…`

Followed from the emailed link. Returns a freshly issued API key:

```json
{ "status": "ok", "api_key": "…64 hex chars…" }
```

The token is compared in constant time against a stored hash and expires after
30 minutes.

### `POST /account/login`

```bash
curl -X POST http://localhost:8200/account/login \
  -H "Content-Type: application/json" \
  -d '{"email":"user@example.com","password":"a sufficiently long password"}'
```

```json
{ "status": "ok", "api_key": "…64 hex chars…" }
```

Each login issues a **new random key** and supersedes the previous one. An
unknown address and a wrong password return the same `401`.

---

## ⛓️ Blockchain Endpoints

### `GET /blockchain`

```json
{ "num_blocks": 42, "num_transactions_in_queue": 3 }
```

### `GET /blockchain/blocks?page=1&limit=10`

Returns a **paginated envelope**, not a bare array:

```json
{
  "page": 1,
  "limit": 10,
  "total": 42,
  "blocks": [ { "header": { … }, "transactions": [ … ], "index": "0", "hash": "…" } ]
}
```

`limit` is clamped to 1000. A page past the end returns an empty `blocks` array
rather than the last block (and no longer panics on an empty chain).

### `GET /blockchain/blocks/{index}`

One block by its **block index**, not its slice position. `404` if absent.

A block includes its `helios_proof`, which is what makes it independently
verifiable after it has been written to disk.

### `GET /blockchain/blocks/{index}/transactions`

Every transaction in the block.

### `GET /blockchain/blocks/{index}/transactions/{id}`

One transaction by ID. If `{id}` matches a protocol name (`BANK`, `MESSAGE`,
`COINBASE`, `PERSIST`, `CHAIN`, `P2P`) the endpoint instead returns every
transaction in that block using that protocol.

### `GET /blockchain/transactions`

Every transaction in the chain plus the mempool.

### `GET /blockchain/transactions/{id}`

One transaction by ID, or — as above — all transactions of a protocol.

---

## 💰 Wallet Endpoints

### `POST /blockchain/wallets/new`

**The caller supplies the passphrase.** The server no longer generates one and
returns it in the response body.

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
`201 Created`. A passphrase that fails the strength check returns `400`.

### `GET /blockchain/wallets`

```json
[ { "wallet_id": "…", "address": "…", "encrypted": true, "balance": 0 } ]
```

### `GET /blockchain/wallets/{id}` — `{id}` is the address

```json
{ "wallet_id": "…", "address": "…", "encrypted": true, "balance": 12.5 }
```

### `POST /blockchain/wallets/{id}` — update name/tags

```json
{ "passphrase": "…", "name": "renamed", "tags": ["a","b"] }
```

Requires the wallet passphrase; a wrong one returns `401`.

### `GET /blockchain/wallets/{id}/balance`

```json
{ "address": "…", "balance": 12.5 }
```

Balance is derived from the chain: coinbase output is credited to its recipient,
bank transfers move value between parties, and fees are paid out to the miner and
developer addresses per `MINER_REWARD_PCT` / `DEV_REWARD_PCT`.

### `GET /blockchain/wallets/{id}/transactions`

Transaction history for the address.

### `GET /blockchain/wallets/{id}/transactions/{txid}`

One transaction from that history, or all of a protocol.

### `POST /blockchain/wallets/tx` — submit a transaction

```bash
curl -X POST http://localhost:8200/blockchain/wallets/tx \
  -H "Authorization: Bearer $BLOCKCHAIN_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
        "protocol":"BANK",
        "from":"<sender address>",
        "to":"<recipient address>",
        "amount":10.0,
        "passphrase":"<sender passphrase>"
      }'
```

```json
{
  "status": "ok",
  "accepted": true,
  "id": "1:1:1:8624972568939678",
  "protocol": "BANK",
  "from": "…",
  "to": "…",
  "amount": 10.0
}
```
`202 Accepted`.

This endpoint **builds, signs and enqueues a real transaction**. It previously
validated the request and returned `{"accepted": true}` without creating, signing
or storing anything.

- `protocol` is `BANK` (requires `amount > 0`) or `MESSAGE` (requires `message`).
- `passphrase` unlocks the sender's stored wallet so the transaction can be
  signed. An unknown or unopenable sender returns `401`.
- Insufficient funds, a zero amount or an empty message return `422`.

---

## 🤝 Consensus Endpoints

Node-to-node. Guarded by a separate check that also fails closed: with no API key
configured these return `503`, not `200`.

### `POST /consensus/tx`

Body: a transaction in the canonical wire encoding (what `GET /blockchain/transactions/{id}` returns).

```json
{ "status": "ok", "accepted": true, "id": "…" }
```

| Outcome | Status |
|---|---|
| Signature verifies, transaction enqueued | `202` |
| Already known | `200` with `"status":"duplicate"` |
| Undecodable or unknown protocol | `400` |
| Invalid, unsigned, or signature mismatch | `422` with a `reason` |

The signature is verified against the sender's public key, which travels with the
transaction. This endpoint previously parsed the body into a discarded map and
answered `{"accepted": true}` for any payload at all.

### `POST /consensus/block`

Body: a block in the canonical wire encoding.

| Outcome | Status |
|---|---|
| Extends the head, validates, proof of work verifies | `202` |
| Undecodable | `400` |
| Does not extend the head, or fails validation | `422` with a `reason` |

**A block that does not extend the current head is refused.** Fork choice and
reorganisation are not implemented, so the honest answer is a rejection rather
than a silent acceptance.

### `POST /consensus/p2p`

Enqueues a `P2PTransaction` envelope onto the P2P processing queue. `201` on
success.

---

## 🔒 Transport & limits

- Request bodies are capped at 1 MiB.
- The server sets read-header (10s), read (30s), write (30s) and idle (120s)
  timeouts and a 1 MiB header cap. The zero-value `http.Server` it used to rely on
  has none of these, which leaves it open to Slowloris-style connection exhaustion.
- There is **no TLS**. Terminate it in front of the node.
- `X-Forwarded-For` is ignored unless `TRUST_PROXY_HEADERS=true`, because it is
  client-controlled.
