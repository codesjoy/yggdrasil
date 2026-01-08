# Yggdrasil

[English](README.md) | [简体中文](README_CN.md)

<div align="center">

[![Go Version](https://img.shields.io/badge/Go-%3E%3D%201.24-blue)](https://go.dev/)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)

A modern, high-performance Go microservice framework providing a solid foundation, flexible branches, and efficient connections for microservice architecture.

</div>

## ✨ Features

- 🚀 **High Performance** - Based on efficient RPC protocols, optimized connection pool management
- 🔌 **Pluggable Architecture** - Modular design, supporting multiple protocols (gRPC, HTTP/REST)
- 🎯 **Service Discovery** - Integrated service registry and resolver, supporting load balancing
- 📊 **Observability** - Integrated OpenTelemetry, supporting tracing and metrics monitoring
- 🔧 **Configuration Management** - Flexible configuration management, supporting multiple sources (files, environment variables, command-line arguments)
- 📝 **Code Generation** - Protobuf-based code generation tools, supporting RPC and REST API
- 🎨 **Interceptor** - Powerful middleware system for handling cross-cutting concerns
- 🌐 **Multi-Protocol** - Support for both RPC and RESTful APIs from the same service definition

## 📦 Installation

```bash
go get -u github.com/codesjoy/yggdrasil/v2
```

### Requirements

- Go 1.24 or higher
- Protocol Buffers compiler (protoc)

## 🚀 Quick Start

### 1. Define Service (Protocol Buffers)

```protobuf
syntax = "proto3";

package helloworld.v1;

service Greeter {
  rpc SayHello (HelloRequest) returns (HelloReply);
}

message HelloRequest {
  string name = 1;
}

message HelloReply {
  string message = 1;
}
```

### 2. Generate Code

Use the provided code generation tools:

```bash
# Generate RPC code
protoc --go_out=. --go_opt=paths=source_relative \
  --yggdrasil-rpc_out=. --yggdrasil-rpc_opt=paths=source_relative \
  your_service.proto

# Generate REST code (optional)
protoc --yggdrasil-rest_out=. --yggdrasil-rest_opt=paths=source_relative \
  your_service.proto
```

### 3. Implement Service

```go
package main

import (
	"context"
	"log/slog"

	"github.com/codesjoy/yggdrasil/v2"
	pb "your_module/api/helloworld/v1"
)

type GreeterService struct {
	pb.UnimplementedGreeterServer
}

func (s *GreeterService) SayHello(ctx context.Context, req *pb.HelloRequest) (*pb.HelloReply, error) {
	return &pb.HelloReply{
		Message: "Hello " + req.Name,
	}, nil
}

func main() {
	// Initialize framework
	if err := yggdrasil.Init("helloworld"); err != nil {
		slog.Error("Initialization failed", slog.Any("error", err))
		return
	}

	// Create and register service
	service := &GreeterService{}

	// Start server
	if err := yggdrasil.Serve(
		yggdrasil.WithServiceDesc(&pb.GreeterServiceDesc, service),
	); err != nil {
		slog.Error("Service start failed", slog.Any("error", err))
	}
}
```

### 4. Create Client

```go
package main

import (
	"context"
	"log/slog"

	"github.com/codesjoy/yggdrasil/v2"
	pb "your_module/api/helloworld/v1"
)

func main() {
	// Create client
	client, err := yggdrasil.NewClient("helloworld")
	if err != nil {
		slog.Error("Failed to create client", slog.Any("error", err))
		return
	}
	defer client.Close()

	// Invoke RPC call
	var reply pb.HelloReply
	err = client.Invoke(context.Background(), "/helloworld.v1.Greeter/SayHello",
		&pb.HelloRequest{Name: "World"}, &reply)
	if err != nil {
		slog.Error("Call failed", slog.Any("error", err))
		return
	}

	slog.Info("Response", slog.String("message", reply.Message))
}
```

### 5. Configuration File

Create a `config.yaml` file:

```yaml
yggdrasil:
  server:
    protocol:
      - grpc
    grpc:
      address: :9000

  rest:
    enable: true
    address: :8080

  logger:
    handler:
      default:
        type: console
        config:
          level: info
```

## 🏗️ Architecture

Yggdrasil adopts a modular architecture with clearly separated concerns:

```
┌─────────────────────────────────────────────────────────┐
│                     Application Layer                   │
├─────────────────────────────────────────────────────────┤
│    Server       │    Client       │    Registry         │
├─────────────────┼─────────────────┼─────────────────────┤
│    Interceptor  │  Load Balancer  │  Service Resolver   │
├─────────────────┼─────────────────┼─────────────────────┤
│ Remote Protocol │ Connection Mgmt │  Service Discovery  │
├─────────────────────────────────────────────────────────┤
│            Configuration & Observability                │
└─────────────────────────────────────────────────────────┘
```

### Core Components

- **Application**: Lifecycle management and graceful shutdown
- **Server**: Multi-protocol server implementation (gRPC, HTTP/REST)
- **Client**: Connection pooling, load balancing, and fault tolerance
- **Registry**: Service registration and discovery
- **Resolver**: Address resolution and health checking
- **Balancer**: Load balancing strategies (round-robin, weighted, etc.)
- **Interceptor**: Middleware for logging, tracing, metrics, etc.
- **Config**: Multi-source configuration management
- **Logger**: Structured logging with multiple handlers
- **Stats**: OpenTelemetry integration for observability

## 📚 Documentation

### Core Concepts

- **Service Registration**: Automatic service registration with support for health checks
- **Load Balancing**: Various strategies including round-robin and weighted
- **Interceptor**: Chainable client and server middleware
- **Metadata**: Context propagation for tracing and authentication
- **Streaming**: Supports unary, client streaming, server streaming, and bidirectional streaming

### Advanced Features

- **Governor**: Built-in management server for health checks and debugging
- **Stats Handler**: Custom metrics and tracing integration

## 🛠️ Code Generation Tools

Yggdrasil provides three protoc plugins:

1. **protoc-gen-yggdrasil-rpc**: Generates RPC service code
2. **protoc-gen-yggdrasil-rest**: Generates RESTful API handlers
3. **protoc-gen-yggdrasil-reason**: Generates error reason codes

Installation:

```bash
# Install all code generation tools
make install

# Or install manually
go install github.com/codesjoy/yggdrasil/v2/cmd/protoc-gen-yggdrasil-rpc@latest
go install github.com/codesjoy/yggdrasil/v2/cmd/protoc-gen-yggdrasil-rest@latest
go install github.com/codesjoy/yggdrasil/v2/cmd/protoc-gen-yggdrasil-reason@latest
```

## 📖 Examples

Check the [examples](example/) directory for complete working examples.

## 🤝 Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

1. Fork this repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## 📄 License

This project is licensed under the Apache License 2.0 - see the [LICENSE](LICENSE) file for details.
