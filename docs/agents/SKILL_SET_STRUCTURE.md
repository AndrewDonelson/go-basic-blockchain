# Skill Set Structure For go-basic-blockchain

Date: 2026-07-06

## Goal

Provide repository-local agent skills that are narrowly scoped, trigger reliably,
and are aligned to this codebase's real architecture and upgrade roadmap.

## Proposed Repository Skill Tree

```text
.agents/
  skills/
    gbb-runtime-hardening/
      SKILL.md
    gbb-api-implementation/
      SKILL.md
    gbb-helios-safety/
      SKILL.md
    gbb-strata-migration/
      SKILL.md
    gbb-architecture-refactor/
      SKILL.md
```

## Skill Definitions

### 1) gbb-runtime-hardening

- Files: `.agents/skills/gbb-runtime-hardening/SKILL.md`
- Scope:
  - Config/port consistency
  - env variable and startup validation
  - removal of hardcoded runtime assumptions
- Out of scope:
  - new product features
  - large package moves
- Triggers:
  - "port mismatch", "config drift", "hardcoded key", "startup config"
- Prompt starter:
  - "Use gbb-runtime-hardening. Keep behavior stable, harden runtime config and auth config paths, and preserve make test green."

### 2) gbb-api-implementation

- Files: `.agents/skills/gbb-api-implementation/SKILL.md`
- Scope:
  - convert stub handlers to production handlers
  - request/response validation
  - auth middleware compatibility
  - endpoint tests for each implemented route
- Out of scope:
  - consensus algorithm changes
- Triggers:
  - "Not Yet Implemented", "implement endpoint", "API contract"
- Prompt starter:
  - "Use gbb-api-implementation. Implement only requested endpoints and add tests for happy path and error path."

### 3) gbb-helios-safety

- Files: `.agents/skills/gbb-helios-safety/SKILL.md`
- Scope:
  - consensus safety checks
  - transaction validation ordering
  - sidechain and rollup validation correctness
- Out of scope:
  - frontend/CLI UX work
- Triggers:
  - "consensus", "Helios", "validation", "double spend", "replay"
- Prompt starter:
  - "Use gbb-helios-safety. Review the change for safety/liveness/regression risk and provide concrete tests."

### 4) gbb-strata-migration

- Files: `.agents/skills/gbb-strata-migration/SKILL.md`
- Scope:
  - introduce Strata-backed persistence adapters
  - schema registration and migration flow
  - phased migration from local JSON persistence
- Out of scope:
  - replacing consensus/P2P logic
- Triggers:
  - "Strata", "migrate persistence", "repository adapter"
- Prompt starter:
  - "Use gbb-strata-migration. Migrate one bounded persistence slice at a time behind interfaces."

### 5) gbb-architecture-refactor

- Files: `.agents/skills/gbb-architecture-refactor/SKILL.md`
- Scope:
  - package boundary cleanup
  - interface extraction at application boundaries
  - adapter isolation (HTTP/P2P/CLI/storage)
- Out of scope:
  - behavior rewrites without characterization tests
- Triggers:
  - "organize packages", "interfaces", "clean architecture", "refactor"
- Prompt starter:
  - "Use gbb-architecture-refactor. Keep behavior unchanged while extracting interfaces and adapters incrementally."

## Integration Of Ready-Made Local Skills

External local skills directory:
- `/home/andrew/Documents/Agent-Skills`

Recommended usage in this repo:

1. Keep repository-local skills as orchestration wrappers.
2. Delegate domain depth to specialized external skills:
- `go-blockchain-security` for consensus/key/P2P hardening
- `go-evm-compat` for EVM execution path work
- `zk-rollup-architecture` for L2 architecture
- `cc-skills-golang-main` for Go testing/perf/safety patterns

## Trigger Policy

- Trigger narrow skill first.
- Trigger one secondary expert skill only when needed.
- Avoid loading broad skills unless task scope truly requires them.

## Success Criteria

- Skills produce deterministic plans and limited-scope edits.
- Skills always include verification steps (`make test`).
- Skills reduce regressions in high-risk areas (API, consensus, persistence).
