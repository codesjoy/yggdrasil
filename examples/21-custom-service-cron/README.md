# 21 Custom Service Cron

## Framework capabilities demonstrated

- Show how business code wraps a third-party background service as `BusinessInstallable`.
- Show how `BusinessInstallable.Install(...)` uses `InstallContext.AddTask(...)` to hand a custom service to Yggdrasil lifecycle management.
- Show how a `robfig/cron/v3` background scheduler follows app start/stop and waits for graceful shutdown.

## How to run

```bash
cd examples/21-custom-service-cron
go run .
```

Optional check:

```bash
curl http://127.0.0.1:56024/diagnostics?pretty=true
```

## What to observe

- `business.Compose(...)` returns an `Extensions` item instead of registering `Tasks` directly. This models a real business-side custom service integration.
- `cronIntegration.Install(...)` validates the cron expression during install and registers a managed background task.
- `cronTask.Stop(...)` uses the context returned by `cron.Stop()` to wait for triggered jobs while respecting the framework shutdown context.

## Key source entry points

- Lifecycle entry: [main.go](main.go)
- Custom cron integration: [business/compose.go](business/compose.go)
- Integration test: [business/compose_test.go](business/compose_test.go)

## What to read next

- To understand the full `BusinessBundle` installation surface first, read [02 Runtime Bundle](../02-runtime-bundle/README.md).
- For provider-only extension instead of business-side custom services, read [20 Capability Registration](../20-capability-registration/README.md).
