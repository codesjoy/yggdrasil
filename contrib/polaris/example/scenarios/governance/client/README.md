# Governance Client（请求标签路由示例）

该示例演示：通过 out-metadata 透传“请求标签”（如 `env/lane/user_id`），配合 `polaris` balancer/picker 进行路由过滤与选点。

关键点是：resolver 侧配置 `skipRouteFilter: true`，避免先做静态路由过滤，把“按请求标签”的决策留给 picker。

## 前置条件

1. 已启动 [server](file:///Users/zhangwei/go/src/github.com/codesjoy/yggdrasil/contrib/polaris/example/scenarios/governance/server/README.md)，并确认在 Polaris 里存在至少 1 个实例。
2. 可访问的 Polaris Server（默认 `8091`），以及控制台（默认 `8080`，用于配置路由规则）。

## 启动方式

1. 修改 [config.yaml](file:///Users/zhangwei/go/src/github.com/codesjoy/yggdrasil/contrib/polaris/example/scenarios/governance/client/config.yaml)：
   - `yggdrasil.polaris.default.addresses`：Polaris naming 地址
   - `yggdrasil.polaris.default.token`：可选
   - `yggdrasil.client.<serverName>.balancer: polaris`：启用 polaris picker
2. 启动：

```bash
cd contrib/polaris/example/scenarios/governance/client
go run ./
```

## 请求标签从哪里来

在 [main.go](file:///Users/zhangwei/go/src/github.com/codesjoy/yggdrasil/contrib/polaris/example/scenarios/governance/client/main.go#L38-L43) 里，client 会把以下 out-metadata 带到请求上下文：

- `env=dev`
- `lane=stable`
- `user_id=123`

这些标签会被 `polaris` picker 用作“请求标签路由”的输入。

## Polaris 控制台：创建一个最小路由规则

目标：让“带 `lane=stable` 的请求”只命中 `lane=stable` 的实例。

建议的准备：
- 至少启动两份 server（见 server README 的“准备两组实例用于灰度”），分别注册 `lane=stable` 与 `lane=canary`。

操作步骤（不同版本控制台菜单名可能略有差异）：

1. 打开控制台：`http://127.0.0.1:8080`
2. 进入“服务管理/服务列表”，找到目标服务：
   - `github.com.codesjoy.yggdrasil.contrib.polaris.example.governance.server`
3. 进入“治理/路由规则”（或“流量治理/路由”）
4. 创建规则（示意）：
   - 规则条件：请求标签 `lane = stable`
   - 路由目标：实例 metadata `lane = stable`
5. 保存并发布规则
6. 重新运行 client，观察：
   - 如果你在 server 端打印了本地端口/实例信息（本示例未打印），可以通过控制台/监控来确认命中实例

## 配置说明（重点）

### 1) Resolver

- `protocols: ["grpc"]`：只下发 gRPC 实例
- `skipRouteFilter: true`：把请求标签路由留给 picker

### 2) Governance 开关（示例默认只开 routing）

`yggdrasil.polaris.governance.config.routing.enable: true`

- `rateLimit/circuitBreaker` 默认关闭；如果你要演示限流/熔断，需要先在 Polaris 控制台配置对应规则，再把开关打开。

## 常见问题

- 路由规则生效但仍会命中“非 stable”实例：
  - 确认实例 metadata 里真的有 `lane`（在 server 的 `yggdrasil.application.metadata` 设置）。
  - 确认 resolver 配置为 `skipRouteFilter: true`（示例默认即如此）。
  - 确认路由规则已“发布”，而不是仅保存草稿。
  - 确认控制台规则使用的 key/value 与请求 out-metadata 完全一致（区分大小写）。
  - 如果规则没有命中任何“已就绪”的实例，client 会回退到普通选点；此时需要检查实例是否健康、连接是否 Ready。
