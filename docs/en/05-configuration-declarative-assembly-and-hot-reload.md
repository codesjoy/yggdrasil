---
status: Design Baseline
applies_to: Yggdrasil v3
document_type: architecture documentation
last_reviewed: TBD
---

# 05. Configuration, Declarative Assembly, and Hot Reload

> This document describes Yggdrasil's layered configuration model, declarative assembly planner, spec diffing, and staged hot reload semantics.

## 1. Layered Configuration Model

Yggdrasil uses immutable configuration snapshots. `Manager` owns multiple named layers, each with a priority. When any layer changes, the manager merges all layers, creates a new atomic snapshot, and notifies watchers only when their subscribed path actually changes.

Priorities from lowest to highest:

| Priority | Value | Source |
| --- | ---: | --- |
| `PriorityDefaults` | 0 | Hard-coded defaults |
| `PriorityFile` | 1 | YAML/JSON/TOML files |
| `PriorityRemote` | 2 | Remote configuration center |
| `PriorityEnv` | 3 | Environment variables |
| `PriorityFlag` | 4 | Command-line flags |
| `PriorityOverride` | 5 | Programmatic overrides |

Merge order: priority ascending, then insertion order ascending. Higher-priority values override lower-priority values; maps are deep-merged.

### 1.1 Source Loading

Configuration layers can come from explicit files, declarative sources, environment variables, flags, and programmatic overrides. File-based bootstrap starts from `WithConfigPath(...)` or the bootstrap config flag. If no config file, bootstrap source, or programmatic config source is loaded, the App installs default sources:

- an environment source with `YGGDRASIL` prefix, array parsing with `,`, and priority `PriorityEnv`;
- a command-line flag source with priority `PriorityFlag`.

Application identity is not part of the configuration tree. Pass it explicitly to `yggdrasil.Run(ctx, appName, ...)`, `yggdrasil.New(appName, ...)`, or `app.New(appName, ...)`.

### 1.2 Declarative Config Sources

Config sources can be declared in a config file under `yggdrasil.config.sources`, or during bootstrap with `YGGDRASIL_CONFIG_SOURCES` / `--yggdrasil-config-sources`. Bootstrap declarations are useful when the config file location or remote source itself must be discovered from env or flags.

Bootstrap source declarations accept:

- a JSON object: `{"kind":"env","priority":"env","config":{"prefixes":["APP"]}}`;
- a JSON array of source specs;
- a compact list: `env:APP:env,flag::flag`.

Built-in source kinds:

| Kind | Purpose | Notable config |
| --- | --- | --- |
| `file` | Load YAML/JSON/TOML config files | path and priority |
| `env` | Load environment variables | `prefixes`, `stripped_prefixes`, `parse_array`, `array_sep`, `ignored_vars` |
| `flag` | Load command-line flags | `ignored_names` |

The built-in env source ignores `YGGDRASIL_CONFIG_SOURCES` by default. The built-in flag source ignores bootstrap flags such as `yggdrasil-config` and `yggdrasil-config-sources` by default, so bootstrap controls do not leak into the application config snapshot.

Custom declarative sources can be registered with `WithConfigSourceBuilder(kind, builder)` or by modules implementing `module.ConfigSourceProvider`. Context-aware builders receive the snapshot loaded before the source is built, which lets a source use base config to locate credentials, endpoints, or namespaces.

## 2. Snapshot and View

A snapshot is immutable:

```go
type Snapshot struct { /* unexported */ }
```

Key methods:

| Method | Purpose |
| --- | --- |
| `Section(path ...string)` | Returns a sub-snapshot |
| `Decode(target any)` | Decodes into a struct |
| `Map()` | Returns a cloned map |
| `Bytes()` | Returns JSON encoding |
| `Empty()` | Reports whether the snapshot is empty |
| `Value()` | Returns a normalized clone |

A `View` is a scoped lens used by modules:

```go
type View interface {
    Path() string
    Decode(target any) error
    Sub(path string) View
    Exists() bool
}
```

When a module implements `Configurable`, the Hub automatically provides a scoped view for the declared path.

## 3. Declarative Assembly Planner

The planner is a pure function: it takes a configuration snapshot, module candidates, and overrides, then produces a deterministic result:

- active modules;
- capability defaults;
- chain selections;
- canonical spec;
- SHA-256 hash.

