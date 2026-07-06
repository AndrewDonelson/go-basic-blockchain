---
name: gbb-helios-safety
description: Use this skill for Helios consensus, sidechain validation, transaction verification order, replay protection, and blockchain safety review in go-basic-blockchain.
---

# gbb-helios-safety

## Scope

- Helios stage correctness and validation flow
- Sidechain routing/rollup safety checks
- Transaction verification ordering and replay protections
- Consensus and state-integrity regression checks

## Out of Scope

- UI/UX changes
- General non-consensus refactors

## Trigger Phrases

- Helios
- consensus safety
- replay attack
- double spend
- sidechain validation

## Execution Pattern

1. Verify deterministic validation order.
2. Confirm fail-closed behavior on errors.
3. Add/strengthen tests for safety-critical paths.
4. Flag liveness or fork-choice ambiguity explicitly.
5. Run `make test`.

## Prompt Template

"Use gbb-helios-safety. Analyze this change for consensus/security regressions and add concrete tests for the highest-risk paths."
