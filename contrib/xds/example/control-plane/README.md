# xDS控制平面示例

本示例演示如何实现一个现代化的xDS控制平面服务器，支持Aggregated Discovery Service (ADS)协议。

## 目录结构

```
control-plane/
├── main.go                 # 主入口和配置加载
├── config.yaml             # 控制平面配置
├── xds-config.yaml         # xDS资源配置
├── server/                 # xDS服务器实现
│   └── server.go
├── snapshot/               # xDS快照构建
│   ├── types.go
│   └── builder.go
├── watcher/                # 文件监控
│   └── file_watcher.go
└── README.md              # 本文件
```

## 功能特性

- **Aggregated Discovery Service**: 支持LDS、RDS、CDS、EDS的统一发现服务
- **动态配置**: 支持配置文件热更新，无需重启服务
- **模块化设计**: 清晰的代码结构，易于维护和扩展
- **负载均衡策略**: 支持ROUND_ROBIN、RANDOM、LEAST_REQUEST、RING_HASH、MAGLEV
- **熔断器配置**: 支持MaxConnections、MaxPendingRequests、MaxRequests、MaxRetries
- **异常检测配置**: 完整的Outlier Detection支持
- **限流器配置**: 通过Metadata配置Token Bucket限流
- **优雅关闭**: 支持SIGINT、SIGTERM信号处理
- **版本控制**: 原子版本号，支持增量更新
- **回调系统**: 完整的xDS Server回调，便于调试和监控
- **Delta xDS**: 支持Delta流协议

## 运行步骤

### 1. 启动控制平面

```bash
cd contrib/xds/example/control-plane
go run .
```

控制平面将在18000端口启动。

### 2. 配置说明

**控制平面配置** (config.yaml):

```yaml
server:
  port: 18000              # 控制平面监听端口
  nodeID: "yggdrasil.example.xds.control-plane"

xds:
  configFile: "xds-config.yaml"  # xDS资源配置文件
  watchInterval: 1s            # 配置文件监控间隔

yggdrasil:
  logger:
    handler:
      default:
        type: "console"
        level: "info"
    writer:
      default:
        type: "console"
```

**xDS资源配置** (xds-config.yaml):

```yaml
clusters:
  - name: "library-cluster"
    connectTimeout: "5s"
    type: "EDS"
    lbPolicy: "ROUND_ROBIN"
    circuitBreakers:          # 熔断器配置
      maxConnections: 10000
      maxPendingRequests: 1000
      maxRequests: 5000
      maxRetries: 3
    outlierDetection:          # 异常检测配置
      consecutive5xx: 5
      consecutiveGatewayFailure: 3
      consecutiveLocalOriginFailure: 2
      interval: "10s"
      baseEjectionTime: "30s"
      maxEjectionTime: "300s"
      maxEjectionPercent: 10
    rateLimiting:             # 限流器配置（通过Metadata）
      maxTokens: 1000
      tokensPerFill: 100
      fillInterval: "1s"

endpoints:
  - clusterName: "library-cluster"
    endpoints:
      - address: "127.0.0.1"
        port: 55555

listeners:
  - name: "library-service"
    address: "0.0.0.0"
    port: 10000
    filterChains:
      - filters:
          - name: "envoy.filters.network.http_connection_manager"
            routeConfigName: "library-route"

routes:
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
```

**支持的负载均衡策略**:
- `ROUND_ROBIN`: 轮询
- `LEAST_REQUEST`: 最少请求
- `RANDOM`: 随机
- `RING_HASH`: 一致性哈希
- `MAGLEV`: Maglev哈希

## 工作原理

### 1. 客户端连接流程

```
客户端
  ↓ ADS请求
控制平面
  ↓ 配置响应
客户端
```

### 2. 资源类型

| 类型 | 说明 | 示例 |
|------|------|------|
| LDS | Listener Discovery Service | library-service |
| RDS | Route Discovery Service | library-route |
| CDS | Cluster Discovery Service | library-cluster |
| EDS | Endpoint Discovery Service | 127.0.0.1:55555 |

### 3. 资源配置关系

```
Listener (library-service)
  ↓ 关联
Route (library-route)
  ↓ 路由到
Cluster (library-cluster)
  ↓ 包含
Endpoints (127.0.0.1:55555)
```

### 4. 配置热更新机制

```
配置文件修改
  ↓ fsnotify监控
防抖动处理
  ↓ 加载新配置
构建Snapshot
  ↓ 更新缓存
  ↓ 推送新版本
客户端
```

## 配置管理

### 添加新服务

修改 `xds-config.yaml` 文件：

