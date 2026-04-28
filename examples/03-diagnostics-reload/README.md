# 03 Diagnostics Reload

## Framework capabilities demonstrated

- Use a watchable config source to trigger reload automatically instead of manually restarting the example.
- Use governor `/diagnostics` and `/module-hub` to observe `AssemblySpec`, plan hash, spec diff, and reload errors.
- Demonstrate why config changes can be classified as `restart-required` when a business bundle is already installed.

## How to run

```bash
cd examples/03-diagnostics-reload
go run .
```

Trigger one observable reload:

```bash
perl -0pi -e 's/mode: dev/mode: prod-grpc/' reload.yaml
curl http://127.0.0.1:56032/diagnostics?pretty=true
curl http://127.0.0.1:56032/module-hub?pretty=true
```

## What to observe

- `main.go` still uses root `yggdrasil.Run(ctx, appName, ...)`, with a watchable `reload.yaml` injected through `yggdrasil.WithConfigSource(...)`.
- `config.yaml` is the base configuration; `reload.yaml` is loaded and watched as an additional layer.
- When `reload.yaml` changes `mode` from `dev` to `prod-grpc`, the framework replans the assembly and records the new spec hash and diff in diagnostics.

## Key source entry points

- Lifecycle entry: [main.go](main.go)
- Business composition: [business/compose.go](business/compose.go)
- Reload smoke test: [smoke_test.go](smoke_test.go)

## What to read next

- For focused REST behavior, read [10 REST Gateway](../10-rest-gateway/README.md).
- For provider-only capability extension, read [20 Capability Registration](../20-capability-registration/README.md).
