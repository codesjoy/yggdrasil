# Transport & Service Discovery

> This document covers the protocol-agnostic transport abstraction, security profiles, service discovery, and load balancing in Yggdrasil.

## Overview

Yggdrasil decouples the RPC framework from specific network protocols through transport providers. Server-side and client-side transports are registered as capabilities, and the assembly planner selects them based on configuration. Service discovery and load balancing are similarly pluggable.

Key source files:

| File | Responsibility |
|---|---|
| `transport/transport.go` | `TransportServerProvider`, `TransportClientProvider`, constructors |
| `transport/types.go` | `Server`, `Client`, `ServerStream`, `MethodHandle`, `State` |
| `transport/support/security/security.go` | Security profiles, TLS, authentication |
| `discovery/registry/registry.go` | `Registry`, `Instance`, `Provider` |
| `discovery/resolver/resolver.go` | `Resolver`, `Endpoint`, `Provider` |
| `transport/runtime/client/balancer/balancer.go` | `Balancer`, `Picker`, `Provider` |

---

## TransportServerProvider / TransportClientProvider

### TransportServerProvider

```go
type TransportServerProvider interface {
    Protocol() string
    NewServer(handle MethodHandle) (Server, error)
}
```

Creates a server for a specific protocol. The `MethodHandle` function is called for each incoming RPC method.

### TransportClientProvider

```go
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

Creates a client for a specific protocol and target endpoint.

### Constructor Helpers

```go
func NewTransportServerProvider(protocol string, builder ServerBuilder) TransportServerProvider
func NewTransportClientProvider(protocol string, builder ClientBuilder) TransportClientProvider
```

These wrap function literals into the provider interfaces.

---

## Transport Types

### Server

```go
type Server interface {
    Start() error
    Handle() error
    Stop(context.Context) error
    Info() ServerInfo
}
```

| Method | Description |
|---|---|
| `Start` | Bind and begin listening |
| `Handle` | Block and serve (called in errgroup) |
| `Stop` | Graceful shutdown |
| `Info` | Returns `ServerInfo{Protocol, Address, Attributes}` |

### Client

```go
type Client interface {
    NewStream(ctx context.Context, desc *stream.Desc, method string) (stream.ClientStream, error)
    Close() error
    Protocol() string
    State() State
    Connect()
}
```

### Connection State

```go
const (
    Idle            State = iota
    Connecting
    Ready
    TransientFailure
    Shutdown
)
```

---

## Security Profiles

Security in Yggdrasil follows a Provider → Profile → Material pipeline.

### Provider

```go
type Provider interface {
    Type() string
    Compile(name string, raw map[string]any) (Profile, error)
}
```

A security provider compiles raw configuration into a `Profile`.

### Profile

```go
type Profile interface {
    Name() string
    Type() string
    Build(spec BuildSpec) (Material, error)
}
```

A profile builds security material for a specific service and side (client/server).

### BuildSpec

```go
type BuildSpec struct {
    Protocol    string
    Side        Side    // "client" or "server"
    ServiceName string
    Authority   string
}
```

### Material

```go
type Material struct {
    Mode        Mode               // "insecure", "local", "tls"
    ClientTLS   *tls.Config
    ServerTLS   *tls.Config
    RequestAuth RequestAuthenticator
    ConnAuth    ConnAuthenticator
}
```

### Security Modes

| Mode | Description |
|---|---|
| `ModeInsecure` | No encryption or authentication |
| `ModeLocal` | Local-only security |
| `ModeTLS` | TLS encryption with optional mTLS |

### Authentication Interfaces

```go
type ConnAuthenticator interface {
    ClientHandshake(context.Context, string, net.Conn) (net.Conn, AuthInfo, error)
    ServerHandshake(net.Conn) (net.Conn, AuthInfo, error)
    Info() ProtocolInfo
    Clone() ConnAuthenticator
    OverrideServerName(string) error
}

