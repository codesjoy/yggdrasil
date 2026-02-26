# Documentation

This directory is the entry point for Yggdrasil documentation.

## Start Here

- Project README (EN): [../README.md](../README.md)
- Project README (ZH-CN): [../README_CN.md](../README_CN.md)
- Examples overview: [../example/README.md](../example/README.md)

## Repository Quick Commands

- Install dev tools (buf, golangci-lint, go-junit-report): `make tools`
- Install repo binaries (including protoc plugins): `make install`
- Run stable tests (no default race): `make test`
- Run race tests: `make test.race`
- Lint (stable profile): `make lint`
- Full stable gate (CI default): `make check`
- Strict gate (examples + race + strict lint): `make check.strict`
- Check dependency tidy drift: `make go.mod.tidy.check`

By default, `example/` modules are excluded from lint/test/coverage. Add `INCLUDE_EXAMPLES=1` to include them.

Go version requirement is defined in `../go.mod`.

## Code Generation

### Generate code for the examples

The `example/` directory uses Buf.

```bash
cd example
buf generate
```

### Generate code for the Reason error-handling proto

```bash
cd example/proto/error-handling
make generate
```

## Examples

- Overview and learning path: [../example/README.md](../example/README.md)
- Sample server: [../example/sample/server/README.md](../example/sample/server/README.md)
- Sample client: [../example/sample/client/README.md](../example/sample/client/README.md)

## Contrib Modules

- etcd: [yggdrasil-ecosystem/integrations/etcd](https://github.com/codesjoy/yggdrasil-ecosystem/tree/main/integrations/etcd/README.md)
- Kubernetes: [yggdrasil-ecosystem/integrations/k8s](https://github.com/codesjoy/yggdrasil-ecosystem/tree/main/integrations/k8s/README.md)
- OpenTelemetry exporters: [yggdrasil-ecosystem/integrations/otlp](https://github.com/codesjoy/yggdrasil-ecosystem/tree/main/integrations/otlp/README.md) and [OTLP Quickstart](https://github.com/codesjoy/yggdrasil-ecosystem/tree/main/integrations/otlp/QUICKSTART.md)
- xDS: [yggdrasil-ecosystem/integrations/xds](https://github.com/codesjoy/yggdrasil-ecosystem/tree/main/integrations/xds/README.md)
- Polaris: [yggdrasil-ecosystem/integrations/polaris](https://github.com/codesjoy/yggdrasil-ecosystem/tree/main/integrations/polaris/README.md)
