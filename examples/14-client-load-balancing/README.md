# 14 Client Load Balancing

## Framework capabilities demonstrated

- Show request distribution when one client service target maps to multiple endpoints.
- Show how direct endpoint configuration enters client runtime without an additional service discovery system.
- Show that multiple backend instances can serve the same `RPCBinding` installation result.

## How to run

Run three servers:

```bash
cd examples/14-client-load-balancing/server
go run . --port 55884
```

```bash
cd examples/14-client-load-balancing/server
go run . --port 55885
```

```bash
cd examples/14-client-load-balancing/server
go run . --port 55886
```

Run the client:

```bash
cd examples/14-client-load-balancing/client
go run .
```

## What to observe

- `client/config.yaml` configures three endpoints under one service target. This is the key client runtime behavior demonstrated by the example.
- `server/config.yaml` describes the base server shape; `server/main.go` uses the `--port` config override to set the actual gRPC listen address.
- `client/main.go` uses standalone `app.New(...)->NewClient(...)` bootstrap and observes the selected backend through the `server` trailer field.

## Key source entry points

- Lifecycle entry: [server/main.go](server/main.go)
- Bundle composition: [server/business/compose.go](server/business/compose.go)
- Client endpoint config: [client/config.yaml](client/config.yaml)

## What to read next

- For the basic client service target path, revisit [01 Quickstart](../01-quickstart/README.md).
- To see provider-only extension entering planner/runtime, read [20 Capability Registration](../20-capability-registration/README.md).
