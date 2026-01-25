# etcd 集成

本插件将 etcd 作为配置中心、注册中心与发现服务接入 Yggdrasil 框架。

## 架构概览

etcd contrib 模块提供三个核心能力：

1. **配置源** - 从 etcd 读取应用配置，支持 blob 和 kv 两种模式
2. **注册中心** - 将服务实例注册到 etcd，使用 lease 机制维持心跳
3. **服务发现** - 从 etcd 发现服务实例，支持 watch 实时更新

```
┌─────────────────┐         ┌─────────────────┐
│   Application   │         │   Application   │
│   (Server)      │         │   (Client)      │
└────────┬────────┘         └────────┬────────┘
         │                           │
         │ Register                  │ Discover/Watch
         ▼                           ▼
┌─────────────────────────────────────────────┐
│                  etcd                       │
│  /config/app   (配置源)                      │
│  /registry/... (注册中心/发现)               │
└─────────────────────────────────────────────┘
```

## 快速开始

### 1. 启用插件

在业务 main 中空导入一次：

```go
import _ "github.com/codesjoy/yggdrasil/contrib/etcd/v2"
```

### 2. 本地启动 etcd（开发测试）

```bash
docker run -d --name etcd \
  -p 2379:2379 \
  -p 2380:2380 \
  -e ALLOW_NONE_AUTHENTICATION=yes \
  bitnami/etcd:latest
```

- HTTP API: `http://127.0.0.1:2379`
- 验证：`etcdctl --endpoints=127.0.0.1:2379 endpoint health`

### 3. 配置 etcd 连接

在 `config.yaml` 中配置 etcd 客户端：

```yaml
etcd:
  client:
    endpoints: ["127.0.0.1:2379"]
    dialTimeout: 5s
    username: ""
    password: ""
```

更多示例见 [example/](example/) 目录。

## 功能说明

### 配置中心

#### 模式说明

- **blob 模式**：单个 key 存储整份配置（yaml/json/toml）
- **kv 模式**：prefix 下多 key，按路径映射成层级配置

#### 配置结构

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `client.endpoints` | []string | `["127.0.0.1:2379"]` | etcd 地址列表 |
| `client.dialTimeout` | duration | `5s` | 连接超时 |
| `client.username` | string | 空 | 用户名（可选） |
| `client.password` | string | 空 | 密码（可选） |
| `client.tls.certFile` | string | 空 | TLS 证书文件路径 |
| `client.tls.keyFile` | string | 空 | TLS 私钥文件路径 |
| `client.tls.caFile` | string | 空 | TLS CA 证书文件路径 |
| `mode` | string | 自动推断 | `blob` 或 `kv` |
| `key` | string | 空 | blob 模式下的配置 key |
| `prefix` | string | 空 | kv 模式下的配置前缀 |
| `watch` | bool | `true` | 是否监听配置变更 |
| `format` | string | `yaml` | 配置格式：`yaml`/`json`/`toml` |
| `name` | string | 自动推断 | 配置源名称 |

#### 使用方式

```go
import "github.com/codesjoy/yggdrasil/v2/config"
import "github.com/codesjoy/yggdrasil/contrib/etcd/v2"

var cfg etcd.ConfigSourceConfig
_ = config.Get("etcd.configSource").Scan(&cfg)
src, err := etcd.NewConfigSource(cfg)
if err != nil { ... }
_ = config.LoadSource(src)

// 监听配置变更
_ = config.AddWatcher("", func(ev config.WatchEvent) {
    if ev.Type() == config.WatchEventUpd || ev.Type() == config.WatchEventAdd {
        data := string(ev.Value().Bytes())
        log.Printf("config updated: %s", data)
    }
})
```

### 注册中心

#### 工作原理

- 使用 etcd 的 lease + keepalive 机制维持实例注册
- key 布局：`<prefix>/<namespace>/<service>/<instanceKey(hash)>`
- 支持自动心跳续约，服务停止时自动反注册

#### 配置结构

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `client.endpoints` | []string | `["127.0.0.1:2379"]` | etcd 地址列表 |
| `client.dialTimeout` | duration | `5s` | 连接超时 |
| `prefix` | string | `/yggdrasil/registry` | 注册前缀 |
| `ttl` | duration | `10s` | 租约 TTL |
| `keepAlive` | bool | `true` | 是否启用心跳续约 |
| `retryInterval` | duration | `3s` | 失败重试间隔 |

#### 使用方式

```yaml
yggdrasil:
  registry:
    type: etcd
    config:
      client:
        endpoints: ["127.0.0.1:2379"]
        dialTimeout: 5s
      prefix: /yggdrasil/registry
      ttl: 10s
      keepAlive: true
      retryInterval: 3s
```

```go
import "github.com/codesjoy/yggdrasil/v2/registry"

reg, err := registry.Get()
if err != nil { ... }

inst := &myInstance{
    namespace: "default",
    name:      "my-service",
    version:   "1.0.0",
    endpoints: []registry.Endpoint{
        &myEndpoint{scheme: "grpc", address: "127.0.0.1:9000"},
    },
}

if err := reg.Register(context.Background(), inst); err != nil {
    log.Fatalf("register failed: %v", err)
}
```

### 服务发现

#### 工作原理

- 按 service 维度 watch prefix，事件触发后"快照刷新"并 `UpdateState`
- 支持可选 `protocols` 过滤与 `debounce` 降噪
- 自动过滤不匹配 namespace 和 protocol 的实例

