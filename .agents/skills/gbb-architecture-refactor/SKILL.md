---
name: gbb-architecture-refactor
description: Use this skill to reorganize go-basic-blockchain into logical domain/application/infrastructure packages with interface boundaries and minimal behavioral change.
---

# gbb-architecture-refactor

## Scope

- Extract interfaces at application boundaries
- Split domain/application/infrastructure concerns
- Isolate transport and persistence adapters
- Maintain SDK compatibility during transition

## Out of Scope

- Functional rewrites without characterization tests
- Simultaneous broad migration across all modules

## Trigger Phrases

- organize packages
- clean architecture
- interfaces
- adapters
- layering

## Execution Pattern

1. Write package boundary map and move plan.
2. Extract interfaces first.
3. Move one bounded area at a time.
4. Preserve old entrypoints as adapters.
5. Run `make test` each phase.

## Prompt Template

"Use gbb-architecture-refactor. Reorganize one bounded module into domain/application/infrastructure with interfaces while preserving behavior and test stability."
