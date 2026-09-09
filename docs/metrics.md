# Observability

`GET /metrics` exposes node metrics in the Prometheus text exposition format.
Implemented in [`sdk/metrics.go`](../sdk/metrics.go).

---

## 📉 What it replaces

Nothing about a running node was measurable. The status struct carried a literal
`HashRate: 0, // TODO`, and there were no counters at all — so questions like "is
this node rejecting blocks?", "is the mempool churning?" or "did peers fail
authentication?" had no answer short of reading logs.

---

## 🔢 What is exported

**Counters** (monotonic — a collector can difference two samples without knowing
whether it missed one):

| Metric | Meaning |
|---|---|
| `gbb_blocks_mined` | Blocks mined by this node |
| `gbb_blocks_accepted` | Blocks accepted onto the chain |
| `gbb_blocks_rejected` | Blocks refused by validation |
| `gbb_reorgs` / `gbb_reorg_blocks` | Reorganisations, and blocks rolled back |
| `gbb_tx_submitted` / `gbb_tx_rejected` / `gbb_tx_mined` | Mempool and block inclusion |
| `gbb_mempool_evicted` | Transactions evicted or trimmed |
| `gbb_peers_connected` / `gbb_peer_auth_failed` | Handshake outcomes |

**Gauges** (a current reading):

| Metric | Meaning |
|---|---|
| `gbb_hash_rate` | Estimated hashes per second |
| `gbb_average_block_time_seconds` | Mean recent block interval |
| `gbb_chain_height`, `gbb_difficulty` | Chain position |
| `gbb_mempool_size`, `gbb_utxo_count`, `gbb_total_supply` | State sizes |
| `gbb_peers`, `gbb_sync_passes`, `gbb_sync_blocks_pulled` | Network |
| `gbb_uptime_seconds` | Process lifetime |

Counters and gauges are typed correctly in the output. This matters: labelling a
counter as a gauge loses `rate()` entirely for whoever scrapes it.

---

## ⚡ How the hash rate is estimated

Expected work for difficulty `d` is `2^d` hashes, so each block contributes
`2^d / interval` and the estimate is the **mean of those per-block rates** over a
20-block window.

Two decisions worth explaining:

- **A window, not one interval.** Proof of work is a Poisson process; consecutive
  block times routinely differ by an order of magnitude. A single interval is
  noise, not a measurement.
- **Mean of rates, not total work over total time.** Averaging the per-block
  rates keeps one unusually fast block from dominating, and it handles a window
  whose difficulty changed partway through — which is now normal, since
  [difficulty retargets](difficulty.md).

Degenerate inputs contribute nothing rather than distorting the average:
identical timestamps, backwards timestamps, and a zero timestamp are all skipped.
`exp2` saturates at `maxAcceptableDifficulty` rather than at float64's limit,
because `2^1023` divided by a sub-second interval overflows to `+Inf` — which
would poison the entire average, not just that sample.

---

## 🔓 Why the endpoint is public

`/metrics` sits alongside `/health` in `publicPaths`. A metrics endpoint behind
authentication is one no scraper will be configured for, and nothing exported
here is a secret: counts, timings and a hash rate — no addresses, balances, keys
or peer identities.

---

## 🧩 Live gauges are not mirrored

Values the recorder cannot know are passed in at render time rather than being
copied into it:

```go
extra["mempool_size"] = float64(api.bc.GetMempoolSize())
extra["peers"] = float64(node.P2P.PeerCount())
```

The mempool knows its own depth, the P2P layer knows its peer count, and the
syncer already keeps its own stats. Mirroring them into the recorder would create
a second copy to drift out of date.

---

## 🧪 Tests

[`sdk/metrics_test.go`](../sdk/metrics_test.go) covers counters, unknown-metric
tolerance, nil-receiver safety, hash rate against known work and time, scaling
with difficulty, degenerate intervals, window bounding, overflow saturation,
exposition format and typing, stable ordering, concurrency, and that the counters
are genuinely wired to the accept, mine and mempool paths.

```bash
go test ./sdk/ -run 'Metrics|HashRate|Prometheus' -v
```

---

## 🔗 Related

- [Dynamic Difficulty](difficulty.md) — why a window may span a difficulty change
- [API Reference](api.md)
