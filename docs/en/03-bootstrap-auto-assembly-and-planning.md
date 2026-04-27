---
status: Design Baseline
applies_to: Yggdrasil v3
document_type: architecture documentation
last_reviewed: TBD
---

# 03. Bootstrap, Auto Assembly, and Planning

> This document describes the Bootstrap / Auto Assembly / Compose / Install layer built on top of the Yggdrasil v3 modular kernel.

## 1. Background

Yggdrasil's modular kernel is intentionally strict. However, if every application manually selects modules, providers, capability defaults, and interceptor chains, daily business development becomes unnecessarily expensive.

The Bootstrap layer solves this by compiling common configuration-driven scenarios into an explicit, explainable, comparable, and hashable plan. It does not weaken the kernel; it produces inputs for the kernel.

## 2. Core Objects

### 2.1 assembly.Spec

The public planning type is `assembly.Spec`. It is the single source of truth for planning:

```go
type Spec struct {
    Identity  IdentitySpec
    Mode      Mode
    Modules   []ModuleRef
    Defaults  map[string]string
    Chains    map[string]Chain
    Decisions []Decision
    Warnings  []Warning
    Conflicts []Conflict
}
```

Requirements:

- contains only stable, serializable data;
- contains no `module.Module` instances;
- all maps and slices are canonicalized before output;
- explain, dry-run, diff, and hash are based on it;
- reload comparisons use it instead of runtime instance identity.

### 2.2 Prepared Runtime Assembly

The prepared runtime assembly is an internal App implementation detail instantiated from `assembly.Spec`. It is not an exported public API. Conceptually it contains:

```go
type preparedRuntimeGraph struct {
    Spec      *assembly.Spec
    Modules   []module.Module
    Runtime   app.Runtime
    Server    server.Server
    CloseFunc func(context.Context) error
}
```

It does not participate in hash or diff. It owns runtime resources and the close/rollback path.

## 3. Mode System

Users see only one concept: `mode`.

```yaml
yggdrasil:
  mode: prod-grpc
```

Internally, a mode resolves into a profile and a bundle:

| mode | profile | bundle | Typical meaning |
| --- | --- | --- | --- |
| `dev` | `dev` | `server-basic` | text logging, basic server, REST enabled, development-friendly defaults |
| `prod-grpc` | `prod` | `grpc-server` | JSON logging, OpenTelemetry, gRPC server, production defaults |
| `prod-http-gateway` | `prod` | `http-gateway` | HTTP gateway, REST middleware, production defaults |

A mode may recommend default bundles, logger/tracer/meter providers, and chain template versions. It must never bypass Hub DAG validation, capability validation, or lifecycle control.

## 4. AutoDescribed and AutoRule

A module that wants to participate in auto assembly may implement:

```go
type AutoDescribed interface {
    AutoSpec() AutoSpec
}

type AutoSpec struct {
    Provides      []CapabilitySpec
    AutoRules     []AutoRule
    DefaultPolicy *DefaultPolicy
}
```

`AutoRule` must be pure:

```go
type AutoRule interface {
    Match(ctx AutoRuleContext) bool
    Describe() string
    AffectedPaths() []string
}
```

Constraints:

- no dependence on time, randomness, or mutable global state;
- reads only immutable config snapshots, resolved mode, and static context;
- declares affected config paths for reload classification;
- never lets map iteration order influence decisions.

## 5. Auto Assembly Pipeline

```text
resolveMode
  -> resolveModules
  -> collectProviders
  -> resolveDefaults
  -> resolveChains
  -> buildEffectiveResolved
  -> compileCapabilityBindings
  -> validateBindings
```

1. `resolveMode`: parses `yggdrasil.mode`; unknown modes return `InvalidMode`.
2. `resolveModules`: handles disabled modules, force-enabled modules, required modules, auto rules, and dependency closure.
3. `collectProviders`: groups providers by capability spec.
4. `resolveDefaults`: chooses providers for defaultable capabilities.
5. `resolveChains`: resolves explicit lists or interceptor/middleware chain templates.
6. `buildEffectiveResolved`: merges defaults and chains into effective resolved settings.
7. `compileCapabilityBindings`: produces capability binding maps for the Hub/runtime.
8. `validateBindings`: validates explicit provider references and types.

## 6. Default Selection Algorithm

Default selection is deterministic:

```text
1. code override: ForceDefault
2. config override: yggdrasil.overrides.force_defaults
3. explicit config: telemetry.tracer = xxx
4. mode default
5. module fallback: AutoSpec.DefaultPolicy score
6. framework fallback
```

Conflict rules:

- if multiple legal candidates exist at the same source level with the same score, return `AmbiguousDefault`;
- do not silently choose the lexicographically first module for convenience;
- `WithModules(...)` affects the candidate set but is not a forced binding;
- forced binding requires `ForceDefault` or explicit config.

## 7. Default Chain Templates

Chain templates must be named and versioned. They should not use a black-box `auto` value.

```yaml
extensions:
  interceptors:
    unary_server: default-observable@v1
    unary_client: default-client-safe@v1
```

Template definition:

```go
type ChainTemplate struct {
    Name    string
    Version string
    Items   []string
}
```

Rules:

- once `default-observable@v1` is published, its content is frozen;
- breaking or behavior-changing updates should publish `@v2`;
- current `@v1` built-in templates are intentionally minimal: RPC templates expand to `logging`, and REST observable templates expand to `logger`;
- future templates may add other low-risk components such as recovery, tracing, metrics, or request-id only through a new version;
- auth, retry, hedging, and circuit-breaker should not be enabled by default because they change business semantics;
- expanded templates are still validated through `ResolveOrdered`.

## 8. Explain / Dry-Run / Diff / Hash

### Explain

Outputs App identity, mode/profile/bundle, enabled modules, selected defaults and sources, expanded chain templates, decision records, warnings, and conflicts.

### Dry-Run

Generates only `assembly.Spec`. It does not instantiate runtime objects and does not bind, listen, or register instances.

### Diff

Compares two `assembly.Spec` values: mode, module refs, defaults, chains, and overrides.

### Hash

Computes a SHA-256 hash over canonical JSON. Addresses, interface instances, and non-deterministic fields are forbidden.

## 9. Error Semantics

| Error | Meaning | Fix |
| --- | --- | --- |
| `InvalidMode` | Unknown mode | Use a built-in mode or register a new mode |
| `UnknownTemplate` | Chain template does not exist | Check template name |
| `TemplateVersionNotFound` | Template version does not exist | Use a published version |
| `AmbiguousDefault` | Default provider conflict | Explicitly choose a provider or disable candidates |
| `ConflictingOverride` | Overrides conflict with each other | Merge or remove conflicting configuration |
| `UnknownExplicitBinding` | Explicit provider does not exist | Check whether the module is enabled and the name is correct |
| `InvalidAutoRule` | AutoRule is invalid | Fix purity, affected paths, or matching logic |

## 10. Recommended High-Level Entry

```go
return yggdrasil.Run(
    ctx,
    business.Compose,
    yggdrasil.WithConfigPath("app.yaml"),
)
```

## 11. Canonicalization Checklist

- [ ] Convert maps into sorted slices before they affect the next decision stage.
- [ ] Emit decision records in stable order.
- [ ] Sort provider candidates by module name for deterministic processing.
- [ ] Emit warnings and conflicts in stable order.
- [ ] Compute hash from canonical `assembly.Spec` only.
- [ ] Never read prepared runtime assembly state during diff.
