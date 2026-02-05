# Layout and Monorepo Patterns (`go.work`) (Kratos-inspired, adapted)

This reference standardizes directory layout and how to structure Go monorepos.

## Defaults
- Prefer `cmd/<app>/main.go` for entrypoints and composition root.
- Prefer `internal/` for application-private code.
- Use `pkg/` only for truly shared, stable libraries consumed by many services.

## Single service baseline (adapted Kratos-style)
Use this as the default service layout:
```
api/                 # proto + generated code output location (repo-specific)
cmd/<app>/           # composition root: create servers, register handlers, start app
configs/             # local config files (dev/test)
internal/
  conf/              # config structs (often from proto or manual)
  core/              # domain core + ports (repo interfaces)
  data/              # DB/cache/external clients + repo implementations (adapters)
  service/           # API implementation + use-case orchestration
```

Rule:
- Do not create `internal/server`. Server/router/grpc registration belongs in `cmd/<app>/`.

## Monorepo (multi-module) with `go.work` (default)
Use when:
- many components with independent modules, or
- you need separate dependency graphs / versioning.

This repo is an example of `go.work` usage (see root `go.work`).

Example pattern:
```
go.work
services/
  svc-a/
    go.mod
    api/
    cmd/svc-a/
    configs/
    internal/{core,service,data,conf}/
  svc-b/
    go.mod
    api/
    cmd/svc-b/
    configs/
    internal/{core,service,data,conf}/
pkg/                 # optional
  shared/             # shared modules (use sparingly)
    go.mod
```

Minimal `go.work` example:
```go
go 1.24.0

use (
  ./services/svc-a
  ./services/svc-b
  ./pkg/shared
)
```

## `internal/` vs `pkg/`
- `internal/`: enforced by Go; use for application/service-private code.
- `pkg/`: exported libs; only use when you are confident about stability and multi-consumer reuse.

## Dependency direction (core/service/data)
- `core/` depends on nothing outside itself (no DB/HTTP/logging frameworks).
- `service/` depends on `core/` (orchestrate use cases, implement APIs).
- `data/` depends on `core/` to implement ports (repo interfaces).
- `cmd/<app>/` wires everything together.

## Practical naming conventions
- Keep package names short and noun-like: `repo`, `store`, `httpapi`, `grpcapi`.
