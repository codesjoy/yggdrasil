# 06. Transport, Service Discovery, and Observability

> This document describes the protocol-agnostic transport model, REST gateway, security profiles, registry, resolver, balancer, interceptors, middleware, and observability integration.

## 1. Protocol-Agnostic Transport Model

Yggdrasil decouples the RPC framework from concrete network protocols through transport providers. Server-side and client-side transports are registered as capabilities and selected by the assembly plan and Hub.

```go
type TransportServerProvider interface {
    Protocol() string
    NewServer(handle MethodHandle) (Server, error)
}

type TransportClientProvider interface {
    Protocol() string
    NewClient(
        ctx context.Context,
        serviceName string,
        endpoint resolver.Endpoint,
        statsHandler stats.Handler,
        onStateChange OnStateChange,
    ) (Client, error)
}
```

The Hub holds only providers. Concrete server/client instances are created and managed by the relevant runtime subsystem.

## 2. Server / Client Abstractions

### 2.1 Server

```go
type Server interface {
    Start() error
    Handle() error
    Stop(context.Context) error
    Info() ServerInfo
}
```

- `Start`: bind and start listening;
- `Handle`: block and handle requests, usually under an errgroup;
- `Stop`: graceful shutdown;
- `Info`: returns protocol, address, and attributes.

### 2.2 Client

```go
type Client interface {
    NewStream(ctx context.Context, desc *stream.Desc, method string) (stream.ClientStream, error)
    Close() error
    Protocol() string
    State() State
    Connect()
}
```

Connection state: `Idle -> Connecting -> Ready -> TransientFailure -> Shutdown`.

## 3. REST Gateway and Raw HTTP

Business code may register:

- RPC service descriptors;
- REST service descriptors;
- raw HTTP handlers.

REST and Raw HTTP route conflicts must be checked during installation, typically by method + path.

## 4. Security Profiles

Security follows a Provider -> Profile -> Material pipeline:

```go
type Provider interface {
    Type() string
    Compile(name string, raw map[string]any) (Profile, error)
}

type Profile interface {
    Name() string
    Type() string
    Build(spec BuildSpec) (Material, error)
}
```

`Material` includes TLS configuration and request/connection authenticators:

```go
type Material struct {
    Mode        Mode
    ClientTLS   *tls.Config
    ServerTLS   *tls.Config
    RequestAuth RequestAuthenticator
    ConnAuth    ConnAuthenticator
}
```

Security modes:

| Mode | Description |
| --- | --- |
| `insecure` | No encryption or authentication |
| `local` | Local-only security strategy |
| `tls` | TLS / optional mTLS |

## 5. Service Registry

```go
type Registry interface {
    Register(context.Context, Instance) error
    Deregister(context.Context, Instance) error
    Type() string
}
```

Applications register instances after startup and deregister during graceful shutdown. `multi_registry` can fan out register/deregister calls to multiple backends and may support fail-fast behavior.

## 6. Service Resolver

```go
type Resolver interface {
    AddWatch(serviceName string, client Client) error
    DelWatch(serviceName string, client Client) error
    Type() string
}
```

A resolver observes endpoint changes and updates client state through callbacks. Resolver watches are dynamic objects and are not registered in the Hub.

## 7. Balancer / Picker

```go
type Balancer interface {
    UpdateState(resolver.State)
    Close() error
    Type() string
}

type Picker interface {
    Next(RPCInfo) (PickResult, error)
}
```

The built-in round-robin balancer uses an atomic counter to select the next available endpoint. Balancer runtime state belongs to the client subsystem and does not enter the Hub.

## 8. RPC Client Call Path

```text
Business Call
  -> Client Interface
  -> Unary/Stream Client Interceptor Chain
  -> Balancer / Picker
  -> remoteClientManager
  -> transport.Client
```

The Hub only provides resolver providers, balancer providers, transport client providers, credential providers, and client interceptor providers.

## 9. Interceptors and Middleware

A chain extension point consists of three parts:

1. modules provide named providers;
2. configuration provides the final ordered name list or a template;
3. the subsystem resolves providers in order and builds the chain.

Execution order comes only from configuration, not module registration order, DAG order, `InitOrder()`, or map iteration.

```go
ints, err := module.ResolveOrdered[interceptor.UnaryServerInterceptorProvider](
    hub,
    capabilities.UnaryServerInterceptorSpec,
    resolved.OrderedExtensions.UnaryServer,
)
```

## 10. Observability

### 10.1 Logger

The logger system is split into handler builders, writer builders, and logger core. Built-in logger handler and writer capabilities are `NamedOne`; the runtime resolves the configured names through explicit capability bindings, never by taking the first candidate.

### 10.2 Tracer / Meter

TracerProvider and MeterProvider builders are `NamedOne` capabilities selected by `yggdrasil.observability.telemetry.tracer` and `yggdrasil.observability.telemetry.meter`. Multiple available providers are fine; an unknown selected provider fails during planning or runtime preparation.

### 10.3 Stats Handler

Stats handler builders are `NamedOne` capabilities. Server and client runtime build handler chains from the configured telemetry stats settings.

### 10.4 Diagnostics

Governor should expose:

- topology order and layer;
- capability conflicts;
- reload state;
- restart-required;
- plan hash and spec diff;
- transport server info;
- registry / resolver / balancer status summary.

## 11. Custom Transport Module Example

```go
type MyTransportModule struct{}

func (m *MyTransportModule) Name() string { return "transport.my_protocol" }

func (m *MyTransportModule) Capabilities() []module.Capability {
    return []module.Capability{
        {
            Spec: capabilities.TransportServerProviderSpec, // NamedOne
            Name: "my_protocol",
            Value: transport.NewTransportServerProvider(
                "my_protocol",
                func(handle transport.MethodHandle) (transport.Server, error) {
                    return newMyServer(handle), nil
                },
            ),
        },
        {
            Spec: capabilities.TransportClientProviderSpec, // NamedOne
            Name: "my_protocol",
            Value: transport.NewTransportClientProvider(
                "my_protocol",
                func(ctx context.Context, service string, endpoint resolver.Endpoint, stats stats.Handler, onChange transport.OnStateChange) (transport.Client, error) {
                    return newMyClient(ctx, service, endpoint), nil
                },
            ),
        },
    }
}
```

Configuration:

```yaml
yggdrasil:
  server:
    transports:
      - "my_protocol"
  transports:
    my_protocol:
      server:
        address: ":9090"
```
