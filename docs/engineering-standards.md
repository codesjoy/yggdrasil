# Yggdrasil Engineering Standards

This document is the self-contained engineering standard for the Yggdrasil
repository. It states the rules that apply to framework code, modules, providers,
tests, configuration, documentation, and AI-assisted changes.

The words **MUST**, **MUST NOT**, **SHOULD**, **SHOULD NOT**, and **MAY** are
normative. A change that violates a MUST rule requires an explicit design
decision recorded in the change review.

The numbered documents under `en/` are *descriptive*: they explain how the
framework works. This document is *normative*: it states what a contributor must
do. Where a rule below has an explanatory counterpart, the rule cites that
document instead of restating its content.

## 1. Repository Structure

### 1.1 Workspace and Module Layout

Yggdrasil is a multi-module Go workspace. `go.work` and `go.work.sum` **MUST NOT**
be committed: the workspace file is a local convenience for IDE tooling, and the
task runner discovers modules from `go.mod` files.

The workspace contains these five modules:

```text
.                              root library        github.com/codesjoy/yggdrasil/v3
cmd/protoc-gen-yggdrasil-rest  REST code generator github.com/codesjoy/yggdrasil/cmd/protoc-gen-yggdrasil-rest/v3
cmd/protoc-gen-yggdrasil-rpc   RPC code generator  github.com/codesjoy/yggdrasil/cmd/protoc-gen-yggdrasil-rpc/v3
examples                       runnable samples    github.com/codesjoy/yggdrasil/v3/examples
examples/protogen              generated protobuf  github.com/codesjoy/yggdrasil/v3/examples/protogen
```

- Adding a module **MUST** add a `go.mod` at the module root. The task runner
  discovers modules from `go.mod`, so no `go.work` change is committed; a
  developer **MAY** add the module to their local (untracked) `go.work` for IDE
  support.
- The root module path **MUST** stay `github.com/codesjoy/yggdrasil/v3`. The
  `/v3` suffix is part of the compatibility contract (see §4.4).
- A new `cmd/` module **SHOULD** use the path
  `github.com/codesjoy/yggdrasil/cmd/<name>/v3`.
- Every module **MUST** build with `GOWORK=off`; the task runner always sets it
  so that each module is verified against its own `go.mod`.
- Every `.go` and `.sh` file **MUST** carry the Apache 2.0 header with the holder
  `The codesjoy Authors.`; `task copyright:verify` **MUST** pass.
- Build output and local state **MUST NOT** be committed: `_output/`, `.cache/`,
  `bin/`, `.task/`.

### 1.2 Package Ownership and Boundaries

```text
admin/governor/     governor admin HTTP server (monitoring and debugging routes)
app/                stable advanced application control API: App, Runtime, BusinessBundle
assembly/           declarative plan/spec types, planner errors, default selection
capabilities/       built-in framework capability contracts
cmd/                protoc code generator plugins
config/             layered configuration management and typed views
discovery/          registry and resolver extension points
internal/           module-private helpers, never importable from another module
module/             module hub runtime core: Hub, Module, capability, DAG, scope
observability/      slog handlers and writers, OpenTelemetry tracing and metrics
rpc/                interceptors, metadata, status, streaming helpers
transport/          protocol-agnostic transport, REST gateway, client/server runtime
```

- Each top-level package **MUST** own exactly one responsibility, matching its
  `// Package ...` doc comment. Changing a package's responsibility **MUST**
  update that doc comment in the same commit.
- `internal/` packages **MUST NOT** be imported by any module other than the root
  module. Cross-module helpers belong in an exported package.
- Exported `app` API **MUST** live in `app/`; App implementation details **MUST**
  stay under `app/internal/`. See [01](en/01-architecture-overview-and-design-principles.md) §6.
- The root `yggdrasil` package **MUST** re-export business-facing types as type
  aliases to the `app` types and **MUST NOT** redefine them.
- `capabilities` **MUST** remain the single declaration site for built-in
  capability specs. A built-in capability name literal **MUST NOT** be
  re-declared in another package.
- Package directories **MUST NOT** be named `common`, `utils`, `misc`, or
  `helper`. Name a package after the responsibility it owns.

### 1.3 Generated Code and Dependencies

- Generated protobuf code **MUST** live under `examples/protogen/` or the
  generator's documented output path, and **MUST NOT** be hand-edited.
- After changing a `cmd/protoc-gen-*` template, `task proto:generate` **MUST** be
  re-run and the regenerated output **MUST** be committed.
- `go.mod` and `go.sum` **MUST** be tidy; `task deps:tidy:check` **MUST** pass.
  Run `task deps:tidy` after any import change.
