# Kubernetes 服务发现示例

本示例演示如何使用 contrib/k8s 作为服务发现中心，从 Kubernetes 发现服务实例，并实时监听实例变更，驱动负载均衡。

## 你会得到什么

- 从 Kubernetes 发现指定服务的所有实例
- 实时监听实例变更（新增、删除、更新）
- 支持 Endpoints 和 EndpointSlice 两种模式
- 自动过滤不匹配 namespace 和 protocol 的实例
- 端点元信息管理（ nodeName、zone 等）
- 实时演示服务发现效果

## 前置条件

1. 有一个可访问的 Kubernetes 集群
2. 集群中已部署了要发现的服务
3. 本地安装了 Go 1.19+
4. kubectl 已配置好集群访问权限

### 快速启动本地 Kubernetes

如果你没有 Kubernetes 集群，可以使用以下工具之一快速启动：

**使用 Minikube：**
```bash
minikube start
```

**使用 Kind：**
```bash
kind create cluster
```

**使用 Docker Desktop：**
```bash
# Docker Desktop 内置了 Kubernetes，在设置中启用即可
```

验证 Kubernetes 集群是否正常运行：
```bash
kubectl cluster-info
kubectl get nodes
```

## 启动方式

### 1. 部署测试服务

首先在 Kubernetes 中部署一个测试服务：

```bash
# 创建部署
kubectl create deployment nginx --image=nginx:latest -n default

# 创建服务
kubectl expose deployment nginx --port=80 --target-port=80 -n default

# 查看服务
kubectl get svc nginx -n default
```

或者使用 YAML 文件：

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx
  namespace: default
spec:
  replicas: 3
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      labels:
        app: nginx
    spec:
      containers:
      - name: nginx
        image: nginx:latest
        ports:
        - containerPort: 80
---
apiVersion: v1
kind: Service
metadata:
  name: nginx
  namespace: default
spec:
  selector:
    app: nginx
  ports:
  - port: 80
    targetPort: 80
  type: ClusterIP
```

```bash
kubectl apply -f service.yaml
```

### 2. 验证服务

查看服务的 Endpoints：
```bash
kubectl get endpoints nginx -n default -o yaml
```

查看服务的 EndpointSlice：
```bash
kubectl get endpointslices -n default -l kubernetes.io/service-name=nginx -o yaml
```

### 3. 修改配置（可选）

如果需要修改配置，请编辑 [config.yaml](config.yaml)：

```yaml
yggdrasil:
  resolver:
    k8s-resolver:
      type: kubernetes
      config:
        namespace: default          # Kubernetes 命名空间
        mode: endpointslice         # 模式：endpoints 或 endpointslice
        portName: http            # 端口名称（可选）
        port: 0                   # 端口号（可选，与 portName 二选一）
        protocol: http             # 协议类型
        kubeconfig: ""             # kubeconfig 文件路径（空表示使用 in-cluster config）
        resyncPeriod: 30s         # 重新同步周期
        timeout: 10s               # 超时时间
        backoff:
          baseDelay: 1s           # 基础退避延迟
          multiplier: 1.6          # 退避倍数
          jitter: 0.2              # 抖动系数
          maxDelay: 30s           # 最大退避延迟
