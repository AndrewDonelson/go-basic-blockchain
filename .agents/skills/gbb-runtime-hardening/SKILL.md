---
name: gbb-runtime-hardening
description: Use this skill when hardening runtime config, port binding, startup validation, and auth configuration for go-basic-blockchain. Trigger for port mismatches, config drift, hardcoded values, env wiring, and startup fail-fast checks.
---

# gbb-runtime-hardening

## Scope

- Runtime config resolution (`API_HOSTNAME`, `P2P_HOSTNAME`, env paths)
- Port unification across daemon, API, P2P, CLI, and tests
- Auth config wiring (API key/header/seed via env)
- Startup validation and fail-fast checks

## Out of Scope

- Feature development
- Consensus logic changes
- Large package refactors

## Trigger Phrases

- hardcoded key
- port mismatch
- config drift
- startup config
- bind address

## Execution Pattern

1. Identify current source of truth for runtime config.
2. Ensure server bind addresses use config, not constants.
3. Ensure clients use env-overridable endpoints/keys.
4. Add guardrails for production mode.
5. Run `make test`.

## Prompt Template

"Use gbb-runtime-hardening. Keep behavior stable while unifying runtime config and removing hardcoded runtime assumptions. Validate with make test and report changed files plus env vars."
