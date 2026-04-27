# Yggdrasil Examples Documentation

This directory contains the English optimized documentation for Yggdrasil v3 examples. Examples follow a learning path instead of simple sample / advanced buckets.

## Reading path

### Onboarding path

1. [01 Quickstart](01-quickstart/README.md): the shortest end-to-end path. The server uses `yggdrasil.Run(...)`; the client uses standalone `app.New(...)->NewClient(...)`.
2. [02 Runtime Bundle](02-runtime-bundle/README.md): understand the `Runtime` and `BusinessBundle` installation surface behind the root facade.
3. [03 Diagnostics Reload](03-diagnostics-reload/README.md): observe watchable config, diagnostics, spec hash/diff, and restart-required reload behavior.

### Feature examples

- [10 REST Gateway](10-rest-gateway/README.md): install HTTP/JSON APIs through `RESTBinding`.
- [11 RPC Streaming](11-rpc-streaming/README.md): unary, client-streaming, server-streaming, and bidirectional-streaming.
- [12 Transport Metadata](12-transport-metadata/README.md): metadata, headers, and trailers.
- [13 Error Reason](13-error-reason/README.md): structured reason, gRPC code, HTTP code, and metadata.
- [14 Client Load Balancing](14-client-load-balancing/README.md): distribute requests from one service target to multiple endpoints.

### Extension examples

- [20 Capability Registration](20-capability-registration/README.md): provider-only capability registration.
- [21 Custom Service Cron](21-custom-service-cron/README.md): custom business-side `BusinessInstallable` for a third-party background scheduler.

### Recipes

- [Config Layers Recipe](90-recipes/config-layers.md): config layering, typed sections, and watchable overlays.
- [Raw gRPC Recipe](90-recipes/raw-grpc.md): send and receive `[]byte` directly with the `raw` content subtype.
- [JSON Raw gRPC Recipe](90-recipes/jsonraw-grpc.md): pass JSON text bytes with the `jsonraw` content subtype.

## Documentation conventions

- Server-side examples prefer root `yggdrasil.Run(...)`.
- Standalone clients, provider-heavy scenarios, or lower-level control paths use `app.New(...)`.
- The formal business installation boundary is always `BusinessBundle`.
- All links use repository-relative paths instead of local absolute paths.
