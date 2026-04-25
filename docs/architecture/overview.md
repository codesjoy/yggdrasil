# Architecture Overview

Yggdrasil is organized around a thin root facade and a set of domain-oriented package families.

## Layers

- `yggdrasil`: default bootstrap entrypoint for application code.
- `app`: lifecycle orchestration and runtime composition root.
- `assembly`: planning, selection, and declarative runtime assembly artifacts.
- `module`: module hub contracts, lifecycle interfaces, and capability model.
- `config`: layered configuration sources and compiled views.
- `transport/*`: transport implementations, clients, servers, codecs, credentials, REST bridge, and balancing.
- `rpc/*`: RPC-facing metadata, status, stream contracts, and interceptors.
- `discovery/*`: service registry and resolver contracts.
- `observability/*`: logging, OpenTelemetry, and stats handlers.
- `admin/*`: operational endpoints such as governor.

## Design Intent

- Keep the root bootstrap surface small.
- Group public packages by responsibility instead of historical accretion.
- Preserve the existing framework mental model instead of inventing a new container abstraction.
- Make transport packages discoverable without the old `remote/transport/...` nesting.

## Repository Entry Points

- Read [`README.md`](/Users/zhangwei/go/src/github.com/codesjoy/yggdrasil/README.md) first for onboarding.
- Read [`docs/package-map.md`](/Users/zhangwei/go/src/github.com/codesjoy/yggdrasil/docs/package-map.md) when migrating imports.
- Read [`examples/README.md`](/Users/zhangwei/go/src/github.com/codesjoy/yggdrasil/examples/README.md) for runnable samples.
