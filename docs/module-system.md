# Module System Design

> This document details the module registration, lifecycle, capability model, and hot-reload mechanics inside the Yggdrasil Hub.
> For a high-level overview, see [Architecture Overview](overview.md).

## Overview

The Yggdrasil module system is built around a single `Hub` that manages a directed acyclic graph (DAG) of modules. Each module advertises optional capability interfaces. The Hub validates the graph at seal time, resolves dependency order via topological sort, and drives lifecycle transitions (Init/Start/Stop/Reload) in the correct sequence.

Key source files:

| File | Responsibility |
|---|---|
| `module/module.go` | Core interfaces: `Module`, `Dependent`, `Ordered`, `Configurable`, `Initializable`, `Startable`, `Stoppable`, `Reloadable`, `CapabilityProvider`, `AutoDescribed`, `Scoped` |
| `module/hub.go` | `Hub` struct, `Use()`, `Seal()` |
| `module/dag.go` | `buildDAG`, cycle detection, `Ordered` tie-breaker |
| `module/lifecycle.go` | `Init`, `Start`, `Stop`, `Reload`, two-phase commit |
| `module/scope.go` | `Scope` enum, `Scoped` interface |
| `module/auto.go` | `AutoDescribed`, `AutoRule`, `AutoSpec` |
| `module/capability.go` | Capability types, cardinality, resolve helpers |
| `assembly/selection.go` | Capability constants, default selection, chain resolution |

---

## Module Core Interface

The only required method is `Name()`:

```go
type Module interface {
    Name() string
}
```

All other behaviours are declared via optional interfaces that the Hub inspects at runtime:

| Interface | Method | Purpose |
|---|---|---|
| `Dependent` | `DependsOn() []string` | Declares hard dependencies on other modules |
| `Ordered` | `InitOrder() int` | Tie-breaker for modules in the same DAG layer |
| `Configurable` | `ConfigPath() string` | Declares the dot-path config view consumed by this module |
| `Initializable` | `Init(ctx, View) error` | Initializes long-lived resources |
| `Startable` | `Start(ctx) error` | Starts serving behaviour |
| `Stoppable` | `Stop(ctx) error` | Stops resources (must be idempotent) |
| `Reloadable` | `PrepareReload(ctx, View) (ReloadCommitter, error)` | Supports staged hot-reload |
| `CapabilityProvider` | `Capabilities() []Capability` | Exposes capability contracts |
| `AutoDescribed` | `AutoSpec() AutoSpec` | Declares auto-assembly metadata |
| `Scoped` | `Scope() Scope` | Declares runtime lifetime scope |

The `ReloadCommitter` interface returned by `PrepareReload` provides:

```go
type ReloadCommitter interface {
    Commit(ctx context.Context) error
    Rollback(ctx context.Context) error
}
```

### Scope

Modules declare their runtime lifetime via the `Scoped` interface:

| Scope | Value | Description |
|---|---|---|
| `ScopeApp` | 0 | Lives for the entire App lifetime (default) |
| `ScopeProvider` | 1 | Exposes factories/providers without owning hot-path instances |
| `ScopeRuntimeFactory` | 2 | Builds hot-path runtime objects; **must not** be registered in Hub |

The Hub rejects modules with `ScopeRuntimeFactory` at registration time.

---

## Hub Registration and Sealing

### Use

```go
func (h *Hub) Use(modules ...Module) error
```

`Use()` registers one or more modules before the Hub is sealed. It validates:

- Module is non-nil.
- `Name()` returns a non-empty string.
- No duplicate module names.
- Module scope is not `ScopeRuntimeFactory`.

Once sealed, further `Use()` calls return `errHubSealed`.

### Seal

```go
func (h *Hub) Seal() error
```

`Seal()` performs two critical validations:

1. **DAG Validation** — Calls `buildDAG()` to build the dependency graph, detect cycles, and produce topological ordering.
2. **Capability Validation** — Calls `collectCapabilities()` to index all capability providers and verify cardinality constraints.

