# Application Lifecycle & Business Composition

> This document covers the Yggdrasil application lifecycle state machine, business bundle pattern, two entry points, graceful shutdown, and the hot-reload flow.

## Overview

Yggdrasil provides two ways to start an application:

1. **`yggdrasil.Run()`** — A convenience function that creates an `App`, runs `Prepare` + `ComposeAndInstall` + `Start` in sequence.
2. **`yggdrasil.New()`** — The advanced API that gives fine-grained control over each lifecycle stage: `New` → `Prepare` → `Compose` → `InstallBusiness` → `Start` → `Wait` → `Stop`.

Users declare what the application should serve via a `ComposeFunc` that returns a `BusinessBundle` containing RPC services, REST handlers, HTTP routes, background tasks, and lifecycle hooks. The App layer validates and installs each binding.

Key source files:

| File | Responsibility |
|---|---|
| `app/app.go` | `App` struct, state machine, `New`, `Start`, `Stop`, `Wait`, `initializeLocked` |
| `app/lifecycle.go` | thin lifecycle shim that preserves the `app` package boundary |
| `app/internal/lifecycle/*` | lifecycle runner, signal handling, registry state machine, server management |
| `app/reload.go` | `Reload()`, replan, spec diff, hub reload |
| `app/business.go` | `Runtime`, `ComposeFunc`, binding types, `BusinessBundle`, `BusinessInstallable` |
| `app/business_install.go` | `Prepare`, `Compose`, `ComposeAndInstall`, `InstallBusiness`, install orchestration |
| `app/internal/install/*` | install validation, normalization, and conflict detection |
| `app/internal/bootstrap/*` | stateless config/bootstrap helpers used by `app/config.go` |
| `app/internal/runtime/*` | runtime capability resolution and reload helpers used by snapshot builders |
| `app/internal/assembly/*` | assembly error state and stage-error helpers |
| `module/lifecycle.go` | Hub `Init`, `Start`, `Stop`, `Reload`, two-phase commit |

The important boundary is that `app` keeps the stable public API and all `func (a *App)` methods, while `app/internal/*` holds implementation details with no compatibility promise.

---

## App State Machine

The `App` manages the following states:

```
New → Planned → InfraInitialized → Initialized → BusinessInstalled → Serving → Running → Stopped
```

| State | Description |
|---|---|
| `New` | App created, no initialization performed |
| `Planned` | Assembly plan computed successfully |
| `InfraInitialized` | Hub sealed and modules initialized |
| `Initialized` | Runtime adapters applied, governor and server ready |
| `BusinessInstalled` | Business bundle installed via `InstallBusiness` |
| `Serving` | `prepareStartLocked` succeeded, about to enter Running |
| `Running` | Lifecycle runner executing, servers listening |
| `Stopped` | Terminal state; restart is not supported in-process |

State transitions are guarded by `sync.Mutex`. Attempts to restart a stopped app return `errRestartUnsupported`.

---

## initializeLocked Flow

Called by both `Prepare()` and the internal start path. The flow:

1. **initConfigChain** — Initialize the config manager and load configured sources.
2. **resolveIdentityLocked** — Resolve the application identity (app name, admin config).
3. **buildAssemblyResult** — Run the assembly planner to produce a `Result`.
4. **initHub** — Register planned modules, seal the hub, set capability bindings, call `Hub.Init()`.
5. **validateStartup** — Validate startup configuration.
6. **initInstanceInfo** — Initialize runtime instance metadata.
7. **applyRuntimeAdapters** — Apply tracer, meter, and other runtime adapters.
8. **initGovernor** — Initialize the admin governor server.
9. **initRegistry** — Initialize service registry.
10. **initServer** — Initialize the transport server.
11. Create `Runtime` surface and `preparedAssembly`.

On failure at any step, the state is set to `Stopped` and cleanup is attempted.

---

## Lifecycle Runner

