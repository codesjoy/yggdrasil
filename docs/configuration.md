# Layered Configuration System

> This document describes the immutable snapshot, layered merging, and hot-watch mechanisms in the Yggdrasil configuration system.

## Overview

Yggdrasil uses an immutable-snapshot configuration model. The `Manager` owns multiple named layers, each with a priority. When any layer changes, all layers are merged in priority order to produce a single atomic snapshot. Watchers are notified only when their subscribed path actually changes.

Key source files:

| File | Responsibility |
|---|---|
| `config/manager.go` | `Manager`, layer management, merging, watching |
| `config/snapshot.go` | `Snapshot`, `Section`, `Decode`, `Bytes` |
| `config/view.go` | `View` interface, `NewView`, `Sub`, `Exists` |
| `config/source/source.go` | `Source`, `Watchable`, `Data`, `Parser` |

---

## Manager and Layer

### Priority Levels

Six priority levels from lowest to highest:

| Priority | Value | Source |
|---|---|---|
| `PriorityDefaults` | 0 | Hard-coded defaults |
| `PriorityFile` | 1 | Configuration file (YAML/JSON/TOML) |
| `PriorityRemote` | 2 | Remote config center |
| `PriorityEnv` | 3 | Environment variables |
| `PriorityFlag` | 4 | Command-line flags |
| `PriorityOverride` | 5 | Programmatic overrides |

### Layer Structure

```go
type layer struct {
    name     string
    priority Priority
    order    uint64      // insertion order for stable sort
    data     map[string]any
    src      source.Source
    stop     chan struct{}
}
```

### LoadLayer

```go
func (m *Manager) LoadLayer(name string, priority Priority, src source.Source) error
```

1. Read data from the source via `src.Read()`.
2. Decode and normalize the data into `map[string]any`.
3. If the source implements `Watchable`, start a watch goroutine.
4. Commit the layer, merge all layers, and notify watchers if the snapshot changed.

### Merging

Layers are sorted by:
1. Priority (ascending).
2. Insertion order (ascending, for layers with equal priority).

The merge is a deep map merge: higher-priority values override lower-priority values at each path.

---

## Snapshot and View

### Snapshot

```go
type Snapshot struct {
    // contains filtered or unexported fields
}
```

Snapshots are immutable. Key methods:

| Method | Signature | Description |
|---|---|---|
| `Section` | `(path ...string) Snapshot` | Returns a nested sub-snapshot |
| `Decode` | `(target any) error` | Decodes into a typed struct |
| `Map` | `() map[string]any` | Returns the raw map |
| `Bytes` | `() []byte` | Returns JSON encoding |
| `Empty` | `() bool` | Reports nil/empty state |
| `Value` | `() any` | Returns a normalized clone |

### View

```go
type View interface {
    Path() string
    Decode(target any) error
    Sub(path string) View
    Exists() bool
}
```

A `View` is a scoped lens over a snapshot, created with `NewView(path, snapshot)`. Views support dot-path sub-views via `Sub()`, enabling modules to navigate their configuration hierarchy.

### View Creation

```go
func NewView(path string, snapshot Snapshot) View
```

The module system creates views automatically in `moduleView()`:

```go
func moduleView(mod Module, snap config.Snapshot) config.View {
    path := ""
    if item, ok := mod.(Configurable); ok {
        path = item.ConfigPath()
    }
    if path == "" {
        return config.NewView("", snap)
    }
    return config.NewView(path, snap.Section(splitDotPath(path)...))
}
```

---

## Source Interface and Watchable

### Source

```go
type Source interface {
    Kind() string
    Name() string
    Read() (Data, error)
    io.Closer
}
```

### Data

```go
type Data interface {
    Bytes() []byte
    Unmarshal(v any) error
}
```

Built-in data types:
- `NewBytesData(data, parser)` — Raw bytes with a parser function.
- `NewMapData(data)` — In-memory map data.

### Watchable

```go
type Watchable interface {
    Watch() (<-chan Data, error)
}
```

Sources implementing `Watchable` stream replacement snapshots. The manager reads from the channel in a background goroutine and triggers re-merge on each update.

### Parser

```go
type Parser func([]byte, any) error
```

Supported formats: `json`, `yaml`/`yml`, `toml`.

---

## Module Configuration Usage

Modules consume configuration through the `Configurable` + `Initializable` pattern:

1. Implement `Configurable` to declare the config path:

   ```go
   func (m *MyModule) ConfigPath() string { return "yggdrasil.my_module" }
   ```

2. Implement `Initializable` to receive a scoped view:

   ```go
   func (m *MyModule) Init(ctx context.Context, view config.View) error {
       var cfg MyConfig
       if err := view.Decode(&cfg); err != nil {
           return err
       }
       // use cfg...
       return nil
   }
   ```

3. For hot-reload, implement `Reloadable`:

   ```go
   func (m *MyModule) PrepareReload(ctx context.Context, view config.View) (ReloadCommitter, error) {
       var next MyConfig
       if err := view.Decode(&next); err != nil {
           return nil, err
       }
       return &myCommitter{mod: m, next: next}, nil
   }
   ```

The Hub detects which modules need reload by comparing the old and new snapshot sections at each module's config path.

---

## Config Change Detection in Hot Reload

The `configChanged()` function in `module/lifecycle.go` determines if a module's configuration has changed:

```go
func configChanged(mod Module, oldSnap, newSnap config.Snapshot) bool {
    path := ""
    if item, ok := mod.(Configurable); ok {
        path = item.ConfigPath()
    }
    if path == "" {
        return false
    }
    parts := splitDotPath(path)
    return string(oldSnap.Section(parts...).Bytes()) != string(newSnap.Section(parts...).Bytes())
}
```

It compares the JSON byte representation of the old and new snapshot sections. Only modules with changed config are included in the reload set (unless `reloadAll` is set).

---

## Configuration File Structure Reference

### Full YAML Example

```yaml
yggdrasil:
  mode: dev

  application:
    name: my-service
    version: "1.0.0"

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

  server:
    transports:
      - protocol: grpc
        address: ":8080"
    rest:
      enabled: true
    interceptors:
      unary:
        - logging
        - recovery
      stream:
        - logging

  transports:
    http:
      rest:
        middleware:
          all:
            - logging
            - recovery
          rpc:
            - logging

  discovery:
    registry:
      type: multi_registry

  clients:
    defaults:
      interceptors:
        unary:
          - logging
        stream:
          - logging

  extensions:
    interceptors:
      unary_server: "default-observable@v1"
    middleware:
      rest_all: "default-rest-observable@v1"

  overrides:
    disable_modules:
      - telemetry.stats.otel
    force_defaults:
      observability.logger.handler: json
    force_templates:
      rpc.interceptor.unary_server: "default-observable@v1"
    disable_auto:
      - observability.logger.handler
```

See [App Lifecycle & Business Composition](app-lifecycle.md) for how configuration changes trigger hot reload, and [Module System Design](module-system.md) for how modules consume config views.
