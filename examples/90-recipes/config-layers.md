# Config Layers Recipe

## 何时使用

当你只想演示配置分层、typed section、watchable source，而不想再维护一个完整 runnable app 时，用这个 recipe 即可。

## 核心模式

```go
manager := config.NewManager()

_ = manager.LoadLayer(
    "app:file",
    config.PriorityFile,
    file.NewSource("config.yaml", false),
)

_ = manager.LoadLayer(
    "app:env",
    config.PriorityEnv,
    env.NewSource([]string{"APP_"}, []string{"_"}),
)
```

```go
type AppConfig struct {
    Server struct {
        Host string `mapstructure:"host"`
        Port int    `mapstructure:"port"`
    } `mapstructure:"server"`
}

var cfg AppConfig
_ = manager.Section("app").Decode(&cfg)
```

## Watchable Overlay

如果 source 自己支持 watch，可以直接把它作为高优先级 layer：

```go
_ = manager.LoadLayer(
    "app:override",
    config.PriorityOverride,
    file.NewSource("override.yaml", true),
)
```

然后再订阅 typed section：

```go
cancel := config.Bind[AppConfig](manager, "app").Watch(func(next AppConfig, err error) {
    // apply new config
})
defer cancel()
```

## 相关示例

- [02-runtime-bundle](/Users/zhangwei/go/src/github.com/codesjoy/yggdrasil/examples/02-runtime-bundle)
- [03-diagnostics-reload](/Users/zhangwei/go/src/github.com/codesjoy/yggdrasil/examples/03-diagnostics-reload)
