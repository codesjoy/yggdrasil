# 负载均衡示例

本示例演示如何使用xDS实现不同的负载均衡策略，包括轮询、随机和最少请求。

## 目录结构

```
load-balancing/
├── client/
│   ├── main.go      # 客户端实现，发送请求并统计流量分布
│   └── config.yaml  # 客户端xDS配置
└── server/
    ├── main.go      # 服务端实现，多个实例
    └── config.yaml  # 服务端配置
```

## 功能特性

- **多实例支持**: 启动多个服务实例测试负载均衡
- **流量统计**: 客户端统计每个服务实例的请求分布
- **多种策略**: 支持round_robin、random、least_request等策略
- **实时监控**: 输出每个服务实例的请求统计

## 运行步骤

### 1. 启动xDS控制平面

```bash
cd contrib/xds/example/control-plane
set XDS_CONFIG_FILE=load-balancing-xds-config.yaml
go run main.go --config config.yaml
```

### 2. 启动多个服务实例

打开多个终端窗口，分别启动不同的服务实例：

```bash
# 终端1 - 服务实例1
cd contrib/xds/example/load-balancing/server
set SERVER_ID=1
go run main.go --config config.yaml

# 终端2 - 服务实例2
cd contrib/xds/example/load-balancing/server
set SERVER_ID=2
set PORT=55556
go run main.go --config config.yaml

# 终端3 - 服务实例3
cd contrib/xds/example/load-balancing/server
set SERVER_ID=3
set PORT=55557
go run main.go --config config.yaml
```

### 3. 运行客户端

```bash
cd contrib/xds/example/load-balancing/client
go run main.go --config config.yaml
```

## 预期输出

客户端将发送30个请求并统计每个服务实例的流量分布：

### Round Robin (轮询) 策略

```
2025/01/26 10:00:00 INFO Starting load balancing client...
2025/01/26 10:00:00 INFO Starting load balancing test...
2025/01/26 10:00:00 INFO GetShelf response - index: 0, name: shelves/1, theme: server-1, server: server-1
2025/01/26 10:00:00 INFO GetShelf response - index: 1, name: shelves/1, theme: server-2, server: server-2
2025/01/26 10:00:00 INFO GetShelf response - index: 2, name: shelves/1, theme: server-3, server: server-3
2025/01/26 10:00:00 INFO GetShelf response - index: 3, name: shelves/1, theme: server-1, server: server-1
...
2025/01/26 10:00:00 INFO Load balancing test completed - total_requests: 30
2025/01/26 10:00:00 INFO Traffic Distribution:
2025/01/26 10:00:00 INFO Server - server_id: server-1, requests: 10, percentage: 33.333
2025/01/26 10:00:00 INFO Server - server_id: server-2, requests: 10, percentage: 33.333
2025/01/26 10:00:00 INFO Server - server_id: server-3, requests: 10, percentage: 33.333
```

## 负载均衡策略

### 1. Round Robin (轮询)

**特点**:
- 按顺序轮流分配请求
- 每个实例获得大致相等的流量
- 不考虑实例的当前负载

**适用场景**:
- 所有实例性能相似
- 请求处理时间相近
- 需要公平分配流量

**实现原理**:
```go
func (lb *roundRobinBalancer) Pick() *endpoint {
    index := lb.current
    lb.current = (lb.current + 1) % len(lb.endpoints)
    return lb.endpoints[index]
}
```

### 2. Random (随机)

**特点**:
- 随机选择一个实例
- 在大量请求下趋于均匀分布
- 实现简单

**适用场景**:
- 实例数量较多
- 不需要严格保证均匀分布
- 对公平性要求不高

**实现原理**:
```go
func (lb *randomBalancer) Pick() *endpoint {
    index := rand.Intn(len(lb.endpoints))
    return lb.endpoints[index]
}
```

### 3. Least Request (最少请求)

**特点**:
- 选择当前请求数最少的实例
- 动态适应实例负载
- 适合处理时间差异大的场景

