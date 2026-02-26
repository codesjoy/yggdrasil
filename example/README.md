# Yggdrasil 示例指南

本目录包含 Yggdrasil 微服务框架的完整示例，帮助开发者快速理解和使用框架的核心功能。

## 目录结构

```
example/
├── proto/              # Protocol Buffers 定义文件
│   ├── helloworld/    # 简单的问候服务示例
│   └── library/       # 图书馆服务示例（包含 REST API）
├── protogen/          # 生成的 Go 代码
├── sample/             # 基础示例（入门必看）
│   ├── server/         # 服务端实现
│   └── client/         # 客户端实现
└── advanced/           # 高级示例（进阶学习）
    ├── streaming/       # 流式通信
    ├── config/         # 配置管理
    ├── error-handling/ # 错误处理
    ├── metadata/       # 元数据传递
    ├── load-balancing/ # 负载均衡
    └── rest/          # REST API
```

## 学习路径

### 🌱 初学者路径（推荐）

如果你是第一次使用 Yggdrasil，建议按以下顺序学习：

1. **[Sample 基础示例](sample/)** ⏱️ 15分钟
   - 学习框架的基本使用
   - 理解服务端和客户端的启动流程
   - 了解配置文件的基本结构

2. **[Streaming 流式通信](advanced/streaming/)** ⏱️ 20分钟
   - 学习四种流式 RPC 模式
   - 理解流式通信的使用场景
   - 掌握双向流处理技巧

**预计总时间**: 35分钟

### 🚀 进阶路径（推荐有经验的开发者）

如果你已有 gRPC/微服务经验，可以直接学习进阶内容：

1. **[Sample 基础示例](sample/)** ⏱️ 15分钟
   - 快速了解框架架构
   - 理解配置系统

2. **[Config 配置管理](advanced/config/)** ⏱️ 20分钟
   - 多配置源加载
   - 配置热更新
   - 配置优先级管理

3. **[Error Handling 错误处理](advanced/error-handling/)** ⏱️ 20分钟
   - 自定义错误 reason
   - 错误传播和重试
   - 错误码映射

4. **[Metadata 元数据传递](advanced/metadata/)** ⏱️ 15分钟
   - 跨服务元数据传递
   - Request/Response Metadata
   - Trailer 使用

5. **[Load Balancing 负载均衡](advanced/load-balancing/)** ⏱️ 25分钟
   - 多实例部署
   - 负载均衡策略
   - 健康检查

6. **[REST API](advanced/rest/)** ⏱️ 20分钟
   - 从 proto 生成 REST API
   - HTTP/JSON 编解码
   - REST 中间件配置

**预计总时间**: 1小时45分钟

### 📚 完整路径（深入掌握）

