---
status: Guide
applies_to: Yggdrasil v3
document_type: architecture documentation
last_reviewed: TBD
---

# 07. Developer Practices and Extension Guide

> This document provides practical templates and checklists for application developers, module authors, provider authors, configuration authors, and operators.

## 1. Business Application Template

### 1.1 Recommended main.go

```go
func main() {
    if err := yggdrasil.Run(
        context.Background(),
        "order-service",
        business.Compose,
        yggdrasil.WithConfigPath("app.yaml"),
    ); err != nil {
        panic(err)
    }
}
```

`yggdrasil.Run` targets a single-main-App program and may install process-default logger / OTel / legacy instance facades. `appName` is required and is not read from configuration. Tests, multi-App, sidecar, and embedded scenarios should use `yggdrasil.New(appName, ...)` / `app.New(appName, ...)` and keep the default App-local runtime; set `WithProcessDefaults(true)` only when compatibility with global APIs is required.

### 1.2 Recommended business.Compose

```go
func Compose(rt yggdrasil.Runtime) (*yggdrasil.BusinessBundle, error) {
    userClient, err := rt.NewClient(context.Background(), "user-service")
    if err != nil { return nil, err }

    svc := &OrderService{
        Users:  userClient,
        Logger: rt.Logger(),
    }

    return &yggdrasil.BusinessBundle{
        RPCBindings: []yggdrasil.RPCBinding{{
            ServiceName: "OrderService",
            Desc:        orderpb.OrderServiceDesc,
            Impl:        svc,
        }},
    }, nil
}
```

## 2. Module Author Checklist

### 2.1 Basic module

- [ ] `Name()` is globally unique and stable.
- [ ] Hard dependencies use `DependsOn()`, not `InitOrder()`.
- [ ] Implement `Configurable` when configuration is needed.
- [ ] `Init()` initializes long-lived resources only; it must not serve externally.
- [ ] `Start()` starts background processes or serving behavior.
- [ ] `Stop()` is idempotent, protected by `sync.Once` or atomic state.
- [ ] Implement `Reloadable` when hot reload is supported, with clear prepare/commit/rollback semantics.
- [ ] Capability exposure declares spec name, cardinality, and type.
- [ ] Implement `IsolationReporter` and declare `IsolationModeRequiresProcessDefaults` when process globals are required.

### 2.2 Auto-assembly module

- [ ] Implements `AutoDescribed`.
- [ ] `AutoRule.Match()` is pure.
- [ ] `AffectedPaths()` is declared.
- [ ] `DefaultPolicy` scores are clear and avoid same-score ambiguity with peers.
- [ ] Related decisions can be explained through `assembly.Spec.Decisions`.

## 3. Provider Author Checklist

- [ ] Provider objects may enter the Hub, but dynamic runtime instances must not.
- [ ] Prepare phase constructs objects and internal helpers only.
- [ ] Start phase performs bind/listen/register.
- [ ] Close/Stop paths are complete and do not leak goroutines.
- [ ] If the implementation cannot separate prepare from external serving, it does not satisfy the Yggdrasil provider contract.

### 3.1 When to Use CapabilityRegistration vs Module

- Use `WithCapabilityRegistrations(...)` when you only need to add providers to existing capabilities and do not need `Start/Stop/Reload/DependsOn`.
- Keep using `WithModules(...)` and full `module.Module` implementations when you need lifecycle hooks, dependency ordering, hot reload, or auto assembly.

`CapabilityRegistration` is intended for provider-only extensions such as transport providers, logger handlers, resolver/balancer providers, and REST middleware.

Constraints:

- The `Capabilities` callback may run before `Init`, and it may run more than once.
- `Capabilities` must therefore be deterministic, pure, and side-effect free.
- When capability values depend on configuration, return lazy providers/factories and defer reading shared state until runtime objects are actually built.
- `CapabilityRegistration` does not support `Reloadable`; if provider behavior must be rebuilt on config change, use a full module instead.

### 3.2 Config Source Extensions