If both succeed, the Hub records the topological order, layer map, and capability index. It becomes immutable.

---

## DAG Topological Sort

The `buildDAG()` function in `module/dag.go` implements Kahn's algorithm with extensions:

### Algorithm

1. Build an adjacency list (`outgoing`) and in-degree map from `DependsOn()` declarations.
2. Validate that all dependencies exist in the module index. Missing dependencies produce immediate errors.
3. Collect all modules with in-degree zero into a `ready` queue.
4. Process the `ready` queue layer by layer:
   - Sort the `ready` slice by `compareModules()` (primary key: `InitOrder()` from `Ordered`; secondary key: module name).
   - For each module, decrement in-degree of downstream modules; enqueue those that reach zero.
   - Record the layer index for each module.
5. If the visited count is less than the total module count, a cycle exists. Run `detectCycle()` using DFS to identify and report the cycle path.

### Tie-Breaking

When multiple modules have the same in-degree (same DAG layer), the `compareModules()` function sorts by:

1. `InitOrder()` value (lower first).
2. Module name (lexicographic).

This ensures deterministic ordering across runs.

### Cycle Detection

`detectCycle()` uses DFS with three states (`unvisited`, `visiting`, `done`). When a back-edge is found (visiting an already-visiting node), the cycle path is extracted from the DFS stack.

---

## Module Lifecycle Methods

### Init — Topological Order

```go
func (h *Hub) Init(ctx context.Context, snap config.Snapshot) error
```

Initializes modules in topological order. For each module:

1. Check if it implements `Initializable`.
2. Compute a scoped config view via `moduleView()`:
   - If the module implements `Configurable`, use its `ConfigPath()` to extract a sub-snapshot.
   - Otherwise, use an empty view.
3. Call `Init(ctx, view)`.

Initialization stops on the first error.

### Start — With Compensation

```go
func (h *Hub) Start(ctx context.Context) error
```

Starts modules in topological order with compensation semantics:

1. Iterate through the topological order.
2. For each `Startable` module, call `Start(ctx)`.
3. If a module fails to start, call `stopSequence()` in **reverse order** on all previously started modules to compensate.
4. Non-startable modules are marked as started but skipped.

### Stop — Reverse Topological Order

```go
func (h *Hub) Stop(ctx context.Context) error
```

Stops modules in reverse topological order. Only modules implementing `Stoppable` are stopped. Errors from individual stop calls are aggregated with `errors.Join`.

---

## Capability Model

Capabilities are the inter-module contract discovery mechanism in Yggdrasil. Rather than modules importing each other directly, they publish typed capability values that consumers resolve generically through the Hub. This decouples module implementations from their consumers and enables the assembly planner to make automatic wiring decisions.

### CapabilitySpec and Capability

#### CapabilitySpec

A `CapabilitySpec` declares the contract: a named, typed slot with a cardinality constraint.

```go
type CapabilitySpec struct {
    Name        string
    Cardinality CapabilityCardinality
    Type        reflect.Type
}
```

- **Name** — Unique identifier for the capability (e.g., `"transport.server.provider"`).
- **Cardinality** — Controls how many providers are allowed (see below).
- **Type** — Expected Go type of the capability value. Can be an interface type.

#### Capability

A `Capability` is one concrete value exposed by a module:

```go
type Capability struct {
    Spec  CapabilitySpec
    Name  string   // Provider name (defaults to module name)
    Value any      // The actual capability value
}
```

### CapabilityProvider Interface

Modules that wish to expose capabilities implement:

```go
type CapabilityProvider interface {
    Capabilities() []Capability
}
```

The Hub collects all capabilities during `Seal()`. Each capability's `Spec.Name` groups providers together, and the cardinality constraint is validated at seal time.

### Cardinality Constraints