1. 完成**初学者路径**或**进阶路径**
2. **[Contrib 集成示例](https://github.com/codesjoy/yggdrasil-ecosystem/tree/main/integrations)** ⏱️ 2-3小时
   - [etcd 集成](https://github.com/codesjoy/yggdrasil-ecosystem/tree/main/integrations/etcd/example): 配置中心、服务注册与发现
   - [Kubernetes 集成](https://github.com/codesjoy/yggdrasil-ecosystem/tree/main/integrations/k8s/example): ConfigMap、Secret、服务发现
   - [Polaris 集成](https://github.com/codesjoy/yggdrasil-ecosystem/tree/main/integrations/polaris/example): 配置中心、服务治理
   - [xDS 集成](https://github.com/codesjoy/yggdrasil-ecosystem/tree/main/integrations/xds/example): 动态配置、流量管理

**预计总时间**: 4-5小时

## 示例概览

### 📦 基础示例

| 示例 | 说明 | 难度 | 时间 | 状态 |
|------|------|--------|------|------|
| [Sample](sample/) | 框架基础使用 | ⭐ | 15分钟 | ✅ |
| [Streaming](advanced/streaming/) | 流式通信 | ⭐⭐ | 20分钟 | 📝 |
| [REST API](advanced/rest/) | REST API 使用 | ⭐⭐ | 20分钟 | 📝 |

### 🔧 进阶示例

| 示例 | 说明 | 难度 | 时间 | 状态 |
|------|------|--------|------|------|
| [Config](advanced/config/) | 配置管理 | ⭐⭐⭐ | 20分钟 | 📝 |
| [Error Handling](advanced/error-handling/) | 错误处理 | ⭐⭐⭐ | 20分钟 | 📝 |
| [Metadata](advanced/metadata/) | 元数据传递 | ⭐⭐ | 15分钟 | 📝 |
| [Load Balancing](advanced/load-balancing/) | 负载均衡 | ⭐⭐⭐ | 25分钟 | 📝 |

### 🌐 集成示例 (Contrib)

| 示例 | 说明 | 难度 | 时间 |
|------|------|--------|------|
| [etcd](https://github.com/codesjoy/yggdrasil-ecosystem/tree/main/integrations/etcd/example) | 配置中心、服务注册与发现 | ⭐⭐ | 30分钟 |
| [Kubernetes](https://github.com/codesjoy/yggdrasil-ecosystem/tree/main/integrations/k8s/example) | ConfigMap、Secret、服务发现 | ⭐⭐⭐ | 45分钟 |
| [Polaris](https://github.com/codesjoy/yggdrasil-ecosystem/tree/main/integrations/polaris/example) | 配置中心、服务治理 | ⭐⭐⭐ | 45分钟 |
| [xDS](https://github.com/codesjoy/yggdrasil-ecosystem/tree/main/integrations/xds/example) | 动态配置、流量管理 | ⭐⭐⭐⭐ | 60分钟 |

## 快速开始

### 前置条件

1. Go 1.25 或更高版本
2. Protocol Buffers 编译器 (protoc)
3. Yggdrasil 代码生成工具

### 安装依赖

```bash
# 安装 Yggdrasil
go get -u github.com/codesjoy/yggdrasil/v2

# 安装代码生成工具
go install github.com/codesjoy/yggdrasil/v2/cmd/protoc-gen-yggdrasil-rpc@latest
go install github.com/codesjoy/yggdrasil/v2/cmd/protoc-gen-yggdrasil-rest@latest

# 从 codesjoy/pkg 安装 reason 插件
git clone https://github.com/codesjoy/pkg.git
cd pkg
go install ./tools/protoc-gen-codesjoy-reason
```

### 运行第一个示例

选择 [Sample 基础示例](sample/) 开始你的第一个 Yggdrasil 服务：

```bash
# 1. 启动服务端
cd example/sample/server
go run main.go

# 2. 新开终端，启动客户端
cd example/sample/client
go run main.go
```

## 示例功能对比

### 通信模式

| 模式 | 示例 | 说明 |
|------|------|------|
| Unary RPC | Sample | 单次请求-响应 |
| Client Streaming | Streaming | 客户端流式发送 |
| Server Streaming | Streaming | 服务端流式发送 |
| Bidirectional Streaming | Streaming | 双向流式通信 |

### 配置管理

| 功能 | 示例 | 说明 |
|------|------|------|
| 文件配置 | Sample | 从 YAML 文件加载配置 |
| 多配置源 | Config | 文件、环境变量、命令行 |
| 配置热更新 | Config | 运行时更新配置 |
| 配置优先级 | Config | 配置源优先级管理 |

### 服务治理

| 功能 | 示例 | 说明 |
|------|------|------|
| 错误处理 | Error Handling | 自定义错误和重试 |
| 元数据传递 | Metadata | 跨服务上下文传递 |
| 负载均衡 | Load Balancing | 多实例负载分配 |
| 健康检查 | Load Balancing | 实例健康状态监控 |

### 协议支持

| 协议 | 示例 | 说明 |
|------|------|------|
| gRPC | Sample | gRPC 协议 |
| REST/HTTP | REST | RESTful API |
| 双协议 | Sample | 同时支持 gRPC 和 REST |

## 常见问题

### Q: 从哪个示例开始学习？

A: 如果你完全不了解 Yggdrasil，从 **[Sample 基础示例](sample/)** 开始。如果你有 gRPC 经验，可以直接选择感兴趣的进阶示例。

### Q: 示例可以直接在生产环境使用吗？

A: 示例主要用于学习框架的使用方法。在生产环境使用前，你需要：
1. 添加完善的错误处理和重试机制
2. 实现监控和日志收集
3. 配置适当的安全措施
4. 进行性能测试和优化

### Q: 如何修改示例的配置？

A: 每个示例目录下都有 `config.yaml` 文件，你可以根据需要修改配置。配置文件的详细说明请参考示例的 README 文档。

### Q: 示例需要额外的依赖吗？

A: 基础示例只需要 Yggdrasil 框架。Contrib 示例（etcd、k8s、polaris、xds）需要相应的中间件支持。

### Q: 如何运行所有示例的测试？

A: 每个示例目录下都提供了运行说明，建议按顺序逐个运行。 Contrib 示例可能需要先启动相应的中间件（如 etcd、Kubernetes 等）。

## 技术栈

- **框架**: Yggdrasil v2
- **协议**: gRPC、HTTP/REST
- **配置**: YAML、环境变量、命令行
- **日志**: slog (Go 1.21+)
- **代码生成**: protoc、buf

## 扩展阅读

- [Yggdrasil 主文档](../../README.md)
- [Yggdrasil 中文文档](../../README_CN.md)
- [Protocol Buffers 文档](https://protobuf.dev/)
- [gRPC 文档](https://grpc.io/docs/)
- [REST API 设计指南](https://cloud.google.com/apis/design)

## 反馈与贡献

如果你在使用示例过程中遇到问题或有改进建议，欢迎：

1. 提交 [Issue](https://github.com/codesjoy/yggdrasil/issues)
2. 提交 [Pull Request](https://github.com/codesjoy/yggdrasil/pulls)
3. 在社区讨论

## 许可证

本示例遵循 Yggdrasil 项目的 [Apache License 2.0](../../LICENSE)。
