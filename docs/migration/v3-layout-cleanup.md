# v3 Layout Cleanup Migration Guide

## Summary

This repository applies a breaking layout cleanup inside `v3` to make package ownership clearer and reduce top-level sprawl.

## Decisions Kept from the Analysis

- `app`, `assembly`, and `module` remain top-level packages.
- `transport` replaces the old `remote` family and groups transport packages by layer: runtime, protocol, gateway, and support.
- `internal` stays mostly flat instead of mirroring every public domain.
- `rest` middleware is exposed from `transport/gateway/rest` instead of a separate public subpackage.

## Old-to-New Paths

| Old | New |
| --- | --- |
| `governor` | `admin/governor` |
| `registry` | `discovery/registry` |
| `resolver` | `discovery/resolver` |
| `metadata` | `rpc/metadata` |
| `status` | `rpc/status` |
| `stream` | `rpc/stream` |
| `interceptor` | `rpc/interceptor` |
| `interceptor/logging` | `rpc/interceptor/logging` |
| `logger` | `observability/logger` |
| `otel` | `observability/otel` |
| `stats` | `observability/stats` |
| `stats/otel` | `observability/stats/otel` |
| `remote` | `transport` |
| `remote/credentials` | `transport/support/security` |
| `remote/marshaler` | `transport/support/marshaler` |
| `remote/peer` | `transport/support/peer` |
| `remote/transport/grpc` | `transport/protocol/grpc` |
| `remote/transport/rpchttp` | `transport/protocol/rpchttp` |
| `client` | `transport/runtime/client` |
| `server` | `transport/runtime/server` |
| `server/rest` | `transport/gateway/rest` |
| `balancer` | `transport/runtime/client/balancer` |
| `example` | `examples` |

## Source-Level Cleanup

- `config/global.go` was renamed to `config/default_manager.go`.
- weakly named test files such as `*_extra_test.go`, `helpers_test.go`, and `fixtures_test.go` were renamed to behavior-oriented or explicit helper names.
- transport REST middleware helpers were folded into `transport/gateway/rest`.

## Import Migration Strategy

1. Replace old imports using the package map above.
2. Update example and generated-code module paths from `.../example` to `.../examples`.
3. Regenerate or re-run tests for any code that imported old transport or REST helper packages.

## Notes

- No compatibility aliases are provided.
- `docs/analysis.md` was treated as planning input and is intentionally not part of the public documentation set after the cleanup.