The lifecycle runner manages the serving phase. Public callers still interact through `app.App`, but the runner implementation now lives in `app/internal/lifecycle`.

### Hook Stages

| Stage | Execution Order | Purpose |
|---|---|---|
| `lifecycleStageBeforeStart` | 1 | Pre-start hooks (e.g., `Hub.Start()`) |
| `lifecycleStageBeforeStop` | 2 | Pre-stop hooks (deregister, drain) |
| `lifecycleStageCleanup` | 3 | Resource cleanup (e.g., `Hub.Stop()`) |
| `lifecycleStageAfterStop` | 4 | Post-stop notifications |

### Server Management

The runner manages:
- **Main server** — The primary transport server.
- **Governor** — Admin/diagnostics server.
- **Internal servers** — Background tasks registered via `InternalServer` interface.

All servers start concurrently via `errgroup.Group`. If any server fails, the runner triggers async stop.

### Signal Handling

On `SIGINT` or `SIGTERM`:
1. Call `runner.Stop()` gracefully.
2. If the process does not exit within the shutdown timeout, force-exit with signal code.

The default shutdown timeout is 30 seconds, configurable via `WithShutdownTimeout`.

### Graceful Shutdown Sequence

```
BeforeStop hooks → Deregister from registry → Stop servers → Cleanup hooks → AfterStop hooks
```

---

## Hot Reload Flow

Hot reload is triggered when a watched configuration source changes:

1. **Config Watch** — `Manager.Watch()` detects a config change and calls `a.reloadAsync()`.
2. **Replan** — `Reload()` calls `buildAssemblyResult()` to compute a new plan.
3. **Spec Diff** — `assembly.Diff(prevSpec, newSpec)` identifies what changed.
4. **Restart Decision** — `ReloadRequiresRestart()` evaluates the diff:
   - Module additions/removals require restart.
   - Capability binding changes may require restart.
   - Business bundle changes always require restart.
5. **Hub.Reload** — If restart is not required, the Hub performs a two-phase reload of affected modules.

### Two-Phase Commit

The Hub's `Reload()` method implements a staged protocol:

```
idle → preparing → committing → idle
```

**Prepare Phase:**
1. Identify affected modules (those with changed config sections, or all if `reloadAll` is set).
2. For each affected module:
   - If not `Reloadable`, mark `restartRequired = true` and skip.
   - Call `PrepareReload(ctx, newView)` to get a `ReloadCommitter`.
3. If any prepare fails, roll back all prepared modules in reverse order.

**Commit Phase:**
1. Call `Commit(ctx)` on each prepared committer in order.
2. If any commit fails, roll back remaining uncommitted modules.
3. If rollback also fails, enter `degraded` state.

**Rollback:**
- `Rollback(ctx)` is called in reverse order on all prepared but uncommitted modules.
- If rollback fails, the Hub enters `ReloadPhaseDegraded` and requires a full restart.

### Reload State

```go
type ReloadState struct {
    Phase           ReloadPhase
    RestartRequired bool
    Diverged        bool
    FailedModule    string
    FailedStage     ReloadFailedStage
    LastError       error
}
```

| Phase | Description |
|---|---|
| `idle` | Normal operation |
| `preparing` | Preparing modules for reload |
| `committing` | Committing prepared reloads |
| `rollback` | Rolling back after failure |
| `degraded` | Unrecoverable state; restart required |

---

## Restart Required Mechanism

When the reload system determines that in-process reload is insufficient:

1. `Hub.MarkRestartRequired(moduleName)` sets `restartRequired = true` and records the failing module.
2. The reload caller receives `ErrReloadRequiresRestart`.
3. The application must be restarted externally (e.g., via process supervisor).

This happens when:
- A non-reloadable module has config changes.
- The assembly diff adds or removes modules.
- Business bindings have changed.
- The two-phase commit enters degraded state.

---

## Business Bundle

