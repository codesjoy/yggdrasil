# 02. 模块中心 Hub 与 Capability 模型


> 本文说明 Yggdrasil v3 的 Module Hub、模块生命周期、依赖排序、Capability 模型、Scope 边界和诊断契约。
>
> 关键词：App、Hub、Module、Capability、assembly.Spec、prepared runtime assembly、Prepare、Compose、BusinessBundle、Staged Reload。


## 1. Hub 的职责

`module.Hub` 是 Yggdrasil 运行时组合的核心。它负责收集模块、建立名称索引、校验依赖 DAG、执行生命周期、收集 capability，并提供带基数约束的查询 API。

Hub 不负责业务对象图，也不负责高频动态实例。它管理的是长期存在、低频变化、可诊断的能力载体。

## 2. Module 最小接口

```go
type Module interface {
    Name() string
}
```

所有其它行为都是可选接口：

| 接口 | 作用 |
|---|---|
| `Dependent` | 声明硬依赖 |
| `Ordered` | 同一 DAG 层内排序 |
| `Configurable` | 声明配置路径 |
| `Initializable` | 初始化长期资源 |
| `Startable` | 启动服务行为或后台过程 |
| `Stoppable` | 停止资源，必须幂等 |
| `Reloadable` | 支持 staged reload |
| `CapabilityProvider` | 暴露 capability |
| `AutoDescribed` | 参与自动装配 |
| `Scoped` | 声明生命周期作用域 |

## 3. 依赖 DAG 与拓扑排序

模块依赖通过 `DependsOn()` 显式声明：

```go
type Dependent interface {
    DependsOn() []string
}
```

排序规则：

1. 读取所有模块名称并建立索引。
2. 读取 `DependsOn()` 构造有向图。
3. 校验依赖目标存在。
4. 使用 Kahn 算法拓扑排序。
5. 同层模块按 `InitOrder()` 升序，再按名称字典序排序。
6. 若存在环，输出完整环路径。

错误信息必须可排障，例如：

```text
module dependency cycle detected:
logger.default -> tracer.default -> server.default -> logger.default
```

```text
module "server.default" depends on missing module "stats.default"
available modules: logger.default, tracer.default, otel.stats
```

## 4. 生命周期语义

### 4.1 Init

按拓扑顺序调用 `Init(ctx, view)`。若模块实现 `Configurable`，Hub 会根据 `ConfigPath()` 提供 scoped config view。

```go
func (m *MyModule) ConfigPath() string { return "yggdrasil.my_module" }
func (m *MyModule) Init(ctx context.Context, view config.View) error {
    var cfg MyConfig
    if err := view.Decode(&cfg); err != nil { return err }
    return nil
}
```

### 4.2 Start 与失败补偿

`Start` 按拓扑顺序执行。若某个模块启动失败，Hub 必须按逆序停止已经启动成功的模块。

```text
start A -> start B -> start C failed -> stop B -> stop A
```

补偿停止不能因为单个模块 stop 失败而中断；错误应聚合返回。

### 4.3 Stop

`Stop` 按拓扑逆序执行，只调用实现 `Stoppable` 的模块。`Stop()` 必须幂等，推荐用 `sync.Once`：

```go
func (m *myModule) Stop(ctx context.Context) error {
    m.stopOnce.Do(func() {
        m.stopErr = m.closeResources(ctx)
    })
    return m.stopErr
}
```

### 4.4 Reload

Reload 使用两阶段协议：

```text
idle -> preparing -> committing -> idle
              \-> rollback -> degraded
```

- Prepare 阶段：所有受影响模块先准备新状态。
- Commit 阶段：全部 prepare 成功后按稳定顺序提交。
- Rollback 阶段：prepare 或 commit 失败时回滚未提交状态。
- Degraded：rollback 失败或状态分叉，需要外部重启。

## 5. Capability 模型

Capability 是模块间能力发现机制。模块不直接互相 import，而是发布 typed capability value，消费者通过 Hub 按 spec 解析。

```go
type CapabilitySpec struct {
    Name        string
    Cardinality CapabilityCardinality
    Type        reflect.Type
}

type Capability struct {
    Spec  CapabilitySpec
    Name  string
    Value any
}
```

### 5.1 基数规则

| 基数 | 语义 | 典型场景 |
|---|---|---|
| `ExactlyOne` | 必须且只能一个 | logger handler、registry provider |
| `OptionalOne` | 0 或 1 个 | 可选 writer、可选 profiler |
| `Many` | 0 到多个，无顺序 | transport provider、stats handler |
| `OrderedMany` | 多个，顺序来自配置 | interceptor、middleware |
| `NamedOne` | 同类下名称唯一 | 具名 resolver、具名 balancer |

### 5.2 静态校验

`Seal()` 时必须校验：

- spec name 非空；
- capability value 非 nil；
- 同名 capability 的基数一致；
- 类型声明一致；
- value 类型实现或可赋值给 spec type；
- `ExactlyOne / OptionalOne / NamedOne` 满足数量约束。

### 5.3 Resolve API

禁止“取第一个”。所有查询都必须表达基数意图：

```go
func ResolveExactlyOne[T any](h *Hub, spec CapabilitySpec) (T, error)
func ResolveOptionalOne[T any](h *Hub, spec CapabilitySpec) (T, bool, error)
func ResolveMany[T any](h *Hub, spec CapabilitySpec) ([]T, error)
func ResolveNamed[T any](h *Hub, spec CapabilitySpec, name string) (T, error)
func ResolveOrdered[T any](h *Hub, spec CapabilitySpec, names []string) ([]T, error)
```

`ResolveOrdered` 还需要校验配置列表中的重复、缺失名称和类型不匹配。

## 6. Scope 边界

```go
type Scope int
const (
    ScopeApp Scope = iota
    ScopeProvider
    ScopeRuntimeFactory
)
```

- `ScopeApp`：App 生命周期内常驻。
- `ScopeProvider`：提供能力 / 工厂，不持有高频实例。
- `ScopeRuntimeFactory`：用于按 service / endpoint 动态创建对象，不得注册进 Hub。

Hub 应拒绝 `ScopeRuntimeFactory` 模块，避免把高频动态对象放入全局生命周期管理。

## 7. Diagnostics

Hub diagnostics 应包含：

- 模块拓扑序与拓扑层级；
- 每个模块依赖列表；
- 已启动模块集合；
- capability 冲突；
- dependency errors；
- reload phase、failed module、failed stage、last error；
- restart-required 与 degraded / diverged 状态。

推荐结构：

```go
type ModuleDiag struct {
    Name                string
    DependsOn           []string
    TopoIndex           int
    TopoLayer           int
    Started             bool
    RestartRequired     bool
    ReloadPhase         string
    LastReloadError     string
    CapabilityConflicts []string
    DependencyErrors    []string
}
```

## 8. 自定义模块模板

```go
type MyModule struct {
    cfg MyConfig
    stopOnce sync.Once
    stopErr error
}

func (m *MyModule) Name() string { return "my.module" }
func (m *MyModule) DependsOn() []string { return []string{"foundation.runtime"} }
func (m *MyModule) ConfigPath() string { return "yggdrasil.my_module" }

func (m *MyModule) Init(ctx context.Context, view config.View) error {
    var cfg MyConfig
    if err := view.Decode(&cfg); err != nil { return err }
    m.cfg = cfg
    return nil
}

func (m *MyModule) Start(ctx context.Context) error { return nil }
func (m *MyModule) Stop(ctx context.Context) error {
    m.stopOnce.Do(func() { m.stopErr = m.close(ctx) })
    return m.stopErr
}
```
