---
status: Design Baseline
applies_to: Yggdrasil v3
document_type: architecture documentation
last_reviewed: TBD
---

# 03. Bootstrap 自动装配与规划系统


> 本文说明 Yggdrasil v3 的 Bootstrap、自动装配、规划、Compose 和 Install 层。
>
> 关键词：App、Hub、Module、Capability、assembly.Spec、prepared runtime assembly、Prepare、Compose、BusinessBundle、Staged Reload。


## 1. 背景

Yggdrasil 的模块化内核非常严格，但日常业务接入如果完全手工选择模块、provider、capability 默认实现和 interceptor 链，会带来较高使用成本。因此，Yggdrasil v3 在内核之上提供 Bootstrap / Auto Assembly / Compose / Install 层。

这层的目标不是削弱显式内核，而是把常见配置驱动场景编译成一个显式、可解释、可比较、可 hash 的计划。

## 2. 核心对象

### 2.1 assembly.Spec

当前公共规划类型是 `assembly.Spec`。它是规划层唯一真相源：

```go
type Spec struct {
    Identity  IdentitySpec
    Mode      Mode
    Modules   []ModuleRef
    Defaults  map[string]string
    Chains    map[string]Chain
    Decisions []Decision
    Warnings  []Warning
    Conflicts []Conflict
}
```

要求：

- 只包含稳定、可序列化的数据；
- 不包含 `module.Module` 实例；
- 所有 map 和 slice 进入输出前 canonical sort；
- explain、dry-run、diff、hash 都基于它；
- reload 比较也基于它，而不是运行时实例。

### 2.2 Prepared Runtime Assembly

prepared runtime assembly 是基于 `assembly.Spec` 实例化后的 App 内部对象图，不是导出的公共 API。概念上它包含：

```go
type preparedRuntimeGraph struct {
    Spec      *assembly.Spec
    Modules   []module.Module
    Runtime   app.Runtime
    Server    server.Server
    CloseFunc func(context.Context) error
}
```

它不参与 hash / diff，只负责进程内运行时资源和关闭路径。

## 3. Mode 系统

对用户只暴露一个 `mode`：

```yaml
yggdrasil:
  mode: prod-grpc
```

内部拆成：

| mode | profile | bundle | 典型含义 |
|---|---|---|---|
| `dev` | `dev` | `server-basic` | 文本日志、基础 server、REST 可用、开发友好 |
| `prod-grpc` | `prod` | `grpc-server` | JSON 日志、OTel、gRPC server、生产配置 |
| `prod-http-gateway` | `prod` | `http-gateway` | HTTP gateway、REST middleware、生产配置 |

mode 可以提供默认 bundle、默认 logger/tracer/meter、默认链模板与版本，但不能绕过 Hub 的 DAG、Capability 和生命周期校验。

## 4. AutoDescribed 与 AutoRule

希望参与自动装配的模块可实现：

```go
type AutoDescribed interface {
    AutoSpec() AutoSpec
}

type AutoSpec struct {
    Provides      []CapabilitySpec
    AutoRules     []AutoRule
    DefaultPolicy *DefaultPolicy
}
```

AutoRule 必须是纯函数：

```go
type AutoRule interface {
    Match(ctx AutoRuleContext) bool
    Describe() string
    AffectedPaths() []string
}
```

约束：

- 不依赖时间、随机数、全局可变状态；
- 只读取只读配置快照、resolved mode 和静态上下文；
- 必须声明受影响配置路径，用于 reload 分类；
- 不允许让 map 遍历顺序影响结果。

## 5. 自动装配流水线

```mermaid
flowchart LR
    A["resolveMode"] --> B["resolveModules"]
    B --> C["collectProviders"]
    C --> D["resolveDefaults"]
    D --> E["resolveChains"]
    E --> F["buildEffectiveResolved"]
    F --> G["compileCapabilityBindings"]
    G --> H["validateBindings"]
```

说明：

