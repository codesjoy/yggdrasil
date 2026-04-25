# 配置管理示例

这个示例展示重构后的 `config` 用法：

- 使用一个 `config.Manager` 统一加载文件和环境变量 layer
- 通过 `Snapshot` / `Section(path...)` 读取不可变配置快照
- 通过 typed section 解码结构体
- 启动时由 `app := app.New(...); app.Start(...)` 或根包 `yggdrasil.Run(...)` 装配框架自己的 `yggdrasil.*` 配置

## 当前模型

`config` 不再提供全局 key/value 风格的 `Get` / `GetString` / `LoadSource` API，也不再区分 `config` 与 `config/runtime` 两套运行时模型。

现在统一使用：

- `config.Default()` 或 `config.NewManager()`
- `manager.LoadLayer(name, priority, source)`
- `manager.Section(path...).Decode(&target)`
- `config.Bind[T](manager, path...).Current()`
- `config.Bind[T](manager, path...).Watch(...)`

优先级从低到高固定为：

- `config.PriorityDefaults`
- `config.PriorityFile`
- `config.PriorityRemote`
- `config.PriorityEnv`
- `config.PriorityFlag`
- `config.PriorityOverride`

## 示例代码

示例主程序在 [main.go](/Users/zhangwei/go/src/github.com/codesjoy/yggdrasil/example/advanced/config/main.go)。

核心加载逻辑：

```go
func loadConfigSources() error {
    manager := config.Default()

    if err := manager.LoadLayer(
        "example:file",
        config.PriorityFile,
        file.NewSource("config.yaml", false),
    ); err != nil {
        return err
    }

    if err := manager.LoadLayer(
        "example:env",
        config.PriorityEnv,
        env.NewSource([]string{"APP_"}, []string{"_"}),
    ); err != nil {
        return err
    }
    return nil
}
```

读取业务配置：

```go
appConfig := &AppConfig{}
if err := config.Default().Section("app").Decode(appConfig); err != nil {
    return err
}
```

读取单个子树或标量：

```go
var host string
_ = config.Default().Section("app", "server", "host").Decode(&host)

var port int
_ = config.Default().Section("app", "server", "port").Decode(&port)
```

也可以绑定 typed section：

```go
section := config.Bind[AppConfig](config.Default(), "app")
current, err := section.Current()
```

## 配置文件示例

```yaml
app:
  server:
    host: "0.0.0.0"
    port: 8080
  database:
    host: "localhost"
    port: 5432
    name: "mydb"
  cache:
    enabled: true
    host: "localhost"
    port: 6379
    ttl: 3600
```

环境变量覆盖示例：

```bash
export APP_SERVER_HOST=192.168.1.1
export APP_SERVER_PORT=9090
```

## 运行

```bash
cd example/advanced/config
go run main.go
```

## 热更新

如果 source 自己支持 watch，例如：

```go
src := file.NewSource("config.yaml", true)
_ = manager.LoadLayer("example:file", config.PriorityFile, src)
```

可以直接 watch typed section：

```go
cancel := config.Bind[AppConfig](manager, "app").Watch(func(next AppConfig, err error) {
    if err != nil {
        slog.Error("decode config update failed", slog.Any("error", err))
        return
    }
    applyNewConfig(&next)
})
defer cancel()
```

不需要再使用 `config/runtime.Store`。

## 覆盖配置

运行时或测试覆盖统一追加 `override` layer：

```go
override := memory.NewSource("app-override", map[string]any{
    "app": map[string]any{
        "server": map[string]any{
            "port": 9090,
        },
    },
})

if err := config.Default().LoadLayer("app-override", config.PriorityOverride, override); err != nil {
    return err
}
```

## 建议

- 框架配置放在 `yggdrasil.*`
- 业务配置放在独立顶层，例如 `app.*`
- 框架内部通过 `internal/settings` 做 typed catalog 和 resolved spec
- 业务代码直接使用 `Manager.Section(...).Decode(...)` 或 `config.Bind[T](...)`
