# Architecture Overview

Yggdrasil is organized around a thin root facade and a set of domain-oriented package families.

## Layers

- `yggdrasil`: default bootstrap entrypoint for application code.
- `app`: lifecycle orchestration and runtime composition root.
- `assembly`: planning, selection, and declarative runtime assembly artifacts.
- `module`: module hub contracts, lifecycle interfaces, and capability model.
- `config`: layered configuration sources and compiled views.
- `transport/*`: transport implementations, clients, servers, codecs, credentials, REST bridge, and balancing.
- `rpc/*`: RPC-facing metadata, status, stream contracts, and interceptors.
- `discovery/*`: service registry and resolver contracts.
- `observability/*`: logging, OpenTelemetry, and stats handlers.
- `admin/*`: operational endpoints such as governor.

## Package Map

### Top-Level Layout

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

### Public Package Families

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

## Module Hub

The module hub is the center of runtime composition in Yggdrasil.

### Responsibilities

- register modules
- validate dependencies
- resolve named and ordered capabilities
- coordinate initialization, start, stop, and reload phases
- surface diagnostics and conflict information

### Core Concepts

- `module.Module`: minimum identity contract.
- `module.Dependent`: declares hard dependencies.
- `module.Initializable`, `Startable`, `Stoppable`: lifecycle hooks.
- `module.Reloadable`, `ReloadCommitter`, `ReloadReporter`: staged reload protocol.
- `module.Capability`, `module.CapabilitySpec`: typed extension surface owned by the relevant subsystem.

### Relationship to `app`

`app.App` owns the runtime lifecycle, while `module.Hub` owns capability and dependency orchestration.
`app` compiles configuration, selects modules, builds the runtime snapshot, and delegates capability resolution to the hub.

### App Facade Boundary

`app` follows the same facade direction used elsewhere in the repo: the package keeps the stable control surface, while implementation-heavy helpers live under `app/internal/*`.

Current internal split:

- `app/internal/assembly`: assembly error state and wrapper helpers
- `app/internal/bootstrap`: stateless config/bootstrap helpers
- `app/internal/install`: binding validation, normalization, and conflict detection
- `app/internal/lifecycle`: lifecycle runner, registry state machine, and signal handling
- `app/internal/runtime`: capability resolution, provider maps, reload/diff helpers

Dependency direction is intentionally one-way:

```text
app
  -> app/internal/assembly
  -> app/internal/bootstrap
  -> app/internal/install
  -> app/internal/lifecycle
  -> app/internal/runtime
```

The internal packages do not define compatibility guarantees. The stable API remains `app` and the root `yggdrasil` facade.

See [Module System Design](module-system.md) for the full module and capability model.

### Diagnostics

The diagnostics schema used by tooling lives at:

- [`schemas/module-hub-diagnostics.schema.json`](schemas/module-hub-diagnostics.schema.json)

## Design Intent

- Keep the root bootstrap surface small.
- Group public packages by responsibility instead of historical accretion.
- Preserve the existing framework mental model instead of inventing a new container abstraction.
- Make transport packages discoverable without the old `remote/transport/...` nesting.
