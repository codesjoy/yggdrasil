# 服务注册中心示例

本示例演示如何使用 etcd 作为注册中心，将服务实例注册到 etcd，并通过 lease + keepalive 机制维持心跳，确保实例在线状态。

## 你会得到什么

- 将服务实例注册到 etcd
- 自动心跳续约，保持实例在线
- 支持多协议端点（如 gRPC、HTTP）
- 优雅下线：服务停止时自动反注册
- 元信息管理：记录实例的 region、zone、campus 等信息

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
  registry:
    config:
      client:
        endpoints:
          - "your-etcd-endpoint:2379"
```

### 2. 运行示例

```bash
cd contrib/etcd/example/registry
go run server.go
```

## 预期输出

```
2024/01/26 10:00:00 [server] listening on 127.0.0.1:54321
2024/01/26 10:00:00 [registry] instance registered successfully
2024/01/26 10:00:00 [registry] service: default/example-registry-server/1.0.0
2024/01/26 10:00:00 [registry] endpoints: 2
2024/01/26 10:00:00   - grpc://127.0.0.1:54321
2024/01/26 10:00:00   - http://127.0.0.1:54321
2024/01/26 10:00:00 [server] running, press Ctrl+C to shutdown
2024/01/26 10:00:30 [registry] instance re-registered
...
```

## 验证注册

使用 `etcdctl` 查看注册的实例：

```bash
# 查看所有注册的实例
etcdctl --endpoints=127.0.0.1:2379 get /yggdrasil/registry --prefix

# 查看特定服务的实例
etcdctl --endpoints=127.0.0.1:2379 get /yggdrasil/registry/default/example-registry-server --prefix

# 格式化输出
etcdctl --endpoints=127.0.0.1:2379 get /yggdrasil/registry --prefix -w json
```

预期输出（格式化后）：
```json
{
  "namespace": "default",
  "name": "example-registry-server",
  "version": "1.0.0",
  "region": "us-west",
  "zone": "us-west-1",
  "campus": "campus-a",
  "metadata": {
    "env": "dev",
    "pod": "pod-1234567890",
    "started": "2024-01-26T10:00:00Z"
  },
  "endpoints": [
    {
      "scheme": "grpc",
      "address": "127.0.0.1:54321",
      "metadata": null
    },
    {
      "scheme": "http",
      "address": "127.0.0.1:54321",
      "metadata": null
    }
  ]
}
```

## 配置说明

### Key 布局

实例在 etcd 中的 key 布局：

```
<yggdrasil/registry>/<namespace>/<service>/<instanceKey(hash)>
```

示例：
```
/yggdrasil/registry/default/example-registry-server/a1b2c3d4e5f6...
```

### 注册中心配置

| 参数 | 说明 | 默认值 |
|------|------|--------|
| `client.endpoints` | etcd 地址列表 | `["127.0.0.1:2379"]` |
| `client.dialTimeout` | 连接超时 | `5s` |
| `prefix` | 注册前缀 | `/yggdrasil/registry` |
| `ttl` | 租约 TTL | `10s` |
| `keepAlive` | 是否启用心跳续约 | `true` |
| `retryInterval` | 失败重试间隔 | `3s` |

### Instance 接口

```go
type Instance interface {
    Namespace() string      // 命名空间
    Name() string          // 服务名
    Version() string       // 版本号
    Region() string        // 区域
    Zone() string          // 可用区
    Campus() string        // 园区
    Metadata() map[string]string  // 元信息
    Endpoints() []Endpoint  // 端点列表
}
```

### Endpoint 接口

```go
type Endpoint interface {
    Scheme() string              // 协议：grpc、http、tcp 等
    Address() string             // 地址：ip:port
    Metadata() map[string]string // 端点元信息
}
```

## 工作原理

### 注册流程

```
1. 创建 Instance 对象
   ↓
2. 调用 reg.Register(context, inst)
   ↓
3. 生成实例 key（基于 namespace、name、version、endpoints）
   ↓
4. 创建 etcd lease（TTL）
   ↓
5. 将实例信息写入 etcd（使用 lease）
   ↓
6. 启动 keepalive goroutine
   ↓
7. 定期发送 keepalive 保持 lease 有效
```

### KeepAlive 机制

- **Lease TTL**：租约过期时间（默认 10s）
- **KeepAlive 间隔**：自动由 etcd 客户端管理（通常 TTL/3）
- **失败重试**：如果 keepalive 失败，会重新创建 lease 并注册
- **自动恢复**：网络恢复后自动恢复心跳

### 反注册流程

```
1. 收到停止信号
   ↓
2. 调用 reg.Deregister(context, inst)
   ↓