- A new third-party dependency **SHOULD** be justified in the change review. The
  framework **MUST NOT** adopt a competing logging, error, or dependency
  injection library (see §3.4 and §7.1).

## 2. Framework Extension-Point Model

Yggdrasil's product is its extension points. This section states which one to use
and what each one owes the framework.

### 2.1 Module and Optional Interfaces

- A module **MUST** implement `module.Module`. The only mandatory method is
  `Name() string`.
- `Name()` **MUST** be non-empty, globally unique, and stable across releases,
  because it appears in `assembly.Spec`, in diagnostics, and in reload state.
  `Hub.Use` **MUST** reject a nil module, an empty name, and a duplicate name.
- Every behaviour beyond `Name()` **MUST** be expressed as an optional interface,
  declared in `module/module.go` alongside `Module`. The complete set is
  `IsolationReporter` (`IsolationMode() IsolationMode`), `Dependent`
  (`DependsOn() []string`), `Ordered` (`InitOrder() int`), `Configurable`
  (`ConfigPath() string`), `ConfigSourceProvider`
  (`ConfigSourceBuilders() map[string]configchain.ContextBuilder`),
  `Initializable` (`Init(ctx, config.View) error`), `Startable`
  (`Start(ctx) error`), `Stoppable` (`Stop(ctx) error`), `Reloadable`
  (`PrepareReload(ctx, config.View) (ReloadCommitter, error)`), `ReloadCommitter`
  (`Commit(ctx) error`, `Rollback(ctx) error`), and `ReloadReporter`
  (`ReloadState() ReloadState`).
- A new behaviour **MUST** be added as a new optional interface with a type
  assertion at the call site. `module.Module` **MUST NOT** be widened, and a
  method **MUST NOT** be added to any existing exported interface (see §4.2).
- A module **MUST NOT** be registered after `Hub.Seal`. Registration always
  happens before seal, and `Hub.Use` rejects a late registration.

See [02](en/02-module-hub-and-capability-model.md) §2 for the rationale and the
full lifecycle semantics.

### 2.2 Dependency Declaration

- Hard dependencies **MUST** be declared with `DependsOn()`. They **MUST NOT** be
  encoded with `InitOrder()`.
- `InitOrder()` **MUST** be used only as a tie-break between modules in the same
  DAG layer.
- Every `DependsOn()` target **MUST** name an existing module. A missing target
  **MUST** fail `Seal` with an error naming both the module and the missing
  dependency.
- The dependency graph **MUST** be acyclic. A cycle **MUST** fail `Seal`, and the
  error **MUST** carry the full cycle path (`a -> b -> c -> a`), not just the
  fact that a cycle exists.
- `Init` and `Start` **MUST** run in the topological order produced by the DAG,
  never in registration order.

### 2.3 Capability Model

- A module that publishes capabilities **MUST** implement
  `module.CapabilityProvider` and return `[]module.Capability`.
- Every capability **MUST** declare a non-empty `CapabilitySpec.Name`, a
  `Cardinality`, and a `Type`. A capability with a nil `Value` **MUST** fail
  `Seal`.
- The same capability name **MUST** use one cardinality and one declared type
  across all providers. A cardinality or type mismatch **MUST** be a hard error
  at `Seal`, and **MUST NOT** be downgraded to a warning. See
  [02](en/02-module-hub-and-capability-model.md) §5.1 for the cardinality
  meanings.
- A provider value **MUST** implement or be assignable to the declared spec
  type.
- `ExactlyOne`, `OptionalOne`, and `NamedOne` **MUST** be enforced at `Seal`. For
  `NamedOne`, provider names **MUST** be unique within the capability.
- Consumers **MUST** resolve providers through the cardinality-typed helpers
  `ResolveExactlyOne`, `ResolveOptionalOne`, `ResolveMany`, `ResolveNamed`, and
  `ResolveOrdered`. A consumer **MUST NOT** implement "first provider wins"
  itself.
- Built-in specs **MUST** be referenced through the constants in
  `capabilities/specs.go` (`capabilities.LoggerHandlerSpec`,
  `capabilities.TransportServerProviderSpec`, ...) and **MUST NOT** be
  re-declared inline. Publish through `capabilities.ProvideNamed` /
  `capabilities.ProvideOrdered` where the cardinality fits.
- A `Capabilities()` callback **MUST** be deterministic, pure, and free of side
  effects: it may run during planning and during `Hub.Seal`, before `Init`, and
  it may run more than once. When a capability value depends on configuration, the
  callback **MUST** return a lazy provider or factory and defer reading state
  until the object is constructed.
- A capability value **MUST** be a provider object or factory. It **MUST NOT** be
  a per-request, per-stream, or per-endpoint instance.

