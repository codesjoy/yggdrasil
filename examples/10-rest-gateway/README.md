# 10 REST Gateway

## Framework capabilities demonstrated

- Show how generated `RESTBinding` enters the same `BusinessBundle` as `RPCBinding`.
- Show that REST route installation is still a formal business bundle boundary, not a separate route table maintained outside the framework.
- Use an external HTTP caller to validate the exposed HTTP/JSON interface.

## How to run

Server:

```bash
cd examples/10-rest-gateway/server
go run .
```

Client:

```bash
cd examples/10-rest-gateway/client
go run .
```

## What to observe

- The server entry is centralized on root `yggdrasil.Run(ctx, appName, ...)`; `RESTBinding` installation is decided by `server/business/compose.go`.
- `server/business/compose.go` returns both `LibraryServiceServiceDesc` and `LibraryServiceRestServiceDesc`.
- `client/main.go` is not a Yggdrasil client. It is a plain HTTP caller used to validate route exposure, JSON encoding/decoding, and status codes from outside the framework.

## Key source entry points

- Lifecycle entry: [server/main.go](server/main.go)
- Bundle composition: [server/business/compose.go](server/business/compose.go)
- External HTTP caller: [client/main.go](client/main.go)

## What to read next

- To understand `RESTBinding` as one installation surface of `BusinessBundle`, read [02 Runtime Bundle](../02-runtime-bundle/README.md).
- To see transport context propagation in the call path, read [12 Transport Metadata](../12-transport-metadata/README.md).