- Use `WithConfigSourceBuilder(kind, builder)` when an application owns one custom declarative config source.
- Implement `module.ConfigSourceProvider` when a module contributes reusable source kinds.
- Builders should be deterministic and validate their `SourceSpec` eagerly.
- Context-aware builders may inspect the already loaded base snapshot, but should not mutate global state or assume later sources are present.

## 4. Configuration Author Checklist

- [ ] Use `yggdrasil.mode` to express environment and bundle mode.
- [ ] Pass app identity in code through `Run` / `New`; do not configure it in YAML.
- [ ] When defaults are not desired, prefer explicit configuration or `force_defaults`.
- [ ] Prefer versioned templates for chain extensions.
- [ ] Use `YGGDRASIL_CONFIG_SOURCES` or `--yggdrasil-config-sources` only for bootstrap source discovery.
- [ ] Use source `ignored_vars` / `ignored_names` to keep bootstrap-only controls out of the application snapshot.
- [ ] Configure high-risk components such as auth, retry, and circuit-breaker explicitly; do not rely on default templates.
- [ ] When `AmbiguousDefault` occurs, do not reorder module registration; explicitly choose a provider or disable candidates.

## 5. Operations Troubleshooting Checklist

### 5.1 Startup failure

1. Identify the failed stage: Plan / Seal / Init / Prepare / Compose / Install / Start.
2. Plan failure: inspect mode, template, default selector, and overrides.
3. Seal failure: inspect DAG, missing dependencies, and capability cardinality.
4. Init failure: inspect module configuration decoding and resource initialization.
5. Compose failure: inspect business clients and dependency construction.
6. Install failure: inspect service names, desc/impl types, and route conflicts.
7. Start failure: inspect transport binding, registry, and server handlers.

### 5.2 Reload issue

1. Inspect old/new plan diff first.
2. Module set changes usually require restart-required.
3. Business bundle structure changes require restart-required in the first implementation.
4. `PrepareReload` failure usually indicates invalid new configuration.
5. `Commit` failure points to module commit logic or external dependencies.
6. `Rollback` failure should be treated as degraded state and followed by restart planning.

## 6. Common Anti-Patterns

| Anti-pattern | Problem | Recommended approach |
| --- | --- | --- |
| Registering global providers in package `init()` | Breaks App instance isolation | Use explicit module registration or auto assembly |
| Reading slog/OTel/instance globals from module runtime paths | Depends on the process-default facade and cross-talks between Apps | Receive App-local dependencies through Runtime, capabilities, or constructor parameters |
| Using `InitOrder` for hard dependency | Tie-breaker cannot guarantee dependency ordering | Use `DependsOn` |
| Taking the first provider on capability conflict | Non-deterministic and not diagnosable | Use cardinality validation and explicit config |
| Mutating server runtime directly in Compose | Bypasses install boundary | Return `BusinessBundle` |
| Listening in Prepare phase | Breaks the runtime-ready contract | Bind/listen only in Start |
| Automatically re-running business Compose during reload | Too complex for first implementation | Mark restart-required |
| Putting resolver watches in the Hub | Violates dynamic object boundary | Let client runtime manage them |

## 7. Recommended Error Message Format

Error messages should include:

- stage: Plan / Seal / Init / Prepare / Compose / Install / Start / Reload;
- target: module / capability / template / binding;
- reason: failure cause;
- available: possible candidates;
- fix: recommended fix.

Example:

```text
stage=Plan target=capability:observability.logger.handler reason=AmbiguousDefault
candidates=json-handler,text-handler
fix=configure yggdrasil.overrides.force_defaults.observability.logger.handler or disable one candidate module
```

## 8. Documentation Maintenance Guidance

- Architecture and Hub documents should remain stable as long-term design baselines.
- Bootstrap documentation should be versioned with the proposal or implementation version.
- Lifecycle documentation must be updated with public API changes.
- Config/Reload documentation must include current error semantics and restart-required rules.
- Transport documentation should evolve with new protocol providers, security profiles, and balancers.
