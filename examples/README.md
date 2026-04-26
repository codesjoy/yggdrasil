# Examples

Yggdrasil examples follow a learning path instead of `sample/advanced` buckets.

`01-03` are the root-facade onboarding path. Service-side examples default to `yggdrasil.Run(...)`; `app` is kept for standalone client bootstrap or capability-heavy scenarios that need lower-level control.

## Start Here

1. [01-quickstart](/Users/zhangwei/go/src/github.com/codesjoy/yggdrasil/examples/01-quickstart): the shortest end-to-end path using root `yggdrasil.Run(...)` plus a standalone `app.New(...)->NewClient(...)` client.
2. [02-runtime-bundle](/Users/zhangwei/go/src/github.com/codesjoy/yggdrasil/examples/02-runtime-bundle): the `Runtime` + `BusinessBundle` installation surface behind the root facade.
3. [03-diagnostics-reload](/Users/zhangwei/go/src/github.com/codesjoy/yggdrasil/examples/03-diagnostics-reload): watchable config, diagnostics, spec hash/diff, and restart-required reload behavior.

## Feature Examples

- [10-rest-gateway](/Users/zhangwei/go/src/github.com/codesjoy/yggdrasil/examples/10-rest-gateway): generated `RESTBinding` installation via the root facade, plus an external HTTP caller that validates the exposed REST surface.
- [11-rpc-streaming](/Users/zhangwei/go/src/github.com/codesjoy/yggdrasil/examples/11-rpc-streaming): unary and streaming RPC shapes with a root-facade server and a standalone client bootstrap.
- [12-transport-metadata](/Users/zhangwei/go/src/github.com/codesjoy/yggdrasil/examples/12-transport-metadata): transport metadata propagation, header/trailer observation, and stream metadata handling.
- [13-error-reason](/Users/zhangwei/go/src/github.com/codesjoy/yggdrasil/examples/13-error-reason): structured reason/status mapping and client-side parsing.
- [14-client-load-balancing](/Users/zhangwei/go/src/github.com/codesjoy/yggdrasil/examples/14-client-load-balancing): client-side endpoint selection and request distribution over split server/client config.

## Extension Example

- [20-capability-registration](/Users/zhangwei/go/src/github.com/codesjoy/yggdrasil/examples/20-capability-registration): `WithCapabilityRegistrations(...)` on the root server path plus the same registration on a standalone client bootstrap.

## Recipes

- [90-recipes/config-layers.md](/Users/zhangwei/go/src/github.com/codesjoy/yggdrasil/examples/90-recipes/config-layers.md)
- [90-recipes/raw-grpc.md](/Users/zhangwei/go/src/github.com/codesjoy/yggdrasil/examples/90-recipes/raw-grpc.md)
- [90-recipes/jsonraw-grpc.md](/Users/zhangwei/go/src/github.com/codesjoy/yggdrasil/examples/90-recipes/jsonraw-grpc.md)

## Shared Assets

- `examples/proto`: shared protobuf sources.
- `examples/protogen`: generated Go code imported by the runnable examples.
- `examples/go.mod`: the standalone module used to compile and run all examples.

## Related Docs

- [README.md](/Users/zhangwei/go/src/github.com/codesjoy/yggdrasil/README.md)
- [README_zh_CN.md](/Users/zhangwei/go/src/github.com/codesjoy/yggdrasil/README_zh_CN.md)
