# 完整示例（All-in-One）

本示例演示 etcd contrib 模块的所有功能：配置源、注册中心和服务发现，在一个应用中完整展示。

## 你会得到什么

- **配置源**：从 etcd 读取配置并监听变更
- **注册中心**：将服务实例注册到 etcd，自动维持心跳
- **服务发现**：从 etcd 发现服务实例，实时监听变更
- **完整流程**：演示三个组件如何协同工作

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
etcd:
  client:
    endpoints:
      - "your-etcd-endpoint:2379"
```

### 2. 运行示例

```bash
cd contrib/etcd/example/allinone
go run main.go
```

## 预期输出

```
2024/01/26 10:00:00 [registry] instance registered
2024/01/26 10:00:00 [config] updated: message: hello from etcd at 2024-01-26T10:00:00Z
2024/01/26 10:00:00 [resolver] endpoints: 1
2024/01/26 10:00:00   - grpc://127.0.0.1:9000
2024/01/26 10:00:05 [config] updated: message: hello from etcd at 2024-01-26T10:00:05Z
2024/01/26 10:00:10 [config] updated: message: hello from etcd at 2024-01-26T10:00:10Z
...
```

## 架构说明

```
┌─────────────────────────────────────────────────────────┐
│                 Application                          │
│                                                      │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐ │
│  │ Config      │  │ Registry    │  │ Resolver    │ │
│  │ Source      │  │             │  │             │ │
│  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘ │
│         │                │                │          │
└─────────┼────────────────┼────────────────┼──────────┘
          │                │                │
          ▼                ▼                ▼
┌─────────────────────────────────────────────────────┐
│                    etcd                          │
│                                                     │
│  /demo/allinone/config     (配置源）                 │
│  /yggdrasil/registry/... (注册中心/发现）            │
└─────────────────────────────────────────────────────┘
```

## 功能详解

### 1. 配置源

- **模式**：Blob 模式，单个 key 存储配置
- **Key**：`/demo/allinone/config`
- **Watch**：启用，每 5 秒自动更新配置
- **监听器**：配置变更时打印最新内容

```go
cfgSrc, err := etcd.NewConfigSource(etcd.ConfigSourceConfig{
    Mode:  etcd.ConfigSourceModeBlob,
    Key:   "/demo/allinone/config",
    Watch:  boolPtr(true),
})
config.LoadSource(cfgSrc)

config.AddWatcher("", func(ev config.WatchEvent) {
    if ev.Type() == config.WatchEventUpd {
        log.Printf("[config] updated: %s", string(ev.Value().Bytes()))
    }
})
```

### 2. 注册中心

- **前缀**：`/yggdrasil/registry`
- **服务名**：`demo-allinone`
- **版本**：`1.0.0`
- **协议**：`grpc`
- **TTL**：10 秒
- **KeepAlive**：启用，自动维持心跳

```go
inst := demoInstance{
    namespace: "default",
    name:      "demo-allinone",
    version:   "1.0.0",
    metadata:  map[string]string{"env": "dev"},
    endpoints: []registry.Endpoint{
        demoEndpoint{scheme: "grpc", address: "127.0.0.1:9000"},
    },
}

reg, _ := registry.Get()
reg.Register(context.Background(), inst)
```

### 3. 服务发现

- **服务名**：`demo-allinone`（监听自身）
- **协议过滤**：只监听 `grpc` 和 `http` 协议
- **Debounce**：200ms
- **自动更新**：实例变更时打印端点列表

```go
res, _ := resolver.Get("default")

stateCh := make(chan resolver.State, 1)
res.AddWatch("demo-allinone", &mockClient{stateCh: stateCh})

go func() {
    for st := range stateCh {
        for _, ep := range st.GetEndpoints() {
            log.Printf("  - %s://%s", ep.GetProtocol(), ep.GetAddress())
        }
    }
}()
```

## 验证功能

### 查看配置

```bash
# 查看当前配置
etcdctl --endpoints=127.0.0.1:2379 get /demo/allinone/config

# 手动更新配置
etcdctl --endpoints=127.0.0.1:2379 put /demo/allinone/config "message: manually updated"
```

### 查看注册

```bash
# 查看所有注册的实例
etcdctl --endpoints=127.0.0.1:2379 get /yggdrasil/registry --prefix

# 查看特定服务的实例
etcdctl --endpoints=127.0.0.1:2379 get /yggdrasil/registry/default/demo-allinone --prefix
```

### 查看 Lease

```bash
# 查看所有 lease
etcdctl --endpoints=127.0.0.1:2379 lease list

# 查看 lease 详细信息
etcdctl --endpoints=127.0.0.1:2379 lease list --keys
```

## 配置说明

### 配置源配置

```yaml
etcd:
  configSource:
    mode: blob
    key: /demo/allinone/config
    watch: true
    format: yaml
```

### 注册中心配置

```yaml
yggdrasil:
  registry:
    type: etcd
    config:
      prefix: /yggdrasil/registry
      ttl: 10s
      keepAlive: true
      retryInterval: 3s
```

### 服务发现配置

```yaml
yggdrasil:
  resolver:
    default:
      type: etcd
      config:
        prefix: /yggdrasil/registry
        namespace: default
        protocols: ["grpc", "http"]
        debounce: 200ms
```

## 与其他示例的区别

| 示例 | 配置源 | 注册中心 | 服务发现 | 说明 |
|------|--------|---------|---------|------|
| allinone | ✅ Blob | ✅ | ✅ | 完整示例，所有功能 |
| config-source/blob | ✅ Blob | ❌ | ❌ | 单独演示配置源 |
| config-source/kv | ✅ KV | ❌ | ❌ | 演示 KV 模式配置源 |
| registry | ❌ | ✅ | ❌ | 单独演示注册中心 |
| resolver | ❌ | ❌ | ✅ | 单独演示服务发现 |

## 学习路径

1. **第一步**：运行 [allinone](.) 示例，了解整体流程
2. **第二步**：学习 [config-source/blob](../config-source/blob/)，掌握配置源用法
3. **第三步**：学习 [config-source/kv](../config-source/kv/)，了解 KV 模式
4. **第四步**：学习 [registry](../registry/)，掌握注册中心用法
5. **第五步**：学习 [resolver](../resolver/)，掌握服务发现用法

## 常见问题

**Q: 为什么要监听自己的服务？**

A: 这个示例为了演示服务发现功能，实际使用中 resolver 通常监听其他服务。

**Q: 配置更新后为什么有延迟？**

A: 配置更新需要经过 etcd watch 和应用监听，通常延迟 <100ms。

**Q: 实例会自动续期吗？**

A: 会，配置中 `keepAlive: true` 启用了自动心跳续约。

**Q: 如何停止示例？**

A: 按 `Ctrl+C`，示例会优雅退出，自动反注册实例。

## 扩展建议

1. **多服务部署**：启动多个 allinone 实例，观察服务发现效果
2. **负载均衡集成**：将 resolver 与 balancer 集成，实现真正的负载均衡
3. **gRPC 服务**：实现真正的 gRPC 服务，而不是模拟端点
4. **配置加密**：使用加密的配置，保护敏感信息
5. **监控告警**：添加监控和告警，监控配置、注册、发现状态

## 相关文档

- [etcd 主文档](../../../readme.md)
- [配置源 Blob 模式](../config-source/blob/)
- [配置源 KV 模式](../config-source/kv/)
- [注册中心](../registry/)
- [服务发现](../resolver/)
- [示例总览](../README.md)