| Cardinality | String | Constraint | Typical Usage |
|---|---|---|---|
| `ExactlyOne` | `exactly_one` | Exactly 1 provider required | Transport server, logger handler |
| `OptionalOne` | `optional_one` | 0 or 1 provider allowed | Optional middleware |
| `Many` | `many` | 0+ providers allowed | Interceptors, middleware chains |
| `OrderedMany` | `ordered_many` | 0+ providers, explicit ordering | Ordered interceptor chains |
| `NamedOne` | `named_one` | One per unique name, no duplicates | Named registry endpoints |

#### Validation Logic

During `collectCapabilities()`, the following validations occur:

1. **Spec name must be non-empty.** A module with an empty capability name is rejected.
2. **Value must be non-nil.** Nil capability values are rejected.
3. **Cardinality must be consistent.** All providers for the same capability must declare the same cardinality.
4. **Type must be consistent.** All providers for the same capability must have compatible types.
5. **Value type must match spec type.** The actual value's type must implement or be assignable to the declared spec type.

#### Cardinality-Specific Checks

- **ExactlyOne**: `len(entries) != 1` produces a conflict.
- **OptionalOne**: `len(entries) > 1` produces a conflict.
- **NamedOne**: Duplicate provider names within the same capability produce a conflict.
- **Many** and **OrderedMany**: No additional constraints beyond type consistency.

### Generic Resolve Functions

The Hub provides type-safe generic resolve helpers for consuming capabilities:

#### ResolveExactlyOne[T]

```go
func ResolveExactlyOne[T any](h *Hub, spec CapabilitySpec) (T, error)
```

Returns the single provider value for an `ExactlyOne` capability. Fails if zero or more than one provider exists.

#### ResolveOptionalOne[T]

```go
func ResolveOptionalOne[T any](h *Hub, spec CapabilitySpec) (T, bool, error)
```

Returns the provider value and a found flag for an `OptionalOne` capability. Returns `(zero, false, nil)` when no provider exists.

#### ResolveMany[T]

```go
func ResolveMany[T any](h *Hub, spec CapabilitySpec) ([]T, error)
```

Returns all provider values for a `Many` capability.

#### ResolveNamed[T]

```go
func ResolveNamed[T any](h *Hub, spec CapabilitySpec, name string) (T, error)
```

Returns the provider value for a specific name in a `NamedOne` capability.

#### ResolveOrdered[T]

```go
func ResolveOrdered[T any](h *Hub, spec CapabilitySpec, names []string) ([]T, error)
```

Returns provider values in an explicit order for an `OrderedMany` capability. Duplicates in the name list are rejected.

### Capability Collection and Conflict Detection

The `collectCapabilities()` function performs the following steps:

1. Iterate all modules implementing `CapabilityProvider`.
2. For each capability:
   - Validate spec name and value.
   - Check cardinality and type consistency against previously seen specs.
   - Infer type from value when spec type is nil.
3. After collection, validate cardinality constraints (ExactlyOne/OptionalOne/NamedOne).
4. Build a sorted capability index and binding map.
5. Return conflicts as both a flat list and a per-module map.

When conflicts exist, `Seal()` returns an error containing all conflict messages.

### Built-in Capability Catalog