### 2.4 Scope Boundary

- A module **MAY** implement `Scoped` to declare `ScopeApp` (the default),
  `ScopeProvider`, or `ScopeRuntimeFactory`.
- A module that builds per-service, per-endpoint, or per-stream objects **MUST**
  declare `ScopeRuntimeFactory`.
- A `ScopeRuntimeFactory` module **MUST NOT** be registered in the Hub;
  `Hub.Use` rejects it as `module %q has unsupported scope runtime_factory`.
- Resolver watches, pickers, connection managers, and balancers **MUST NOT** be
  modules or capabilities. They are owned by the client and server runtimes. See
  [06](en/06-transport-discovery-and-observability.md) §6-§8 and
  [08](en/08-implementation-boundaries-and-optimization-notes.md) §8.2.
- The Hub **MUST** hold only long-lived, low-frequency, diagnosable capability
  carriers. See [02](en/02-module-hub-and-capability-model.md) §1.

### 2.5 Hub versus BusinessBundle

The Hub manages framework structure. Business content always belongs to the
business bundle.

- Business services, handlers, tasks, hooks, and extensions **MUST** be installed
  through `BusinessBundle` and **MUST NOT** be registered in the Hub.
- `Compose` **MUST** return a `BusinessBundle`. It **MUST NOT** mutate the server
  runtime directly. See [04](en/04-application-lifecycle-and-business-composition.md) §5-§6.
- Business code **MUST** obtain framework services through `Runtime`. It **MUST
  NOT** reach for `*App` or `*Hub`.
- A non-standard install **MUST** implement `BusinessInstallable` and register
  through `InstallContext`; `BusinessBundle.Extensions` carries it.
- `CapabilityRegistration` **MUST** be used only for provider-only extensions
  that need no `Start`, `Stop`, `Reload`, or `DependsOn`. Anything requiring
  lifecycle **MUST** be a full `module.Module`.
- `CapabilityRegistration` **MUST NOT** be relied on for hot reload; it does not
  support `Reloadable`.
- A `CapabilityRegistration` that sets `ConfigPath` **MUST** also set `Init`,
  otherwise registration fails with `config_path requires init callback`.

Use this table to choose an extension point:

| Extension point | Use when | Lifecycle | Hot reload | Isolation |
| --- | --- | --- | --- | --- |
| `module.Module` via `WithModules` | You own long-lived resources or need `DependsOn`, `Start`, `Stop`, or reload | Full DAG lifecycle | `Reloadable` supported | App-local |
| `CapabilityRegistration` via `WithCapabilityRegistrations` | You only add provider values to existing capabilities | `Init` only | Not supported | App-local |
| `BusinessBundle` binding | You expose a business service, handler, task, hook, or extension | Compose, install, then Start/Stop | Restart-required | Business-owned |
| `BusinessInstallable` | Standard bindings cannot express the installation | Install only | Restart-required | Business-owned |

## 3. Go Coding Style

### 3.1 Formatting Authority

- All Go code **MUST** be formatted by the `golangci-lint` formatters: `gofmt`,
  `gofumpt`, `goimports`, and `golines`. `task fmt:check` **MUST** pass.
- Standalone `gofmt`, `gofumpt`, and `goimports` binaries **MUST NOT** be used.
  `golangci-lint` is the single formatting authority, so `task fmt` and
  `task go:lint` can never disagree.
- Lines **MUST NOT** exceed 100 characters. Indentation **MUST** be tabs. Struct
  tags **MUST** be reformatted and long method chains **MUST** be split, as
  configured under `formatters.settings.golines`.
- Hand-alignment **MUST NOT** be committed. A formatting-only change **MUST NOT**
  be mixed into a functional change.
- Generated protobuf output is exempt; it is excluded by
  `formatters.exclusions.generated: lax`.
- A change **MUST** pass `task go:lint` with the enabled linter set: `govet`,
  `staticcheck`, `errcheck`, `ineffassign`, `unused`, `revive`, `unconvert`,
  `gocritic`, `gosec`, `bodyclose`, `noctx`.

### 3.2 Naming and GoDoc

- Package names **MUST** be short, lowercase, and free of underscores.
- Exported identifiers **MUST** use MixedCaps and **MUST** carry a GoDoc comment;
  the `revive` `exported` rule enforces this. GoDoc is not required inside
  `*_test.go`.
- Acronyms **MUST** stay capitalised: `ID`, `URL`, `HTTP`, `RPC`, `REST`, `TLS`.
- Unexported identifiers **MUST** use camelCase.
- An intentional name stutter **MUST** carry an explicit
  `//nolint:revive // <reason>` justification, as on `TransportClientProvider`
  and `TransportServerProvider`.
