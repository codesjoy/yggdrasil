# 服务发现示例

本示例演示如何使用 etcd 作为服务发现中心，从 etcd 发现服务实例，并实时监听实例变更，驱动负载均衡。

## 你会得到什么

- 从 etcd 发现指定服务的所有实例
- 实时监听实例变更（新增、删除、更新）
- 自动过滤不匹配 namespace 和 protocol 的实例
- Debounce 机制，避免频繁更新导致的抖动
- 实时演示：每 5 秒注册新实例，观察服务发现效果

## 前置条件

1. 可访问的 etcd 集群（默认 `127.0.0.1:2379`）
2. 已安装 Go 1.19+

### 快速启动 etcd

```bash
docker run -d --name etcd \
  -p 2379:2379 \
  -p 2380:2380 \
  -e ALLOW_NONE_AUTHENTICATION=yes \
  bitnami/etcd:latest
```

验证 etcd 是否正常运行：
```bash
etcdctl --endpoints=127.0.0.1:2379 endpoint health
```

## 启动方式

### 1. 修改配置（可选）

如果 etcd 地址不是默认的 `127.0.0.1:2379`，请修改 [config.yaml](config.yaml)：

```yaml
yggdrasil:
  resolver:
    default:
      config:
        client:
          endpoints:
            - "your-etcd-endpoint:2379"
```

### 2. 运行示例

```bash
cd contrib/etcd/example/resolver
go run client.go
```

## 预期输出

```
2024/01/26 10:00:00 [resolver] watching service: example-registry-server
2024/01/26 10:00:00 [client] running, press Ctrl+C to exit
2024/01/26 10:00:05 [registry] registered instance 1 (grpc://127.0.0.1:9001)
2024/01/26 10:00:05 [resolver] state updated
2024/01/26 10:00:05   service: example-registry-server
2024/01/26 10:00:05   namespace: default
2024/01/26 10:00:05   revision: 123
2024/01/26 10:00:05   endpoints: 2
2024/01/26 10:00:05     - grpc://127.0.0.1:9001
2024/01/26 10:00:05       version: 1.0.0
2024/01/26 10:00:05       region: us-west
2024/01/26 10:00:05       zone: us-west-1
2024/01/26 10:00:05     - http://127.0.0.1:8081
2024/01/26 10:00:10 [registry] registered instance 2 (grpc://127.0.0.1:9002)
2024/01/26 10:00:10 [resolver] state updated
2024/01/26 10:00:10   endpoints: 4
...
```

## 手动添加实例

你也可以手动使用 `etcdctl` 添加实例，观察服务发现效果：

```bash
# 查看当前实例
etcdctl --endpoints=127.0.0.1:2379 get /yggdrasil/registry --prefix

# 手动添加实例
etcdctl put /yggdrasil/registry/default/example-registry-server/abc123 '{
  "namespace": "default",
  "name": "example-registry-server",
  "version": "1.0.0",
  "endpoints": [
    {
      "scheme": "grpc",
      "address": "192.168.1.100:8080"
    }
  ]
}'

# 删除实例
etcdctl del /yggdrasil/registry/default/example-registry-server/abc123
```

## 配置说明

### 服务发现工作原理

```
1. 添加 Watch
   ↓
2. Watch etcd prefix (/yggdrasil/registry/<namespace>/<service>)
   ↓
3. 收到变更事件
   ↓
4. Debounce（防抖）
   ↓
5. 全量拉取当前实例列表
   ↓
6. 过滤不匹配的实例（namespace、protocol）
   ↓
7. 更新 State
   ↓
8. 通知所有 Watcher
```

### Resolver 配置

| 参数 | 说明 | 默认值 |
|------|------|--------|
| `client.endpoints` | etcd 地址列表 | `["127.0.0.1:2379"]` |
| `client.dialTimeout` | 连接超时 | `5s` |
| `prefix` | 注册前缀 | `/yggdrasil/registry` |
| `namespace` | 命名空间 | `default` |
| `protocols` | 支持的协议列表 | `["grpc", "http"]` |
| `debounce` | 防抖延迟 | `200ms` |

### 过滤规则

Resolver 会自动过滤以下实例：

1. **Namespace 不匹配**：实例的 namespace 与配置的 namespace 不一致
2. **Protocol 不匹配**：实例的 endpoint protocol 不在配置的 protocols 列表中
3. **服务名不匹配**：实例的 service name 与 watch 的 service name 不一致

### State 结构

```go
type State interface {
    GetEndpoints() []Endpoint  // 获取所有端点
    GetAttributes() map[string]any  // 获取状态属性
}

type Endpoint interface {
    Name() string              // 端点名称
    Protocol() string         // 协议
    Address() string         // 地址
    GetAttributes() map[string]any  // 端点属性
}
```