| Constant | Name | Cardinality | Purpose |
|---|---|---|---|
| `capLoggerHandler` | `observability.logger.handler` | `ExactlyOne` | Log format handler (text/json) |
| `capLoggerWriter` | `observability.logger.writer` | `ExactlyOne` | Log output writer (console/file) |
| `capTracer` | `observability.otel.tracer_provider` | `ExactlyOne` | OpenTelemetry tracer provider |
| `capMeter` | `observability.otel.meter_provider` | `ExactlyOne` | OpenTelemetry meter provider |
| `capStatsHandler` | `observability.stats.handler` | `Many` | RPC stats handlers |
| `capSecurity` | `security.profile.provider` | `Many` | Security profile providers |
| `capMarshaler` | `marshaler.scheme` | `Many` | Serialization marshalers |
| `capServerTrans` | `transport.server.provider` | `Many` | Server transport providers |
| `capClientTrans` | `transport.client.provider` | `Many` | Client transport providers |
| `capUnaryServer` | `rpc.interceptor.unary_server` | `Many` | Unary server interceptors |
| `capStreamServer` | `rpc.interceptor.stream_server` | `Many` | Stream server interceptors |
| `capUnaryClient` | `rpc.interceptor.unary_client` | `Many` | Unary client interceptors |
| `capStreamClient` | `rpc.interceptor.stream_client` | `Many` | Stream client interceptors |
| `capRESTMW` | `transport.rest.middleware` | `Many` | REST middleware |
| `capRegistry` | `discovery.registry.provider` | `ExactlyOne` | Service registry |
| `capResolver` | `discovery.resolver.provider` | `Many` | Service resolvers |
| `capBalancer` | `transport.balancer.provider` | `Many` | Load balancer providers |

### Custom Provider Example

```go
package mytransport

import (
    "reflect"

    "github.com/codesjoy/yggdrasil/v3/module"
    "github.com/codesjoy/yggdrasil/v3/transport"
)

type MyTransportModule struct{}

func (m *MyTransportModule) Name() string { return "transport.my" }

func (m *MyTransportModule) Capabilities() []module.Capability {
    return []module.Capability{
        {
            Spec: module.CapabilitySpec{
                Name:        "transport.server.provider",
                Cardinality: module.Many,
                Type:        reflect.TypeOf((*transport.TransportServerProvider)(nil)).Elem(),
            },
            Name:  "my_protocol",
            Value: transport.NewTransportServerProvider("my_protocol", myServerBuilder),
        },
    }
}
```

This module publishes a transport server capability under the name `"my_protocol"`. The assembly planner can select it via mode defaults or explicit config. See [Assembly Planning](assembly-planning.md) for selection mechanics.

---

## Writing a Custom Module

Below is a minimal custom module that uses most optional interfaces:

```go
package mymodule

import (
    "context"
    "fmt"

    "github.com/codesjoy/yggdrasil/v3/config"
    "github.com/codesjoy/yggdrasil/v3/module"
)

type MyModule struct {
    cfg MyConfig
}

func (m *MyModule) Name() string { return "my.module" }

// Dependent — declare hard dependencies
func (m *MyModule) DependsOn() []string {
    return []string{"foundation.runtime"}
}

// Ordered — prefer earlier initialization
func (m *MyModule) InitOrder() int { return -10 }

// Configurable — declare config view path
func (m *MyModule) ConfigPath() string { return "yggdrasil.my_module" }

// Initializable — allocate resources
func (m *MyModule) Init(ctx context.Context, view config.View) error {
    var cfg MyConfig
    if err := view.Decode(&cfg); err != nil {
        return fmt.Errorf("decode config: %w", err)
    }
    m.cfg = cfg
    return nil
}

// Startable — begin serving
func (m *MyModule) Start(ctx context.Context) error {
    // Start background work
    return nil
}

// Stoppable — release resources (idempotent)
func (m *MyModule) Stop(ctx context.Context) error {
    // Cleanup
    return nil
}

// Reloadable — support hot reload
func (m *MyModule) PrepareReload(ctx context.Context, view config.View) (module.ReloadCommitter, error) {
    var next MyConfig
    if err := view.Decode(&next); err != nil {
        return nil, err
    }
    return &myCommitter{mod: m, next: next}, nil
}

type myCommitter struct {
    mod  *MyModule
    next MyConfig
}

func (c *myCommitter) Commit(ctx context.Context) error {
    c.mod.cfg = c.next
    return nil
}

func (c *myCommitter) Rollback(ctx context.Context) error {
    return nil
}
```

Register the module during application setup:

```go
app.Use(mymoduleModule)
```

See [Assembly Planning](assembly-planning.md) for how modules are selected automatically.
