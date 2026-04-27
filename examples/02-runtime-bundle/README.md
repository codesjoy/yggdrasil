# 02 Runtime Bundle

## Framework capabilities demonstrated

- Show the safe `Runtime` surface: `Config()`, `Logger()`, `TracerProvider()`, `MeterProvider()`, and `Lookup(...)`.
- Show the `BusinessBundle` installation boundary: `RPCBindings`, `RESTBindings`, `RawHTTP`, `Tasks`, `Hooks`, and `Diagnostics`.
- Show how business code reads framework and business configuration in `business.Compose(...)`, then returns one bundle.

## How to run

```bash
cd examples/02-runtime-bundle
go run .
```

Optional checks:

```bash
curl http://127.0.0.1:56021/healthz
curl http://127.0.0.1:56021/v1/shelves/runtime-bundle
curl http://127.0.0.1:56022/diagnostics?pretty=true
```

## What to observe

- `main.go` keeps only root `yggdrasil.Run(...)`; bundle composition is centralized in `business.Compose`.
- `/healthz` comes from `RawHTTPBinding`; `/v1/shelves/runtime-bundle` comes from `RESTBinding`; gRPC comes from `RPCBinding`.
- Governor `/diagnostics` includes `BusinessBundle.Diagnostics`, which helps confirm the installed bundle.

## Key source entry points

- Lifecycle entry: [main.go](main.go)
- Bundle composition: [business/compose.go](business/compose.go)
- Bundle test: [business/compose_test.go](business/compose_test.go)

## What to read next

- For watchable config, reload, and spec diff, read [03 Diagnostics Reload](../03-diagnostics-reload/README.md).
- For a focused REST example, read [10 REST Gateway](../10-rest-gateway/README.md).
