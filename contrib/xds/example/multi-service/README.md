# 多服务示例

本示例演示如何使用yggdrasil v2框架和xDS实现多服务场景下的服务发现和通信。

## 目录结构

```
multi-service/
├── client/
│   ├── main.go      # 客户端实现，调用多个服务
│   └── config.yaml  # 客户端配置
└── server/
    ├── main.go      # 服务端实现，同时提供多个服务
    └── config.yaml  # 服务端配置
```

## 功能特性

- **多服务支持**: 客户端可以同时调用多个不同的gRPC服务
- **统一管理**: 使用yggdrasil统一管理所有服务的连接和配置
- **服务发现**: 通过xDS发现所有服务的端点信息
- **元数据传递**: 使用metadata传递流标识
- **灵活配置**: 支持动态配置多个服务的xDS设置

## 运行步骤

### 1. 启动xDS控制平面

```bash
cd contrib/xds/example/control-plane
set XDS_CONFIG_FILE=multi-service-xds-config.yaml
go run main.go --config config.yaml
```

### 2. 启动多服务服务器

```bash
cd contrib/xds/example/multi-service/server
go run main.go --config config.yaml
```

### 3. 运行客户端

```bash
cd contrib/xds/example/multi-service/client
go run main.go --config config.yaml
```

## 预期输出

客户端将交替调用Library服务和Greeter服务：

```
2025/01/26 10:00:00 INFO Starting multi-service client...
2025/01/26 10:00:00 INFO Starting multi-service test loop...
2025/01/26 10:00:00 INFO Greeter service response message="Hello, world"
2025/01/26 10:00:00 INFO Library service response name="shelf-" theme=""
2025/01/26 10:00:00 INFO Greeter service response message="Hello, world"
2025/01/26 10:00:00 INFO Library service response name="shelf-" theme=""
...
2025/01/26 10:00:09 INFO Multi-service client completed successfully
```

## 技术架构

### 服务架构

```
┌─────────────────┐
│   Client        │
│                 │
│ ┌─────────────┐ │
│ │ Library     │ │
│ │ Service     │ │
│ │ Client      │ │
│ └─────────────┘ │
│                 │
│ ┌─────────────┐ │
│ │ Greeter     │ │
│ │ Service     │ │
│ │ Client      │ │
│ └─────────────┘ │
└────────┬────────┘
         │
         │ xDS Discovery
         │
┌────────▼────────┐
│  xDS Control     │
│  Plane           │
└────────┬────────┘
         │
         │ Service Discovery
         │
┌────────▼────────┐
│  Multi-Service   │
│  Server          │
│                  │
│  - Library Svc   │
│  - Greeter Svc   │
└──────────────────┘
```

### 配置结构

```yaml
yggdrasil:
  client:
    # Library服务配置
    github.com.codesjoy.yggdrasil.example.library:
      resolver: "xds"
      balancer: "xds"

    # Greeter服务配置
    github.com.codesjoy.yggdrasil.example.greeter:
      resolver: "xds"
      balancer: "xds"

  xds:
    default:
      server:
        address: "127.0.0.1:18000"
      node:
        id: "multi-service-client"
        cluster: "test-cluster"
      protocol: "grpc"
```

## 配置说明

### 客户端配置 (config.yaml)

```yaml
yggdrasil:
  resolver:
    xds:
      type: "xds"
      config:
        name: "default"

  # 配置多个服务的客户端
  client:
    github.com.codesjoy.yggdrasil.example.library:
      resolver: "xds"
      balancer: "xds"
    github.com.codesjoy.yggdrasil.example.greeter:
      resolver: "xds"
      balancer: "xds"

  xds:
    default:
      server:
        address: "127.0.0.1:18000"
      node:
        id: "multi-service-client"
        cluster: "test-cluster"
      protocol: "grpc"
```

### xDS配置 (multi-service-xds-config.yaml)

```yaml
clusters:
  # Library服务集群
  - name: "library-cluster"
    connectTimeout: "5s"
    type: "EDS"
    lbPolicy: "ROUND_ROBIN"

  # Greeter服务集群
  - name: "greeter-cluster"
    connectTimeout: "5s"
    type: "EDS"
    lbPolicy: "ROUND_ROBIN"

endpoints:
  # Library服务端点
  - clusterName: "library-cluster"
    endpoints:
      - address: "127.0.0.1"
        port: 55890

  # Greeter服务端点
  - clusterName: "greeter-cluster"
    endpoints:
      - address: "127.0.0.1"
        port: 55890

listeners:
  # Library服务监听器
  - name: "library-service"
    address: "0.0.0.0"
    port: 10000
    filterChains:
      - filters:
          - name: "envoy.filters.network.http_connection_manager"
            routeConfigName: "library-route"

  # Greeter服务监听器
  - name: "greeter-service"
    address: "0.0.0.0"
    port: 10001
    filterChains:
      - filters:
          - name: "envoy.filters.network.http_connection_manager"
            routeConfigName: "greeter-route"

routes:
  # Library服务路由
  - name: "library-route"
    virtualHosts:
      - name: "library"
        domains: ["*"]
        routes:
          - match:
              path:
                prefix: "/"
            route:
              cluster: "library-cluster"

  # Greeter服务路由
  - name: "greeter-route"
    virtualHosts:
      - name: "greeter"
        domains: ["*"]
        routes:
          - match:
              path:
                prefix: "/"
            route:
              cluster: "greeter-cluster"
```

