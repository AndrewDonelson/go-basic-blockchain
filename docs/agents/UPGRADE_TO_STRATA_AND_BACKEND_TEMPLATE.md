# Upgrade Blueprint: Strata + com.nlaak.backend-template + Package Interfaces

Date: 2026-07-06

## Objective

Upgrade this repository to:

1. use Strata for persistence patterns,
2. adopt proven backend-template layering patterns,
3. reorganize current SDK-heavy code into logical packages with interfaces,
4. preserve behavior and keep `make test` green each phase.

## Non-Goals (Initial Migration)

- No big-bang rewrite.
- No simultaneous consensus redesign and persistence migration.
- No endpoint contract changes without explicit versioning plan.

## Source Pattern References

- Strata: `github.com/AndrewDonelson/strata`
- Backend template: `github.com/AndrewDonelson/com.nlaak.backend-template`

High-value patterns to import:

- clear domain/application/infrastructure boundaries,
- interface-first application services,
- thin HTTP handlers,
- Strata-backed repository adapters,
- additive migration discipline,
- phased conversion workflow.

## Target Package Layout (Incremental)

```text
cmd/
  chaind/
  gbb-cli/
  demo/

internal/
  domain/
    blockchain/
    wallet/
    transaction/
    node/

  application/
    interfaces.go
    services/
      blockchain_service.go
      wallet_service.go
      tx_service.go
      node_service.go

  infrastructure/
    config/
    http/
    p2p/
    storage/
      localjson/
      strata/
    logging/

  helios/
    algorithm/
    difficulty/
    sidechain/
    validation/

sdk/ (temporary compatibility facade during migration)
```

## Interface Boundaries (First Extraction)

Create these interfaces in `internal/application/interfaces.go`:

- `BlockchainRepository`
  - LoadState/SaveState
  - LoadBlocks/SaveBlock
  - Query blocks/tx history

- `WalletRepository`
  - SaveWallet/LoadWallet/ListWallets

- `TxQueue`
  - Enqueue/Dequeue/List/Remove

- `ConsensusEngine`
  - Mine/ValidateProof/AdjustDifficulty

- `NetworkTransport`
  - Broadcast/Register/ListPeers/Start/Stop

- `AuthProvider`
  - ValidateAPIKey/IssueKey

## Strata Adoption Strategy

### Phase S1: Strata Adapter Proof of Concept

- Add Strata data store initialization package in `internal/infrastructure/storage/strata`.
- Define schema for one bounded model (recommended: node metadata or API account metadata).
- Keep existing local JSON persistence as primary path.
- Add adapter behind repository interface.

Exit criteria:
- New adapter compiles and is covered by tests.
- Old behavior unchanged by default.

### Phase S2: Dual-Write for One Slice

- For selected model, write to local JSON + Strata.
- Read remains from local JSON for safety.
- Add consistency checks in tests.

Exit criteria:
- dual-write stable for one full test pass.

### Phase S3: Read Switch for One Slice

- Flip reads to Strata for migrated model.
- Keep local JSON as fallback during transition window.

Exit criteria:
- no behavior regressions in tests.

### Phase S4+: Expand Model-by-Model

Recommended order:
1. node metadata
2. API/auth metadata
3. wallets
4. transaction index
5. block metadata/state

## Backend-Template Pattern Adoption

Adopt these patterns directly:

1. App layering discipline
- domain: pure models/rules
- application: use cases + interfaces
- infrastructure: concrete adapters

2. Thin transport handlers
- move business logic out of API handlers into application services.

3. Strata migration discipline
- additive-only migration strategy.

4. Operational workflow
- phased conversion with per-phase validation and stop/go gates.

## Compatibility Strategy For Existing `sdk/`

- Keep `sdk/` as compatibility facade while extracting internals.
- Re-route SDK methods to application services progressively.
- Mark compatibility adapters with explicit deprecation comments only after migration is stable.

## Phase Plan (Execution)

### Phase 0 (Done)
- runtime hardening: config-based API/P2P binding and auth de-hardcoding path.

### Phase 1
- introduce `internal/application/interfaces.go` and service skeletons.
- no behavior changes.

### Phase 2
- move API route behavior to application services (one feature group at a time).

### Phase 3
- add Strata storage adapter and migrate first bounded model.

### Phase 4
- split P2P and blockchain runtime adapters from SDK package.

### Phase 5
- complete endpoint implementation matrix and remove route stubs.

## Risk Register

1. Global singleton coupling (`node`, `Args`) can break parallel tests.
2. Partial sidechain conversion currently uses placeholder payload mapping.
3. API surface has many stubs; migration may accidentally lock in incomplete contracts.
4. Port/auth drift can reappear if docs/config/code are not updated together.

## Quality Gates Per Phase

- `make test` pass with sentinel output.
- Updated docs for any behavior changes.
- No interface-breaking change without migration notes.
- One PR per bounded phase.

## Immediate Next Implementation Slice

1. Add `internal/application/interfaces.go`.
2. Add `internal/application/services/blockchain_service.go` with wrapper methods calling existing SDK behavior.
3. Wire one API endpoint group through service layer.
4. Keep everything else untouched.
