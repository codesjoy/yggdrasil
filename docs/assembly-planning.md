# Declarative Assembly Planning

> This document describes the config-driven, declarative assembly planner that determines which modules are active, how capabilities are wired, and what defaults apply.

## Overview

The Yggdrasil assembly planner is a pure-function pipeline that takes immutable input (config snapshot, module candidates, overrides) and produces a deterministic `Result` containing:

- The list of active modules (in input order).
- Resolved capability defaults and chain selections.
- A canonical `Spec` for auditing and diffing.
- A SHA-256 hash for change detection.

The planner does **not** instantiate runtime objects. It only makes decisions. The App layer consumes the `Result` to register modules in the Hub and apply runtime bindings.

Key source files:

| File | Responsibility |
|---|---|
| `assembly/planner.go` | `Plan()`, `DryRun()`, pipeline orchestration |
| `assembly/selection.go` | Mode definitions, default/chain selection, capability constants |
| `assembly/modules.go` | Module resolution, auto-rule matching, dependency expansion |
| `assembly/overrides.go` | Override types: `EnableModule`, `DisableModule`, `ForceDefault`, `ForceTemplate`, `DisableAuto` |
| `assembly/spec.go` | `Spec`, `SpecDiff`, `Hash`, `Diff`, `Explain` |

---

## Planner Pipeline

The planner executes eight sequential steps inside `build()`:

```
resolveMode → resolveModules → collectProviders → resolveDefaults →
resolveChains → buildEffectiveResolved → compileCapabilityBindings → validateBindings
```

### Step 1: resolveMode

Resolves the built-in mode name from `yggdrasil.mode` config. If the mode is unknown, the planner returns `ErrInvalidMode`. When empty, the planner runs without mode defaults.

### Step 2: resolveModules

Determines which modules are active:

1. Process `DisabledModules` overrides (reject unknown or protected modules).
2. For each candidate module:
   - Skip disabled modules.
   - Required modules (`foundation.capabilities`, `connectivity.capabilities`, `foundation.runtime`, `connectivity.runtime`) are always included.
   - Non-auto modules (those not implementing `AutoDescribed`) are always included.
   - Auto modules are included when at least one `AutoRule` matches the current context.
   - `EnableModule` overrides force-include auto modules regardless of rule matching.
3. Expand the dependency closure: transitively include all `DependsOn()` targets.

### Step 3: collectProviders

Builds an index of available capability providers from the active modules. For each `CapabilityProvider`, records the provider name keyed by capability spec name.

### Step 4: resolveDefaults

For each defaultable capability, selects a default provider using a priority chain (see below).

### Step 5: resolveChains

For each chain path (interceptor chains, REST middleware chains), selects a chain template or explicit items using a priority chain similar to defaults.

### Step 6: buildEffectiveResolved

Merges the selected defaults and chains into the effective resolved settings that downstream code will use.

### Step 7: compileCapabilityBindings

Compiles the effective resolved settings into a `map[string][]string` of capability bindings. This map is later passed to `Hub.SetCapabilityBindings()`.

### Step 8: validateBindings

Verifies that all explicitly referenced providers are available in the active module set.

---

## Three Built-in Modes

### dev

| Property | Value |
|---|---|
| Profile | `dev` |
| Bundle | `server-basic` |
| Logger Handler | `text` |
| Logger Writer | `console` |
| Registry | `multi_registry` |
| Interceptor Templates | `default-observable@v1` (client + server), includes REST |

### prod-grpc

| Property | Value |
|---|---|
| Profile | `prod` |
| Bundle | `grpc-server` |
| Logger Handler | `json` |
| Logger Writer | `console` |
| Tracer | `otel` |
| Meter | `otel` |
| Registry | `multi_registry` |
| Interceptor Templates | `default-observable@v1` (server), `default-client-safe@v1` (client), no REST |

### prod-http-gateway

| Property | Value |
|---|---|
| Profile | `prod` |
| Bundle | `http-gateway` |
| Logger Handler | `json` |
| Logger Writer | `console` |
| Tracer | `otel` |
| Meter | `otel` |
| Registry | `multi_registry` |
| Interceptor Templates | `default-observable@v1` (client + server), includes REST |

---

## Auto-Assembly Rules

Modules implementing `AutoDescribed` provide an `AutoSpec`:

```go
type AutoSpec struct {
    Provides      []CapabilitySpec
    AutoRules     []AutoRule
    DefaultPolicy *DefaultPolicy
}
```

### AutoRule

```go
type AutoRule interface {
    Match(ctx AutoRuleContext) bool
    Describe() string
    AffectedPaths() []string
}
```

The planner evaluates each rule against an `AutoRuleContext` containing the app name, config snapshot, and resolved mode. A module is included when **any** rule matches.

### DefaultPolicy

```go
type DefaultPolicy struct {
    Profiles []string
    Score    int
}
```

Modules can declare a fallback preference for default selection. Higher scores win. Profile filters restrict the policy to specific mode profiles.

---

## Override System

The planner supports five override types, applicable from both code and config:

| Override | Effect |
|---|---|
| `EnableModule(name)` | Force-include an auto module regardless of rules |
| `DisableModule(name)` | Skip a module (protected modules reject this) |
| `ForceDefault(path, module)` | Force a specific provider for a capability |
| `ForceTemplate(path, template, version)` | Force a specific chain template |
| `DisableAuto(path)` | Disable automatic default/chain selection for a path |

### Config Overrides (YAML)

```yaml
yggdrasil:
  overrides:
    disable_modules:
      - observability.stats.otel
    force_defaults:
      observability.logger.handler: json
    force_templates:
      rpc.interceptor.unary_server: default-observable@v1
    disable_auto:
      - observability.logger.handler
```

### Code Overrides

```go
assembly.Plan(ctx, assembly.Input{
    Modules:   modules,
    Overrides: []assembly.Override{
        assembly.ForceDefault("observability.logger.handler", "json"),
        assembly.DisableModule("observability.stats.otel"),
    },
})
```

---

## Spec, Hash, and Diff

### Spec

The `Spec` struct is the canonical, serializable representation of one assembly plan:

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

### Hash

```go
func Hash(spec *Spec) (string, error)
```

Computes a stable SHA-256 hash over the canonical JSON encoding of the spec. The canonical encoding sorts all maps and slices for determinism.

### Diff

```go
func Diff(oldSpec, newSpec *Spec) (*SpecDiff, error)
```

Computes a stable diff between two specs, identifying changes in mode, modules (added/removed), defaults, chains, and overrides. The diff is used by the hot-reload system to decide whether a restart is required.

### Explain

```go
func Explain(spec *Spec) ([]byte, error)
```

Renders the canonical spec as pretty-printed JSON for diagnostics.

---

## Default Selection Priority Chain

For each defaultable capability, the planner tries these sources in order, stopping at the first match:

```
1. code override (ForceDefault from Go code)
2. config override (force_defaults from YAML)
3. explicit config (e.g., yggdrasil.observability.logging.handlers.default.type)
4. mode default (dev/prod-grpc/prod-http-gateway preset)
5. module fallback (AutoSpec.DefaultPolicy, highest score wins)
6. framework fallback (hard-coded preference order)
```

If no source produces a value and multiple providers are available, the planner returns `ErrAmbiguousDefault` for that capability.

See [App Lifecycle & Business Composition](app-lifecycle.md) for how spec diffs drive reload decisions.
