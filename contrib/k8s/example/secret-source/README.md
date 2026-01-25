# Secret 配置源示例

本示例演示如何使用 contrib/k8s 的 Secret 配置源功能，从 Kubernetes Secret 读取配置并支持配置热更新。

## 你会得到什么

- 从 Kubernetes Secret 加载配置
- 自动解析 YAML/JSON/TOML 格式（根据文件扩展名自动识别）
- 支持配置热更新，监听 Secret 变化
- 按优先级管理多个配置源
- 适合存储敏感配置（密码、密钥等）
- 实时演示配置变更通知

## 前置条件

1. 有一个可访问的 Kubernetes 集群
2. 已在集群中部署了包含配置的 Secret
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

### 1. 创建 Secret

首先在 Kubernetes 中创建包含配置的 Secret：

```bash
kubectl create secret generic example-secret --from-literal=config.yaml='message: "Hello from Secret!"' -n default
```

或者使用 YAML 文件创建：

```bash
kubectl apply -f secret.yaml
```

**注意：**
- `data` 字段的值必须是 Base64 编码
- `stringData` 字段的值会自动进行 Base64 编码（推荐使用）
- 不要将 Secret 提交到版本控制系统

### 2. 验证 Secret

查看 Secret 内容：
```bash
kubectl get secret example-secret -n default -o yaml
```

### 3. 修改配置（可选）

如果 Kubernetes 集群地址不是默认的，请修改 [main.go](main.go) 中的 Namespace 和 Name 配置：

```go
src, err := k8s.NewSecretSource(k8s.ConfigSourceConfig{
    Namespace: "your-namespace",
    Name:      "your-secret",
    Key:       "config.yaml",
    Watch:     true,
    Priority:  source.PriorityRemote,
})
```

或者通过环境变量修改（如果使用环境变量读取配置）：

```bash
export KUBERNETES_NAMESPACE=your-namespace
```

### 4. 运行示例

```bash
cd example/secret-source
go run main.go
```

程序将从 Secret 中读取配置并监听变化。

## 预期输出

```
watching secret changes, press Ctrl+C to exit...
config changed: type=1, version=1
message: Hello from Secret!
```

## 测试配置更新

### 方式 1：使用 kubectl 更新

```bash
kubectl patch secret example-secret -n default --type='json' -p='{"data":{"config.yaml":"bWVzc2FnZTogVXBkYXRlZCBtZXNzYWdlIQp2ZXJzaW9uOiAiMi4w"}}'
```

或者直接替换：
```bash
kubectl create secret generic example-secret --from-literal=config.yaml='message: "Updated message!"' --dry-run=client -o yaml | kubectl apply -f -
```

### 方式 2：使用 YAML 文件更新

创建更新后的 Secret YAML 文件：

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: example-secret
  namespace: default
type: Opaque
stringData:
  config.yaml: |
    message: "Updated from YAML file!"
    version: "2.0"
    database:
      host: db.example.com
      port: 5432
      username: admin
      password: new-secret-password