- GoDoc and code comments **MUST** be written in English.
- Comments **SHOULD** explain intent or a non-obvious constraint. A comment that
  only restates the following line **MUST NOT** be added.

### 3.3 Language and API Idioms

- `context.Context` **MUST** be the first parameter of any blocking or I/O
  function, and **MUST NOT** be stored in a long-lived struct.
- A constructor **SHOULD** accept only the dependencies it needs.
- A wrapper that only forwards a standard-library call **MUST NOT** be added. An
  abstraction **MUST** add meaning, observability, or a test boundary.
- Package-level `init()` **MUST NOT** register providers, mutate shared state, or
  perform side effects that break App-instance isolation.
- Nil and empty-input behaviour **MUST** be explicit at every exported function.

### 3.4 Error Handling

- Errors that cross a service boundary **MUST** be created and inspected with
  `github.com/codesjoy/pkg/basic/xerror`: `xerror.New`, `xerror.NewWithReason`,
  `xerror.Wrap`, `xerror.IsCode`, `xerror.IsReason`, `xerror.ReasonOf`.
- An error that crosses the wire **MUST** be convertible through `rpc/status`.
- Framework, planner, and install errors **MUST** use `assembly.Error` with an
  `assembly.ErrorCode` from `assembly/errors.go`, not an ad-hoc string.
- Standard-library sentinels (`errors.New` package variables, `fmt.Errorf` with
  `%w`, `errors.Is`, `errors.As`) **MAY** be used for internal, non-wire errors.
- A wrapped error **MUST** preserve its cause, through `%w` or `xerror.Wrap`.
- A test **MUST** assert the error code or reason, not merely that an error
  occurred.
- Driver errors, connection strings, credentials, and raw downstream response
  bodies **MUST NOT** be returned to callers or written to logs.

## 4. Public API and Compatibility Standard

A library's product is its public API. This section states what is stable and how
to change it.

### 4.1 Stable Surface versus Internal Surface

- Everything exported from `app`, the root `yggdrasil` package, `module`,
  `assembly`, `config`, `capabilities`, `transport`, `rpc`, `observability`,
  `discovery`, and `admin` **MUST** be treated as public API and **MUST** obey
  this section.
- Implementation helpers **MUST** live under a package's `internal/` and **MUST
  NOT** be exported from a public package.
- The root package **MUST** expose only business-facing entry points. Low-level
  runtime control **MUST NOT** be added there.
- An exported symbol added only so a test can reach it **MUST NOT** be committed.
  Write the test in an external `_test` package instead (see §8.1).

### 4.2 Growing Extension Points Without Breaking

- A method **MUST NOT** be added to an existing exported interface. Every
  implementer outside the repository breaks when one is. New behaviour **MUST**
  be added as a new optional interface plus a type assertion, exactly as
  `module.Module` grows.
- A new optional field **MAY** be added to an exported struct. Removing or
  renaming an exported field or method **MUST** be treated as a breaking change.
- New configuration keys under the `yggdrasil` root **MUST** be additive. A
  removed key **MUST** be documented as breaking.

### 4.3 Stable Identifier Contracts

The following **MUST** be treated as stable contracts and **MUST NOT** be renamed
without a breaking change:

- capability spec names in `capabilities/specs.go`, such as
  `observability.logger.handler` and `transport.server.provider`;
- `ConfigPath()` strings, which are user-facing YAML paths;
- published chain template names and versions;
- `assembly.ErrorCode` values;
- module `Name()` values;
- `xerror` codes and reasons.

### 4.4 Deprecation and Breaking Changes

- A removed or renamed symbol **SHOULD** pass through at least one release
  carrying a `// Deprecated:` GoDoc directive that names the replacement.
- A break that cannot be absorbed in a release **MUST** publish a new major
  version; the major version is encoded in the module path (`/v3`).
- A breaking change **MUST** carry `!` in the commit title and a
  `BREAKING CHANGE:` footer (see §9.2).
- A breaking change to a generated descriptor, a config schema, or an example
  **MUST** ship the corresponding documentation and example updates in the same
  change.

## 5. Lifecycle and Isolation Invariants

These are the correctness rules of the framework. See
[04](en/04-application-lifecycle-and-business-composition.md) for the lifecycle
it explains and [02](en/02-module-hub-and-capability-model.md) §4 for the module
state machine.

### 5.1 Prepare, Start, and Stop Two-Phase Contract

- `Prepare()` **MUST NOT** serve. It **MUST NOT** call `Listen`, `Serve`, or
  `Accept`, register a service instance, receive external requests, or start an
  external request loop.