The Business Bundle is Yggdrasil's mechanism for user code to declare what the application should serve. Rather than manually wiring RPC services, REST handlers, and HTTP routes to a server, the user provides a `ComposeFunc` that receives a prepared `Runtime` and returns a `BusinessBundle`. The App layer then validates and installs each binding.

### Runtime Interface

The `Runtime` interface is the business-safe surface exposed after `Prepare()` succeeds:

```go
type Runtime interface {
    NewClient(ctx context.Context, service string) (client.Client, error)
    Config() *config.Manager
    Logger() *slog.Logger
    TracerProvider() trace.TracerProvider
    MeterProvider() metric.MeterProvider
    Lookup(target any) error
}
```

| Method | Returns |
|---|---|
| `NewClient` | A transport client for calling remote services |
| `Config` | The configuration manager |
| `Logger` | The default structured logger |
| `TracerProvider` | The OpenTelemetry tracer provider |
| `MeterProvider` | The OpenTelemetry meter provider |
| `Lookup` | Generic capability lookup by pointer target type |

### BusinessBundle Structure

```go
type BusinessBundle struct {
    RPCBindings  []RPCBinding
    RESTBindings []RESTBinding
    RawHTTP      []RawHTTPBinding
    Tasks        []BackgroundTask
    Hooks        []BusinessHook
    Extensions   []BusinessInstallable
    Diagnostics  []BundleDiag
}
```

| Field | Type | Purpose |
|---|---|---|
| `RPCBindings` | `[]RPCBinding` | gRPC/RPC service registrations |
| `RESTBindings` | `[]RESTBinding` | REST gateway service registrations |
| `RawHTTP` | `[]RawHTTPBinding` | Raw HTTP handler registrations |
| `Tasks` | `[]BackgroundTask` | Background goroutines managed by lifecycle |
| `Hooks` | `[]BusinessHook` | Lifecycle hooks (before start, before/after stop) |
| `Extensions` | `[]BusinessInstallable` | Custom install extensions |
| `Diagnostics` | `[]BundleDiag` | Diagnostic items exposed via governor |

### Binding Types

#### RPC Binding

```go
type RPCBinding struct {
    ServiceName string
    Desc        any   // must be *server.ServiceDesc
    Impl        any   // must satisfy desc.HandlerType
}
```

Validation:
1. At least one server transport must be configured (`yggdrasil.server.transports`).
2. `Desc` must be `*server.ServiceDesc` and non-nil.
3. `Impl` must be non-nil.
4. `Impl` must satisfy `desc.HandlerType` (interface check via reflection).
5. Service name must not already be installed (conflict detection).

Install: calls `server.RegisterService(desc, impl)` on the transport server.

#### REST Binding

```go
type RESTBinding struct {
    Name     string
    Desc     any         // must be *server.RestServiceDesc
    Impl     any         // must satisfy desc.HandlerType
    Prefixes []string    // route prefix
}
```

Validation:
1. REST must be enabled (`yggdrasil.transports.http.rest`).
2. `Desc` must be `*server.RestServiceDesc` and non-nil.
3. `Impl` must be non-nil and satisfy `desc.HandlerType`.
4. No route conflicts: each method+path combination must be unique across all REST and raw HTTP bindings.

Install: calls `server.RegisterRestService(desc, impl, prefixes...)` on the transport server.

#### Raw HTTP Binding

```go
type RawHTTPBinding struct {
    Method  string
    Path    string
    Handler any

    Desc *server.RestRawHandlerDesc  // legacy compatibility
}
```

Validation:
1. REST must be enabled.
2. If `Desc` is provided, it is normalized. Otherwise, `Method`/`Path`/`Handler` are used directly.
3. Handler must be non-nil.
4. No route conflict with existing REST or raw HTTP routes.

Install: calls `server.RegisterRestRawHandlers(desc)` on the transport server.

### Background Tasks and Lifecycle Hooks

#### BackgroundTask

```go
type BackgroundTask interface {
    Serve() error
    Stop(context.Context) error
}
```