```

应用更新：
```bash
kubectl apply -f updated-secret.yaml
```

观察程序输出，应该能看到配置变化的通知：
```
config changed: type=2, version=2
message: Updated from YAML file!
```

### 方式 3：使用 kubectl edit

```bash
kubectl edit secret example-secret -n default
```

编辑后保存，程序会自动检测到变化。

## 配置说明

### ConfigSourceConfig 参数

| 参数 | 类型 | 说明 | 默认值 |
|------|------|------|--------|
| `Namespace` | string | Kubernetes 命名空间 | 从环境变量 `KUBERNETES_NAMESPACE` 或 `default` |
| `Name` | string | Secret 名称 | 必填 |
| `Key` | string | Secret 中的 key | 必填 |
| `MergeAllKey` | bool | 是否合并所有 key | `false` |
| `Format` | Parser | 配置解析器 | 根据文件扩展名自动推断 |
| `Priority` | Priority | 配置优先级 | `source.PriorityRemote` |
| `Watch` | bool | 是否监听配置变更 | `false` |
| `Kubeconfig` | string | kubeconfig 文件路径 | 空（使用 in-cluster config） |

### 工作模式

**单 Key 模式（默认）：**
```go
src, err := k8s.NewSecretSource(k8s.ConfigSourceConfig{
    Namespace: "default",
    Name:      "my-secret",
    Key:       "config.yaml",
    Watch:     true,
})
```
- 只读取 Secret 中的 `config.yaml` key
- 根据 `.yaml` 扩展名自动使用 YAML 解析器
- 支持热更新

**多 Key 合并模式：**
```go
src, err := k8s.NewSecretSource(k8s.ConfigSourceConfig{
    Namespace: "default",
    Name:      "my-secret",
    MergeAllKey: true,
    Watch:     true,
})
```
- 读取 Secret 中的所有 key
- 将所有 key 合并为一个 map
- 不进行格式解析，直接作为 map 注入

### 配置解析

根据 Key 的扩展名自动选择解析器：

| 扩展名 | 解析器 | 示例 |
|--------|--------|------|
| `.json` | JSON | `config.json` |
| `.yaml`, `.yml` | YAML | `config.yaml` |
| `.toml` | TOML | `config.toml` |
| 其他 | YAML（默认） | `config` |

### Secret 与 ConfigMap 的区别

| 特性 | ConfigMap | Secret |
|------|-----------|--------|
| 数据存储 | 明文存储 | Base64 编码存储 |
| 使用场景 | 非敏感配置 | 敏感配置（密码、密钥等） |
| 大小限制 | 1 MiB | 1 MiB |
| RBAC 权限 | 需要 `configmaps` 权限 | 需要 `secrets` 权限 |
| 可见性 | 任何有权限的用户都可以查看 | 需要特定权限查看 |
| 加密 | 不支持 | 不支持（Base64 不是加密） |

### 代码结构说明

```go
// 1. 创建 Secret 配置源
src, err := k8s.NewSecretSource(k8s.ConfigSourceConfig{
    Namespace: "default",
    Name:      "example-secret",
    Key:       "config.yaml",
    Watch:     true,
    Priority:  source.PriorityRemote,
})
if err != nil {
    panic(err)
}

// 2. 加载配置源到框架
if err := config.LoadSource(src); err != nil {
    panic(err)
}

// 3. 添加配置变更监听器
if err := config.AddWatcher("example", func(ev config.WatchEvent) {
    fmt.Printf("config changed: type=%v, version=%d\n", ev.Type(), ev.Version())
    
    if ev.Type() == config.WatchEventUpd || ev.Type() == config.WatchEventAdd {
        var cfg struct {
            Message string `mapstructure:"message"`
        }
        if err := ev.Value().Scan(&cfg); err != nil {
            fmt.Printf("failed to scan config: %v\n", err)
            return
        }
        fmt.Printf("message: %s\n", cfg.Message)
    }
}); err != nil {
    panic(err)
}

// 4. 等待信号
sig := make(chan os.Signal, 1)
signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
<-sig
```

## 高级用法

### 多配置源优先级

```go
// 1. 加载本地配置（低优先级）
localSrc, _ := source.NewFileSource("config.local.yaml", source.PriorityLocal)
config.LoadSource(localSrc)

// 2. 加载 ConfigMap 配置（中优先级）
configMapSrc, _ := k8s.NewConfigMapSource(k8s.ConfigSourceConfig{
    Namespace: "default",
    Name:      "my-config",
    Key:       "config.yaml",
    Priority:  source.PriorityRemote,
})
config.LoadSource(configMapSrc)

// 3. 加载 Secret 配置（高优先级，覆盖其他配置）
secretSrc, _ := k8s.NewSecretSource(k8s.ConfigSourceConfig{
    Namespace: "default",
    Name:      "my-secret",
    Key:       "config.yaml",
    Priority:  source.PriorityRemote + 1,
})
config.LoadSource(secretSrc)

// Secret 的配置会覆盖 ConfigMap 和本地配置的同名字段
```

### 敏感配置分离

将敏感配置和非敏感配置分开存储：

```yaml
# ConfigMap：非敏感配置
apiVersion: v1
kind: ConfigMap
metadata:
  name: app-config
data:
  config.yaml: |
    server:
      host: "0.0.0.0"
      port: 8080
    logging:
      level: "info"
---
# Secret：敏感配置
apiVersion: v1
kind: Secret
metadata:
  name: app-secret
type: Opaque
stringData:
  config.yaml: |
    database:
      password: "secret-password"
    api:
      apiKey: "api-key-123"
```

在代码中加载：
```go
// 加载 ConfigMap
configMapSrc, _ := k8s.NewConfigMapSource(k8s.ConfigSourceConfig{
    Namespace: "default",
    Name:      "app-config",
    Key:       "config.yaml",
    Priority:  source.PriorityLocal,
})
config.LoadSource(configMapSrc)