```yaml
clusters:
  - name: "new-service"
    connectTimeout: "5s"
    type: "EDS"
    lbPolicy: "ROUND_ROBIN"

endpoints:
  - clusterName: "new-service"
    endpoints:
      - address: "127.0.0.1"
        port: 8080

listeners:
  - name: "new-service-listener"
    address: "0.0.0.0"
    port: 10001
    filterChains:
      - filters:
          - name: "envoy.filters.network.http_connection_manager"
            routeConfigName: "new-service-route"

routes:
  - name: "new-service-route"
    virtualHosts:
      - name: "new-service"
        domains: ["*"]
        routes:
          - match:
              path:
                prefix: "/"
            route:
              cluster: "new-service"
```

文件保存后，控制平面会自动检测变更并重新加载配置。

### 修改负载均衡策略

修改 `xds-config.yaml` 中的 `lbPolicy`：

```yaml
clusters:
  - name: "library-cluster"
    lbPolicy: "LEAST_REQUEST"  # 改为最少请求策略
```

### 路由匹配规则

支持多种路由匹配方式：

```yaml
routes:
  - name: "library-route"
    virtualHosts:
      - name: "library"
        domains: ["*"]
        routes:
          # 前缀匹配
          - match:
              path:
                prefix: "/api/"
            route:
              cluster: "library-cluster"
          # 精确路径匹配
          - match:
              path:
                path: "/exact/path"
            route:
              cluster: "library-cluster"
          # Header匹配
          - match:
              path:
                prefix: "/"
              headers:
                - name: "x-version"
                  pattern: "exact"
                  value: "v2"
            route:
              cluster: "library-cluster-v2"
```

## 高级功能

### 熔断器配置

```yaml
circuitBreakers:
  maxConnections: 10000        # 最大连接数
  maxPendingRequests: 1000      # 最大待处理请求数
  maxRequests: 5000             # 最大并发请求数
  maxRetries: 3                # 最大重试次数
```

### 异常检测配置

```yaml
outlierDetection:
  consecutive5xx: 5                       # 连续5xx错误次数
  consecutiveGatewayFailure: 3            # 连续网关失败次数
  consecutiveLocalOriginFailure: 2         # 连续本地失败次数
  interval: "10s"                         # 检测间隔
  baseEjectionTime: "30s"                  # 基础弹出时间
  maxEjectionTime: "300s"                   # 最大弹出时间
  maxEjectionPercent: 10                   # 最大弹出百分比
  enforcingConsecutive5xx: 100              # 强制连续5xx检测
  enforcingSuccessRate: 100                   # 强制成功率检测
  successRateMinimumHosts: 5                # 成功率最小主机数
  successRateRequestVolume: 100             # 成功率请求卷
  successRateStdevFactor: 1900              # 成功率标准差因子
  failurePercentageThreshold: 85            # 失败率阈值
  failurePercentageMinimumHosts: 5          # 失败率最小主机数
  failurePercentageRequestVolume: 50          # 失败率请求卷
```

### 限流器配置

```yaml
rateLimiting:
  maxTokens: 1000        # 最大令牌数
  tokensPerFill: 100       # 每次填充的令牌数
  fillInterval: "1s"       # 填充间隔
```

## API接口

### StreamAggregatedResources

实现 `AggregatedDiscoveryServiceServer` 接口，支持以下回调：

- **OnStreamOpen**: 流打开时触发
- **OnStreamClosed**: 流关闭时触发
- **OnStreamRequest**: 收到流请求时触发
- **OnStreamResponse**: 发送流响应时触发
- **OnFetchRequest**: 收到Fetch请求时触发
- **OnFetchResponse**: 发送Fetch响应时触发
- **OnDeltaStreamOpen**: Delta流打开时触发
- **OnDeltaStreamClosed**: Delta流关闭时触发
- **OnStreamDeltaRequest**: 收到Delta请求时触发
- **OnStreamDeltaResponse**: 发送Delta响应时触发

### 请求/响应格式

**DiscoveryRequest**:
- Node: 客户端节点信息
- TypeUrl: 请求的资源类型
- ResourceNames: 资源名称列表
- VersionInfo: 当前版本
- ResponseNonce: 上次响应的nonce

**DiscoveryResponse**:
- VersionInfo: 配置版本
- Nonce: 响应nonce
- TypeUrl: 资源类型
- Resources: 资源列表（Any类型）

## 与其他示例集成

### 基础集成示例

客户端配置：

```yaml
xds:
  server:
    address: "127.0.0.1:18000"
  node:
    id: "basic-client"
```