- `Prepare()` **MAY** construct providers, server objects, handler registries,
  codecs, muxes, and unbound listener state.
- `Init()` **MUST** initialise long-lived resources only. It **MUST NOT** serve
  externally.
- `Start()` **MUST** perform all bind, listen, serve, and service-registration
  work.
- A transport or provider implementation that cannot separate preparation from
  external serving **MUST NOT** claim the Yggdrasil provider contract. See
  [04](en/04-application-lifecycle-and-business-composition.md) §11.

### 5.2 Idempotence and Compensation

- `Stop()` **MUST** be idempotent. Use `sync.Once` or `module.StopOnce`, which
  caches the first result and returns it for every later call.
- `Stop` **MUST** run in reverse topological order, and **MUST** call only
  modules that implement `Stoppable`.
- A `Start` failure **MUST** be compensated by stopping the modules already
  started, in reverse order. Compensation **MUST** continue even when an
  individual stop fails, and the errors **MUST** be aggregated.
- Compensation paths **MUST** be safe to call more than once.
- In-process restart **MUST NOT** be implemented. Starting a stopped App fails
  as unsupported.
- A resource created during `Compose` but not returned in the `BusinessBundle` is
  business-owned. A resource that must survive Start and Stop **MUST** be
  returned through `Tasks`, `Hooks`, or `Extensions`.

### 5.3 App-Local Isolation and Process Defaults

- Runtime state **MUST** be App-local by default. `app.New` and `yggdrasil.New`
  **MUST NOT** install process-global defaults.
- A module **MUST NOT** read `slog.Default()`, OpenTelemetry globals, or legacy
  process-level facades unless it declares that requirement.
- A module that requires process globals **MUST** implement `IsolationReporter`
  and return `module.IsolationModeRequiresProcessDefaults`.
- An App that loads such a module **MUST** be constructed with
  `WithProcessDefaults(true)`. Otherwise planning fails with
  `assembly.ErrIsolationRequiresProcessDefaults`.
- At most one App **MAY** own process defaults. A second attempt fails with
  `app.ErrProcessDefaultsAlreadyInstalled`.
- `yggdrasil.Run` **MAY** install process-default compatibility facades for a
  single-App program. Embedded, sidecar, multi-App, and test scenarios **MUST**
  use `yggdrasil.New` or `app.New` and leave defaults untouched.

### 5.4 Determinism of Planning and Callbacks

- `AutoRule.Match()` **MUST** be pure: no clock, no randomness, no mutable global
  state. It **MUST** read only immutable snapshots, the resolved mode, and static
  context.
- An `AutoDescribed` module **MUST** declare its affected config paths so reload
  classification is correct.
- The planner **MUST** be a pure function of the config snapshot, the module
  candidates, and the overrides. See
  [03](en/03-bootstrap-auto-assembly-and-planning.md) §3 and
  [05](en/05-configuration-declarative-assembly-and-hot-reload.md) §3.
- `assembly.Spec` **MUST** contain only stable, serialisable data. It **MUST NOT**
  hold `module.Module` instances, addresses, interface values, or any
  non-deterministic field.
- Maps **MUST** be canonicalised into sorted slices before they influence a later
  stage. Decisions, warnings, and conflicts **MUST** be emitted in a stable
  order.
- An unresolvable default **MUST** fail with `assembly.ErrAmbiguousDefault`. The
  planner **MUST NOT** silently pick the lexicographically first candidate.
- The plan hash **MUST** be computed from the canonical `assembly.Spec` alone.

### 5.5 Hot Reload Classification

- A configuration-only change to a module implementing `Reloadable` **MAY**
  hot-reload.
- The following **MUST** be classified restart-required: module addition or
  removal; transport protocol change; server port or listener change; RPC, REST,
  or raw-HTTP binding change; `BusinessBundle` structure change; and any change
  that requires `business.Compose` to run again. See
  [08](en/08-implementation-boundaries-and-optimization-notes.md) §6.
- Reload **MUST** follow prepare, then commit, then rollback. A partial implicit
  commit **MUST NOT** occur.
- A `PrepareReload` failure **MUST NOT** enter commit, and **MUST** roll back the
  prepared modules in reverse order.
- A rollback failure **MUST** set the reload phase to `degraded` with
  `RestartRequired`, expose it through diagnostics, and **MUST NOT** retry
  indefinitely.

## 6. Configuration and Assembly Standards

### 6.1 Layering and Priorities

- Framework configuration **MUST** live under the `yggdrasil` root key.
- Layers **MUST** use the declared priorities: `PriorityDefaults` (0),
  `PriorityFile` (1), `PriorityRemote` (2), `PriorityEnv` (3), `PriorityFlag` (4),
  `PriorityOverride` (5). See
  [05](en/05-configuration-declarative-assembly-and-hot-reload.md) §1.
