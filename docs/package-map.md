# Package Map

## Top-Level Layout

```text
/
  admin/
  app/
  assembly/
  cmd/
  config/
  discovery/
  docs/
  examples/
  internal/
  module/
  observability/
  rpc/
  transport/
  instance.go
  version.go
  yggdrasil.go
```

## Public Package Families

| Family | Purpose |
| --- | --- |
| `app` | application lifecycle and runtime assembly |
| `assembly` | planning, selection, and explain/diff artifacts |
| `config` | layered config loading and views |
| `module` | module hub runtime core |
| `admin/governor` | admin and operational server |
| `discovery/*` | registry and resolver APIs |
| `rpc/*` | metadata, status, stream, and interceptors |
| `transport` | transport contracts and provider abstractions |
| `transport/runtime/*` | client and server runtime orchestration, including balancing |
| `transport/protocol/*` | concrete RPC transport implementations |
| `transport/gateway/*` | HTTP exposure and REST gateway behavior |
| `transport/support/*` | shared transport helpers such as credentials, marshaling, and peer metadata |
| `observability/*` | logger, otel, stats |
