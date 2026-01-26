# 配置管理示例

本示例演示如何在 Yggdrasil 框架中使用多配置源、配置优先级和配置热更新。

## 你会得到什么

- 多配置源加载（文件、环境变量、命令行）
- 配置优先级管理
- 配置热更新和监听
- 配置验证
- 配置读取和修改

## 功能特性

- **多配置源**: 支持文件、环境变量、命令行等多种配置源
- **优先级管理**: 配置源按优先级合并，高优先级覆盖低优先级
- **配置热更新**: 监听配置变更，实时更新应用配置
- **配置验证**: 启动时验证配置的合法性
- **类型安全**: 使用结构体映射配置，支持类型检查

## 前置条件

- Go 1.24 或更高版本
- 已安装 Yggdrasil 框架

## 配置源优先级

Yggdrasil 支持以下配置优先级（从低到高）：

| 优先级 | 常量 | 说明 |
|--------|------|------|
| 最低 | `PriorityLow` | 默认配置 |
| 低 | `PriorityLocal` | 本地配置（文件） |
| 中 | `PriorityEnvironment` | 环境变量 |
| 高 | `PriorityFlag` | 命令行参数 |
| 最高 | `PriorityRemote` | 远程配置（etcd、k8s 等） |

## 配置源类型

### 1. 文件配置源

从 YAML、JSON、TOML 等格式的文件加载配置。

**创建文件配置源**:

```go
src, err := file.NewSource("config.yaml", false)
if err != nil {
    return err
}

config.LoadSource(src)
```

**配置文件示例 (config.yaml)**:

```yaml
app:
  server:
    host: "0.0.0.0"
    port: 8080
  database:
    host: "localhost"
    port: 5432
    name: "mydb"
```

### 2. 环境变量配置源

从环境变量加载配置，支持前缀过滤。

**创建环境变量配置源**:

```go
src, err := env.NewSource("APP_", config.PriorityLocal)
if err != nil {
    return err
}

config.LoadSource(src)
```

**使用示例**:

```bash
export APP_SERVER_HOST=192.168.1.1
export APP_SERVER_PORT=9090
export APP_DATABASE_HOST=db.example.com
```

**环境变量命名规则**:

- 格式: `{PREFIX}{SECTION}_{KEY}`
- 示例: `APP_SERVER_HOST` → `app.server.host`
- 大小写不敏感

### 3. 命令行配置源

从命令行参数加载配置。

**创建命令行配置源**:

```go
src, err := flag.NewSource("config", config.PriorityFlag)
if err != nil {
    return err
}

config.LoadSource(src)
```

**使用示例**:

```bash
./app --config.server.host=192.168.1.1 --config.server.port=9090
```

## 配置结构定义

使用结构体定义配置，支持类型安全：

```go
type AppConfig struct {
    Server struct {
        Host string `mapstructure:"host"`
        Port int    `mapstructure:"port"`
    } `mapstructure:"server"`
    Database struct {
        Host string `mapstructure:"host"`
        Port int    `mapstructure:"port"`
        Name string `mapstructure:"name"`
    } `mapstructure:"database"`
}
```

## 配置读取

### 读取整个配置

```go
appConfig := &AppConfig{}
if err := config.Get("app").Scan(appConfig); err != nil {
    slog.Error("failed to scan config", slog.Any("error", err))
    return err
}
```

### 读取单个配置值

```go
host := config.Get("app.server.host").String()
port := config.Get("app.server.port").Int()
enabled := config.Get("app.cache.enabled").Bool()
```

### 读取嵌套配置

```go
dbHost := config.Get("app.database.host").String()
dbPort := config.Get("app.database.port").Int()
dbName := config.Get("app.database.name").String()
```

## 配置修改

### 设置配置值

```go
if err := config.Get("app").Set("server.port", 9090); err != nil {
    slog.Error("failed to set config", slog.Any("error", err))
    return err
}
```

### 批量设置配置

```go
updates := map[string]interface{}{
    "server.port": 9090,
    "cache.enabled": false,
}

for key, value := range updates {
    if err := config.Get("app").Set(key, value); err != nil {
        slog.Error("failed to set config", "key", key, "error", err)
    }
}
```

## 配置热更新

### 添加配置监听器

```go
if err := config.AddWatcher("app", func(ev config.WatchEvent) {
    slog.Info("Config changed",
        "type", ev.Type(),
        "version", ev.Version(),
    )

    appConfig := &AppConfig{}
    if err := ev.Value().Scan(appConfig); err != nil {
        slog.Error("failed to scan new config", slog.Any("error", err))
        return
    }

    applyNewConfig(appConfig)
}); err != nil {
    slog.Error("failed to add watcher", slog.Any("error", err))
    return err
}
```

### WatchEvent 类型

| 类型 | 常量 | 说明 |
|------|------|------|
| 添加 | `WatchEventAdd` | 配置被添加 |
| 更新 | `WatchEventUpd` | 配置被更新 |
| 删除 | `WatchEventDel` | 配置被删除 |

## 配置验证

### 启动时验证

