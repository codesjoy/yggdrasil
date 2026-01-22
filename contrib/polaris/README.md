# Polaris Contrib

该子模块为 Yggdrasil 框架提供 Polaris 的能力接入：

- Server 侧：实现 `registry.Registry`，应用启动后注册实例，停止时反注册。
- Client 侧：实现 `resolver.Resolver`（schema=`polaris`），客户端从 Polaris 拉取实例列表并驱动负载均衡。
- 配置中心：实现 `config/source.Source`，从 Polaris 拉取配置文件并支持订阅变更（热更新）。
- 服务治理：提供 client unary interceptor（限流/熔断），可通过 Yggdrasil 配置启用。

## 依赖与启用

```go
import _ "github.com/codesjoy/yggdrasil/contrib/polaris/v2"
```

### SDKContext 初始化（按 name 区分并优先用原生配置）

推荐按 SDK 名称（如 `default`、`blue`）区分上下文，并优先使用 Polaris 原生配置文件初始化：

```yaml
yggdrasil:
  polaris:
    default:
      config_file: "./polaris.yaml"
      token: "token"
      addresses:
        - "naming.127.0.0.1:8091"
      config_addresses:
        - "config.127.0.0.1:8093"
```

说明：
- 当 `config_file` 存在时，使用 `LoadConfigurationByFile` + `NewSDKContextByConfig` 初始化；
- 否则回退到 `addresses`（`NewSDKContextByAddress`）；都未配置则使用 `NewSDKContext()`。
- 当 `token` 存在时，会写入 Polaris configuration 的 `global.serverConnector.token` 与 `config.configConnector.token`。
 - 当 `config_addresses` 存在时，会用于 Polaris config connector 的地址；未配置则回退使用 `addresses`。

## Server 侧：启用注册

```yaml
yggdrasil:
  polaris:
    default:
      addresses:
        - "127.0.0.1:8091"

  registry:
    schema: polaris
    config:
      sdk: default
      namespace: "default"
      serviceToken: ""
      ttl: "5s"
      autoHeartbeat: true
      timeout: "2s"
      retryCount: 0
```

说明：
- `service` 默认使用 `Instance.Name()`（通常为你的 appName）。
- `namespace` 优先使用配置；若未配置则回退到 `Instance.Namespace()`，再回退到 `default`。
- 每个 endpoint 会作为一个 Polaris instance 注册（host/port/protocol）。

## Client 侧：启用发现

```yaml
yggdrasil:
  polaris:
    default:
      addresses:
        - "127.0.0.1:8091"

  resolver:
    default:
      schema: polaris
      config:
        sdk: default
        namespace: "default"
        protocols: ["grpc"]
        refreshInterval: "10s"
        timeout: "2s"
        retryCount: 0
        skipRouteFilter: false
        metadata: {}

  client:
    example.hello.server:
      resolver: default
      balancer: round_robin
```

说明：
- `resolver` 的 `name` 由 `yggdrasil.client.<appName>.resolver` 指定（上例为 `default`）。
- `protocols` 默认为 `["grpc"]`，用于避免将非 RPC 协议的实例下发给 RPC client。
- 当前实现采用定时拉取（`refreshInterval`）方式刷新实例列表，并将结果推送给 client。
 - SDKContext 以 `sdk` 字段解析出来的 SDK 名称作为区分维度（例如 `default`），同名会复用同一个 Polaris SDKContext。

## 配置中心：启用 Polaris 配置源

该子模块提供 `polaris.NewConfigSource(cfg)`，用于将 Polaris 配置文件作为 Yggdrasil 的一个配置源加载，并支持配置变更事件驱动的热更新。

```go
import (
    "github.com/codesjoy/yggdrasil/v2/config"
    "github.com/codesjoy/yggdrasil/v2/config/source/file"
    _ "github.com/codesjoy/yggdrasil/contrib/polaris/v2"
    "github.com/codesjoy/yggdrasil/contrib/polaris/v2"
)

func main() {
    _ = config.LoadSource(file.NewSource("./bootstrap.yaml", false))
    src, _ := polaris.NewConfigSource(polaris.ConfigSourceConfig{
        SDK:       "default",
        Namespace: "default",
        FileGroup: "app",
        FileName:  "service.yaml",
    })
    _ = config.LoadSource(src)
}
```

建议：将 Polaris addresses 放在 bootstrap 配置里（文件/env/flag），确保能初始化 SDKContext。

## 服务治理：启用 Polaris 限流/熔断

该子模块支持两种方式接入治理能力：

1) **推荐：通过 balancer/picker 接入**（路由 + 负载均衡选择 + 熔断/限流检查/上报都在 picker 闭环完成）。
2) 通过 **client unary interceptor** 接入（更轻量，且更贴近“调用语义层”，例如只做限流/熔断开关判断）。

### 方式 1：balancer/picker（推荐）

```yaml
yggdrasil:
  polaris:
    default:
      addresses:
        - "127.0.0.1:8091"
    governance:
      config:
        sdk: default
        namespace: default
        callerService: "example.caller"
        callerNamespace: "default"
        routing:
          enable: true
          routers: []
          lbPolicy: ""
          timeout: 200ms
          retryCount: 0
          arguments: {}
        rateLimit:
          enable: true
          token: 1
          timeout: 200ms
          retryCount: 0
          arguments: {}
        circuitBreaker:
          enable: true

  resolver:
    default:
      schema: polaris
      config:
        sdk: default
        namespace: "default"
        skipRouteFilter: true

  client:
    example.hello.server:
      resolver: default
      balancer: polaris
```

说明：
- balancer `polaris` 会复用 resolver 的 `InstancesResponse`（通过 State.Attributes 透出）来做路由过滤与 LB 选点，并通过 `PickResult.Report()` 上报调用结果用于熔断统计。
- 建议 `skipRouteFilter: true`，避免 resolver 侧先做一轮静态路由过滤；把“按请求标签”的路由决策留给 picker（需要在请求 context 的 out-metadata 放标签）。

### 方式 2：client unary interceptor

该子模块注册了两个 client unary interceptor（按需开启）：

- `polaris_ratelimit`
- `polaris_circuitbreaker`

启用示例：

```yaml
yggdrasil:
  polaris:
    default:
      addresses:
        - "127.0.0.1:8091"
    governance:
      config:
        sdk: default
        namespace: default
        rateLimit:
          enable: true
          token: 1
          timeout: 200ms
          retryCount: 0
          arguments: {}
        circuitBreaker:
          enable: true

  client:
    interceptor:
      unary: ["polaris_ratelimit", "polaris_circuitbreaker"]
```

说明：
- `namespace` 为目标服务命名空间（默认为 `default`）。
- `rateLimit.arguments` 支持通过 `BuildCustomArgument(key,value)` 注入标签，用于 Polaris 限流/路由匹配。