3. 删除 etcd 中的实例 key
   ↓
4. 关闭 keepalive goroutine
   ↓
5. 优雅退出
```

## 代码结构说明

```go
// 1. 初始化 Yggdrasil 框架
if err := yggdrasil.Init(appName); err != nil {
    log.Fatalf("yggdrasil.Init: %v", err)
}

// 2. 获取注册中心
reg, err := registry.Get()
if err != nil {
    log.Fatalf("registry.Get: %v", err)
}

// 3. 创建实例对象
inst := demoInstance{
    namespace: "default",
    name:      appName,
    version:   "1.0.0",
    region:    "us-west",
    zone:      "us-west-1",
    campus:    "campus-a",
    metadata: map[string]string{
        "env": "dev",
        "pod": "pod-1234567890",
    },
    endpoints: []registry.Endpoint{
        demoEndpoint{scheme: "grpc", address: addr},
        demoEndpoint{scheme: "http", address: addr},
    },
}

// 4. 注册实例
if err := reg.Register(context.Background(), inst); err != nil {
    log.Fatalf("Register: %v", err)
}

// 5. 反注册实例（优雅下线）
if err := reg.Deregister(context.Background(), inst); err != nil {
    log.Printf("[registry] deregister failed: %v", err)
}
```

## 高级用法

### 多实例注册

一个服务可以注册多个实例（多网卡、多端口）：

```go
inst := demoInstance{
    name: "my-service",
    endpoints: []registry.Endpoint{
        demoEndpoint{scheme: "grpc", address: "10.0.0.1:8080"},
        demoEndpoint{scheme: "grpc", address: "10.0.0.2:8080"},
        demoEndpoint{scheme: "http", address: "10.0.0.1:8081"},
    },
}
```

### 动态更新元信息

定期更新实例元信息：

```go
go func() {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()

    for range ticker.C {
        inst.metadata["heartbeat"] = time.Now().Format(time.RFC3339)
        inst.metadata["load"] = fmt.Sprintf("%.2f", getLoadAverage())
        if err := reg.Register(context.Background(), inst); err != nil {
            log.Printf("re-register failed: %v", err)
        }
    }
}()
```

### 使用命名空间隔离

不同环境使用不同的命名空间：

```go
// 开发环境
devInst := demoInstance{
    namespace: "dev",
    name:      "my-service",
    version:   "1.0.0",
}

// 生产环境
prodInst := demoInstance{
    namespace: "prod",
    name:      "my-service",
    version:   "1.0.0",
}
```

### 自定义实例 Key

默认情况下，实例 key 基于实例内容的 SHA1 哈希生成。你也可以通过 metadata 自定义实例标识：

```go
inst := demoInstance{
    metadata: map[string]string{
        "instance_id": "my-custom-instance-id",
    },
}
```

## 常见问题

**Q: 为什么实例在 etcd 中消失了？**

A: 可能的原因：
1. keepAlive 失败，检查网络连接和 etcd 状态
2. TTL 设置过短，建议设置为 10-30s
3. 服务进程崩溃，未执行反注册

**Q: 如何设置合理的 TTL？**

A: TTL 的选择建议：
- 开发环境：10-15s（快速发现问题）
- 生产环境：30-60s（减少网络开销）
- 高可用要求：10-20s（快速故障转移）

**Q: 多实例如何区分？**

A: 每个实例有唯一的 key（基于实例内容哈希）：
- 不同端口：生成不同实例
- 不同 IP：生成不同实例
- 不同版本：生成不同实例

**Q: 如何监控注册状态？**

A: 建议监控以下指标：
1. 注册成功/失败次数
2. keepAlive 成功/失败次数
3. 实例数量变化
4. etcd 连接状态

**Q: 支持 TLS 连接吗？**

A: 支持，在配置中添加 TLS 证书：
```yaml
client:
  tls:
    certFile: "/path/to/cert.pem"
    keyFile: "/path/to/key.pem"
    caFile: "/path/to/ca.pem"
```

## 最佳实践

1. **合理设置 TTL**：根据业务需求选择合适的 TTL，平衡性能和可用性
2. **使用命名空间**：不同环境使用不同的命名空间，避免混淆
3. **记录元信息**：在 metadata 中记录有用的信息（版本、部署时间、负载等）
4. **优雅下线**：确保在服务停止前执行反注册
5. **监控注册状态**：监控注册/反注册和 keepAlive 状态
6. **使用多协议端点**：如果服务支持多种协议，建议都注册

## 相关文档

- [etcd 主文档](../../../readme.md)
- [服务发现示例](../resolver/)
- [Blob 模式示例](../config-source/blob/)
- [KV 模式示例](../config-source/kv/)
