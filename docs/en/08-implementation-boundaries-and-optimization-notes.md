# 08. Implementation Boundaries and Optimization Notes

> This document adds implementation-facing constraints for the optimized documentation set: implementation status, tests, error shape, first hot reload boundary, and production failure semantics.

## 1. Document Status

Each document should keep one of the following status values in the README status table or document header:

| Status | Meaning |
|---|---|
| `Implemented` | Implemented and covered by tests |
| `Experimental` | Usable, but APIs may change |
| `Design Baseline` | Design is accepted, implementation may be staged |
| `Proposal` | Still under review |
| `Guide` | Practice or maintenance guide |

The Bootstrap / Auto Assembly / Compose / Install layer is part of the Yggdrasil v3 architecture baseline. If implementation is delivered in phases, release notes should clarify which capabilities are implemented and which remain proposal-level.

## 2. Recommended Reading Paths

| Audience | Recommended order | Goal |
|---|---|---|
| Application developers | 00 -> 07 -> 04 -> 05 -> 06 | Onboard services and understand lifecycle/configuration |
| Module authors | 00 -> 02 -> 03 -> 07 -> 05 | Implement modules, capabilities, and auto assembly |
| Framework maintainers | 00 -> 01 -> 02 -> 03 -> 04 -> 05 -> 06 | Maintain core architecture and runtime boundaries |
| Operators | 00 -> 05 -> 07 -> 02 | Troubleshoot configuration, reload, diagnostics, and startup failures |

## 3. Hub Test Checklist

Hub implementations should cover at least:

- missing dependency errors;
- dependency cycle errors with full cycle path;
- stable topological ordering;
- start failure compensation in reverse order;
- idempotent stop;
- capability cardinality conflicts for `ExactlyOne`, `OptionalOne`, and `NamedOne`;
- ordered resolution failures for duplicate names, missing providers, and type mismatches;
- rejection of `ScopeRuntimeFactory` modules.

Recommended error example:

```text
stage=Seal target=capability:observability.logger.handler reason=CardinalityViolation
spec=NamedOne name=json
providers=observability.logger.json.default, custom.logger.json
fix=rename one provider, disable one module, or configure an explicit binding
```

## 4. Recommended Decision / Warning / Conflict Shape

```go
type Decision struct {
    Stage      string   `json:"stage"`
    Target     string   `json:"target"`
    Selected   string   `json:"selected,omitempty"`
    Source     string   `json:"source"`
    Reason     string   `json:"reason"`
    Candidates []string `json:"candidates,omitempty"`
}
```

Example:

```json
{
  "stage": "resolveDefaults",
  "target": "observability.telemetry.tracer",
  "selected": "otel",
  "source": "mode-default",
  "reason": "mode prod-grpc selects otel tracer",
  "candidates": ["noop", "otel"]
}
```

`Warning` and `Conflict` records should also contain `stage`, `target`, `reason`, and `fix`, and should be emitted in stable order.

## 5. Failure Recovery Matrix

| Failed stage | Possible existing resources | Framework action | Recommended final state |
|---|---|---|---|
| Plan | No runtime resources | Return error | Stopped |
| Seal | Temporary Hub state | Discard plan and temporary Hub | Stopped |
| Init | Some modules initialized | Stop or close initialized resources | Stopped |
| Prepare runtime | Partial runtime resources | Run prepared runtime assembly close path | Stopped |
| Compose | Runtime ready; business-local resources may exist | Close framework-managed resources; business owns local resources | Stopped / InfraInitialized |
| InstallBusiness | Partial business bindings/tasks/hooks | Roll back business install, then Hub.Stop | Stopped |
| Start | Some modules or servers started | Async stop and reverse compensation | Stopped |

All compensation paths should be safe to call repeatedly and aggregate multiple close errors.

## 6. First Hot Reload Boundary

Supported:

- single-module configuration changes when the module implements `Reloadable`;
- logger level / writer parameter changes;
- telemetry exporter parameter changes;
- registry / resolver client parameter changes when the provider supports staged reload.

Restart-required:

- module addition or removal;
- transport protocol changes;
- server port / listener changes;
- RPC / REST / RawHTTP binding changes;
- `BusinessBundle` structure changes;
- any change that requires re-running `business.Compose`.

## 7. Configuration Path Convention

Module paths should follow `yggdrasil.<subsystem>.<module>`. The framework reserves:

- `yggdrasil.server`
- `yggdrasil.transports`
- `yggdrasil.observability`
- `yggdrasil.discovery`
- `yggdrasil.extensions`
- `yggdrasil.overrides`

Third-party modules should avoid reserved paths and may use `yggdrasil.modules.<vendor>.<module>`.

## 8. Production Failure Semantics

### 8.1 multi_registry Strategy

| Strategy | Semantics |
|---|---|
| `fail_fast` | Any registry failure fails startup and rolls back already-registered instances |
| `best_effort` | Record diagnostics but allow the service to continue running |

### 8.2 Resolver Watch Lifecycle

```text
Runtime.NewClient -> Resolver.AddWatch
Client.Close -> Resolver.DelWatch
App.Stop -> close client manager -> DelWatch all active watches
```

Resolver watches are dynamic client runtime state and must not be registered in the Hub.

### 8.3 Security Profile Reload

- TLS certificate rotation: may support hot reload;
- RequestAuth rule changes: depend on provider staged reload support;
- mTLS CA or security mode changes: usually restart-required unless the provider explicitly supports safe switching.

## 9. Minimal Review Checklist

Before merging a new module or provider, verify:

- App-local isolation is preserved;
- Prepare does not externally listen or serve;
- capability cardinality is explicit;
- default selection can be explained;
- reload failures can roll back or mark restart-required;
- diagnostics contain enough context for troubleshooting.
