# 流量分割示例

本示例演示如何使用xDS实现流量分割，将流量按权重分配到不同的后端服务。

## 目录结构

```
traffic-splitting/
├── client/
│   ├── main.go      # 客户端实现，发送请求并统计流量分布
│   └── config.yaml  # 客户端xDS配置
└── server/
    ├── main.go      # 服务端实现，支持多个后端
    └── config.yaml  # 服务端配置
```

## 功能特性

- **流量分割**: 按权重将流量分配到不同后端
- **实时统计**: 客户端统计每个后端的流量分布
- **灵活配置**: 通过xDS配置动态调整权重
- **多后端支持**: 支持两个或更多后端服务

## 运行步骤

### 1. 启动xDS控制平面

```bash
cd contrib/xds/example/control-plane
set XDS_CONFIG_FILE=traffic-splitting-xds-config.yaml
go run main.go --config config.yaml
```

### 2. 启动后端服务1

```bash
cd contrib/xds/example/traffic-splitting/server
set BACKEND_ID=1
go run main.go --config config.yaml
```

### 3. 启动后端服务2

打开新终端：

```bash
cd contrib/xds/example/traffic-splitting/server
set BACKEND_ID=2
set PORT=55556
go run main.go --config config.yaml
```

### 4. 运行客户端

```bash
cd contrib/xds/example/traffic-splitting/client
go run main.go --config config.yaml
```

## 预期输出

客户端将发送20个请求并统计每个后端的流量分布：

```
2025/01/26 10:00:00 INFO Starting traffic splitting client...
2025/01/26 10:00:00 INFO Starting traffic splitting test...
2025/01/26 10:00:00 INFO GetShelf response - index: 0, name: shelves/1, theme: backend-1, server: backend-1
2025/01/26 10:00:00 INFO GetShelf response - index: 1, name: shelves/1, theme: backend-2, server: backend-2
2025/01/26 10:00:00 INFO GetShelf response - index: 2, name: shelves/1, theme: backend-1, server: backend-1
2025/01/26 10:00:00 INFO GetShelf response - index: 3, name: shelves/1, theme: backend-2, server: backend-2
...
2025/01/26 10:00:00 INFO Traffic splitting test completed - total_requests: 20
2025/01/26 10:00:00 INFO Traffic Distribution:
2025/01/26 10:00:00 INFO Backend - backend_id: backend-1, requests: 16, percentage: 80.00
2025/01/26 10:00:00 INFO Backend - backend_id: backend-2, requests: 4, percentage: 20.00
```

## 流量分割场景

### 场景1：A/B测试

- **目的**: 比较不同版本的性能
- **比例**: 50% / 50%
- **示例**: 测试新算法vs旧算法

### 场景2：灰度发布

- **目的**: 逐步推出新功能
- **比例**: 90% / 10% → 80% / 20% → 50% / 50% → 0% / 100%
- **示例**: 新功能灰度发布

### 场景3：容量管理

- **目的**: 根据后端容量分配流量
- **比例**: 根据实际容量设置
- **示例**: 高性能后端处理更多流量

### 场景4：多租户

- **目的**: 不同租户使用不同后端
- **比例**: 根据租户需求
- **示例**: VIP租户使用专用后端

## 配置说明

### xDS配置 (traffic-splitting-xds-config.yaml)

```yaml
endpoints:
  - clusterName: "library-cluster"
    endpoints:
      - address: "127.0.0.1"
        port: 55555
        weight: 80      # 后端1权重80%
      - address: "127.0.0.1"
        port: 55556
        weight: 20      # 后端2权重20%
```

通过修改权重，可以动态调整流量分配比例。

## 动态调整流量

### 调整为70/30

修改`traffic-splitting-xds-config.yaml`:

```yaml
endpoints:
  - clusterName: "library-cluster"
    endpoints:
      - address: "127.0.0.1"
        port: 55555
        weight: 70
      - address: "127.0.0.1"
        port: 55556
        weight: 30
```

控制平面会自动检测文件变化并重新加载配置，客户端会立即应用新的流量分配。

### 调整为100/0（完全切换到后端1）

```yaml
endpoints:
  - clusterName: "library-cluster"
    endpoints:
      - address: "127.0.0.1"
        port: 55555
        weight: 100
      - address: "127.0.0.1"
        port: 55556
        weight: 0
```

## 最佳实践

### 1. 监控指标

- **流量分布**: 实时监控各后端的实际流量比例
- **错误率**: 每个后端的错误率不应超过阈值
- **响应时间**: 各后端的P95、P99延迟
- **资源使用**: CPU、内存、网络使用量

### 2. 流量切换策略

- **渐进式切换**: 分多个阶段逐步调整权重
- **监控验证**: 每次调整后验证系统稳定性
- **快速回滚**: 发现问题立即调整回原权重

### 3. 后端健康检查

- **主动健康检查**: 定期检查后端健康状态
- **故障隔离**: 故障后端自动从流量分配中移除
- **故障恢复**: 健康后端自动重新加入流量分配

### 4. 权重计算

- **基于容量**: weight ∝ capacity
- **基于性能**: weight ∝ 1/response_time
- **综合评估**: weight = f(capacity, performance, cost)

## 与金丝雀部署的区别

| 特性 | 金丝雀部署 | 流量分割 |
|------|-----------|----------|
| 目的 | 安全发布新版本 | 分配流量到不同后端 |
| 流量比例 | 逐步增加 | 固定或动态调整 |
| 后端版本 | 稳定版 + 新版本 | 可以是不同版本 |
| 回滚 | 简单（切换回100%） | 简单（调整权重） |
| 适用场景 | 版本升级 | A/B测试、容量管理等 |

## 常见问题

Q: 流量分割和金丝雀部署有什么区别？
A: 金丝雀部署关注版本升级的安全性，流量分割关注流量分配的灵活性。金丝雀是流量分割的一种特殊应用。

Q: 如何确保流量分配精确？
A: 使用加权轮询算法，大量请求下会接近理论比例。小样本会有一定偏差。

Q: 可以同时分割到3个或更多后端吗？
A: 可以，只需在xDS配置中添加更多endpoint并设置权重。

Q: 权重之和必须为100吗？
A: 不需要，系统会计算每个权重占总权重的比例。

Q: 如何动态调整流量比例？
A: 修改xDS配置文件中的权重，控制平面会自动重新加载，客户端立即生效。

Q: 如果某个后端宕机会怎样？
A: 健康的后端会按权重比例重新分配流量，故障后端被自动排除。

Q: 流量分割会影响性能吗？
A: 几乎不影响。权重计算和选择是O(1)复杂度，开销极小。