## 代码结构说明

```go
// 1. 初始化 Yggdrasil 框架
if err := yggdrasil.Init(appName); err != nil {
    log.Fatalf("yggdrasil.Init: %v", err)
}

// 2. 获取 Resolver
res, err := resolver.Get("default")
if err != nil {
    log.Fatalf("resolver.Get: %v", err)
}

// 3. 创建 Watcher
stateCh := make(chan resolver.State, 10)
watcher := &mockClient{stateCh: stateCh}

// 4. 添加 Watch
serviceName := "example-registry-server"
if err := res.AddWatch(serviceName, watcher); err != nil {
    log.Fatalf("AddWatch: %v", err)
}

// 5. 实现 Watcher 接口
type mockClient struct {
    stateCh chan resolver.State
}

func (m *mockClient) UpdateState(st resolver.State) {
    select {
    case m.stateCh <- st:
    default:
        // channel 满，丢弃更新
    }
}

// 6. 处理 State 更新
go func() {
    for st := range stateCh {
        eps := st.GetEndpoints()
        for _, ep := range eps {
            log.Printf("discovered: %s://%s", ep.GetProtocol(), ep.GetAddress())
        }
    }
}()

// 7. 停止 Watch
res.DelWatch(serviceName, watcher)
```

## 高级用法

### 多服务 Watch

同时监听多个服务：

```go
services := []string{"service-a", "service-b", "service-c"}
for _, svc := range services {
    watcher := &mockClient{stateCh: make(chan resolver.State, 10)}
    if err := res.AddWatch(svc, watcher); err != nil {
        log.Printf("add watch failed for %s: %v", svc, err)
    }
}
```

### 协议过滤

只发现特定协议的端点：

```yaml
yggdrasil:
  resolver:
    default:
      config:
        protocols:
          - grpc  # 只发现 grpc 协议的端点
```

### Namespace 隔离

不同环境使用不同的 namespace：

```yaml
yggdrasil:
  resolver:
    dev:
      config:
        namespace: dev  # 开发环境
    prod:
      config:
        namespace: prod  # 生产环境
```

### Debounce 调整

根据业务需求调整防抖延迟：

```yaml
yggdrasil:
  resolver:
    default:
      config:
        debounce: 500ms  # 更大的防抖延迟，减少更新频率
```

### 与负载均衡集成

Resolver 发现的端点可以直接用于负载均衡：

```go
import "github.com/codesjoy/yggdrasil/v2/balancer"

bal := balancer.Get("round_robin")
if err := res.AddWatch("my-service", bal); err != nil {
    log.Fatalf("add watch failed: %v", err)
}

// 使用负载均衡器选择端点
endpoint, err := bal.Pick(context.Background())
if err != nil {
    log.Printf("pick failed: %v", err)
    return
}
log.Printf("selected: %s://%s", endpoint.Protocol(), endpoint.Address())
```

## 常见问题

**Q: 为什么没有发现任何实例？**

A: 检查以下几点：
1. 确认服务已注册到 etcd（使用 `etcdctl get /yggdrasil/registry --prefix` 查看）
2. 确认 namespace 配置正确
3. 确认 protocol 配置正确
4. 确认服务名称正确

**Q: 实例更新后没有收到通知？**

A: 可能的原因：
1. 实例 key 没有变化（相同内容不会触发更新）
2. debounce 延迟导致通知延迟
3. watcher channel 满，丢弃了更新

**Q: 如何控制更新频率？**

A: 调整 debounce 参数：
- 开发环境：100-200ms（快速响应）
- 生产环境：300-500ms（减少抖动）

**Q: 支持多个 Resolver 吗？**

A: 支持，可以为不同的服务使用不同的 Resolver：
```yaml
yggdrasil:
  resolver:
    default:
      type: etcd
      config:
        namespace: default
    special:
      type: etcd
      config:
        namespace: special
```

**Q: 如何查看当前发现的实例？**

A: 使用 `etcdctl` 查看：
```bash
etcdctl --endpoints=127.0.0.1:2379 get /yggdrasil/registry/default/my-service --prefix
```

## 最佳实践

1. **合理设置 Debounce**：根据业务需求选择合适的防抖延迟
2. **使用 Namespace 隔离**：不同环境使用不同的 namespace
3. **限制 Protocol**：只监听需要的协议，减少不必要的实例传递
4. **处理 Channel 满载**：watcher channel 应该有足够的缓冲，或正确处理满载情况
5. **监控发现状态**：监控实例数量、更新频率等指标
6. **优雅停止**：在应用停止时调用 `DelWatch` 停止监听

## 相关文档

- [etcd 主文档](../../../readme.md)
- [注册中心示例](../registry/)
- [Blob 模式示例](../config-source/blob/)
- [KV 模式示例](../config-source/kv/)
