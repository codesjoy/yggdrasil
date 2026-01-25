# 配置源 KV 模式示例

本示例演示如何使用 etcd 的 KV 模式作为配置源，从多个 etcd key 读取配置片段，并按路径层级映射成结构化配置，支持配置热更新。

## 你会得到什么

- 从 etcd prefix 读取多个配置 key，自动映射为层级结构
- 细粒度配置更新：可以单独更新某个配置项，无需更新整个配置文件
- 配置热更新：当 etcd 中的配置变更时，应用自动收到通知
- 实时演示：每 10 秒自动更新部分配置，展示 KV 模式的优势

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

如果 etcd 地址不是默认的 `127.0.0.1:2379`，请修改 [main.go](main.go) 中的 `Endpoints` 配置：

```go
cli, err := clientv3.New(clientv3.Config{
    Endpoints:   []string{"your-etcd-endpoint:2379"},
    DialTimeout: 5 * time.Second,
})
```

### 2. 运行示例

```bash
cd contrib/etcd/example/config-source/kv
go run main.go
```

## 预期输出

```
2024/01/26 10:00:00 [etcd] initial config written to /example/config/kv/*
2024/01/26 10:00:00 [app] config source initialized, watching for changes...
2024/01/26 10:00:00 [app] press Ctrl+C to update config dynamically
2024/01/26 10:00:00 [app] press Ctrl+C again to exit
2024/01/26 10:00:10 [etcd] partial config updated (count=1)
2024/01/26 10:00:10 [config] server config updated: host=0.0.0.0, port=8081, timeout=30s
2024/01/26 10:00:10 [config] database config updated: host=localhost, port=5432, name=mydb, pool=11
2024/01/26 10:00:10 [config] cache config updated: ttl=360 seconds
2024/01/26 10:00:20 [etcd] partial config updated (count=2)
2024/01/26 10:00:20 [config] server config updated: host=0.0.0.0, port=8082, timeout=30s
2024/01/26 10:00:20 [config] database config updated: host=localhost, port=5432, name=mydb, pool=12
```

## KV 结构说明

### etcd 中的 Key-Value

```
/example/config/kv/server/port        -> "8080"
/example/config/kv/server/host        -> "0.0.0.0"
/example/config/kv/server/readTimeout -> "30s"
/example/config/kv/database/host      -> "localhost"
/example/config/kv/database/port      -> "5432"
/example/config/kv/database/name      -> "mydb"
/example/config/kv/database/poolSize  -> "10"
/example/config/kv/logging/level     -> "info"
/example/config/kv/logging/format    -> "json"
/example/config/kv/cache/redis/host  -> "localhost"
/example/config/kv/cache/redis/port  -> "6379"
/example/config/kv/cache/ttl         -> "300"
```

### 映射后的结构

```yaml
server:
  port: 8080
  host: "0.0.0.0"
  readTimeout: "30s"
database:
  host: "localhost"
  port: 5432
  name: "mydb"
  poolSize: 10
logging:
  level: "info"
  format: "json"
cache:
  redis:
    host: "localhost"
    port: 6379
  ttl: 300
```

## 手动更新配置

你可以手动使用 `etcdctl` 更新单个配置项，观察应用是否收到变更通知：

```bash
# 查看所有配置
etcdctl --endpoints=127.0.0.1:2379 get /example/config/kv --prefix

# 更新单个配置项（只更新 server/port）
etcdctl --endpoints=127.0.0.1:2379 put /example/config/kv/server/port "9090"

# 添加新配置项
etcdctl --endpoints=127.0.0.1:2379 put /example/config/kv/server/writeTimeout "60s"

# 删除配置项
etcdctl --endpoints=127.0.0.1:2379 del /example/config/kv/cache/ttl
```

## 配置说明

### KV 模式特点

- **多 key 存储**：配置分散在多个 etcd key 中
- **路径映射**：etcd key 的路径自动映射为配置的层级结构
- **细粒度更新**：可以单独更新某个配置项，无需更新整个配置
- **适合小配置**：适合配置较小但需要频繁更新的场景

### Key 命名规则

- **路径分隔符**：使用 `/` 作为路径分隔符
- **前缀匹配**：所有以 `prefix` 开头的 key 都会被读取
- **自动映射**：`/prefix/a/b/c` 映射为 `a.b.c`

### 配置源配置

| 参数 | 说明 | 默认值 |
|------|------|--------|
| `mode` | 模式：blob 或 kv | `kv` |
| `prefix` | 配置前缀 | 必填 |
| `watch` | 是否监听配置变更 | `true` |
| `format` | 配置格式：yaml/json/toml | 自动推断 |

### 代码结构说明

