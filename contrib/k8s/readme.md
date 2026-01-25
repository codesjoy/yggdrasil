# contrib/k8s

为 yggdrasil 框架提供 Kubernetes 原生集成能力：

- **服务发现 Resolver（type=kubernetes）**：基于 EndpointSlice（优先）/Endpoints（回退）watch，把 Pod IP+Port 映射为框架 resolver.State，直接驱动现有 balancer。
- **配置源 Source（ConfigMap/Secret）**：从 K8s ConfigMap/Secret 读取配置（按指定 key 或 mergeAllKey），支持 watch 热更新。

本模块为独立 Go module，避免核心库引入 Kubernetes 依赖。

## 快速开始

### 1）启用服务发现

在业务程序中引入 contrib/k8s（import side-effect 触发 init 注册）：

```go
import _ "github.com/codesjoy/yggdrasil/contrib/k8s/v2"
```

配置示例（YAML）：

```yaml
yggdrasil:
  resolver:
    my-k8s:
      type: kubernetes
      config:
        namespace: default
        mode: endpointslice
        portName: grpc
        protocol: grpc
        kubeconfig: ""          # 为空时自动使用 in-cluster config
        backoff:
          baseDelay: 1s
          multiplier: 1.6
          jitter: 0.2
          maxDelay: 30s
  client:
    my-service:
      resolver: my-k8s          # 使用上面的 resolver
      balancer: default
```

- `mode` 可选：`endpointslice`（默认，优先）或 `endpoints`。
- `portName` 优先于 `port`；均不填则取第一个端口。
- `kubeconfig` 仅本地开发用；为空时自动走 in-cluster config（服务账户 RBAC）。
- `endpointAttributes` 可把自定义标签写入 resolver.BaseEndpoint.Attributes。

RBAC 最小权限示例：

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  namespace: default
  name: yggdrasil-resolver
rules:
- apiGroups: [""]
  resources: ["endpoints", "endpointslices"]
  verbs: ["get", "list", "watch"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: yggdrasil-resolver
  namespace: default
subjects:
- kind: ServiceAccount
  name: your-serviceaccount
roleRef:
  kind: Role
  name: yggdrasil-resolver
  apiGroup: rbac.authorization.k8s.io
```

### 2）使用 ConfigMap/Secret 配置源

创建 Source 并交给 `config.LoadSource(...)`：

```go
import (
    "github.com/codesjoy/yggdrasil/v2"
    "github.com/codesjoy/yggdrasil/v2/config"
    "github.com/codesjoy/yggdrasil/v2/config/source"
    k8s "github.com/codesjoy/yggdrasil/contrib/k8s/v2"
)

src, err := k8s.NewConfigMapSource(k8s.ConfigSourceConfig{
    Namespace: "default",
    Name:      "my-config",
    Key:       "config.yaml",
    Watch:      true,
    Priority:   source.PriorityRemote,
})
if err != nil {
    panic(err)
}
if err := config.LoadSource(src); err != nil {
    panic(err)
}

yggdrasil.Init("my-app")
yggdrasil.Run()
```

- `Key`：ConfigMap 中的 key（例如 `config.yaml`），内容按扩展名自动解析为 yaml/json/toml。
- `MergeAllKey`：若为 true，则把 ConfigMap 所有 key 作为 map 注入（不按单文件解析）。
- `Secret` 用法一致，仅需调用 `k8s.NewSecretSource(...)`。

RBAC 最小权限（ConfigMap/Secret）：

```yaml
- apiGroups: [""]
  resources: ["configmaps", "secrets"]
  verbs: ["get", "list", "watch"]
```

## 配置结构

### ResolverConfig

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `namespace` | string | `default`（或环境变量 `KUBERNETES_NAMESPACE`） | K8s 命名空间 |
| `mode` | string | `endpointslice` | `endpointslice` 或 `endpoints` |
| `portName` | string | 空 | 按端口名匹配 |
| `port` | int32 | 0 | 按端口号匹配（优先级低于 portName） |
| `protocol` | string | `grpc` | 端点协议标识（写入 Attributes） |
| `kubeconfig` | string | 空 | 本地 kubeconfig 路径；为空则 in-cluster |
| `endpointAttributes` | map[string]string | nil | 额外写入端点 Attributes 的键值对 |
| `backoff.baseDelay` | duration | `1s` | watch 断线重连初始延迟 |
| `backoff.multiplier` | float64 | `1.6` | 退避倍数 |
| `backoff.jitter` | float64 | `0.2` | 随机抖动 |
| `backoff.maxDelay` | duration | `30s` | 最大延迟 |

### ConfigSourceConfig

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `namespace` | string | `default`（或 `KUBERNETES_NAMESPACE`） | 命名空间 |
| `name` | string | （必填） | ConfigMap/Secret 名称 |
| `key` | string | 空 | 读取的单个 key（如 `config.yaml`） |
| `mergeAllKey` | bool | false | 是否把所有 key 合并为 map 注入 |
| `format` | source.Parser | 按扩展名推断 | 显式指定解析器（yaml/json/toml） |
| `priority` | source.Priority | `PriorityRemote` | 配置优先级 |
| `watch` | bool | true | 是否 watch 热更新 |
| `kubeconfig` | string | 空 | 本地 kubeconfig；为空则 in-cluster |

## 示例程序

- `example/config-source`：从 ConfigMap 加载配置并打印。
- `example/resolver`：使用 k8s resolver 发现下游 Service。

## 运行测试

```bash
cd contrib/k8s
GOWORK=off go test ./...
```

注意：仓库根有 `go.work`，默认不包含 `contrib/k8s`，可用 `GOWORK=off` 或把 `./contrib/k8s` 加进 `go.work` 的 `use` 列表。
