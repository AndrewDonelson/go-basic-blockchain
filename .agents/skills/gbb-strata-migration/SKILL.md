---
name: gbb-strata-migration
description: Use this skill to migrate go-basic-blockchain persistence slices to Strata behind interfaces using phased, test-gated conversion.
---

# gbb-strata-migration

## Scope

- Introduce Strata-backed repository adapters
- Register schemas and migration lifecycle
- Migrate one bounded model at a time
- Preserve existing behavior via compatibility path

## Out of Scope

- Big-bang persistence replacement
- Consensus/P2P redesign

## Trigger Phrases

- Strata migration
- repository adapter
- persistence refactor
- schema migration

## Execution Pattern

1. Select one bounded model.
2. Add interface + Strata adapter.
3. Add tests and keep existing backend path.
4. Flip reads/writes gradually with rollback path.
5. Run `make test`.

## Prompt Template

"Use gbb-strata-migration. Convert one persistence slice to Strata behind interfaces, keep compatibility fallback, and validate with make test."