```go
func validateConfig(cfg *AppConfig) error {
    if cfg.Server.Host == "" {
        return fmt.Errorf("server host is required")
    }
    if cfg.Server.Port <= 0 || cfg.Server.Port > 65535 {
        return fmt.Errorf("server port must be between 1 and 65535")
    }
    if cfg.Database.Host == "" {
        return fmt.Errorf("database host is required")
    }
    return nil
}
```

### 使用验证

```go
appConfig := &AppConfig{}
config.Get("app").Scan(appConfig)

if err := validateConfig(appConfig); err != nil {
    slog.Error("invalid config", slog.Any("error", err))
    os.Exit(1)
}
```

## 运行示例

### 1. 默认运行

```bash
cd example/advanced/config
go run main.go
```

### 2. 使用环境变量

```bash
export APP_SERVER_HOST=192.168.1.1
export APP_SERVER_PORT=9090
go run main.go
```

### 3. 使用命令行参数

```bash
go run main.go --config.server.host=192.168.1.1 --config.server.port=9090
```

### 4. 组合使用

```bash
export APP_DATABASE_HOST=db.example.com
go run main.go --config.server.port=9090
```

## 预期输出

```
time=2025-01-26T10:00:00.000Z level=INFO msg="Starting configuration example..."
time=2025-01-26T10:00:00.100Z level=INFO msg="loaded config from file"
time=2025-01-26T10:00:00.200Z level=INFO msg="loaded config from env"
time=2025-01-26T10:00:00.300Z level=INFO msg="loaded config from flag"
time=2025-01-26T10:00:00.400Z level=INFO msg="=== Current Configuration ==="
time=2025-01-26T10:00:00.410Z level=INFO msg="Server" host=0.0.0.0 port=8080
time=2025-01-26T10:00:00.420Z level=INFO msg="Database" host=localhost port=5432 name=mydb
time=2025-01-26T10:00:00.430Z level=INFO msg="Cache" enabled=true host=localhost port=6379 ttl=3600
time=2025-01-26T10:00:00.500Z level=INFO msg="Config watcher setup complete"
time=2025-01-26T10:00:00.600Z level=INFO msg="Configuration example started successfully"
time=2025-01-26T10:00:00.610Z level=INFO msg="Press Ctrl+C to exit..."
```

## 配置最佳实践

### 1. 配置分层

```yaml
# config.yaml - 基础配置
app:
  server:
    host: "0.0.0.0"
    port: 8080

# config.dev.yaml - 开发环境覆盖
app:
  server:
    host: "localhost"
  database:
    host: "dev-db.example.com"

# config.prod.yaml - 生产环境覆盖
app:
  server:
    host: "0.0.0.0"
  database:
    host: "prod-db.example.com"
```

### 2. 环境隔离

```go
env := os.Getenv("APP_ENV")
configFile := fmt.Sprintf("config.%s.yaml", env)
config.LoadSource(file.NewSource(configFile, false))
```

### 3. 敏感信息

敏感信息（密码、密钥）应该使用环境变量或密钥管理系统：

```bash
export APP_DATABASE_PASSWORD=secret-password
```

### 4. 配置验证

在启动时和配置更新时都进行验证：

```go
func validateAndApplyConfig(cfg *AppConfig) error {
    if err := validateConfig(cfg); err != nil {
        return err
    }
    return applyConfig(cfg)
}
```

### 5. 配置文档

为配置项添加文档注释：

```yaml
# Server configuration
app:
  server:
    host: "0.0.0.0"        # Server listening address
    port: 8080               # Server listening port (1-65535)
```

## 常见问题

**Q: 如何处理配置缺失？**

A: 使用默认值：

```go
type AppConfig struct {
    Server struct {
        Host string `mapstructure:"host"`
        Port int    `mapstructure:"port,default=8080"`
    } `mapstructure:"server"`
}
```

**Q: 如何处理类型转换错误？**

A: 使用 `IsErr()` 检查错误：

```go
value := config.Get("app.server.port")
if value.IsErr() {
    slog.Error("config value error", slog.Any("error", value.Err()))
    return
}
port := value.Int()
```

**Q: 配置更新时如何避免重载？**

A: 检查配置是否真的发生了变化：

```go
config.AddWatcher("app", func(ev config.WatchEvent) {
    if ev.Type() == config.WatchEventUpd {
        if isConfigChanged(ev.Value()) {
            applyNewConfig(ev.Value())
        }
    }
})
```

**Q: 如何支持多种配置格式？**

A: 根据文件扩展名自动识别：

```go
config.LoadSource(file.NewSource("config.json", false))
config.LoadSource(file.NewSource("config.yaml", false))
config.LoadSource(file.NewSource("config.toml", false))
```

**Q: 如何在配置中使用环境变量？**

A: 在配置文件中使用 `${VAR}` 语法（需要额外支持）：

```yaml
app:
  database:
    password: "${APP_DATABASE_PASSWORD}"
```

## 相关文档

- [Yggdrasil 主文档](../../../README.md)
- [Kubernetes ConfigMap 示例](../../../contrib/k8s/example/config-source/)
- [etcd 配置源示例](../../../contrib/etcd/example/config-source/)
- [Polaris 配置源示例](../../../contrib/polaris/example/scenarios/config-source/)

## 退出

按 `Ctrl+C` 优雅退出。