- Merging **MUST** proceed by ascending priority, then by insertion order. A
  higher priority **MUST** override a lower one, and maps **MUST** deep-merge.
- Snapshots **MUST** be immutable. A change **MUST** publish a new snapshot, and
  **MUST** notify only watchers whose subscribed path actually changed.
- Application identity **MUST** be passed in code through `New` or `Run`. It
  **MUST NOT** be read from configuration.
- Secrets **MUST NOT** be committed with real values. Example configuration
  **MUST** use placeholders.
- Bootstrap-only controls such as `YGGDRASIL_CONFIG_SOURCES` **MUST** be kept out
  of the application snapshot through the source's ignored-variable settings.

### 6.2 Module Config Paths

- A module that consumes configuration **MUST** implement `Configurable` and
  return its `ConfigPath()`. The Hub then supplies a path-scoped `config.View` to
  `Init`.
- A framework path **MUST** follow `yggdrasil.<subsystem>.<module>`. A third-party
  module **SHOULD** use `yggdrasil.modules.<vendor>.<module>`.
- A third-party module **MUST NOT** claim a reserved path: `yggdrasil.server`,
  `yggdrasil.transports`, `yggdrasil.observability`, `yggdrasil.discovery`,
  `yggdrasil.extensions`, `yggdrasil.overrides`. See
  [08](en/08-implementation-boundaries-and-optimization-notes.md) §7.
- A config struct **MUST** be decoded from the scoped view in `Init` and **MUST**
  declare `mapstructure` tags. Defaults **SHOULD** be applied through
  `creasty/defaults`.
- Configuration **MUST** be validated before the process serves traffic. Invalid
  configuration **MUST** fail startup, not a first request.
- Reload change detection **MUST** compare the serialised bytes of the module's own
  config path.

### 6.3 Declarative Assembly and Default Selection

- An unknown mode **MUST** fail planning with `assembly.ErrInvalidMode`. A module
  **MUST NOT** bypass mode resolution.
- Selection **MUST** follow the documented pipeline and **MUST** expand the
  dependency closure. See [03](en/03-bootstrap-auto-assembly-and-planning.md) §5.
- Default selection **MUST** follow the documented order: code-level force, then
  `force_defaults`, then explicit configuration, then the mode default, then the
  module's `DefaultPolicy` score, then the framework fallback.
- `WithModules(...)` **MUST NOT** be treated as a forced binding. Forcing
  **MUST** require an explicit force or explicit configuration.
- An explicit provider reference that does not resolve **MUST** fail with
  `assembly.ErrUnknownExplicitBinding`.

### 6.4 Chain Templates

- Interceptor and middleware chains **MUST** be expressed as named, versioned
  templates. A black-box `auto` value **MUST NOT** be used.
- A published template version **MUST** be frozen. A behaviour change **MUST**
  publish a new version.
- Chain execution order **MUST** come only from configuration. It **MUST NOT**
  derive from module registration order, DAG order, `InitOrder()`, or map
  iteration.
- An expanded template **MUST** still be validated through `ResolveOrdered`,
  which **MUST** reject duplicate names, missing providers, and type mismatches.
- A default template **MUST NOT** enable auth, retry, hedging, or circuit
  breaking, because those change business semantics.

### 6.5 Diagnostics and Error Message Format

- A framework error message **MUST** include `stage`, `target`, `reason`, and
  `fix`, and **SHOULD** include `candidates` when more than one is possible.
- Stages are `Plan`, `Seal`, `Init`, `Prepare`, `Compose`, `Install`, `Start`,
  and `Reload`.
- The canonical shape is:

  ```text
  stage=Plan target=capability:observability.logger.handler reason=AmbiguousDefault
  candidates=json-handler,text-handler
  fix=configure yggdrasil.overrides.force_defaults.observability.logger.handler or disable one candidate module
  ```

- `assembly.Decision`, `assembly.Warning`, and `assembly.Conflict` records
  **MUST** carry `stage`, `target`, `reason`, and `fix`, and **MUST** be emitted
  in a stable order.
- Diagnostics **MUST** match `schemas/module-hub-diagnostics.schema.json`. A
  shape change **MUST** update that schema in the same change.
- Default selection, reload classification, and error semantics **MUST** remain
  explainable through diagnostics.

## 7. Observability Standards

### 7.1 Structured Logging

- Framework logging **MUST** use the standard library `log/slog`. A competing
  logging library **MUST NOT** be introduced.
- Handlers and writers **MUST** be published as `NamedOne` capabilities
  (`capabilities.LoggerHandlerSpec`, `capabilities.LoggerWriterSpec`) and
  resolved through explicit bindings, never by "first candidate".