type RequestAuthenticator interface {
    AuthenticateRequest(*http.Request) (AuthInfo, error)
}
```

---

## Service Discovery

### Registry Interface

```go
type Registry interface {
    Register(context.Context, Instance) error
    Deregister(context.Context, Instance) error
    Type() string
}
```

The registry manages service instance lifecycle. Applications register themselves on startup and deregister on shutdown.

### Instance Interface

```go
type Instance interface {
    Region() string
    Zone() string
    Campus() string
    Namespace() string
    Name() string
    Version() string
    Metadata() map[string]string
    Endpoints() []Endpoint
}
```

### Registry Provider

```go
type Provider interface {
    Type() string
    New(cfg map[string]any) (Registry, error)
}
```

### Multi-Registry Composite

The built-in `multi_registry` provider composes multiple registries into one. Register/deregister calls are fanned out to all backends. It supports a `FailFast` option to stop on the first error.

```go
func BuiltinProvider() Provider
```

### Resolver Interface

```go
type Resolver interface {
    AddWatch(serviceName string, client Client) error
    DelWatch(serviceName string, client Client) error
    Type() string
}
```

Resolvers watch service endpoints and update the client load balancer via the `Client` interface:

```go
type Client interface {
    UpdateState(state State)
}
```

### Resolver Provider

```go
type Provider interface {
    Type() string
    New(name string) (Resolver, error)
}
```

---

## Load Balancing

### Balancer Interface

```go
type Balancer interface {
    UpdateState(resolver.State)
    Close() error
    Type() string
}
```

### Picker Interface

```go
type Picker interface {
    Next(RPCInfo) (PickResult, error)
}
```

### Round-Robin Implementation

The built-in round-robin balancer uses an atomic counter:

```go
func BuiltinProvider() Provider
```

Returns a provider with type `"round_robin"`. Each `Next()` call advances the counter and selects the next available endpoint.

### Balancer Provider

```go
type Provider interface {
    Type() string
    New(serviceName, balancerName string, cli Client) (Balancer, error)
}
```

### Balancer Configuration

```go
func Configure(defaults map[string]Spec, services map[string]map[string]Spec)
func ResolveType(balancerName string) (string, error)
func LoadConfig(serviceName, balancerName string) map[string]any
func New(serviceName, balancerName string, cli Client) (Balancer, error)
```

Balancer specs can be configured per-service or as defaults.

---

## Custom Transport Extension Example

To add a custom protocol transport:

```go
package mytransport

import (
    "context"

    "github.com/codesjoy/yggdrasil/v3/module"
    "github.com/codesjoy/yggdrasil/v3/transport"
    "github.com/codesjoy/yggdrasil/v3/discovery/resolver"
    "github.com/codesjoy/yggdrasil/v3/observability/stats"
)

type MyTransportModule struct{}

func (m *MyTransportModule) Name() string { return "transport.my_protocol" }

func (m *MyTransportModule) Capabilities() []module.Capability {
    return []module.Capability{
        {
            Spec: module.CapabilitySpec{
                Name:        "transport.server.provider",
                Cardinality: module.Many,
            },
            Name: "my_protocol",
            Value: transport.NewTransportServerProvider(
                "my_protocol",
                func(handle transport.MethodHandle) (transport.Server, error) {
                    return newMyServer(handle), nil
                },
            ),
        },
        {
            Spec: module.CapabilitySpec{
                Name:        "transport.client.provider",
                Cardinality: module.Many,
            },
            Name: "my_protocol",
            Value: transport.NewTransportClientProvider(
                "my_protocol",
                func(
                    ctx context.Context,
                    service string,
                    endpoint resolver.Endpoint,
                    statsHandler stats.Handler,
                    onChange transport.OnStateChange,
                ) (transport.Client, error) {
                    return newMyClient(ctx, service, endpoint), nil
                },
            ),
        },
    }
}
```

Register the module during assembly and configure it via YAML:

```yaml
yggdrasil:
  server:
    transports:
      - protocol: my_protocol
        address: ":9090"
```

See [Module System Design](module-system.md) for module registration and capability contracts.
