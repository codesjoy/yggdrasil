# Documentation

This directory is the entry point for Yggdrasil documentation.

## Start Here

- Project README (EN): [../README.md](../README.md)
- Project README (ZH-CN): [../README_CN.md](../README_CN.md)
- Examples overview: [../example/README.md](../example/README.md)

## Repository Quick Commands

- Install dev tools (buf, golangci-lint, go-junit-report): `make tools`
- Install repo binaries (including protoc plugins): `make install`
- Run tests: `make test`
- Lint: `make lint`

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

- etcd: [../contrib/etcd/readme.md](../contrib/etcd/readme.md)
- Kubernetes: [../contrib/k8s/readme.md](../contrib/k8s/readme.md)
- OpenTelemetry exporters: [../contrib/otlp/README.md](../contrib/otlp/README.md) and [../contrib/otlp/QUICKSTART.md](../contrib/otlp/QUICKSTART.md)
- xDS: [../contrib/xds/README.md](../contrib/xds/README.md)
- Polaris: [../contrib/polaris/README.md](../contrib/polaris/README.md)
