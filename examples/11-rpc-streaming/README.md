# 11 RPC Streaming

## Framework capabilities demonstrated

- Show one `RPCBinding` carrying unary, client-streaming, server-streaming, and bidirectional-streaming methods.
- Show that streaming is still installed through the business bundle, not a separate framework entrypoint.
- Keep implementation minimal so readers can focus on Yggdrasil client/server assembly.

## How to run

Server:

```bash
cd examples/11-rpc-streaming/server
go run .
```

Client:

```bash
cd examples/11-rpc-streaming/client
go run .
```

## What to observe

- `server/main.go` uses root `yggdrasil.Run(ctx, appName, ...)`; the formal installation boundary remains `server/business/compose.go`.
- `client/main.go` uses standalone `app.New(appName, ...)->NewClient(...)` bootstrap because the root facade does not own standalone client startup.
- The service target aligns with examples 12 and 14 for a continuous reading path.

## Key source entry points

- Lifecycle entry: [server/main.go](server/main.go)
- Bundle composition: [server/business/compose.go](server/business/compose.go)
- Client entry: [client/main.go](client/main.go)

## What to read next

- To see metadata/header/trailer in stream context, read [12 Transport Metadata](../12-transport-metadata/README.md).
- To see one client service target distributed across endpoints, read [14 Client Load Balancing](../14-client-load-balancing/README.md).
