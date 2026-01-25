# etcd 集成

本插件将 etcd 作为配置中心、注册中心与发现服务接入 Yggdrasil 框架。

## 启用插件

在业务 main 中空导入一次：

```go
import _ "github.com/codesjoy/yggdrasil/contrib/etcd/v2"
```

## 功能说明

### 配置中心（Config Source）

- **blob 模式**：单个 key 存储整份配置（yaml/json/toml）
- **kv 模式**：prefix 下多 key，按路径映射成层级配置
- 支持 `Watch()` 监听变更并推送最新快照

#### 配置项

```yaml
etcd:
  configSource:
    client:
      endpoints: ["127.0.0.1:2379"]
      dialTimeout: 5s
      username: ""
      password: ""
      tls:
        certFile: ""
        keyFile: ""
        caFile: ""
    mode: blob  # 或 kv
    key: /config/app  # mode=blob 时使用
    prefix: /config/app  # mode=kv 时使用
    watch: true
    format: yaml  # json/yaml/toml
```

#### 使用方式

```go
import "github.com/codesjoy/yggdrasil/v2/config"
import "github.com/codesjoy/yggdrasil/contrib/etcd/v2"

var cfg etcd.ConfigSourceConfig
_ = config.Get("etcd.configSource").Scan(&cfg)
src, err := etcd.NewConfigSource(cfg)
if err != nil { ... }
_ = config.LoadSource(src)
```

### 注册中心（Registry）

- 使用 lease + keepalive 维持实例注册
- key 布局：`<prefix>/<namespace>/<service>/<instanceKey(hash)>`

#### 配置项

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

### 服务发现（Resolver）

- 按 service 维度 watch prefix，事件触发后“快照刷新”并 `UpdateState`
- 支持可选 `protocols` 过滤与 `debounce` 降噪

#### 配置项

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
```

#### 客户端配置

```yaml
yggdrasil:
  client:
    myApp:
      resolver: default
```

## 示例

完整示例见 `example/allinone/main.go`，展示：
- 从 etcd 动态读取配置并 watch
- 服务注册到 etcd
- 通过 resolver 发现服务并调用
