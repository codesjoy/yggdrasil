# 01 Quickstart

## 体现的框架能力

- 使用 root `yggdrasil.Run(...)` 走完服务端默认接入路径。
- 使用 `app.New(...)->NewClient(...)` 做独立 client bootstrap，而不是依赖全局状态。
- 保持示例最小化，只展示一个最短可运行的 gRPC 端到端路径。

## 启动方式

服务端：

```bash
cd /Users/zhangwei/go/src/github.com/codesjoy/yggdrasil/examples/01-quickstart/server
go run .
```

客户端：

```bash
cd /Users/zhangwei/go/src/github.com/codesjoy/yggdrasil/examples/01-quickstart/client
go run .
```

## 观察点

- `server/main.go` 只保留 root facade 和退出控制，正式业务安装边界在 `server/business/compose.go`。
- `client/main.go` 通过 `config.yaml` 里的 app identity 和 service target 启动一个独立 client app，然后调用一次 `SayHello`。
- governor 诊断入口固定为 `http://127.0.0.1:56011/diagnostics?pretty=true`。

## 关键源码入口

- 服务端入口：[server/main.go](/Users/zhangwei/go/src/github.com/codesjoy/yggdrasil/examples/01-quickstart/server/main.go)
- 业务组合：[server/business/compose.go](/Users/zhangwei/go/src/github.com/codesjoy/yggdrasil/examples/01-quickstart/server/business/compose.go)
- 客户端入口：[client/main.go](/Users/zhangwei/go/src/github.com/codesjoy/yggdrasil/examples/01-quickstart/client/main.go)

## 下一步看什么

- 如果你想先理解 `BusinessBundle` 能安装哪些东西，看 [02-runtime-bundle](/Users/zhangwei/go/src/github.com/codesjoy/yggdrasil/examples/02-runtime-bundle)。
- 如果你想看配置 watch、reload 和 diagnostics 的联动，看 [03-diagnostics-reload](/Users/zhangwei/go/src/github.com/codesjoy/yggdrasil/examples/03-diagnostics-reload)。