// 加载 Secret
secretSrc, _ := k8s.NewSecretSource(k8s.ConfigSourceConfig{
    Namespace: "default",
    Name:      "app-secret",
    Key:       "config.yaml",
    Priority:  source.PriorityRemote,
})
config.LoadSource(secretSrc)
```

### 嵌套配置

Secret 中的 YAML 配置支持嵌套结构：

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: app-secret
type: Opaque
stringData:
  config.yaml: |
    server:
      host: "0.0.0.0"
      port: 8080
      timeout: 30s
    database:
      host: "db.example.com"
      port: 5432
      name: "mydb"
      username: "admin"
      password: "secret-password"
    cache:
      type: "redis"
      host: "cache.example.com"
      port: 6379
      ttl: 300
    oauth:
      clientId: "client-id-123"
      clientSecret: "client-secret-456"
```

在代码中读取：
```go
var cfg struct {
    Server   struct {
        Host    string `mapstructure:"host"`
        Port    int    `mapstructure:"port"`
        Timeout string `mapstructure:"timeout"`
    } `mapstructure:"server"`
    Database struct {
        Host     string `mapstructure:"host"`
        Port     int    `mapstructure:"port"`
        Name     string `mapstructure:"name"`
        Username string `mapstructure:"username"`
        Password string `mapstructure:"password"`
    } `mapstructure:"database"`
    Cache struct {
        Type string `mapstructure:"type"`
        Host string `mapstructure:"host"`
        Port int    `mapstructure:"port"`
        TTL  int    `mapstructure:"ttl"`
    } `mapstructure:"cache"`
    OAuth struct {
        ClientID     string `mapstructure:"clientId"`
        ClientSecret string `mapstructure:"clientSecret"`
    } `mapstructure:"oauth"`
}
config.Get("app").Scan(&cfg)
```

### 数组配置

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: array-secret
type: Opaque
stringData:
  config.yaml: |
    apiKeys:
      - name: "service-a"
        key: "key-123"
      - name: "service-b"
        key: "key-456"
      - name: "service-c"
        key: "key-789"
```

在代码中读取：
```go
var cfg struct {
    APIKeys []struct {
        Name string `mapstructure:"name"`
        Key  string `mapstructure:"key"`
    } `mapstructure:"apiKeys"`
}
config.Get("app").Scan(&cfg)
for _, apiKey := range cfg.APIKeys {
    fmt.Printf("api key: %s - %s\n", apiKey.Name, apiKey.Key)
}
```

### 多环境配置

使用不同的 Secret 管理不同环境的配置：

```go
// 根据环境变量选择不同的 Secret
env := os.Getenv("APP_ENV")
if env == "" {
    env = "dev"
}

secretName := fmt.Sprintf("app-secret-%s", env)

src, err := k8s.NewSecretSource(k8s.ConfigSourceConfig{
    Namespace: "default",
    Name:      secretName,
    Key:       "config.yaml",
    Watch:     true,
})
```

## RBAC 权限

程序需要以下 RBAC 权限才能访问 Secret：

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  namespace: default
  name: secret-reader
rules:
- apiGroups: [""]
  resources: ["secrets"]
  verbs: ["get", "list", "watch"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: secret-reader
  namespace: default
subjects:
- kind: ServiceAccount
  name: your-serviceaccount
roleRef:
  kind: Role
  name: secret-reader
  apiGroup: rbac.authorization.k8s.io
```

## 安全注意事项

### Secret 不是加密

Secret 的数据是 Base64 编码，**不是加密**。任何有权限访问 Secret 的用户都可以解码查看。

```bash
# 查看 Secret 内容（可以看到 Base64 编码的数据）
kubectl get secret example-secret -n default -o yaml

# 解码 Secret 数据
echo "bWVzc2FnZTogSGVsbG8gZnJvbSBTZWNyZXQhCg==" | base64 -d
# 输出：message: Hello from Secret!
```

### 安全建议

1. **使用 Kubernetes 加密功能**
   - 启用 etcd 加密（Encryption at Rest）
   - 使用 KMS 加密 etcd 数据

2. **限制 Secret 访问权限**
   - 使用 RBAC 限制谁可以访问 Secret
   - 使用命名空间隔离不同环境的 Secret
   - 避免使用 ClusterRole 授予 Secret 权限

3. **不要将 Secret 提交到版本控制系统**
   - 在 `.gitignore` 中添加 `*.secret.yaml`
   - 使用 `kubectl create secret --dry-run` 生成 Secret YAML

