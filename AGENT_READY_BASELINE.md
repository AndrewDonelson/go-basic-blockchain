# AGENT READY BASELINE

Date: 2026-07-06

## Purpose

This document is the execution baseline for AI-assisted upgrades in this repository.
It captures current reality (not aspirational docs), hardening already applied, and
an actionable checklist before large migrations.

## Repository Snapshot

- Go files: 52
- Test files: 14
- Packages: 11
- Canonical test command: `make test`
- Latest canonical test result: PASS (`---END OF TESTING--- exit_code=0`)
- Canonical test log: `tmp/test.log`

## Runtime Entry Points

- Node daemon: `cmd/chaind/main.go`
- CLI client: `cmd/gbb-cli/main.go`
- Progress demo: `cmd/demo/main.go`

## Current Core Topology

- Runtime orchestration and singleton node lifecycle: `sdk/node.go`
- Chain state, mining, sidechain rollup integration, persistence loading: `sdk/blockchain.go`
- API routes and middleware wiring: `sdk/api.go`
- API key middleware and key generation: `sdk/apikey.go`
- P2P transport, queueing, handshake, node discovery: `sdk/p2p.go`
- Wallet/key/encryption logic: `sdk/wallet.go`
- Local disk persistence manager: `sdk/localstorage.go`
- Helios algorithm and validation pipeline: `internal/helios/**`

## Verified Gaps (Code-Truth)

1. API surface is larger than actual implementation.
- Many routes return "Not Yet Implemented" in `sdk/api.go` and `sdk/apiEndpointsAccount.go`.

2. Config and docs drift existed around runtime ports.
- Defaults in code and references in docs/tests differed.

3. Security/dev key handling relied on embedded defaults.
- API and CLI had hardcoded key assumptions.

4. Sidechain rollup conversion currently uses placeholder payload conversion.
- In `sdk/blockchain.go`, rollup-to-main-chain conversion does not deserialize full BANK/MESSAGE payloads yet.

5. Test suite has intentional skips.
- Useful baseline, but not complete end-to-end confidence.

## Hardening Pass Applied (2026-07-06)

1. API host binding now honors config.
- `sdk/api.go` now binds to `api.GetConfig().APIHostName` with fallback.

2. P2P bind address now honors runtime config.
- `sdk/p2p.go` resolves bind host from node config when available.
- `sdk/node.go` startup log now reflects configured P2P hostname.

3. API auth now supports environment-based configuration.
- `sdk/apikey.go` added env-driven config for API key/header and server seed.
- Backward-compatible fallback retained for local/dev continuity.

4. CLI now supports env-based endpoint/key.
- `cmd/gbb-cli/main.go` resolves API URL from `BLOCKCHAIN_API_URL` or `API_HOSTNAME`.
- API key can be sourced from `BLOCKCHAIN_API_KEY`.

## Environment Variables For Hardened Runtime

- `API_HOSTNAME` (example: `:8200`)
- `P2P_HOSTNAME` (example: `:8201`)
- `BLOCKCHAIN_API_URL` (example: `http://localhost:8200`)
- `BLOCKCHAIN_API_KEY`
- `BLOCKCHAIN_API_EMAIL`
- `API_KEY_HEADER` (default `Authorization`)
- `BLOCKCHAIN_SERVER_SEED`

## Agent Readiness Checklist

### A. Stability Baseline

- [x] Canonical test pass captured with sentinel log
- [x] Core entry points identified
- [x] Primary runtime paths mapped
- [ ] Add one minimal e2e smoke test (daemon + API + CLI health)

### B. Contract Clarity

- [ ] Publish "implemented endpoints" vs "stub endpoints" matrix
- [ ] Pin single source of truth for ports (code + docs + examples)
- [ ] Add API response contract tests for implemented handlers

### C. Security Readiness

- [x] Configurable API key path added
- [x] Configurable seed path added
- [ ] Remove legacy fallback key for production mode
- [ ] Add startup guard: fail-fast in non-test env without explicit API key
- [ ] Add rate limiting for authenticated endpoints

### D. Refactor Readiness

- [ ] Define domain/application/infrastructure package boundaries
- [ ] Extract interfaces at application boundaries first
- [ ] Move persistence behind repository interfaces
- [ ] Move transport (API/P2P/CLI) behind adapters

### E. Upgrade Readiness

- [ ] Introduce Strata-backed repositories behind interfaces
- [ ] Add migration/bootstrapping lifecycle aligned with backend-template
- [ ] Add phased conversion matrix and stop/go quality gates per phase

## Recommended Execution Order

1. Complete endpoint contract matrix and kill drift.
2. Add fail-fast production auth config checks.
3. Introduce application interfaces and adapter layer without moving behavior.
4. Migrate persistence to Strata-backed repositories behind interfaces.
5. Adopt backend-template structure incrementally (no big-bang rewrite).

## Acceptance Criteria For Next Milestone

- Single config source for API/P2P ports used by daemon, tests, and CLI.
- No embedded production secrets.
- Clear package boundaries documented and scaffolded.
- Strata repository adapter proof-of-concept merged for one bounded domain.
- `make test` remains green after each phase.