**适用场景**:
- 请求处理时间差异大
- 部分实例性能较强
- 需要动态负载均衡

**实现原理**:
```go
func (lb *leastRequestBalancer) Pick() *endpoint {
    var selected *endpoint
    minRequests := int64(^uint64(0) >> 1)
    
    for _, ep := range lb.endpoints {
        if ep.activeRequests < minRequests {
            minRequests = ep.activeRequests
            selected = ep
        }
    }
    
    return selected
}
```

## 技术要点

### 1. 服务实例标识

服务端通过环境变量标识实例ID：

```go
serverID := os.Getenv("SERVER_ID")
if serverID == "" {
    serverID = "1"
}
```

### 2. Metadata传递

服务端在响应中添加实例标识：

```go
func (s *LibraryImpl) GetShelf(
    ctx context.Context,
    req *librarypb2.GetShelfRequest,
) (*librarypb2.Shelf, error) {
    _ = metadata.SetTrailer(ctx, metadata.Pairs("server", s.serverID))
    _ = metadata.SetHeader(ctx, metadata.Pairs("server", s.serverID))
    return &librarypb2.Shelf{
        Name:  req.Name,
        Theme: "server-" + s.serverID,
    }, nil
}
```

### 3. 流量统计

客户端统计每个实例的请求数：

```go
serverCounts := make(map[string]int)
var mu sync.Mutex

for i := 0; i < requestCount; i++ {
    // ... 发送请求 ...
    
    mu.Lock()
    serverCounts[serverID]++
    mu.Unlock()
}
```

## 配置说明

### xDS配置 (load-balancing-xds-config.yaml)

```yaml
endpoints:
  - clusterName: "library-cluster"
    endpoints:
      - address: "127.0.0.1"
        port: 55555
        weight: 1
      - address: "127.0.0.1"
        port: 55556
        weight: 1
      - address: "127.0.0.1"
        port: 55557
        weight: 1
```

三个实例权重相同，将均匀分配流量。

## 测试不同负载均衡策略

### 测试Round Robin

修改control-plane/load-balancing-xds-config.yaml:

```yaml
lbPolicy: "ROUND_ROBIN"
```

运行客户端，观察请求分布是否均匀（33.33%每个实例）。

### 测试Random

修改control-plane/load-balancing-xds-config.yaml:

```yaml
lbPolicy: "RANDOM"
```

运行客户端多次，观察每次分布是否大致均匀。

### 测试Least Request

修改control-plane/load-balancing-xds-config.yaml:

```yaml
lbPolicy: "LEAST_REQUEST"
```

需要模拟不同实例的负载差异才能看到效果。

## 性能对比

| 策略 | 复杂度 | 均匀性 | 适应性 | 开销 |
|------|--------|--------|--------|------|
| Round Robin | O(1) | 高 | 低 | 低 |
| Random | O(1) | 中 | 低 | 极低 |
| Least Request | O(n) | 高 | 高 | 中 |

## 常见问题

Q: 如何选择合适的负载均衡策略？
A: 根据业务场景选择：
- 请求处理时间相似 → Round Robin
- 实例数量多且性能相似 → Random
- 请求处理时间差异大 → Least Request

Q: Least Request策略如何追踪请求数？
A: 每个endpoint维护activeRequests计数器，请求开始时+1，结束时-1。

Q: 可以动态切换负载均衡策略吗？
A: 可以，通过更新xDS配置的lb_policy字段，客户端会自动应用新策略。

Q: 负载均衡策略在客户端还是服务端实现？
A: 在客户端实现，xDS只提供endpoint列表和策略配置，实际的负载均衡由客户端执行。

Q: 如何验证负载均衡是否生效？
A: 通过客户端输出的流量分布统计，观察请求是否按预期分配到各个实例。

Q: 如果某个实例宕机怎么办？
A: xDS控制平面会从endpoint列表中移除故障实例，客户端会自动将流量分配到其他健康实例。