### 流量分割示例

配置多个端点：

```yaml
endpoints:
  - clusterName: "library-cluster"
    endpoints:
      - address: "127.0.0.1"
        port: 55555
      - address: "127.0.0.2"
        port: 55555
```

## 调试技巧

### 查看请求日志

```bash
[xDS Control Plane] Stream opened: id=0 type=ads
[xDS Control Plane] Stream request: id=0 node=basic-client resources=[library-service] version= type=type.googleapis.com/envoy.config.listener.v3.Listener
[xDS Control Plane] Stream response: id=0 version=1 type=type.googleapis.com/envoy.config.listener.v3.Listener resources=1
```

### 验证配置

检查日志中的配置信息：

```bash
[xDS Control Plane] Building snapshot version=1 with 1 clusters, 1 endpoints, 1 listeners, 1 routes
[xDS Control Plane] Snapshot updated successfully: version=1
```

### 测试连接

使用 grpcurl 测试：

```bash
grpcurl -plaintext 127.0.0.1:18000 list
```

### 配置热更新测试

修改 `xds-config.yaml` 文件后，观察日志：

```bash
[File changed: /path/to/xds-config.yaml
[xDS Control Plane] Configuration file changed, reloading: /path/to/xds-config.yaml
[xDS Control Plane] Building snapshot version=2 with 1 clusters, 1 endpoints, 1 listeners, 1 routes
[xDS Control Plane] Snapshot updated successfully: version=2
```

## 常见问题

### Q: 如何添加TLS支持？

A: 在 `server/server.go` 的 `NewServer` 函数中添加 TLS 凭据：

```go
import "google.golang.org/grpc/credentials"

creds, err := credentials.NewServerTLSFromFile("cert.pem", "key.pem")
if err != nil {
    log.Fatal(err)
}

grpcServer := grpc.NewServer(
    grpc.Creds(creds),
    grpc.MaxConcurrentStreams(grpcMaxConcurrentStreams),
)
```

### Q: 如何实现动态配置管理接口？

A: 可以添加 HTTP API 或 gRPC 管理接口，允许运行时修改配置：

```go
func (s *Server) UpdateConfig(newConfig *snapshot.XDSConfig) error {
    version := snapshotVersion.Add(1)
    versionStr := strconv.FormatUint(version, 10)
    builder := snapshot.NewBuilder(versionStr)
    snap, err := builder.BuildSnapshot(newConfig)
    if err != nil {
        return err
    }
    return s.cache.SetSnapshot(context.Background(), "", snap)
}
```

### Q: 控制平面如何处理并发请求？

A: 使用 Envoy 官方 `cache.SnapshotCache`，它内部已经处理了并发访问和版本一致性。

### Q: 如何实现配置持久化？

A: 当前使用 YAML 文件配置，可以扩展为：

1. **数据库存储**: 使用 SQLite、PostgreSQL 等数据库
2. **配置中心**: 集成 Consul、Etcd、ZooKeeper
3. **GitOps**: 使用 Git 管理配置版本

### Q: 如何实现多租户支持？

A: 在 SnapshotCache 中使用 Node ID 作为键：

```go
snapshotCache.SetSnapshot(context.Background(), nodeID, snap)
```

不同租户使用不同的 Node ID 连接，可以获取不同的配置。

## 性能优化建议

1. **批量更新**: 多个配置变更合并为一次快照更新
2. **增量更新**: 使用 Delta xDS 协议减少网络传输
3. **缓存预热**: 启动时预加载常用配置
4. **连接池**: 使用连接池复用 gRPC 连接
5. **异步处理**: 使用独立 goroutine 处理文件监控

## 监控和可观测性

### 添加指标导出

可以集成 Prometheus 或 OpenTelemetry：

```go
import "github.com/prometheus/client_golang"

var (
    requestCount = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "xds_requests_total",
            Help: "Total xDS requests",
        },
        []string{"type", "node_id"},
    )
)
```

### 日志级别

配置日志级别：

```yaml
yggdrasil:
  logger:
    handler:
      default:
        type: "console"
        level: "debug"  # debug, info, warn, error
```

## 版本兼容性

本实现基于 Envoy v3 API，兼容以下版本：
- Envoy 1.18+
- go-control-plane v0.14.0+

## 扩展阅读

- [Envoy xDS v3 API](https://www.envoyproxy.io/docs/envoy/latest/api-docs/xds_protocol)
- [go-control-plane](https://github.com/envoyproxy/go-control-plane)
- [xDS Protocol](https://www.envoyproxy.io/docs/envoy/latest/api-docs/xds_protocol)
