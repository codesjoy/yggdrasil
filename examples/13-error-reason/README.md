# 13 Error Reason

## Framework capabilities demonstrated

- Show structured error responses and client parsing based on proto reason enums.
- Show reason-to-gRPC-code / HTTP-code mapping and metadata propagation with errors.
- Show that error semantics remain part of the business installation boundary.

## How to run

Server:

```bash
cd examples/13-error-reason/server
go run .
```

Client:

```bash
cd examples/13-error-reason/client
go run .
```

## What to observe

- The server entry uses root `yggdrasil.Run(...)`; the error semantics service is installed through `BusinessBundle`.
- `server/main.go` keeps service implementation and `composeBundle(...)` together so error cases are easy to compare.
- The client uses standalone `app.New(...)->NewClient(...)` bootstrap and focuses on `status.FromError(...)`, `Code()`, `HTTPCode()`, and `ErrorInfo()`.

## Key source entry points

- Lifecycle entry and error cases: [server/main.go](server/main.go)
- Bundle test: [server/compose_test.go](server/compose_test.go)
- Client entry: [client/main.go](client/main.go)

## What to read next

- To review the `BusinessBundle` installation boundary first, read [02 Runtime Bundle](../02-runtime-bundle/README.md).
- To see client runtime behavior across multiple endpoints, read [14 Client Load Balancing](../14-client-load-balancing/README.md).
