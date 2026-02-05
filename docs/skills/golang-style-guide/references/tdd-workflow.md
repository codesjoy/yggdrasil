# TDD Workflow (Practical)

## Default loop
1. **Red**: write a failing test that describes the desired behavior.
2. **Green**: implement the minimal code to make it pass.
3. **Refactor**: clean up design, remove duplication, improve names.

## Test pyramid (guidance)
- Prefer many fast unit tests (domain + use cases).
- Add fewer integration tests (DB adapters, external APIs).
- Add minimal end-to-end tests when needed for confidence.

## Characterization tests (legacy code)
When behavior is unclear or risky to change:
- Write tests that lock in current behavior first.
- Refactor behind those tests.

## What to unit test (Clean/Hex-friendly)
- Domain invariants: constructors/methods enforce rules.
- Use cases: orchestration and decision logic.
- Edge cases: validation, missing data, conflict states.

## Definition of Done (per change)
- New behavior has unit tests.
- Negative/edge cases are covered.
- Tests are deterministic (no real time/network unless integration).
- `go test ./...` passes locally.
- Lint passes (if enabled).