#### 配置结构

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `client.endpoints` | []string | `["127.0.0.1:2379"]` | etcd 地址列表 |
| `client.dialTimeout` | duration | `5s` | 连接超时 |
| `prefix` | string | `/yggdrasil/registry` | 注册前缀 |
| `namespace` | string | `default` | 命名空间 |
| `protocols` | []string | `["grpc", "http"]` | 支持的协议列表 |
| `debounce` | duration | `200ms` | 防抖延迟 |

#### 使用方式

```yaml
yggdrasil:
  resolver:
    default:
      type: etcd
      config:
        client:
          endpoints: ["127.0.0.1:2379"]
          dialTimeout: 5s
        prefix: /yggdrasil/registry
        namespace: default
        protocols: ["grpc", "http"]
        debounce: 200ms

  client:
    myApp:
      resolver: default
```

```go
import "github.com/codesjoy/yggdrasil/v2/resolver"

res, err := resolver.Get("default")
if err != nil { ... }

stateCh := make(chan resolver.State, 1)
res.AddWatch("my-service", &myClient{stateCh: stateCh})

go func() {
    for st := range stateCh {
        eps := st.GetEndpoints()
        log.Printf("discovered %d endpoints", len(eps))
        for _, ep := range eps {
            log.Printf("  - %s://%s", ep.GetProtocol(), ep.GetAddress())
        }
    }
}()
```

## 示例程序

| 示例 | 说明 |
|------|------|
| [allinone](example/allinone/) | 完整示例：配置源 + 注册中心 + 服务发现 |
| [config-source/blob](example/config-source/blob/) | 配置源 blob 模式 |
| [config-source/kv](example/config-source/kv/) | 配置源 kv 模式 |
| [registry](example/registry/) | 服务注册中心 |
| [resolver](example/resolver/) | 服务发现 |

详细说明见各示例目录下的 README.md。

## 故障排查

### 连接问题

```bash
# 检查 etcd 是否运行
etcdctl --endpoints=127.0.0.1:2379 endpoint health

# 检查配置是否写入
etcdctl --endpoints=127.0.0.1:2379 get /config/app --prefix

# 检查服务注册
etcdctl --endpoints=127.0.0.1:2379 get /yggdrasil/registry --prefix
```

### 配置未更新

1. 确认 `watch: true` 已配置
2. 检查 `AddWatcher` 是否正确注册
3. 查看日志是否有配置更新事件

### 服务未发现

1. 确认 server 和 client 使用相同的 `namespace`
2. 确认 server 注册的 instance protocol 与 client 配置的 `protocols` 匹配
3. 检查 `prefix` 配置是否一致
4. 使用 etcdctl 验证注册数据：
   ```bash
   etcdctl --endpoints=127.0.0.1:2379 get /yggdrasil/registry --prefix
   ```

### KeepAlive 失败

1. 检查 `ttl` 是否设置合理（建议 10-30s）
2. 检查 `retryInterval` 是否过短（建议 3-5s）
3. 检查 etcd 连接是否稳定
4. 查看 lease 状态：
   ```bash
   etcdctl --endpoints=127.0.0.1:2379 lease list
   ```

### Debug 日志

启用 debug 日志查看详细信息：

```yaml
yggdrasil:
  logger:
    level: debug
```

关键日志标识：
- `[etcd] client connected` - etcd 客户端连接成功
- `[config] updated` - 配置更新事件
- `[registry] instance registered` - 服务注册成功
- `[resolver] endpoints` - 服务发现更新

## 性能考虑

### 配置源

- **blob 模式**：适合配置较大但更新不频繁的场景
- **kv 模式**：适合配置较小且需要细粒度更新的场景
- **watch 性能**：etcd watch 流式推送，性能优秀

### 注册中心

- **TTL 设置**：过短会增加网络开销，过长会导致故障发现延迟
- **KeepAlive**：使用 etcd 原生 keepalive 机制，高效可靠
- **并发注册**：支持多实例并发注册

### 服务发现

- **Debounce**：避免频繁更新导致的抖动，建议 200-500ms
- **协议过滤**：减少不必要的实例传递
- **快照刷新**：每次事件触发后全量刷新，保证一致性

### 推荐配置

```yaml
# 生产环境推荐配置
etcd:
  client:
    endpoints: ["etcd-1:2379", "etcd-2:2379", "etcd-3:2379"]  # 集群
    dialTimeout: 5s

  configSource:
    client: ${etcd.client}
    watch: true
    debounce: 500ms

registry:
  type: etcd
  config:
    client: ${etcd.client}
    ttl: 30s
    keepAlive: true
    retryInterval: 5s

resolver:
  default:
    type: etcd
    config:
      client: ${etcd.client}
      debounce: 300ms
```

## 限制与注意事项

1. **etcd 版本**：要求 etcd >= 3.4.0
2. **配置大小**：单 key 建议不超过 1.5MB
3. **连接数**：每个组件（config/registry/resolver）维护独立连接
4. **Watch 限制**：etcd 对 watch 数量有限制，大量服务时注意控制

## 最佳实践

1. **集群部署**：生产环境使用 etcd 集群（3 或 5 节点）
2. **TLS 加密**：生产环境启用 TLS 加密通信
3. **认证授权**：使用用户名密码或证书认证
4. **命名空间**：使用 namespace 隔离不同环境
5. **版本管理**：在 instance metadata 中记录版本信息
6. **优雅下线**：确保 Deregister 在服务停止前执行
7. **监控告警**：监控 etcd 连接状态和注册/发现延迟

## 参考

- [etcd 官方文档](https://etcd.io/docs/latest/)
- [Yggdrasil 框架文档](../../README.md)
- [其他 contrib 模块](../)