```go
// 1. 创建 etcd 客户端
cli, err := clientv3.New(clientv3.Config{
    Endpoints:   []string{"127.0.0.1:2379"},
    DialTimeout: 5 * time.Second,
})

// 2. 写入初始配置到 etcd（多个 key）
ops := []clientv3.Op{
    clientv3.OpPut("/example/config/kv/server/port", "8080"),
    clientv3.OpPut("/example/config/kv/server/host", "0.0.0.0"),
    clientv3.OpPut("/example/config/kv/database/host", "localhost"),
    clientv3.OpPut("/example/config/kv/database/port", "5432"),
    clientv3.OpPut("/example/config/kv/database/name", "mydb"),
}
_, err = cli.Txn(ctx).Then(ops...).Commit()

// 3. 创建配置源（KV 模式）
cfgSrc, err := etcd.NewConfigSource(etcd.ConfigSourceConfig{
    Client: etcd.ClientConfig{
        Endpoints:   []string{"127.0.0.1:2379"},
        DialTimeout: 5 * time.Second,
    },
    Mode:   etcd.ConfigSourceModeKV,
    Prefix: "/example/config/kv",
    Watch:  boolPtr(true),
})

// 4. 加载配置源到框架
if err := config.LoadSource(cfgSrc); err != nil {
    log.Fatalf("config.LoadSource: %v", err)
}

// 5. 添加配置变更监听器
_ = config.AddWatcher("server", func(ev config.WatchEvent) {
    if ev.Type() == config.WatchEventUpd || ev.Type() == config.WatchEventAdd {
        var serverConfig struct {
            Port         int    `mapstructure:"port"`
            Host         string `mapstructure:"host"`
            ReadTimeout  string `mapstructure:"readTimeout"`
        }
        if err := ev.Value().Scan(&serverConfig); err != nil {
            log.Printf("[config] failed to scan server config: %v", err)
            return
        }
        log.Printf("[config] server config updated: host=%s, port=%d",
            serverConfig.Host, serverConfig.Port)
    }
})
```

## 与 Blob 模式的区别

| 特性 | KV 模式 | Blob 模式 |
|------|---------|-----------|
| 存储 | 多个 key 存储配置片段 | 单个 key 存储整份配置 |
| 适合场景 | 配置较小、需要细粒度更新 | 配置较大、更新不频繁 |
| 更新粒度 | 可单独更新某个配置项 | 整体更新 |
| 解析方式 | 需要合并多个 key 的值 | 一次性解析整个配置 |
| etcd key 数量 | N 个（取决于配置层级） | 1 个 |
| 网络开销 | 更新单个配置时开销小 | 任何更新都需要传输整个配置 |

## 高级用法

### 动态配置项

KV 模式特别适合需要动态调整的配置项：

```go
// 运行时调整线程池大小
etcdctl put /example/config/kv/server/poolSize "20"

// 运行时调整缓存过期时间
etcdctl put /example/config/kv/cache/ttl "600"

// 运行时调整日志级别
etcdctl put /example/config/kv/logging/level "debug"
```

### 配置继承与覆盖

可以通过多个 prefix 实现配置继承：

```go
// 1. 加载基础配置（低优先级）
baseSrc, _ := etcd.NewConfigSource(etcd.ConfigSourceConfig{
    Prefix: "/config/base",
    Priority: source.PriorityLocal,
})
config.LoadSource(baseSrc)

// 2. 加载环境配置（中优先级）
envSrc, _ := etcd.NewConfigSource(etcd.ConfigSourceConfig{
    Prefix: "/config/prod",
    Priority: source.PriorityRemote,
})
config.LoadSource(envSrc)

// 3. 加载实例配置（高优先级，覆盖其他配置）
instanceSrc, _ := etcd.NewConfigSource(etcd.ConfigSourceConfig{
    Prefix: "/config/instance-123",
    Priority: source.PriorityRemote + 1,
})
config.LoadSource(instanceSrc)
```

### 配置加密

对于敏感配置，建议：
1. 使用 etcd 的加密功能
2. 或在应用层解密配置
3. 或使用密钥管理服务（如 HashiCorp Vault）

## 常见问题

**Q: 为什么有些配置项没有被读取？**

A: 检查以下几点：
1. 确认 key 的前缀与 `prefix` 配置一致
2. 确认 key 路径格式正确（使用 `/` 分隔）
3. 检查是否有非法字符（`{}` 用于特殊处理）

**Q: 如何更新嵌套配置？**

A: 直接更新对应的 key：
```bash
# 更新 cache.redis.host
etcdctl put /example/config/kv/cache/redis/host "redis.example.com"
```

**Q: 如何处理复杂类型的配置（如数组）？**

A: 在单个 key 中存储 JSON/YAML 字符串：
```bash
# 存储数组
etcdctl put /example/config/kv/server/servers '["10.0.0.1", "10.0.0.2", "10.0.0.3"]'

# 存储对象
etcdctl put /example/config/kv/server/metadata '{"env":"prod","region":"us-west"}'
```

然后在代码中解析：
```go
var servers []string
config.Get("server.servers").Scan(&servers)

var metadata map[string]string
config.Get("server.metadata").Scan(&metadata)
```

**Q: KV 模式的性能如何？**

A: KV 模式的性能取决于配置项数量：
- 少量配置项（<100）：性能优秀
- 大量配置项（>1000）：建议考虑使用 Blob 模式或分片

## 最佳实践

1. **合理设计 key 路径**：使用有意义的路径结构，如 `/config/{app}/{module}/{key}`
2. **控制配置项数量**：避免创建过多的配置 key（建议 <100）
3. **使用命名空间**：不同环境使用不同的 prefix，如 `/config/dev`、`/config/prod`
4. **监控配置变更**：记录配置变更日志，便于审计和排查问题
5. **配置版本管理**：在配置中包含版本信息，便于回滚

## 相关文档

- [etcd 主文档](../../../readme.md)
- [Blob 模式示例](../blob/)
- [注册中心示例](../../registry/)
- [服务发现示例](../../resolver/)