1. `resolveMode`：解析 `yggdrasil.mode`，未知 mode 返回 `InvalidMode`。
2. `resolveModules`：处理禁用、强制启用、required modules、auto rules 与依赖闭包。
3. `collectProviders`：按 capability spec 收集 provider。
4. `resolveDefaults`：为默认能力选择 provider。
5. `resolveChains`：解析 interceptor / middleware 链模板或显式列表。
6. `buildEffectiveResolved`：合并默认值与链选择。
7. `compileCapabilityBindings`：生成 Hub 可消费的 capability binding map。
8. `validateBindings`：校验显式引用的 provider 存在且类型正确。

## 6. 默认实现选择算法

默认实现选择必须是确定性算法：

```text
1. code override: ForceDefault
2. config override: yggdrasil.overrides.force_defaults
3. explicit config: telemetry.tracer = xxx
4. mode default
5. module fallback: AutoSpec.DefaultPolicy score
6. framework fallback
```

冲突规则：

- 同一 capability、同一来源层级、同分且多个合法候选时，返回 `AmbiguousDefault`。
- 不能为了“方便”隐式选字典序第一个。
- `WithModules(...)` 只影响候选集，不等于强制绑定。
- 真正强制绑定必须使用 `ForceDefault` 或显式配置。

## 7. 默认链模板

链模板必须命名且版本化，不能使用黑盒 `auto`：

```yaml
extensions:
  interceptors:
    unary_server: default-observable@v1
    unary_client: default-client-safe@v1
```

模板定义：

```go
type ChainTemplate struct {
    Name    string
    Version string
    Items   []string
}
```

规则：

- `default-observable@v1` 发布后内容冻结；
- 后续变化新增 `@v2`；
- 当前内置 `@v1` 模板刻意保持最小：RPC 模板展开为 `logging`，REST observable 模板展开为 `logger`；
- 后续若要加入 recovery、tracing、metrics、request-id 等低争议能力，应发布新版本；
- auth、retry、hedging、circuit-breaker 等改变业务语义的能力不应默认放入模板；
- 模板展开后仍交给 `ResolveOrdered` 校验。

## 8. Explain / Dry-Run / Diff / Hash

### Explain

输出：App identity、mode/profile/bundle、启用模块、默认实现来源、链模板展开、decision records、warnings、conflicts。

### Dry-Run

只生成 `assembly.Spec`，不实例化 runtime，不 bind/listen/register。

### Diff

比较两个 `assembly.Spec`：mode、module refs、defaults、chains、overrides。

### Hash

对 canonical JSON 做 SHA-256。禁止包含地址、接口实例、非确定性字段。

## 9. 错误语义

常见规划阶段错误：

| 错误 | 说明 | 修复方向 |
|---|---|---|
| `InvalidMode` | mode 名称未知 | 改用内置 mode 或注册新 mode |
| `UnknownTemplate` | 链模板不存在 | 检查模板名 |
| `TemplateVersionNotFound` | 模板版本不存在 | 使用已发布版本 |
| `AmbiguousDefault` | 默认实现冲突 | 显式指定 provider 或禁用候选 |
| `ConflictingOverride` | override 互相冲突 | 合并或删除冲突配置 |
| `UnknownExplicitBinding` | 显式 provider 不存在 | 检查模块是否启用或名称是否正确 |
| `InvalidAutoRule` | AutoRule 非法 | 修复纯函数、路径声明或匹配逻辑 |

## 10. 推荐高层入口

```go
return yggdrasil.Run(
    ctx,
    business.Compose,
    yggdrasil.WithConfigPath("app.yaml"),
)
```

## 11. Canonicalization Checklist

- [ ] 所有 map 转换为排序 slice 后再进入下一阶段。
- [ ] 决策记录按 stable key 输出。
- [ ] provider 候选按 module name 升序稳定排序。
- [ ] warnings / conflicts 输出顺序稳定。
- [ ] hash 基于 canonical `assembly.Spec`。
- [ ] diff 不读取 prepared runtime assembly 状态。
