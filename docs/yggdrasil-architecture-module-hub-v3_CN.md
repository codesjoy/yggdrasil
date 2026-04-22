# Yggdrasil 架构与设计文档（模块化内核版，v3）

> 本文档描述 Yggdrasil 在统一模块中心（Module Hub）架构下的目标设计。本文档仅关注目标架构、代码设计与运行时语义，不讨论兼容策略、迁移步骤与过渡实现。

## 目录

1. [概述](#1-概述)
2. [设计目标与原则](#2-设计目标与原则)
3. [整体架构](#3-整体架构)
4. [核心抽象](#4-核心抽象)
5. [依赖模型与排序规则](#5-依赖模型与排序规则)
6. [生命周期管理](#6-生命周期管理)
7. [配置系统与重加载](#7-配置系统与重加载)
8. [统一模块中心（Hub）](#8-统一模块中心hub)
9. [能力模型与冲突规则](#9-能力模型与冲突规则)
10. [作用域与运行时边界](#10-作用域与运行时边界)
11. [RPC 服务端](#11-rpc-服务端)
12. [RPC 客户端](#12-rpc-客户端)
13. [拦截器与中间件](#13-拦截器与中间件)
14. [远程协议抽象层](#14-远程协议抽象层)
15. [服务注册、发现与负载均衡](#15-服务注册发现与负载均衡)
16. [可观测性](#16-可观测性)
17. [推荐包结构](#17-推荐包结构)
18. [关键接口汇总](#18-关键接口汇总)
19. [配置结构参考](#19-配置结构参考)
20. [与 gRPC / HTTP 的关系](#20-与-grpc--http-的关系)

---

## 1. 概述

### 1.1 框架定位

Yggdrasil 是一个基于 Go 语言开发的微服务框架库。业务应用通过显式组合创建 `App` 实例，并通过统一模块中心（Hub）装配日志、传输协议、注册中心、服务发现、拦截器、可观测性等能力。

框架提供三类核心能力：

- 稳定的 RPC / REST 微服务运行时。
- 一个统一的扩展能力装配模型。
- 一个面向实例的应用容器，而非依赖包级全局状态的进程级单例。

### 1.2 核心能力

| 能力 | 说明 |
|------|------|
| 多协议 RPC | 通过远程协议抽象层支持 gRPC、HTTP 等传输协议 |
| REST 网关 | 基于 chi 提供 HTTP/REST 接口，支持 RPC 映射与原始 Handler |
| 服务发现 | 通过 Resolver 模块支持静态端点与动态注册中心 |
| 负载均衡 | 通过 Balancer / Picker 模块支持端点选择与状态更新 |
| 统一模块中心 | 使用 `Hub` 管理所有扩展能力，替代分散的 builder registry |
| 配置管理 | 保留分层优先级配置模型，统一编译并向模块与核心子系统分发 |
| 可观测性 | 日志、指标、链路追踪、stats handler 统一纳入模块化装配 |
| 链式扩展 | 拦截器与 REST middleware 作为正式的有序扩展点建模 |

### 1.3 关键结论

本版架构在模块化方向上保持简洁，同时把运行时稳定性语义补全到可工程化落地的程度：

1. 模块依赖采用显式 `DependsOn()` + DAG 校验，`InitOrder()` 仅用于同层 tie-breaker。
2. 生命周期启动失败必须进行逆序补偿停止，并通过幂等 `Stop()` 保障重复关闭安全。
3. 能力查询必须具备明确的基数与冲突规则，禁止含糊的“取第一个”。
4. 基数规则必须由 capability owner 显式声明，并在 `Seal()` / `Resolve()` 两阶段校验。
5. 配置重加载采用 staged reload 语义，并定义 rollback 失败时的降级状态与 `restart-required` 策略。
6. DAG 校验失败必须输出可读的缺失依赖路径、环路径与候选模块信息。
7. Hub 仅管理低频、长期存在、可诊断的模块；高频动态对象必须由对应子系统运行时管理。

---

## 2. 设计目标与原则

### 2.1 设计目标

1. **单一装配入口**：所有扩展能力统一通过 `Hub` 显式装配。
2. **去全局化**：扩展能力的状态、实例和配置绑定归属于 `App` 实例。
3. **核心稳定**：新增扩展点时不修改核心枚举，不引入新的中心化注册表族。
4. **领域自定义能力接口**：日志、传输、拦截器、registry、resolver 等能力由各自子系统定义 capability 接口。
5. **链式扩展可预测**：拦截器与 middleware 的最终执行顺序只来自配置。
6. **配置与运行时分离**：配置系统负责读取、合并、编译；Hub 负责分发结构化配置视图并协调重加载。
7. **生命周期集中管理**：模块初始化、启动、停止、重配置由 `App` / `Hub` 统一协调。
8. **边界硬约束**：Hub 不承载 per-request / per-endpoint / per-stream 的高频动态对象。

### 2.2 设计原则

#### 原则一：核心只定义组织结构，不定义业务能力枚举

核心仅提供：

- `App`
- `Hub`
- `Module`
- 生命周期接口
- 依赖声明接口
- 查询与装配工具

具体能力（如 `TransportServerProvider`、`TracerProvider`、`UnaryServerInterceptorProvider`）由所属子系统定义。

#### 原则二：模块依赖顺序与调用链顺序分离

- 模块的初始化 / 启动 / 停止顺序由显式依赖 DAG 决定。
- 拦截器与 middleware 的执行顺序由配置中的有序名称列表决定。

#### 原则三：字符串仅保留在配置边界

运行时可以通过名称查找模块与能力，但核心消费的是类型安全 capability 接口与结构化配置视图，而不是裸字符串分发。

#### 原则四：Hub 管理“能力载体”，不管理“高频动态实例”

Hub 持有的是长期存在、可诊断、低频变化的模块实例。按 service / endpoint / stream 变化的对象，仍由对应核心子系统在运行时根据 capability 动态创建与管理。

#### 原则五：失败语义必须明确

- 启动失败必须回滚已启动模块。
- 重加载失败不得隐式部分提交。
- 查询冲突必须报错，不能“随机选中一个实现”。

---

## 3. 整体架构

### 3.1 整体架构图

```text
+============================================================================+
|                              Business Application                           |
|       app := yggdrasil.New("my-app", yggdrasil.WithModules(...))           |
+============================================================================+
                                  |
                                  v
+----------------------------------------------------------------------------+
|                                    App                                     |
|  - Hub                                                                      |
|  - Config Manager                                                           |
|  - Settings Compiler                                                        |
|  - Application Lifecycle                                                    |
|  - Core Runtime (Server / Client / Governor / Instance Info)                |
+----------------------------------------------------------------------------+
          |                         |                           |
          v                         v                           v
+--------------------+   +-------------------------+   +---------------------+
|   Config System    |   |       Module Hub        |   |   Core Subsystems   |
|  file/env/flag/... |   | Module / DAG / Query /  |   | server / client /   |
|  layered snapshot  |   | start-stop-reload       |   | application / admin |
+--------------------+   +-------------------------+   +---------------------+
                                   |
     +--------------+--------------+--------------+--------------+-----------+
     |              |              |              |              |           |
     v              v              v              v              v           v
+---------+   +-----------+   +-----------+   +-----------+   +------+  +--------+
| Logger  |   | Transport |   | Registry  |   | Resolver  |   | OTel |  | Stats  |
| Module  |   | Module    |   | Module    |   | Module    |   | ...  |  | ...    |
+---------+   +-----------+   +-----------+   +-----------+   +------+  +--------+
                                   |
                                   v
                        +------------------------------+
                        | Ordered Extension Points      |
                        | Interceptors / Middleware     |
                        +------------------------------+
```

### 3.2 核心分层

| 层次 | 职责 | 关键包 |
|------|------|--------|
| **入口层** | 创建 `App`、装配模块、暴露高层 API | `yggdrasil`, `app` |
| **容器层** | 模块注册、依赖校验、生命周期协调、能力查询 | `module` |
| **应用层** | 应用生命周期、信号处理、Hook、实例状态 | `application` |
| **核心服务层** | RPC 服务端、客户端、治理、服务描述 | `server`, `client`, `governor` |
| **传输层** | 协议抽象、编解码、REST 网关、凭证 | `remote` |
| **基础设施层** | 配置、日志、stats、registry、resolver、balancer、otel | `config`, `logger`, `stats`, `registry`, `resolver`, `balancer`, `otel` |

---

## 4. 核心抽象

### 4.1 App — 唯一组合根

`App` 是应用实例的唯一组合根，持有一切运行时状态。

```go
package app

type App struct {
    name string

    hub      *module.Hub
    cfg      *config.Manager
    resolved settings.Resolved

    lifecycle lifecycleState

    application *application.Application
    server      *server.Server
    governor    *governor.Server
}
```

`App` 负责：

- 持有 `Hub` 与配置系统。
- 编译配置并初始化核心子系统。
- 协调模块初始化、启动、停止与重配置。
- 创建服务端、客户端与治理端口。

### 4.2 Module — 最小核心抽象

```go
package module

type Module interface {
    Name() string
}
```

`Module` 本身不携带能力枚举，也不要求实现固定的大接口。能力通过子系统自定义的 capability 接口表达。

### 4.3 生命周期接口

```go
package module

import "context"

type Initializable interface {
    Init(ctx context.Context, view config.View) error
}

type Startable interface {
    Start(ctx context.Context) error
}

type Stoppable interface {
    Stop(ctx context.Context) error
}

type Reloadable interface {
    PrepareReload(ctx context.Context, view config.View) (ReloadCommitter, error)
}

type ReloadCommitter interface {
    Commit(ctx context.Context) error
    Rollback(ctx context.Context) error
}

// 可选：用于暴露 reload 的内部状态与诊断信息。
type ReloadReporter interface {
    ReloadState() ReloadState
}
```

职责划分：

- `Init`：读取配置、构造长期持有的静态资源，但不对外提供服务。
- `Start`：启动网络监听、后台 goroutine、watcher、provider 等运行时过程。
- `Stop`：释放资源，必须幂等；同一模块可能因启动补偿、正常停机、重加载切换失败而被重复调用。
- `PrepareReload`：验证新配置并准备待切换状态，不直接提交。
- `Commit / Rollback`：统一提交或回滚 staged reload 结果。
- `ReloadReporter`：向 diagnostics 暴露模块当前 reload 阶段与降级状态。

### 4.4 配置接口

模块不直接接收裸 `map[string]any`，而是接收带 decode 能力的 `config.View`：

```go
package config

type View interface {
    Path() string
    Decode(target any) error
    Sub(path string) View
    Exists() bool
}
```

模块若关心某段配置，显式声明路径：

```go
package module

type Configurable interface {
    ConfigPath() string
}
```

### 4.5 依赖声明接口

```go
package module

type Dependent interface {
    DependsOn() []string
}

type Ordered interface {
    InitOrder() int
}
```

依赖规则：

- `DependsOn()` 是主排序依据。
- `InitOrder()` 仅用于同一 DAG 层内的 tie-breaker。
- 禁止仅依赖 `InitOrder()` 表达强依赖关系。

### 4.6 Capability — 子系统自定义能力接口

例：日志 handler 能力。

```go
package logger

type HandlerProvider interface {
    module.Module
    Handler() (slog.Handler, error)
}
```

例：gRPC 服务端传输能力。

```go
package remote

type TransportServerProvider interface {
    module.Module
    Protocol() string
    NewServer(handle MethodHandle) (Server, error)
}
```

例：服务端一元拦截器能力。

```go
package interceptor

type UnaryServerInterceptorProvider interface {
    module.Module
    UnaryServerInterceptor() UnaryServerInterceptor
}
```

---

## 5. 依赖模型与排序规则

### 5.1 显式 DAG

Hub 在 `Init` 前必须先构建模块依赖图：

```text
Module A ----depends on----> Module B
Module B ----depends on----> Module C
```

流程：

1. 收集全部模块名称。
2. 读取每个模块的 `DependsOn()`。
3. 校验依赖目标是否存在。
4. 执行拓扑排序。
5. 对同一层级节点按 `InitOrder()` 升序排列。
6. 若检测到依赖环，直接初始化失败并输出环路径。

### 5.2 规则说明

- `DependsOn()` 用于表达强依赖。
- `InitOrder()` 不得替代强依赖，只能优化同层初始化顺序。
- 启动顺序使用拓扑序。
- 停止顺序使用拓扑序逆序。

### 5.3 示例

```go
func (m *tracerModule) DependsOn() []string {
    return []string{"logger.default"}
}

func (m *serverModule) DependsOn() []string {
    return []string{"tracer.default", "stats.default"}
}
```

### 5.4 诊断输出

Hub 必须暴露诊断快照：

- 最终拓扑序
- 每个模块的依赖列表
- DAG 校验结果
- 冲突或缺失依赖原因
- 最近一次成功拓扑序
- 缺失依赖与环检测的详细错误路径

错误输出必须满足“人可读、可直接排障”，至少遵循以下约束：

- 缺失依赖错误必须包含：出错模块、缺失依赖名、当前已注册模块清单中的候选项。
- 依赖环错误必须包含：完整环路径，而不是只输出 `cycle detected`。
- 若 `InitOrder()` 只在同层内排序，则 diagnostics 中应明确区分“拓扑层级”与“层内顺序”。

示例：

```text
module dependency cycle detected:
logger.default -> tracer.default -> server.default -> logger.default
```

```text
module "server.default" depends on missing module "stats.default"
available modules: logger.default, tracer.default, otel.stats
```

---

## 6. 生命周期管理

### 6.1 App 状态机

```text
+------+    Configure()    +------------+    Init()     +-------------+
| New  |------------------>| Configured |-------------> | Initialized |
+------+                   +------------+               +-------------+
                                                           |
                                                           | Start()
                                                           v
                                                     +-------------+
                                                     |   Running   |
                                                     +-------------+
                                                           |
                                                           | Stop()
                                                           v
                                                     +-------------+
                                                     |   Stopped   |
                                                     +-------------+
```

约束：

- `Stopped` 不可回退。
- `Start()` 不可重复调用。
- `Configure()` 与 `Init()` 必须在 `Start()` 前完成。

### 6.2 初始化序列

```text
New(appName, opts...)
    |
    |-- [1] create Config Manager
    |-- [2] create Hub and register modules
    |-- [3] build config chain
    |-- [4] compile settings.Resolved
    |-- [5] topological sort modules
    |-- [6] init core runtime settings
    |-- [7] hub.Init(ctx, snapshot)
    |-- [8] create application / server / governor
    +-- state = Initialized
```

### 6.3 启动序列与失败补偿

```text
App.Start(ctx)
    |
    |-- [1] hub.Start(ctx)
    |       |-- start module A
    |       |-- start module B
    |       |-- start module C (failed)
    |       +-- reverse stop: B -> A
    |
    |-- [2] build transport servers
    |-- [3] build interceptor / middleware chains
    |-- [4] application.Start()
    |-- [5] start server / governor
    |-- [6] register service instance
    +-- state = Running
```

规则：

- `hub.Start()` 仅对成功进入 `Start()` 的模块记录到 `started[]`。
- 任意模块 `Start()` 失败时，Hub 必须对 `started[]` 按逆序执行 `Stop()`。
- `Stop()` 必须幂等，以允许补偿式回滚与正常停机共用同一接口。
- 若补偿 `Stop()` 返回错误，Hub 需要聚合错误并继续执行剩余补偿，不得因单个模块停止失败而中断回滚流程。

### 6.4 Stop 幂等性的工程约束

幂等不是纯文档约定，必须通过统一工程模式保障：

1. 模块内部必须使用 `sync.Once`、原子状态位或等价机制保护底层资源释放逻辑。
2. 重复 `Stop()` 调用应返回第一次关闭结果或 `nil`，不得再次执行底层关闭动作。
3. 任何可能 panic 的底层关闭动作都必须被模块内封装为“无 panic、可返回 error”的安全关闭函数。
4. Hub 层不得假设第三方资源天然幂等，只能假设模块暴露的 `Stop()` 满足幂等契约。

推荐模式：

```go
// 伪代码
func (m *myModule) Stop(ctx context.Context) error {
    m.stopOnce.Do(func() {
        m.stopErr = m.closeResources(ctx)
    })
    return m.stopErr
}
```

### 6.5 关闭序列

```text
App.Stop(ctx)
    |
    |-- [1] deregister service instance
    |-- [2] stop server / governor / background workers
    |-- [3] application.Stop()
    |-- [4] hub.Stop(ctx)        // reverse topo order
    |-- [5] close config sources
    +-- state = Stopped
```

### 6.6 信号处理与优雅关闭

`application` 继续负责 `SIGINT` / `SIGTERM` 监听，但实际资源关闭顺序由 `App.Stop` 与 `Hub.Stop` 统一控制。关闭超时由配置控制，默认 30 秒。

---

## 7. 配置系统与重加载

### 7.1 分层优先级模型

配置系统延续分层优先级合并模型：

```text
PriorityOverride  = 5
PriorityFlag      = 4
PriorityEnv       = 3
PriorityRemote    = 2
PriorityFile      = 1
PriorityDefaults  = 0
```

各层按优先级与加载顺序合并，产出不可变快照。

### 7.2 配置读取与编译

```text
config sources
   |
   +-- file/env/flag/memory
   |
config.Manager
   |
   +-- layered snapshot
   |
internal/settings.Compile(root)
   |
   +-- settings.Resolved
   |
   +-- core subsystem settings
   +-- ordered extension names
   +-- module views
```

`settings.Resolved` 用于：

- 核心子系统（server / client / transport / logging / telemetry）的结构化配置。
- 有序扩展点的最终名称列表。
- 为模块提供更细粒度的配置视图。

### 7.3 staged reload 语义

重加载采用带降级语义的 staged reload 模型：

```text
Reload(new snapshot)
   |
   |-- [1] compile new resolved settings
   |-- [2] collect affected reloadable modules
   |-- [3] PrepareReload() for all affected modules
   |-- [4] if any prepare failed -> rollback prepared modules
   |-- [5] if all prepare succeeded -> Commit() in stable order
   |-- [6] if any commit failed -> stop further commits, mark diverged state
   |-- [7] rollback non-committed prepared modules when possible
   +-- [8] publish new snapshot only when commit fully succeeded
```

规则：

- 任一模块 `PrepareReload()` 失败时，不得进入部分提交状态。
- 已成功 prepare 的模块必须收到 `Rollback()`。
- 所有 `Commit()` 成功后，才更新当前活动快照。
- 不支持 `Reloadable` 的模块若受影响，必须标记 `restart-required`。
- `Rollback()` 失败时不得无限重试；框架必须进入 `ReloadDegraded` 状态并要求人工重启恢复。
- `Commit()` 过程中一旦出现失败，不承诺自动恢复到旧世界；框架必须停止后续提交并标记 `diverged` / `restart-required`。

### 7.4 ReloadState 与降级策略

Hub 维护模块与全局 reload 状态：

```go
type ReloadPhase string

const (
    ReloadPhaseIdle       ReloadPhase = "idle"
    ReloadPhasePreparing  ReloadPhase = "preparing"
    ReloadPhaseCommitting ReloadPhase = "committing"
    ReloadPhaseRollback   ReloadPhase = "rollback"
    ReloadPhaseDegraded   ReloadPhase = "degraded"
)

type ReloadState struct {
    Phase           ReloadPhase
    RestartRequired bool
    Diverged        bool
    FailedModule    string
    LastError       error
}
```

降级规则：

- `PrepareReload()` 失败：回滚所有已 prepare 模块；若 rollback 再失败，进入 `ReloadDegraded`。
- `Commit()` 失败：停止后续提交；对未 commit 但已 prepare 的模块尝试 rollback；整体进入 `RestartRequired`。
- `Rollback()` 失败：记录失败模块与阶段，暴露到 governor / diagnostics；不自动无限重试。
- `ReloadDegraded` 状态下只允许继续对外提供当前尚可工作的能力，不允许隐式再次 reload 覆盖错误状态。

### 7.5 restart-required 语义

当配置变化影响到不支持在线切换的模块，或 staged reload 进入降级状态时：

- 不尝试隐式重启模块。
- 记录 `restart-required`。
- 暴露到 governor / diagnostics 接口中。
- 明确要求通过人工或编排系统重启恢复到干净状态。

---

## 8. 统一模块中心（Hub）

### 8.1 Hub 职责

`Hub` 负责：

- 收集模块。
- 建立名称索引。
- 构建并校验依赖 DAG。
- 在 `Seal()` 阶段执行 capability 基数静态校验。
- 执行 `Init / Start / Stop / Reload` 生命周期。
- 提供带基数约束的 capability 查询。
- 根据配置解析有序链式扩展点。
- 输出诊断快照。

### 8.2 Hub 结构

```go
package module

type Hub struct {
    mu sync.RWMutex

    modules []Module
    index   map[string]Module
    sealed  bool

    topoOrder        []Module
    lastStableTopo   []Module
    started          []Module
    restartFlag      map[string]bool
    reloadState      ReloadState
}
```

### 8.3 注册与封闭

```go
func NewHub() *Hub
func (h *Hub) Use(modules ...Module)
func (h *Hub) Seal() error
```

规则：

- `Use()` 仅在 `Seal()` 前可调用。
- `Module.Name()` 必须全局唯一。
- 模块名冲突时直接报错。
- `Seal()` 阶段执行依赖图校验、拓扑排序与 capability 基数静态校验。

### 8.4 生命周期 API

```go
func (h *Hub) Init(ctx context.Context, snap config.Snapshot) error
func (h *Hub) Start(ctx context.Context) error
func (h *Hub) Stop(ctx context.Context) error
func (h *Hub) Reload(ctx context.Context, snap config.Snapshot) error
```

### 8.5 诊断 API

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

func (h *Hub) Modules() []ModuleDiag
```

---

## 9. 能力模型与冲突规则

### 9.1 Capability 基数

每类 capability 必须定义自己的基数语义：

| 类型 | 语义 | 示例 |
|------|------|------|
| `ExactlyOne` | 必须且只能存在一个实现 | 默认 `TracerProvider` |
| `OptionalOne` | 最多一个实现，可缺省 | 某类可选 writer |
| `Many` | 允许多个实现，无顺序语义 | 多个 transport provider |
| `OrderedMany` | 允许多个实现，顺序由配置决定 | interceptor / middleware |
| `NamedOne` | 按名称解析单个实现 | 某个具名 registry / resolver |

### 9.2 CapabilitySpec — 基数声明机制

基数不是由查询方临时决定，而必须由 capability owner 显式声明：

```go
type CapabilityCardinality int

const (
    ExactlyOne CapabilityCardinality = iota
    OptionalOne
    Many
    OrderedMany
    NamedOne
)

type CapabilitySpec struct {
    Name        string
    Cardinality CapabilityCardinality
    Type        reflect.Type
}
```

声明规则：

- capability 所属子系统负责定义 `CapabilitySpec`。
- `CapabilitySpec` 是查询、校验、diagnostics 的唯一基数来源。
- 禁止调用方跳过 `CapabilitySpec` 自行约定“这里应该只有一个实现”。

示例：

```go
var TracerProviderSpec = module.CapabilitySpec{
    Name:        "otel.tracer_provider",
    Cardinality: module.ExactlyOne,
    Type:        reflect.TypeOf((*otel.TracerProvider)(nil)).Elem(),
}
```

### 9.3 查询 API

禁止以“第一个实现”作为默认策略。Hub 必须暴露显式查询：

```go
func ResolveExactlyOne[T any](h *Hub, spec CapabilitySpec) (T, error)
func ResolveOptionalOne[T any](h *Hub, spec CapabilitySpec) (T, bool, error)
func ResolveMany[T any](h *Hub, spec CapabilitySpec) []T
func ResolveNamed[T any](h *Hub, spec CapabilitySpec, name string) (T, error)
func ResolveOrdered[T any](h *Hub, spec CapabilitySpec, names []string) ([]T, error)
```

### 9.4 校验时机

基数校验必须分两阶段完成：

1. **Seal 阶段静态校验**
   - 校验 `ExactlyOne` / `OptionalOne` 的实现数量。
   - 校验 capability 类型声明是否一致。
   - 建立 capability -> module 的索引与 diagnostics 快照。

2. **Resolve 阶段兜底校验**
   - 校验名称存在性。
   - 校验 ordered list 中的重复项与类型匹配。
   - 防止运行时绕过初始化阶段后拿到脏状态。

### 9.5 冲突处理规则

- `ExactlyOne` 若出现 0 个或超过 1 个实现，直接报错。
- `OptionalOne` 若出现超过 1 个实现，直接报错。
- `NamedOne` 若名称不存在或类型不匹配，直接报错。
- `OrderedMany` 若配置中名称不存在、重复引用或类型不匹配，直接报错。
- 任何 capability 冲突都必须进入 diagnostics，不得静默选择“第一个实现”。

### 9.6 示例

```go
tracer, err := module.ResolveExactlyOne[otel.TracerProvider](hub, otel.TracerProviderSpec)
if err != nil {
    return err
}

ints, err := module.ResolveOrdered[interceptor.UnaryServerInterceptorProvider](
    hub,
    interceptor.UnaryServerInterceptorSpec,
    resolved.Extensions.UnaryServerInterceptors,
)
if err != nil {
    return err
}
```

---

## 10. 作用域与运行时边界

### 10.1 Scope 分类

```go
type Scope int

const (
    ScopeApp Scope = iota
    ScopeProvider
    ScopeRuntimeFactory
)
```

语义：

- `ScopeApp`：App 生命周期内常驻。
- `ScopeProvider`：仅提供能力 / 工厂，本身不持有高频动态实例。
- `ScopeRuntimeFactory`：用于在 server/client 运行时按 service / endpoint 动态创建对象。

### 10.2 Hub 的边界

Hub 只能持有以下对象：

- 日志 handler / writer 模块
- tracing / metrics provider 模块
- transport server / client provider 模块
- registry / resolver / credentials 提供者模块
- interceptor / middleware 提供者模块

Hub 不得直接持有以下对象：

- per-request 对象
- per-stream 对象
- per-endpoint 连接与 picker
- 动态 resolver watch 实例
- balancer 运行时状态对象

### 10.3 强约束

文档与代码层必须同时遵守以下规则：

1. `Hub.Use()` 禁止注册高频动态对象。
2. server / client 子系统只能从 Hub 中拿 provider / factory，再自行创建动态对象。
3. 动态对象的生命周期由其所属子系统管理，不由 Hub 管理。

---

## 11. RPC 服务端

### 11.1 服务描述模型

```go
type ServiceDesc struct {
    ServiceName string
    HandlerType interface{}
    Methods     []MethodDesc
    Streams     []stream.Desc
    Metadata    interface{}
}

type RestServiceDesc struct {
    HandlerType interface{}
    Methods     []RestMethodDesc
}

type RestRawHandlerDesc struct {
    Method  string
    Path    string
    Handler http.HandlerFunc
}
```

### 11.2 服务端初始化流程

```text
App.Start()
   |
   |-- resolve transport server providers
   |-- build unary / stream interceptor chains
   |-- create remote.Server instances by protocol
   |-- register ServiceDesc / RestServiceDesc / RestRawHandlerDesc
   |-- start application server
```

### 11.3 统一分发

服务端继续通过 `/service/method` 路径分发到 unary / stream handler。模块化架构不改变协议无关分发模型，只替换能力装配来源。

---

## 12. RPC 客户端

### 12.1 客户端架构分层

```text
Business Call
   |
   v
Client Interface
   |
   v
Interceptor Chain
   |
   v
Balancer / Picker
   |
   v
remoteClientManager
   |
   v
remote.Client
```

### 12.2 与 Hub 的关系

Hub 提供：

- resolver provider
- balancer provider
- transport client provider
- credentials provider
- client interceptor provider

client 子系统负责：

- 根据 service 配置选择 provider
- 创建 resolver watch
- 创建 balancer 实例
- 管理 endpoint 连接池
- 管理 picker 与连接状态变更

### 12.3 边界说明

resolver watch、picker、连接池等对象均属于运行时动态对象，不注册到 Hub。

---

## 13. 拦截器与中间件

### 13.1 链式扩展点模型

链式扩展点由三部分组成：

1. 模块提供具名 provider。
2. 配置给出最终有序名称列表。
3. 子系统按配置顺序解析并组链。

### 13.2 顺序来源

最终执行顺序必须完全来自配置，而不是：

- 模块注册顺序
- DAG 顺序
- `InitOrder()`
- map 遍历顺序

### 13.3 构造 API

```go
ints, err := module.ResolveOrdered[
    interceptor.UnaryServerInterceptorProvider,
](hub, interceptor.UnaryServerInterceptorSpec, resolved.Extensions.UnaryServerInterceptors)

chain := interceptor.ChainUnaryServerInterceptors(ints)
```

### 13.4 校验规则

- 重复引用默认视为配置错误。
- 名称不存在视为配置错误。
- provider 类型不匹配视为配置错误。
- 如确有必要允许重复，必须由对应子系统显式声明。

---

## 14. 远程协议抽象层

### 14.1 协议 provider

```go
package remote

type TransportServerProvider interface {
    module.Module
    Protocol() string
    NewServer(handle MethodHandle) (Server, error)
}

type TransportClientProvider interface {
    module.Module
    Protocol() string
    NewClient(ctx context.Context, endpoint Endpoint, opts ClientOptions) (Client, error)
}
```

### 14.2 运行时语义

- Hub 仅持有 provider。
- server / client 在运行时根据协议选择 provider 并创建实例。
- 具体的连接状态、重连、backoff、流控等行为仍由 remote 子系统管理。

---

## 15. 服务注册、发现与负载均衡

### 15.1 Provider 能力

```go
package registry

type Provider interface {
    module.Module
    Type() string
    New(config RegistryConfig) (Registry, error)
}
```

```go
package resolver

type Provider interface {
    module.Module
    Type() string
    New(config ResolverConfig) (Resolver, error)
}
```

```go
package balancer

type Provider interface {
    module.Module
    Type() string
    New(service string, cfg Config, c Client) (Balancer, error)
}
```

### 15.2 作用域

- provider 属于 Hub 管理对象。
- registry / resolver / balancer 的具体运行时实例由 application / client 子系统创建。
- resolver.State、picker、balancer 内部状态不进入 Hub。

---

## 16. 可观测性

### 16.1 日志

日志系统由以下能力组成：

- `logger.HandlerProvider`
- `logger.WriterProvider`
- `logger.Core`

默认 logger 解析必须使用 `ResolveExactlyOne` / `ResolveNamed`，不能依赖“找到第一个可用实现”。

### 16.2 Tracer / Meter

Tracer Provider 与 Meter Provider 通常属于 `ExactlyOne` 或 `OptionalOne` 能力点。出现多个默认 provider 时必须立即报错。

### 16.3 Stats

Stats handler 属于 capability provider，由 server/client 子系统在启动时按角色装配。

### 16.4 Diagnostics

Governor 或调试接口应暴露：

- 模块拓扑序与拓扑层级
- 最近一次成功拓扑序
- 已启动模块集合
- restart-required 状态
- reload 当前阶段与最后错误
- capability 冲突信息
- 依赖缺失错误与环路径
- `ReloadDegraded` / `Diverged` 状态
- 失败模块与失败阶段（prepare / commit / rollback）

---

## 17. 推荐包结构

```text
yggdrasil/
  app/
    app.go
    lifecycle.go

  module/
    module.go          // Module / Configurable / Dependent / Ordered / ReloadReporter
    hub.go             // Hub core
    dag.go             // topo sort + cycle detection + readable errors
    lifecycle.go       // Init / Start / Stop / Reload orchestration
    resolve.go         // CapabilitySpec + ResolveExactlyOne / ResolveOrdered ...
    diagnostics.go     // ModuleDiag / reload state / dependency errors
    scope.go           // Scope definitions

  config/
    manager.go
    snapshot.go
    view.go
    source/

  internal/settings/
    compile.go
    settings.go

  server/
  client/
  app/
  governor/

  remote/
    remote.go
    transport/grpc/
    transport/rpchttp/
    credentials/
    marshaler/

  logger/
  stats/
  otel/
  registry/
  resolver/
  balancer/
  interceptor/
```

---

## 18. 关键接口汇总

| 接口 | 包 | 方法 | 用途 |
|------|-----|------|------|
| `Module` | `module` | `Name()` | 最小核心抽象 |
| `Dependent` | `module` | `DependsOn()` | 显式依赖声明 |
| `Ordered` | `module` | `InitOrder()` | 同层 tie-breaker |
| `Configurable` | `module` | `ConfigPath()` | 声明配置路径 |
| `Initializable` | `module` | `Init(ctx, view)` | 模块初始化 |
| `Startable` | `module` | `Start(ctx)` | 模块启动 |
| `Stoppable` | `module` | `Stop(ctx)` | 模块停止 |
| `Reloadable` | `module` | `PrepareReload(ctx, view)` | staged reload |
| `ReloadCommitter` | `module` | `Commit() / Rollback()` | staged reload 提交与回滚 |
| `ReloadReporter` | `module` | `ReloadState()` | 暴露 reload 诊断状态 |
| `CapabilitySpec` | `module` | `Name`, `Cardinality`, `Type` | capability 基数声明 |
| `View` | `config` | `Decode()`, `Sub()`, `Exists()` | 结构化配置视图 |
| `TransportServerProvider` | `remote` | `Protocol()`, `NewServer()` | 协议服务端能力 |
| `TransportClientProvider` | `remote` | `Protocol()`, `NewClient()` | 协议客户端能力 |
| `HandlerProvider` | `logger` | `Handler()` | 日志 handler 能力 |
| `Provider` | `registry/resolver/balancer` | `Type()`, `New()` | 注册、发现、均衡能力 |

---

## 19. 配置结构参考

```yaml
yggdrasil:
  server:
    transports: [grpc, http]

  transports:
    grpc:
      server:
        address: ":9090"
      client:
        connect_timeout: 10s
    http:
      server:
        address: ":8080"

  clients:
    defaults:
      resolver: default-resolver
      balancer: round_robin
    services:
      my-service:
        resolver: etcd-resolver
        balancer: round_robin

  discovery:
    registry:
      name: consul-registry
    resolvers:
      default-resolver:
        type: static
      etcd-resolver:
        type: etcd

  logging:
    default_handler: json-handler
    default_writer: file-writer

  telemetry:
    tracer: otel-tracer
    meter: otel-meter
    stats:
      server: otel-stats
      client: otel-stats

  extensions:
    interceptors:
      unary_server:
        - recovery
        - logging
        - metrics
      unary_client:
        - logging
        - metrics
    middleware:
      rest_all:
        - logging
      rest_rpc:
        - marshaler
      rest_web: []
```

说明：

- 配置中的字符串仅用于引用模块或 capability 名称。
- 链式扩展点的顺序仅由列表顺序决定。
- provider 的具名引用由 `ResolveNamed` / `ResolveOrdered` 解析。

---

## 20. 与 gRPC / HTTP 的关系

### 20.1 协议无关核心

Yggdrasil 的核心 RPC 层（server / client / interceptor）仍保持协议无关。协议相关逻辑被封装在 `remote` 层，通过 provider 注入。

### 20.2 gRPC / HTTP 作为 provider

- gRPC 服务端 / 客户端通过 `TransportServerProvider` / `TransportClientProvider` 接入。
- HTTP / REST 服务端通过相同模式接入。
- 核心不直接依赖具体协议实现的包级注册副作用。

### 20.3 收益

这种设计意味着：

- 业务应用只需要显式装配所需协议模块。
- 可以在不修改核心容器逻辑的情况下新增协议实现。
- 协议 provider 的选择、依赖与配置都进入统一 Hub / App 管理模型。

---

## 结语

本版模块化内核设计将 Yggdrasil 的扩展体系收敛到一个统一的 `App + Hub + Module + Capability` 模型中，同时通过显式 DAG、可读诊断、失败补偿、能力基数声明与校验、带降级语义的 staged reload，以及 scope 约束，补齐统一容器架构在复杂微服务系统中必须具备的稳定性语义。

这意味着：

- 装配路径是显式的。
- 依赖关系是可校验的。
- 启停失败语义是可恢复的。
- 重加载是一致性的。
- 高动态对象与长期模块对象的边界是清晰的。

该模型在保持核心简洁的同时，为 Yggdrasil 的 RPC、REST、可观测性与基础设施能力提供了统一而严格的代码设计基础。