It does not instantiate runtime objects. The App layer consumes the planner result and registers modules in the Hub.

## 4. Planner Pipeline

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

### 4.1 resolveModules

- handles `DisableModule`;
- always includes required modules;
- includes non-auto modules by default;
- includes auto modules when `AutoRule` matches;
- includes `EnableModule` targets;
- expands dependency closure.

### 4.2 resolveDefaults

Default selection order:

```text
code ForceDefault
  -> config force_defaults
  -> explicit config
  -> mode default
  -> module fallback score
  -> framework fallback
```

If no deterministic result exists and multiple providers are available, the planner returns `ErrAmbiguousDefault`.

### 4.3 resolveChains

A chain can be an explicit list or a template:

```yaml
yggdrasil:
  overrides:
    force_templates:
      rpc.interceptor.unary_server: default-observable@v1
```

After expansion, it is still an explicit ordered list validated through Hub `ResolveOrdered`.

## 5. Spec / Hash / Diff / Explain

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

- `Hash(spec)`: computes SHA-256 over canonical JSON.
- `Diff(oldSpec, newSpec)`: compares mode, modules, defaults, chains, and overrides.
- `Explain(spec)`: renders pretty JSON for diagnostics and dry-run.

## 6. Configuration Change Detection

Module-level reload detects changes by comparing JSON bytes for the module's config path:

```go
func configChanged(mod Module, oldSnap, newSnap config.Snapshot) bool {
    path := ""
    if item, ok := mod.(Configurable); ok { path = item.ConfigPath() }
    if path == "" { return false }
    parts := splitDotPath(path)
    return string(oldSnap.Section(parts...).Bytes()) != string(newSnap.Section(parts...).Bytes())
}
```

Only modules whose config path changed enter the reload set, unless `reloadAll` is requested.

## 7. Staged Reload

Full reload path:

```text
Config Watch
  -> build new assembly.Spec
  -> diff old/new Spec
  -> classify restart-required or hot-reloadable
  -> Hub.Reload
  -> PrepareReload for affected modules
  -> Commit all prepared modules
  -> publish new snapshot only when fully committed
```

### 7.1 Prepare failure

- do not enter commit;
- roll back already-prepared modules in reverse order;
- enter degraded state if rollback fails.

### 7.2 Commit failure

- stop later commits;
- roll back prepared but uncommitted modules;
- mark diverged / restart-required;
- do not promise automatic restoration of the old world.

### 7.3 Rollback failure

- record failed module and failed stage;
- enter `ReloadPhaseDegraded`;
- expose status through governor / diagnostics;
- require external restart.

## 8. ReloadState

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

| Phase | Meaning |
| --- | --- |
| `idle` | Normal operation |
| `preparing` | Preparing reload |
| `committing` | Committing new state |
| `rollback` | Rolling back after failure |
| `degraded` | Cannot automatically recover; restart required |

## 9. Restart-Required Classification

The system should mark restart-required when:

- modules are added or removed;
- a capability default changes and cannot be switched smoothly;
- a non-reloadable module's config changes;
- `BusinessBundle` structure changes;
- service bindings change and require rebuilding the business graph;
- staged reload enters degraded state;
- the first reload implementation would need to re-run `Compose`.

## 10. Configuration Example

```yaml
yggdrasil:
  mode: prod-grpc
  server:
    transports:
      - "grpc"
  transports:
    grpc:
      server:
        address: ":9090"
    http:
      rest:
        host: "0.0.0.0"
        port: 8080
  observability:
    logging:
      handlers:
        default:
          type: json
      writers:
        default:
          type: console
    telemetry:
      tracer: otel
      meter: otel
  discovery:
    registry:
      type: multi_registry
  extensions:
    interceptors:
      unary_server: default-observable@v1
  overrides:
    force_defaults:
      observability.logger.handler: json
    disable_modules:
      - observability.stats.otel
```

## 11. Operations Troubleshooting

- Check whether plan hash changed.
- Inspect Spec Diff: module/default/chain/config changes usually imply different actions.
- If restart-required is set, check for business graph changes or non-reloadable modules.
- If degraded, inspect failed module and failed stage.
- If `AmbiguousDefault` occurs, explicitly configure a provider or disable candidates.
- If `UnknownExplicitBinding` occurs, check whether the referenced module was enabled.