## 技术要点

### 1. 创建多个客户端

```go
libraryClient, err := yggdrasil.NewClient("github.com.codesjoy.yggdrasil.example.library")
if err != nil {
    os.Exit(1)
}
defer libraryClient.Close()

greeterClient, err := yggdrasil.NewClient("github.com.codesjoy.yggdrasil.example.greeter")
if err != nil {
    os.Exit(1)
}
defer greeterClient.Close()
```

### 2. 创建多个服务客户端

```go
library := libraryv1.NewLibraryServiceClient(libraryClient)
greeter := helloworldv1.NewGreeterServiceClient(greeterClient)
```

### 3. 交替调用服务

```go
for i := 1; i <= 20; i++ {
    time.Sleep(500 * time.Millisecond)

    if i%2 == 0 {
        ctx := metadata.WithStreamContext(context.Background())
        shelf, err := library.GetShelf(ctx, &libraryv1.GetShelfRequest{
            Name: "shelf-" + string(rune(i)),
        })
        slog.Info("Library service response", "name", shelf.Name, "theme", shelf.Theme)
    } else {
        ctx := metadata.WithStreamContext(context.Background())
        response, err := greeter.SayHello(ctx, &helloworldv1.SayHelloRequest{
            Name: "world",
        })
        slog.Info("Greeter service response", "message", response.Message)
    }
}
```

### 4. 多服务实现

服务端同时注册两个服务：

```go
libraryImpl := &librarypb2.LibraryServiceServer{}
greeterImpl := &greeterpb2.GreeterServiceServer{}

if err := yggdrasil.Serve(
    yggdrasil.WithServiceDesc(&librarypb2.LibraryServiceServiceDesc, libraryImpl),
    yggdrasil.WithServiceDesc(&greeterpb2.GreeterServiceServiceDesc, greeterImpl),
); err != nil {
    os.Exit(1)
}
```

## 服务间通信

### 同步调用

```go
ctx := metadata.WithStreamContext(context.Background())
response, err := greeter.SayHello(ctx, &helloworldv1.SayHelloRequest{
    Name: "world",
})
```

### 错误处理

```go
response, err := greeter.SayHello(ctx, req)
if err != nil {
    slog.Error("Greeter service call failed", "error", err)
    continue
}
```

### 资源清理

```go
defer libraryClient.Close()
defer greeterClient.Close()
```

## 应用场景

### 1. 微服务架构

- 多个独立服务协同工作
- 服务间通过gRPC通信
- 统一的服务发现和负载均衡

### 2. 网关模式

- 客户端调用多个后端服务
- 统一的服务管理和配置
- 简化客户端逻辑

### 3. 多租户系统

- 不同租户使用不同的服务实例
- 通过xDS动态配置租户路由
- 灵活的服务隔离和共享

### 4. 混合部署

- 部分服务本地部署
- 部分服务远程调用
- 统一的服务管理框架

## 最佳实践

### 1. 服务命名

使用清晰、一致的服务命名约定：
- `github.com.codesjoy.yggdrasil.example.library`
- `github.com.codesjoy.yggdrasil.example.greeter`

### 2. 配置管理

- 所有服务使用统一的xDS配置
- 服务特定的配置在客户端配置中设置
- 避免重复配置

### 3. 错误处理

- 每个服务调用都要单独处理错误
- 实现重试和熔断机制
- 记录详细的错误日志

### 4. 资源管理

- 确保所有客户端正确关闭
- 使用defer保证资源释放
- 避免连接泄漏

### 5. 监控和追踪

- 为每个服务调用添加监控
- 使用分布式追踪跟踪服务间调用
- 收集服务性能指标

## 常见问题

Q: 一个客户端可以连接多个服务吗？
A: 可以。yggdrasil支持创建多个客户端实例，每个实例连接不同的服务。

Q: 多个服务必须部署在同一服务器上吗？
A: 不需要。服务可以部署在不同的服务器上，通过xDS配置端点信息。

Q: 如何管理多个服务的版本？
A: 使用xDS的版本控制功能，为每个服务配置不同的版本。

Q: 服务间调用超时如何设置？
A: 在context中设置超时时间，或者通过yggdrasil的配置统一设置。

Q: 如何实现服务间的负载均衡？
A: 在xDS配置中为每个服务配置负载均衡策略。

Q: 多服务场景下的性能如何？
A: 性能良好。yggdrasil使用连接池和并发控制，可以高效处理多个服务的并发调用。

Q: 如何调试多服务调用？
A: 使用结构化日志记录每个服务调用，包括请求、响应、错误信息。使用分布式追踪工具跟踪跨服务调用。