```

### 4. 运行示例

```bash
cd example/resolver
go run main.go
```

## 预期输出

```
INFO client created, press Ctrl+C to exit...
INFO discovered endpoint: http://10.244.0.5:80, protocol=http, nodeName=minikube, zone=
INFO discovered endpoint: http://10.244.0.6:80, protocol=http, nodeName=minikube, zone=
INFO discovered endpoint: http://10.244.0.7:80, protocol=http, nodeName=minikube, zone=
```

## 测试服务发现

### 扩容服务

```bash
kubectl scale deployment nginx --replicas=5 -n default
```

观察程序输出，应该能看到新增的端点。

### 缩容服务

```bash
kubectl scale deployment nginx --replicas=2 -n default
```

观察程序输出，应该能看到端点减少。

### 更新服务

```bash
kubectl set image deployment/nginx nginx=nginx:1.25 -n default
```

观察程序输出，端点会更新（IP 可能变化）。

### 删除服务

```bash
kubectl delete service nginx -n default
```

观察程序输出，应该能看到所有端点被删除。

## 配置说明

### ResolverConfig 参数

| 参数 | 类型 | 说明 | 默认值 |
|------|------|------|--------|
| `Namespace` | string | Kubernetes 命名空间 | 从环境变量 `KUBERNETES_NAMESPACE` 或 `default` |
| `Mode` | string | 模式：`endpoints` 或 `endpointslice` | `endpointslice` |
| `PortName` | string | 端口名称 | 空（使用第一个端口） |
| `Port` | int32 | 端口号 | 0（根据 PortName 选择） |
| `Protocol` | string | 协议类型 | `grpc` |
| `Kubeconfig` | string | kubeconfig 文件路径 | 空（使用 in-cluster config） |
| `ResyncPeriod` | time.Duration | 重新同步周期 | 0（不重新同步） |
| `Timeout` | time.Duration | 超时时间 | 0（无超时） |
| `Backoff` | backoffConfig | 退避配置 | 见下方 |

### Backoff 配置

| 参数 | 类型 | 说明 | 默认值 |
|------|------|------|--------|
| `BaseDelay` | time.Duration | 基础退避延迟 | `1s` |
| `Multiplier` | float64 | 退避倍数 | `1.6` |
| `Jitter` | float64 | 抖动系数（0-1） | `0.2` |
| `MaxDelay` | time.Duration | 最大退避延迟 | `30s` |

### 模式说明

**Endpoints 模式：**
```yaml
mode: endpoints
```
- 使用 Kubernetes `Endpoints` API
- 适合 Kubernetes 1.17 之前的版本
- 不支持大规模集群（Endpoints 有大小限制）

**EndpointSlice 模式（推荐）：**
```yaml
mode: endpointslice
```
- 使用 Kubernetes `EndpointSlice` API
- 适合 Kubernetes 1.17+ 版本
- 支持大规模集群（每个 EndpointSlice 最多 100 个端点）
- 支持拓扑感知路由（zone-aware）

### 工作原理

```
1. 添加 Watch
   ↓
2. Watch Kubernetes Endpoints/EndpointSlice
   ↓
3. 收到变更事件
   ↓
4. 全量拉取当前端点列表
   ↓
5. 过滤不匹配的端点（namespace、port、protocol）
   ↓
6. 转换为 Yggdrasil State
   ↓
7. 通知所有 Watcher
```

### 端点属性

每个端点包含以下属性：

| 属性 | 说明 | 来源 |
|------|------|------|
| `address` | 端点地址（IP:Port） | Endpoints/EndpointSlice |
| `protocol` | 协议类型（grpc/http） | 配置 |
| `nodeName` | 节点名称 | TargetRef |
| `zone` | 可用区 | Zone |
| `podName` | Pod 名称 | TargetRef |

### 代码结构说明

```go
// 1. 初始化 Yggdrasil 框架
if err := yggdrasil.Init("k8s-resolver-example"); err != nil {
    panic(err)
}

// 2. 获取 Client
cli, err := yggdrasil.NewClient("downstream-service")
if err != nil {
    panic(err)
}
defer cli.Close()

// 3. 创建自定义 Watcher
watcher := &stateLogger{}

// 4. 添加 Watch
res, err := resolver.Get("k8s-resolver")
if err != nil {
    panic(err)
}
if err := res.AddWatch("nginx", watcher); err != nil {
    panic(err)
}

// 5. 实现 Watcher 接口
type stateLogger struct{}

func (sl *stateLogger) UpdateState(st resolver.State) {
    for _, ep := range st.GetEndpoints() {
        attrs := ep.GetAttributes()
        slog.Info("discovered endpoint",
            "address", ep.GetAddress(),
            "protocol", ep.GetProtocol(),
            "nodeName", attrs["nodeName"],
            "zone", attrs["zone"],
        )
    }
}

// 6. 等待信号
sig := make(chan os.Signal, 1)
signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
<-sig

// 7. 清理资源
yggdrasil.Stop()
```

## 高级用法

### 多服务 Watch

同时监听多个服务：

```go
services := []string{"service-a", "service-b", "service-c"}
for _, svc := range services {
    watcher := &stateLogger{}
    if err := res.AddWatch(svc, watcher); err != nil {
        slog.Error("add watch failed", "service", svc, "error", err)
    }
}
```

### 指定端口

**使用端口名称（推荐）：**
```yaml
portName: grpc
```

**使用端口号：**
```yaml
port: 9090
```

### Namespace 隔离

不同环境使用不同的 namespace：

```yaml
yggdrasil:
  resolver:
    dev-resolver:
      config:
        namespace: dev  # 开发环境
    prod-resolver:
      config:
        namespace: prod  # 生产环境
```

### 重新同步

定期重新同步端点列表：

```yaml
resyncPeriod: 30s  # 每 30 秒重新同步一次
```

### 超时控制

设置操作超时：

```yaml
timeout: 10s  # 10 秒超时
```

### 与负载均衡集成

Resolver 发现的端点可以直接用于负载均衡：

```go
import "github.com/codesjoy/yggdrasil/v2/balancer"

