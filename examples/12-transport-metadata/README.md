# 12 Transport Metadata

## Framework capabilities demonstrated

- Show how request metadata, response headers, and response trailers flow through the Yggdrasil transport layer.
- Cover both unary and streaming scenarios.
- Keep the focus on transport context rather than business logic.

## How to run

Server:

```bash
cd examples/12-transport-metadata/server
go run .
```

Client:

```bash
cd examples/12-transport-metadata/client
go run .
```

## What to observe

- The server defaults to root `yggdrasil.Run(ctx, appName, ...)`; metadata behavior is installed through `server/business/compose.go`.
- The client uses standalone `app.New(appName, ...)->NewClient(...)` bootstrap, then explicitly reads and writes metadata context in calls.
- This example is best read alongside [11 RPC Streaming](../11-rpc-streaming/README.md).

## Key source entry points

- Lifecycle entry: [server/main.go](server/main.go)
- Bundle composition: [server/business/compose.go](server/business/compose.go)
- Client entry: [client/main.go](client/main.go)

## What to read next

- For error semantics beyond transport behavior, read [13 Error Reason](../13-error-reason/README.md).
- For the structured REST surface exposed by `RESTBinding`, read [10 REST Gateway](../10-rest-gateway/README.md).
