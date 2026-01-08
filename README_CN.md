# Yggdrasil

[English](README.md) | [简体中文](README_CN.md)

<div align="center">

[![Go Version](https://img.shields.io/badge/Go-%3E%3D%201.24-blue)](https://go.dev/)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)

一个现代化、高性能的 Go 微服务框架，为微服务架构提供稳固的根基、灵活的分支和高效的连接。

</div>

## ✨ 特性

- 🚀 **高性能** - 基于高效的 RPC 协议，优化的连接池管理
- 🔌 **可插拔架构** - 模块化设计，支持多种协议（gRPC、HTTP/REST）
- 🎯 **服务发现** - 集成服务注册中心和解析器，支持负载均衡
- 📊 **可观测性** - 集成 OpenTelemetry，支持链路追踪和指标监控
- 🔧 **配置管理** - 灵活的配置管理，支持多种配置源（文件、环境变量、命令行参数）
- 📝 **代码生成** - 基于 Protobuf 的代码生成工具，支持 RPC 和 REST API
- 🎨 **拦截器** - 强大的中间件系统，处理横切关注点
- 🌐 **多协议** - 从同一个服务定义同时支持 RPC 和 RESTful API

## 📦 安装指南

```bash
go get -u github.com/codesjoy/yggdrasil/v2
```

### 环境要求

- Go 1.24 或更高版本
- Protocol Buffers 编译器 (protoc)

## 🚀 快速开始

### 1. 定义服务（Protocol Buffers）

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

### 2. 生成代码

使用提供的代码生成工具：

```bash
# 生成 RPC 代码
protoc --go_out=. --go_opt=paths=source_relative \
  --yggdrasil-rpc_out=. --yggdrasil-rpc_opt=paths=source_relative \
  your_service.proto

# 生成 REST 代码（可选）
protoc --yggdrasil-rest_out=. --yggdrasil-rest_opt=paths=source_relative \
  your_service.proto
```

### 3. 实现服务

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
	// 初始化框架
	if err := yggdrasil.Init("helloworld"); err != nil {
		slog.Error("初始化失败", slog.Any("error", err))
		return
	}

	// 创建并注册服务
	service := &GreeterService{}

	// 启动服务器
	if err := yggdrasil.Serve(
		yggdrasil.WithServiceDesc(&pb.GreeterServiceDesc, service),
	); err != nil {
		slog.Error("服务启动失败", slog.Any("error", err))
	}
}
```

### 4. 创建客户端

```go
package main

import (
	"context"
	"log/slog"

	"github.com/codesjoy/yggdrasil/v2"
	pb "your_module/api/helloworld/v1"
)

func main() {
	// 创建客户端
	client, err := yggdrasil.NewClient("helloworld")
	if err != nil {
		slog.Error("创建客户端失败", slog.Any("error", err))
		return
	}
	defer client.Close()

	// 发起 RPC 调用
	var reply pb.HelloReply
	err = client.Invoke(context.Background(), "/helloworld.v1.Greeter/SayHello",
		&pb.HelloRequest{Name: "World"}, &reply)
	if err != nil {
		slog.Error("调用失败", slog.Any("error", err))
		return
	}

	slog.Info("响应", slog.String("message", reply.Message))
}
```

### 5. 配置文件

创建 `config.yaml` 文件：

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

## 🏗️ 架构设计

Yggdrasil 采用模块化架构，关注点清晰分离：

```
┌─────────────────────────────────────────────────────────┐
│                       应用层                             │
├─────────────────────────────────────────────────────────┤
│  服务端          │  客户端         │  注册中心          │
├──────────────────┼─────────────────┼────────────────────┤
│  拦截器          │  负载均衡       │  服务解析          │
├──────────────────┼─────────────────┼────────────────────┤
│  远程协议        │  连接管理       │  服务发现          │
├─────────────────────────────────────────────────────────┤
│                 配置管理 & 可观测性                      │
└─────────────────────────────────────────────────────────┘
```

### 核心组件

- **Application**: 生命周期管理和优雅关闭
- **Server**: 多协议服务器实现（gRPC、HTTP/REST）
- **Client**: 连接池、负载均衡和容错处理
- **Registry**: 服务注册与发现
- **Resolver**: 地址解析和健康检查
- **Balancer**: 负载均衡策略（轮询、加权等）
- **Interceptor**: 日志、追踪、指标等中间件
- **Config**: 多源配置管理
- **Logger**: 结构化日志，支持多种处理器
- **Stats**: OpenTelemetry 集成，实现可观测性

## 📚 文档

### 核心概念

- **服务注册**: 自动服务注册，支持健康检查
- **负载均衡**: 多种策略，包括轮询和加权
- **拦截器**: 可链式调用的客户端和服务端中间件
- **元数据**: 用于追踪和认证的上下文传播
- **流式处理**: 支持一元、客户端流、服务端流和双向流

### 高级特性

- **Governor**: 内置管理服务器，用于健康检查和调试
- **统计处理器**: 自定义指标和追踪集成

## 🛠️ 代码生成工具

Yggdrasil 提供三个 protoc 插件：

1. **protoc-gen-yggdrasil-rpc**: 生成 RPC 服务代码
2. **protoc-gen-yggdrasil-rest**: 生成 RESTful API 处理器
3. **protoc-gen-yggdrasil-reason**: 生成错误原因码

安装方法：

```bash
# 安装所有代码生成工具
make install

# 或手动安装
go install github.com/codesjoy/yggdrasil/v2/cmd/protoc-gen-yggdrasil-rpc@latest
go install github.com/codesjoy/yggdrasil/v2/cmd/protoc-gen-yggdrasil-rest@latest
go install github.com/codesjoy/yggdrasil/v2/cmd/protoc-gen-yggdrasil-reason@latest
```

## 📖 示例

查看 [examples](example/) 目录获取完整的工作示例。

## 🤝 贡献

欢迎贡献！请随时提交 Pull Request。

1. Fork 本仓库
2. 创建特性分支 (`git checkout -b feature/amazing-feature`)
3. 提交更改 (`git commit -m 'Add some amazing feature'`)
4. 推送到分支 (`git push origin feature/amazing-feature`)
5. 开启 Pull Request

## 📄 许可证

本项目采用 Apache License 2.0 许可证 - 详见 [LICENSE](LICENSE) 文件。
