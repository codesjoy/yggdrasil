# 配置源 Blob 模式示例

本示例演示如何使用 etcd 的 blob 模式作为配置源，从单个 etcd key 读取完整的 YAML 配置文件，并监听配置变更实现热更新。

## 你会得到什么

- 从 etcd 读取完整的 YAML 配置文件（存储在单个 key 中）
- 配置热更新：当 etcd 中的配置变更时，应用自动收到通知并更新内存中的配置
- 实时演示：每 10 秒自动更新配置一次，展示配置变更监听效果

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
cd contrib/etcd/example/config-source/blob
go run main.go
```

## 预期输出

```
2024/01/26 10:00:00 [etcd] initial config written to /example/config/blob
2024/01/26 10:00:00 [app] config source initialized, watching for changes...
2024/01/26 10:00:00 [app] press Ctrl+C to update config dynamically
2024/01/26 10:00:00 [app] press Ctrl+C again to exit
2024/01/26 10:00:10 [etcd] config updated (count=1)
2024/01/26 10:00:10 [config] server config updated: host=0.0.0.0, port=8081
2024/01/26 10:00:10 [config] database config updated: host=localhost, port=5432, name=mydb
2024/01/26 10:00:20 [etcd] config updated (count=2)
2024/01/26 10:00:20 [config] server config updated: host=0.0.0.0, port=8082
```

## 手动更新配置

你也可以手动使用 `etcdctl` 更新配置，观察应用是否收到变更通知：

```bash
# 查看当前配置
etcdctl --endpoints=127.0.0.1:2379 get /example/config/blob

# 更新配置
etcdctl --endpoints=127.0.0.1:2379 put /example/config/blob 'server:
  port: 9090
  host: "0.0.0.0"
database:
  host: "db.example.com"
  port: 3306
  name: "mydb"'
```

## 配置说明

### Blob 模式特点

- **单个 key 存储**：整个配置文件存储在一个 etcd key 中
- **适合大配置**：适合配置较大但更新不频繁的场景
- **原子更新**：配置更新是原子性的，不会出现部分更新的问题
- **格式解析**：支持 yaml/json/toml 格式，根据扩展名或配置自动解析

### 配置源配置

| 参数 | 说明 | 默认值 |
|------|------|--------|
| `mode` | 模式：blob 或 kv | `blob` |
| `key` | 配置存储的 etcd key | 必填 |
| `watch` | 是否监听配置变更 | `true` |
| `format` | 配置格式：yaml/json/toml | `yaml` |

### 代码结构说明

```go
// 1. 创建 etcd 客户端
cli, err := clientv3.New(clientv3.Config{
    Endpoints:   []string{"127.0.0.1:2379"},
    DialTimeout: 5 * time.Second,
})

// 2. 写入初始配置到 etcd
_, err = cli.Put(ctx, "/example/config/blob", initialConfig)

// 3. 创建配置源
cfgSrc, err := etcd.NewConfigSource(etcd.ConfigSourceConfig{
    Client: etcd.ClientConfig{
        Endpoints:   []string{"127.0.0.1:2379"},
        DialTimeout: 5 * time.Second,
    },
    Mode:   etcd.ConfigSourceModeBlob,
    Key:    "/example/config/blob",
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
            Port int    `mapstructure:"port"`
            Host string `mapstructure:"host"`
        }
        if err := ev.Value().Scan(&serverConfig); err != nil {
            log.Printf("[config] failed to scan server config: %v", err)
            return
        }
        log.Printf("[config] server config updated: host=%s, port=%d", serverConfig.Host, serverConfig.Port)
    }
})
```

## 与 KV 模式的区别

| 特性 | Blob 模式 | KV 模式 |
|------|-----------|---------|
| 存储 | 单个 key 存储整份配置 | 多个 key 存储配置片段 |
| 适合场景 | 配置较大、更新不频繁 | 配置较小、需要细粒度更新 |
| 更新粒度 | 整体更新 | 可单独更新某个配置项 |
| 解析方式 | 一次性解析整个配置 | 需要合并多个 key 的值 |
| etcd key 数量 | 1 个 | N 个（取决于配置层级） |

## 常见问题

**Q: 配置更新后没有收到通知？**

A: 检查以下几点：
1. 确认 `watch: true` 已配置
2. 确认配置 key 正确（与代码中的 key 一致）
3. 检查 `AddWatcher` 是否正确注册了监听器

**Q: 配置文件太大怎么办？**

A: Blob 模式适合配置较大的场景，但建议：
1. 单个配置不超过 1.5MB（etcd 限制）
2. 如果配置确实很大，考虑拆分为多个配置源或使用 KV 模式

**Q: 如何支持 JSON/TOML 格式？**

A: 修改 `Format` 参数：
```go
cfgSrc, err := etcd.NewConfigSource(etcd.ConfigSourceConfig{
    // ...
    Format: json.Unmarshal,  // 或 toml.Unmarshal
})
```

## 进阶用法

### 多配置源

你可以同时使用多个配置源，优先级从高到低：

```go
// 1. 加载本地配置文件（低优先级）
fileSrc := file.NewSource("./config.yaml", false)
config.LoadSource(fileSrc)

// 2. 加载 etcd 配置源（高优先级，覆盖本地配置）
etcdSrc, _ := etcd.NewConfigSource(etcd.ConfigSourceConfig{...})
config.LoadSource(etcdSrc)
```

### 配置加密

对于敏感配置（如数据库密码），建议：
1. 使用 etcd 的加密功能
2. 或在应用层解密配置
3. 或使用密钥管理服务（如 HashiCorp Vault）

## 相关文档

- [etcd 主文档](../../../readme.md)
- [KV 模式示例](../kv/)
- [注册中心示例](../../registry/)
- [服务发现示例](../../resolver/)
