# Governance Server（带实例 metadata 注册）

该示例启动一个 gRPC 服务并注册到 Polaris，同时演示如何通过 `yggdrasil.application` 注入 namespace/version/metadata，最终体现在 Polaris 的实例 metadata 上，供路由/灰度等规则使用。

## 你会得到什么

- 服务：`github.com.codesjoy.yggdrasil.contrib.polaris.example.governance.server`
- 实例 metadata（示例）：
  - `env=dev`
  - `lane=stable`
  - `version=v1.0.0`

## 启动方式

1. 修改 [config.yaml](file:///Users/zhangwei/go/src/github.com/codesjoy/yggdrasil/contrib/polaris/example/scenarios/governance/server/config.yaml)：
   - `yggdrasil.polaris.default.addresses`：Polaris naming 地址（通常 `host:8091`）
   - `yggdrasil.polaris.default.token`：可选
   - `yggdrasil.registry.config.namespace`：注册到的 namespace（示例 `default`）
   - `yggdrasil.remote.protocol.grpc.address`：本地监听地址（示例 `127.0.0.1:55880`）
2. 启动：

```bash
cd contrib/polaris/example/scenarios/governance/server
go run ./
```

## 配置说明（重点）

### 1) 实例信息注入（yggdrasil.application）

```yaml
yggdrasil:
  application:
    namespace: "default"
    version: "v1.0.0"
    metadata:
      env: "dev"
      lane: "stable"
```

- `namespace`：会影响默认注册 namespace（如果 registry config 未显式指定）。
- `version` / `metadata`：会作为实例 metadata 合并到注册请求里，供 Polaris 路由/灰度/分组等能力使用。

### 2) Registry 注册

```yaml
yggdrasil:
  registry:
    schema: polaris
    config:
      namespace: "default"
```

## Polaris 控制台核验

1. 打开控制台：`http://127.0.0.1:8080`
2. 进入“服务管理/服务列表”，找到：
   - `github.com.codesjoy.yggdrasil.contrib.polaris.example.governance.server`
3. 打开服务详情的“实例”页：
   - 确认实例端口为 `55880`
   - 确认实例 metadata 里包含 `env/lane`（以及 version）

## 可选：准备两组实例用于灰度

如果你想在路由规则里直观看到“按标签路由”的效果，建议启动两份 server：

1. 复制一份目录或临时改 config：
   - 第 1 份：`lane=stable`，端口 `55880`
   - 第 2 份：`lane=canary`，端口 `55882`
2. 两份都注册到同一个服务名与 namespace 下
3. 在 Polaris 控制台配置路由规则（见 client README）

注意：
- 两份实例都需要处于健康状态；如果某个标签组实例全部不可用，路由无法命中时会退回普通选点。
- 只改 `lane` 还不够，第二份实例需要同时改 `yggdrasil.remote.protocol.grpc.address`，避免端口冲突。