// 获取负载均衡器
bal := balancer.Get("round_robin")

// 将 Resolver 注册为 Balancer 的 Watcher
res, _ := resolver.Get("k8s-resolver")
res.AddWatch("nginx", bal)

// 使用负载均衡器选择端点
endpoint, err := bal.Pick(context.Background())
if err != nil {
    slog.Error("pick failed", "error", err)
    return
}
slog.Info("selected", "endpoint", endpoint.Address())
```

### 拓扑感知路由

EndpointSlice 模式支持 zone-aware 路由：

```go
// 从端点属性获取 zone 信息
for _, ep := range st.GetEndpoints() {
    attrs := ep.GetAttributes()
    zone := attrs["zone"]
    nodeName := attrs["nodeName"]
    
    // 根据 zone 选择最优端点
    if zone == currentZone {
        // 优先选择同 zone 的端点
    }
}
```

## RBAC 权限

程序需要以下 RBAC 权限才能访问 Endpoints/EndpointSlice：

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  namespace: default
  name: endpoints-reader
rules:
- apiGroups: [""]
  resources: ["endpoints"]
  verbs: ["get", "list", "watch"]
- apiGroups: ["discovery.k8s.io"]
  resources: ["endpointslices"]
  verbs: ["get", "list", "watch"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: endpoints-reader
  namespace: default
subjects:
- kind: ServiceAccount
  name: your-serviceaccount
roleRef:
  kind: Role
  name: endpoints-reader
  apiGroup: rbac.authorization.k8s.io
```

## 常见问题

**Q: 为什么没有发现任何端点？**

A: 检查以下几点：
1. 确认服务存在：`kubectl get svc nginx -n default`
2. 确认服务有 Endpoints：`kubectl get endpoints nginx -n default`
3. 确认 namespace 配置正确
4. 确认端口配置正确
5. 检查 RBAC 权限

**Q: Endpoints 和 EndpointSlice 模式有什么区别？**

A: 主要区别：
- Endpoints：旧版 API，不适合大规模集群
- EndpointSlice：新版 API，支持大规模集群和拓扑感知路由
- 推荐使用 EndpointSlice 模式（Kubernetes 1.17+）

**Q: 如何选择端口？**

A: 有两种方式：
1. 使用 `portName`（推荐）：根据端口名称选择
2. 使用 `port`：根据端口号选择
- 如果都不指定，使用第一个端口

**Q: 端点更新后没有收到通知？**

A: 可能的原因：
1. 服务没有变化（相同内容不会触发更新）
2. Pod 没有正常运行：`kubectl get pods -n default`
3. Kubernetes 集群网络问题
4. Watcher channel 满，丢弃了更新

**Q: 如何查看当前发现的端点？**

A: 使用 kubectl 查看：
```bash
kubectl get endpoints nginx -n default -o yaml
kubectl get endpointslices -n default -l kubernetes.io/service-name=nginx -o yaml
```

**Q: 支持多个 Resolver 吗？**

A: 支持，可以为不同的服务使用不同的 Resolver：
```yaml
yggdrasil:
  resolver:
    default:
      type: kubernetes
      config:
        namespace: default
    special:
      type: kubernetes
      config:
        namespace: special
```

**Q: 如何使用不同的 kubeconfig 文件？**

A: 设置 `Kubeconfig` 参数：
```yaml
config:
  kubeconfig: "/path/to/kubeconfig.yaml"
```

## 最佳实践

1. **使用 EndpointSlice 模式**：推荐使用 EndpointSlice 模式（Kubernetes 1.17+）
2. **使用端口名称**：推荐使用 `portName` 而非 `port`，更具语义化
3. **Namespace 隔离**：不同环境使用不同的 namespace
4. **合理设置 ResyncPeriod**：定期重新同步，确保数据一致性
5. **设置超时**：根据业务需求设置合理的超时时间
6. **监控发现状态**：监控端点数量、更新频率等指标
7. **拓扑感知**：利用 EndpointSlice 的 zone 信息实现拓扑感知路由
8. **RBAC 权限**：使用最小权限原则配置 RBAC
9. **优雅停止**：在应用停止时调用 `DelWatch` 停止监听
10. **日志记录**：记录端点变更日志，便于审计和排查问题

## 相关文档

- [k8s 主文档](../../readme.md)
- [ConfigMap 配置源示例](../config-source/)
- [Secret 配置源示例](../secret-source/)
- [yggdrasil resolver 文档](../../../../docs/resolver.md)

## 退出

按 `Ctrl+C` 退出程序。
