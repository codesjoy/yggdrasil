# Yggdrasil

## What Yggdrasil Is

Yggdrasil is a Go microservice framework with a thin root bootstrap API and a composable runtime core.
It provides RPC and REST serving, transport abstraction, service discovery, load balancing, configuration layering, and observability wiring without forcing business code to depend on global process state.

## Quick Start

Install the code generators:

```bash
go install github.com/codesjoy/yggdrasil/v3/cmd/protoc-gen-yggdrasil-rpc@latest
go install github.com/codesjoy/yggdrasil/v3/cmd/protoc-gen-yggdrasil-rest@latest
```

Bootstrap an application from the root package:

```go
package main

import (
	"context"
	"log"

	"github.com/codesjoy/yggdrasil/v3"
)

func main() {
	if err := yggdrasil.Run(context.Background(), func(runtime yggdrasil.Runtime) (yggdrasil.BusinessBundle, error) {
		return yggdrasil.BusinessBundle{}, nil
	}); err != nil {
		log.Fatal(err)
	}
}
```

## Package Map

| Area | Packages |
| --- | --- |
| Root bootstrap | `yggdrasil`, `app`, `assembly`, `module`, `config` |
| Admin | `admin/governor` |
| Discovery | `discovery/registry`, `discovery/resolver` |
| RPC contracts | `rpc/interceptor`, `rpc/metadata`, `rpc/status`, `rpc/stream` |
| Transport family | `transport` contracts; runtime: `transport/runtime/client`, `transport/runtime/server`, `transport/runtime/client/balancer`; protocol: `transport/protocol/grpc`, `transport/protocol/rpchttp`; gateway: `transport/gateway/rest`; support: `transport/support/security`, `transport/support/marshaler`, `transport/support/peer` |
| Observability | `observability/logger`, `observability/otel`, `observability/stats` |
| Examples | `examples/` |

## Examples

The canonical example tree now lives under [`examples/`](/Users/zhangwei/go/src/github.com/codesjoy/yggdrasil/examples/README.md).
Start with:

- `examples/sample/` for the simplest client/server flow
- `examples/advanced/streaming/` for streaming RPCs
- `examples/advanced/rest/` for generated REST handlers

## Migration Notes

This repository intentionally applies breaking layout cleanup inside `v3`.

- Old flat package paths such as `remote`, `server/rest`, `metadata`, and `stats` have moved under `transport`, `rpc`, `discovery`, `admin`, and `observability`.
- No compatibility alias packages are kept.
- The full path mapping and rationale live in [`docs/migration/v3-layout-cleanup.md`](/Users/zhangwei/go/src/github.com/codesjoy/yggdrasil/docs/migration/v3-layout-cleanup.md).
