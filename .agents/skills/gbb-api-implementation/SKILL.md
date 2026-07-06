---
name: gbb-api-implementation
description: Use this skill to implement and test currently stubbed API endpoints in go-basic-blockchain. Trigger for requests to replace Not Yet Implemented handlers, add route validation, and enforce API contracts.
---

# gbb-api-implementation

## Scope

- Implement handlers currently returning "Not Yet Implemented"
- Validate request parameters and response schema
- Keep middleware compatibility intact
- Add endpoint tests (happy/error paths)

## Out of Scope

- P2P protocol redesign
- Helios algorithm redesign

## Trigger Phrases

- implement endpoint
- Not Yet Implemented
- API contract
- route handler

## Execution Pattern

1. Build endpoint contract from route and existing tests.
2. Implement one endpoint group at a time.
3. Add/update tests before broad refactors.
4. Keep response structures backward compatible unless versioning is requested.
5. Run `make test`.

## Prompt Template

"Use gbb-api-implementation. Implement only the requested endpoint group, include validation + tests, and preserve existing API behavior unless explicitly changed."
