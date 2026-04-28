# 20 Capability Registration

## Framework capabilities demonstrated

- Show provider-only capability registration instead of a full `module.Module` extension.
- Show how `WithCapabilityRegistrations(...)` enters both the root server path and standalone client bootstrap.
- Show that custom protocol name, config path, and capability provider name must align for the planner/runtime to select the provider.

## How to run

Server:

```bash
cd examples/20-capability-registration/server
go run .
```

Client:

```bash
cd examples/20-capability-registration/client
go run .
```

## What to observe

- The server uses `yggdrasil.Run(ctx, appName, ..., yggdrasil.WithCapabilityRegistrations(...))`, so the registration enters the current root app together with the business bundle.
- The client uses `app.New(appName, ..., WithCapabilityRegistrations(...))->NewClient(...)`, because standalone client bootstrap is still an advanced entry.
- The extension point is limited to the `grpcx` transport provider layer; the business side still installs a normal `GreeterService`.

## Key source entry points

- Registration definition: [grpcx/registration.go](grpcx/registration.go)
- Server entry: [server/main.go](server/main.go)
- Client entry: [client/main.go](client/main.go)

## What to read next

- For the basic `Runtime` / `BusinessBundle` view, read [02 Runtime Bundle](../02-runtime-bundle/README.md).
- For lower-level transport recipes, read [Raw gRPC](../90-recipes/raw-grpc.md) and [JSON Raw gRPC](../90-recipes/jsonraw-grpc.md).
