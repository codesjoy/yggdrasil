# Go Coding Standards

## Formatting and imports
- Always run `gofmt`.
- Prefer `goimports` if your repo uses it; keep imports grouped consistently.

## Package boundaries
- Keep packages small and cohesive (single responsibility).
- Avoid cyclic dependencies; refactor into interfaces/ports if needed.
- Prefer `internal/` for application code; use `pkg/` only for widely reused stable libraries.

## Naming
- Exported identifiers use UpperCamelCase; unexported use lowerCamelCase.
- Avoid stutter in package names (e.g., `user.UserService`).
- Use clear, domain terms instead of technical noise.

## Context
- Accept `context.Context` as the first parameter for request-scoped functions.
- Set timeouts at boundaries (handlers, cron jobs, consumers), not deep inside helpers.

Example:
```go
ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
defer cancel()
```

## Errors
- Wrap errors with context: `fmt.Errorf("op: %w", err)`.
- Prefer sentinel errors only when the caller needs to branch on behavior.
- Do not log and return the same error at multiple layers; pick one layer (usually boundary) to log.

## Logging
- Log at boundaries: HTTP/gRPC handlers, background job entrypoints, message consumer loops.
- Include stable keys (request_id, user_id, order_id) rather than large payload dumps.

## Concurrency
- Prefer structured concurrency via contexts and errgroup patterns.
- Avoid goroutine leaks: always ensure cancellation/termination paths exist.
- Protect shared state with mutexes; prefer immutable data flow where possible.

## PR review checklist
- Is code formatted (`gofmt`) and imports are clean?
- Are package boundaries respected (no core importing data/transport/infra)?
- Are errors wrapped with useful context?
- Are contexts/timeouts set at boundaries?
- Are tests added for new behavior (and deterministic)?
- Are public interfaces minimal and stable?