Background tasks are managed as internal servers by the lifecycle runner. They start concurrently with the main server and are stopped during the shutdown sequence.

#### BusinessHook

```go
type BusinessHook struct {
    Name  string
    Stage BusinessHookStage
    Func  func(context.Context) error
}
```

| Stage | When |
|---|---|
| `BusinessHookBeforeStart` | Before servers start listening |
| `BusinessHookBeforeStop` | Before servers stop |
| `BusinessHookAfterStop` | After all servers have stopped |

Hooks are registered into the lifecycle runner's hook chain.

### BusinessInstallable Extension Interface

For non-standard installation patterns:

```go
type BusinessInstallable interface {
    Kind() string
    Install(ctx *InstallContext) error
}
```

#### InstallContext

```go
type InstallContext struct {
    Runtime Runtime
    // contains filtered unexported fields
}
```

Methods:

| Method | Purpose |
|---|---|
| `RegisterRPC(binding)` | Install an RPC binding |
| `RegisterREST(binding)` | Install a REST binding |
| `RegisterRawHTTP(binding)` | Install a raw HTTP binding |
| `AddTask(task)` | Register a background task |
| `AddHook(hook)` | Register a lifecycle hook |

This allows extensions to dynamically add bindings based on runtime state.

### Install Flow and Error Handling

#### ComposeAndInstall (one-step)

```go
func (a *App) ComposeAndInstall(ctx context.Context, fn ComposeFunc) error
```

1. `Compose(ctx, fn)` — Execute the composition function, validate the bundle is non-nil.
2. `InstallBusiness(bundle)` — Install each binding with validation.

#### Error Handling

- If `Compose` fails, the app transitions to `Stopped` and cleanup is attempted.
- If `InstallBusiness` encounters a validation error (e.g., nil handler, route conflict), it returns immediately.
- If `InstallBusiness` encounters a conflict (e.g., duplicate service name), it returns `ErrInstallRegistrationConflict`.
- Only one business bundle can be installed per app. Subsequent calls return `ErrInstallRegistrationConflict`.

### Complete Example

From `examples/sample/server/main.go`:

```go
package main

import (
    "context"
    "net/http"

    yggdrasil "github.com/codesjoy/yggdrasil/v3/app"
)

func main() {
    yggdrasil.Run(
        context.Background(),
        func(rt yggdrasil.Runtime) (*yggdrasil.BusinessBundle, error) {
            return &yggdrasil.BusinessBundle{
                // gRPC service
                RPCBindings: []yggdrasil.RPCBinding{
                    {
                        ServiceName: "LibraryService",
                        Desc:        libraryDesc,
                        Impl:        &LibraryImpl{},
                    },
                },
                // REST gateway
                RESTBindings: []yggdrasil.RESTBinding{
                    {
                        Name: "library-rest",
                        Desc: libraryRESTDesc,
                        Impl: &LibraryRESTImpl{},
                    },
                },
                // Raw HTTP handler
                RawHTTP: []yggdrasil.RawHTTPBinding{
                    {
                        Method:  http.MethodGet,
                        Path:    "/web",
                        Handler: http.HandlerFunc(WebHandler),
                    },
                },
            }, nil
        },
        yggdrasil.WithAppName("github.com.codesjoy.yggdrasil.example.sample"),
    )
}
```

This example registers three types of endpoints in a single business bundle.

---

## Functional Options Catalog

Common options used with `app.New()`:

| Option | Purpose |
|---|---|
| `WithAppName(name)` | Set the application name |
| `WithConfigSources(sources...)` | Add configuration sources |
| `WithModules(modules...)` | Register module candidates |
| `WithPlanOverrides(overrides...)` | Apply assembly overrides |
| `WithShutdownTimeout(d)` | Set graceful shutdown timeout |
| `WithBusinessBundle(fn)` | Set the business composition function |

See [Module System Design](module-system.md) for module lifecycle details.