4. **定期轮换 Secret**
   - 定期更新密码、密钥等敏感信息
   - 使用 Secret 管理工具（如 HashiCorp Vault）

5. **使用外部密钥管理服务**
   - HashiCorp Vault
   - AWS Secrets Manager
   - Azure Key Vault
   - Google Secret Manager

6. **审计 Secret 访问**
   - 启用 Kubernetes 审计日志
   - 监控 Secret 访问记录

7. **使用临时凭证**
   - 对于数据库连接等，使用短期有效的凭证
   - 定期自动轮换凭证

### 加密 Secret 示例

使用 Kubernetes 加密功能加密 Secret：

```yaml
# encryption-config.yaml
apiVersion: apiserver.config.k8s.io/v1
kind: EncryptionConfiguration
resources:
  - resources:
    - secrets
    providers:
    - aescbc:
        keys:
        - name: key1
          secret: <base64-encoded-key>
    - identity: {}
```

启动 API Server 时指定加密配置：
```bash
kube-apiserver \
  --encryption-provider-config=/etc/kubernetes/encryption-config.yaml
```

## 常见问题

**Q: 为什么没有读取到配置？**

A: 检查以下几点：
1. 确认 Secret 存在：`kubectl get secret example-secret -n default`
2. 确认 Namespace 配置正确
3. 确认 Key 配置正确
4. 检查 RBAC 权限

**Q: 配置更新后没有收到通知？**

A: 可能的原因：
1. `Watch` 参数设置为 `false`
2. Secret 内容没有变化（相同内容不会触发更新）
3. Kubernetes 集群网络问题
4. Watcher channel 满，丢弃了更新

**Q: 如何处理大量配置？**

A: 建议的方法：
1. 使用多个 Secret，按模块拆分
2. 使用 `MergeAllKey` 模式合并多个 key
3. 考虑使用外部配置中心（如 etcd、Vault）

**Q: Secret 大小限制是多少？**

A: Secret 的大小限制为 1 MiB（1,048,576 字节）。如果配置过大，建议：
1. 拆分为多个 Secret
2. 使用外部配置中心（如 etcd、Vault）
3. 将大文件存储在 Secret 中引用的 Volume 中

**Q: Secret 安全吗？**

A: Secret 的安全性取决于：
1. **etcd 加密**：启用 Encryption at Rest
2. **RBAC 权限**：限制 Secret 访问权限
3. **Kubernetes 版本**：使用最新版本，修复安全漏洞
4. **网络隔离**：使用 NetworkPolicy 限制 etcd 访问

**Q: 如何使用不同的 kubeconfig 文件？**

A: 设置 `Kubeconfig` 参数：
```go
src, err := k8s.NewSecretSource(k8s.ConfigSourceConfig{
    Kubeconfig: "/path/to/kubeconfig.yaml",
    // ...
})
```

**Q: 如何解码 Secret 数据？**

A: 使用 base64 工具：
```bash
# 方式 1：使用 kubectl
kubectl get secret example-secret -n default -o jsonpath='{.data.config\.yaml}' | base64 -d

# 方式 2：使用 base64 命令
echo "bWVzc2FnZTogSGVsbG8gZnJvbSBTZWNyZXQhCg==" | base64 -d
```

## 最佳实践

1. **敏感信息使用 Secret**：密码、密钥、证书等敏感信息必须使用 Secret 存储
2. **非敏感信息使用 ConfigMap**：普通配置使用 ConfigMap，减少 Secret 的使用
3. **配置分层**：将配置分为基础配置、环境配置、实例配置，按优先级加载
4. **配置验证**：在代码中验证配置的合法性
5. **配置监控**：记录配置变更日志，便于审计和排查问题
6. **配置版本管理**：在配置中包含版本信息，便于回滚
7. **启用加密**：启用 Kubernetes Encryption at Rest 加密 etcd 数据
8. **最小权限原则**：使用 RBAC 限制 Secret 访问权限
9. **定期轮换**：定期更新密码、密钥等敏感信息
10. **不要提交 Secret**：不要将 Secret 提交到版本控制系统

## 相关文档

- [k8s 主文档](../../readme.md)
- [ConfigMap 配置源示例](../config-source/)
- [Resolver 示例](../resolver/)
- [yggdrasil 配置文档](../../../../docs/config.md)
- [Kubernetes Secret 文档](https://kubernetes.io/docs/concepts/configuration/secret/)

## 退出

按 `Ctrl+C` 退出程序。