- A module **MUST** obtain its logger through `Runtime.Logger()`, a capability,
  or a constructor parameter. Reading `slog.Default()` **MUST** require the
  isolation declaration of §5.3.
- Logs **MUST NOT** contain passwords, tokens, connection strings, or
  unnecessary personal data.
- Logger level and writer parameter changes **MAY** hot-reload.

### 7.2 Tracing and Metrics

- Tracing and metrics **MUST** use OpenTelemetry through the provider
  capabilities (`capabilities.TracerProviderSpec`,
  `capabilities.MeterProviderSpec`), selected by configuration.
- A module **MUST NOT** read OpenTelemetry globals. It **MUST** use
  `Runtime.TracerProvider()` or `Runtime.MeterProvider()`, or a capability.
- A stats handler **MUST** be published as a `NamedOne` capability
  (`capabilities.StatsHandlerSpec`), and server and client handler chains **MUST**
  be built from the configured telemetry settings.

### 7.3 Security Profiles and Secrets

- Transport security **MUST** go through the provider, profile, and material
  pipeline exposed by `capabilities.SecurityProfileProviderSpec`. A protocol
  implementation **MUST NOT** hard-code TLS or authentication behaviour.
- Certificate rotation **MAY** hot-reload. A change to an mTLS CA or a security
  mode **MUST** be restart-required unless the provider explicitly supports safe
  switching. See
  [08](en/08-implementation-boundaries-and-optimization-notes.md) §8.3.
- A credential or secret **MUST NOT** appear in a log, an error message, a test
  fixture, or committed configuration.

## 8. Testing and Verification

### 8.1 Test Location and Style

- Tests **MUST** be colocated with the code they cover, as `*_test.go`. A
  separate test tree **MUST NOT** be introduced.
- Test functions **MUST** be named `TestXxx`; benchmarks **MUST** be named
  `BenchmarkXxx`.
- Tests **MUST** use the standard `testing` package. Assertions **MUST** use
  `testify`: `require` for preconditions that make the rest of the test
  meaningless, `assert` otherwise.
- Table-driven tests with `t.Run` subtests **SHOULD** be used for related cases.
- A test of the public contract **SHOULD** live in an external `_test` package so
  it exercises only the exported surface.
- A test that needs configuration **SHOULD** use `config/testutil` rather than
  assembling a manager by hand.
- A test **MUST** assert the error code or reason through `errors.Is`,
  `xerror.IsCode`, or `xerror.IsReason`. Asserting only that an error occurred
  **MUST NOT** be accepted.

### 8.2 Minimum Coverage by Surface

A change to one of these surfaces **MUST** include the corresponding tests.
[08](en/08-implementation-boundaries-and-optimization-notes.md) §3 is the
authoritative Hub checklist.

- **Hub** (`module`): missing dependency; dependency cycle with the full path;
  stable topological order; start-failure compensation in reverse order;
  idempotent stop; cardinality conflicts for `ExactlyOne`, `OptionalOne`, and
  `NamedOne`; ordered-resolution failures for duplicate names, missing providers,
  and type mismatches; rejection of `ScopeRuntimeFactory` modules.
- **Capability** (`module`, `capabilities`): empty spec name; nil value;
  cardinality mismatch; type mismatch; duplicate `NamedOne` name.
- **Config** (`config`): layer priority and merge; watch notification on path
  change; module config-change detection.
- **Planner** (`assembly`): mode resolution; module selection and dependency
  closure; default-selection order; chain expansion; binding validation; hash
  determinism; spec diff.
- **App lifecycle** (`app`): prepare, start, and stop ordering; compose and
  install compensation; isolation reporting and `WithProcessDefaults`.
- **Transport** (`transport`): server start, handle, and stop; client state
  machine; metadata, headers, and trailers; streaming; error mapping through
  `rpc/status`.
- **Observability** (`observability`): handler, writer, and provider selection by
  name, including the unknown-name failure.
- **Code generators** (`cmd/*`): regenerated output matches the committed
  `examples/protogen` output.

### 8.3 Coverage Gate and Race Detection

- `task coverage` **MUST** pass the `COVERAGE=80` gate.
- The example modules **MUST** be excluded from coverage. This holds whatever
  `INCLUDE_EXAMPLES` says.
- Concurrency-sensitive code — client state, balancer, resolver watch, and the
  lifecycle runner — **MUST** additionally pass `task test:race`.
- Changed behaviour **MUST** add regression coverage at the narrowest useful
  layer.

### 8.4 Verification Commands

