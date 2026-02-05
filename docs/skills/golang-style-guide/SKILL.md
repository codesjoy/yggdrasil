---
name: golang-style-guide
description: Apply Go coding standards, monorepo project layout (including go.work), DDD with a Clean/Hexagonal bias, and TDD/testing practices. Use when designing Go repo structure, reviewing Go code style/idioms, defining domain boundaries, or writing tests (table tests, testify, gomock).
---

# Golang Style Guide (Monorepo + DDD + TDD)

## Workflow Decision Tree
- If the request is about repo layout, modules, monorepo, or `go.work`: read `references/layout-monorepo.md`.
- If the request is about Go coding conventions and review standards: read `references/coding-standards.md`.
- If the request is about DDD boundaries, Clean/Hex architecture, ports/adapters: read `references/ddd-clean-architecture.md`.
- If the request is about TDD approach and "what to test": read `references/tdd-workflow.md`.
- If the request is about writing tests, table tests, testify, gomock conventions: read `references/testing-toolkit.md`.
Note: If the request is about Makefile/CI/tooling gates (fmt/lint/test/tidy), use `$makefile-scaffold`.

## Defaults
- Prefer a Kratos-inspired (adapted) layering for boundaries and testability:
  - `internal/core`: domain core (entities/value objects/invariants + ports)
  - `internal/service`: application + handler layer (API implementation + orchestration)
  - `internal/data`: infra adapters + repository implementations
  - `internal/conf`: configuration structs
  - transport server wiring lives in `cmd/<app>/` (no `internal/server`)
- Prefer unit tests by default, table-driven style; add integration tests selectively.

## Non-Negotiables Checklist
- Run `gofmt` on all Go code; keep formatting and imports consistent.
- Use `context.Context` for request-scoped work; set timeouts at boundaries.
- Wrap errors with context (`fmt.Errorf("...: %w", err)`); avoid stringly-typed error handling.
- Keep packages small and cohesive; avoid cyclic dependencies.
- Keep tests deterministic and fast; avoid real time/network unless explicitly integration tests.
- Avoid global mutable state; inject dependencies via constructors and interfaces.

## Output Expectations (when asked to design/implement)
- Provide a directory tree (or diff) for the chosen layout.
- Define key interfaces (ports) and concrete adapters.
- Provide a test plan (unit vs integration), plus the exact commands to run locally and in CI.
  - For Makefile/CI gates/templates, refer to `$makefile-scaffold`.
