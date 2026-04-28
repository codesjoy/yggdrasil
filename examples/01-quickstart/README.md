# 01 Quickstart

## Framework capabilities demonstrated

- Use root `yggdrasil.Run(ctx, appName, ...)` for the default server onboarding path.
- Use `app.New(appName, ...)->NewClient(...)` for standalone client bootstrap instead of relying on global state.
- Keep the example minimal and show the shortest runnable gRPC end-to-end path.

## How to run

Server:

```bash
cd examples/01-quickstart/server
go run .
```

Client:

```bash
cd examples/01-quickstart/client
go run .
```

## What to observe

- `server/main.go` keeps only the root facade and shutdown control; the formal business installation boundary is `server/business/compose.go`.
- `client/main.go` starts a standalone client app with an explicit app name and reads the service target from `config.yaml`, then calls `SayHello` once.
- The governor diagnostics endpoint is `http://127.0.0.1:56011/diagnostics?pretty=true`.

## Key source entry points

- Server entry: [server/main.go](server/main.go)
- Business composition: [server/business/compose.go](server/business/compose.go)
- Client entry: [client/main.go](client/main.go)

## What to read next

- To understand what `BusinessBundle` can install, read [02 Runtime Bundle](../02-runtime-bundle/README.md).
- To observe config watch, reload, and diagnostics together, read [03 Diagnostics Reload](../03-diagnostics-reload/README.md).