```sh
task setup            # install the pinned tools and the git hooks (once per clone)
task fmt:check        # verify formatting without rewriting
task go:lint          # run the configured golangci-lint profile per module
task test             # unit tests, examples excluded by default
task coverage         # coverage gate (COVERAGE=80, non-example modules only)
task deps:tidy:check  # verify go.mod/go.sum are tidy
task check:fast       # fmt:check + go:lint + test
task check            # check:fast + coverage + deps:tidy:check
task check:strict     # check + race tests, examples included
task doctor           # toolchain, pinned tools, git hooks
```

- A change **MUST** pass `task check` before handoff.
- A change touching the runtime hot path or concurrency **MUST** additionally
  pass `task check:strict`.
- A change to a `cmd/protoc-gen-*` template **MUST** run `task proto:generate`
  and commit the regenerated output.

## 9. Documentation and Change Review

### 9.1 Documentation

- The numbered documents **MUST** exist in both trees, `en/NN-*.md` and
  `zh_CN/NN-*.md`, and a change to one **MUST** update its counterpart in the
  same commit. This document is the deliberate exception: it is a single English
  file outside the numbered set.
- A new design term **MUST** be added to `en/00-glossary.md` and
  `zh_CN/00-术语表.md` before it is used in a numbered document.
- Code identifiers **MUST NOT** be translated.
- Each numbered document **MUST** declare its status — `Implemented`,
  `Experimental`, `Design Baseline`, `Proposal`, or `Guide` — in its front
  matter and in the documentation index.
- A proposal-level API **MUST** state its implementation status wherever it is
  documented.
- A change to default selection, reload classification, or error semantics
  **MUST** update the corresponding document.

### 9.2 Commit Messages

A commit title **MUST** match:

```text
^(feat|fix|docs|style|refactor|perf|test|build|ci|chore|revert)(\([a-zA-Z0-9_-]+\))?(!)?: [a-z].{8,}[^.]$
```

- The type **MUST** be one of `feat`, `fix`, `docs`, `style`, `refactor`, `perf`,
  `test`, `build`, `ci`, `chore`, `revert`.
- The scope is optional. When present it **MUST** name a module or package
  boundary such as `app`, `transport`, or `config`. It **MUST NOT** name a layer,
  a file type, an issue number, or an author.
- The subject **MUST** be lowercase, **MUST** be at least eight characters after
  the type prefix, and **MUST NOT** end with a period.
- The title **MUST NOT** exceed 72 characters, and body lines **MUST NOT** exceed
  100 characters.
- A breaking change **MUST** carry `!` after the scope and a `BREAKING CHANGE:`
  footer in the body.

### 9.3 Branch Names

A branch **MUST** be `main`, `master`, or `develop`, or **MUST** match
`(feature|release|hotfix)/[a-z0-9._-]+`.

### 9.4 Review Checklist

A review **MUST** answer these questions:

- Which public contract changed, and is it a breaking change requiring `!` and a
  `BREAKING CHANGE:` footer?
- Which package owns the change, and does exactly one package own it?
- Does a new extension point follow the optional-interface pattern instead of
  widening an existing interface?
- Are the capability name, cardinality, and type explicit and conflict-free?
- Does `Prepare()` still avoid every external serving action, and is `Stop()`
  idempotent?
- Is App-local isolation preserved, or is the process-default requirement
  declared?
- Do diagnostics and error messages carry `stage`, `target`, `reason`, and
  `fix`?
- Were generated files regenerated rather than hand-edited?
- Are the numbered documents updated on both sides?
- Which verification commands were run, and did `task check` pass?

## 10. AI Coding Checklist

An AI-assisted change is held to the same rules as a human one.

- Before editing, inspect the owning package, its tests, its configuration, and
  the generated-file boundaries.
- Preserve unrelated worktree changes. Do not reset, restore, or overwrite files
  outside the requested scope.
- Prefer reusing an existing interface or helper over introducing a new one. If a
  new abstraction is needed, it must add meaning, observability, or a test
  boundary.
- Do not hand-edit generated protobuf. Change the generator and run
  `task proto:generate`.
- Do not add hidden global registration such as `init()` side effects or
  package-level registries, and do not read process globals without the
  isolation declaration of §5.3.
- Before handoff, verify: the smallest responsible package owns the behaviour;
  the prepare-must-not-serve contract holds; contexts, timeouts, error mapping,
  and cleanup are handled; tests cover the changed surface; and formatting, lint,
  tests, and coverage pass.
- Report the changed files, the verification commands actually run, and any
  remaining limitation. Do not claim a verification that was not performed.
- When the repository, its tests, and the requested change disagree, follow the
  existing code unless the change explicitly updates the affected contract and
  its regression coverage.
