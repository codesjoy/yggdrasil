# Tooling Templates (lint/test/fmt/tidy)

This reference provides minimal templates you can copy into a Go repo.

## `.golangci.yml` (minimal)
```yaml
run:
  timeout: 5m

linters:
  enable:
    - govet
    - staticcheck
    - errcheck
    - ineffassign
    - unused
    - gofmt

issues:
  exclude-use-default: false
```

Notes:
- Keep the enabled set small to avoid noisy, low-signal failures.
- Add repo-specific excludes only when justified.

## Makefile targets (minimal)
If you use a modular make-rule layout, prefer a `scripts/make-rules/golang.mk` (or `make/go.mk`) that defines `go.test`, `go.lint`, `go.fmt`, `go.tidy` and forward from the root `Makefile`.

Otherwise, a minimal root Makefile is fine:
```makefile
.PHONY: test lint fmt tidy

test:
\tgo test ./...

lint:
\tgolangci-lint run ./...

fmt:
\tgofmt -w .

tidy:
\tgo mod tidy
```

## Recommended CI gates
Run on every PR:
1. `make fmt` (or verify formatting via CI)
2. `make test`
3. `make lint`

Optional (slower, but valuable):
- `go test -race ./...`
