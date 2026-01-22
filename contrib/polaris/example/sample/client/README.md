# Sample Client（通过 Polaris 发现并调用）

该示例启动一个 gRPC client，通过 Polaris resolver 获取实例列表，并使用 `polaris` balancer（picker）执行路由/负载均衡选择，然后发起 RPC 调用。

## 你会得到什么

- 通过 Polaris 发现服务：`github.com.codesjoy.yggdrasil.contrib.polaris.example.server`
- 发起两次 RPC：
  - `GetShelf`：成功，并打印 header/trailer
  - `MoveBook`：返回带 reason 的业务错误，并打印 reason/code/httpCode

## 前置条件

1. 先启动 [sample/server](file:///Users/zhangwei/go/src/github.com/codesjoy/yggdrasil/contrib/polaris/example/sample/server/README.md) 并确认已注册到 Polaris。
2. 可访问的 Polaris Server（示例默认使用 `8091`）。

## 启动方式

1. 修改本目录的 [config.yaml](file:///Users/zhangwei/go/src/github.com/codesjoy/yggdrasil/contrib/polaris/example/sample/client/config.yaml)：
   - 把 `yggdrasil.polaris.default.addresses` 改成你的 Polaris 地址（例如 `127.0.0.1:8091`）。
   - 如需鉴权，填入 `yggdrasil.polaris.default.token`。
2. 启动 client：

```bash
cd contrib/polaris/example/sample/client
go run ./
```

## 配置说明（核心字段）

### 1) Polaris SDK（连接 Polaris）

- `yggdrasil.polaris.default.addresses`：注册发现地址（naming）。
- `yggdrasil.polaris.default.token`：可选。

### 2) Resolver（从 Polaris 拉取实例）

- `yggdrasil.resolver.default.type: polaris`
- `yggdrasil.resolver.default.config.namespace`：目标服务所在 namespace（示例为 `default`）。
- `yggdrasil.resolver.default.config.protocols: ["grpc"]`：只下发 gRPC 实例，避免混入非 RPC 协议实例。
- `refreshInterval`：定时拉取实例列表的周期。
- `skipRouteFilter: true`：
  - 建议保持为 true，让“按请求标签”的路由过滤放在 picker 做（需要 out-metadata）。

### 3) Client 绑定 resolver + balancer

```yaml
yggdrasil:
  client:
    github.com.codesjoy.yggdrasil.contrib.polaris.example.server:
      resolver: default
      balancer: polaris
```

- `resolver`：选择哪个 resolver 实例名（这里是 `default`）。
- `balancer: polaris`：启用 polaris picker（支持路由/限流/熔断闭环）。

### 4) 治理开关（可选）

示例里默认只开 routing：

- `yggdrasil.polaris.governance.config.routing.enable: true`
- `rateLimit/circuitBreaker` 默认关闭（需要你在 Polaris 控制台配置对应规则后再打开）

## Polaris 控制台操作（最少步骤）

1. 进入“服务管理/服务列表”，确认服务存在且至少 1 个健康实例。
2. （可选）如果你想验证路由能力：
   - 进入“治理/路由规则”（不同版本控制台菜单名可能略有差异）
   - 给目标服务创建路由规则：按请求标签（例如 `env=dev`）或实例 metadata 做筛选
   - 重新运行 client，观察是否按规则选到了对应实例

## 代码要点（方便你改造）

- 请求标签通过 out-metadata 透传给 picker：
  - 见 [main.go](file:///Users/zhangwei/go/src/github.com/codesjoy/yggdrasil/contrib/polaris/example/sample/client/main.go#L38-L42)
- header/trailer 从 ctx 读取：
  - 见 [main.go](file:///Users/zhangwei/go/src/github.com/codesjoy/yggdrasil/contrib/polaris/example/sample/client/main.go#L46-L52)
