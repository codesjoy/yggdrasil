# 02 Runtime Bundle

## 体现的框架能力

- 展示 `Runtime` 可安全暴露的能力：`Config()`、`Logger()`、`TracerProvider()`、`MeterProvider()` 和 `Lookup(...)`。
- 展示 `BusinessBundle` 的安装边界：`RPCBindings`、`RESTBindings`、`RawHTTP`、`Tasks`、`Hooks`、`Diagnostics`。
- 展示业务代码如何在 `business.Compose(...)` 里读取框架与业务配置，再统一返回一个 bundle。

## 启动方式

```bash
cd /Users/zhangwei/go/src/github.com/codesjoy/yggdrasil/examples/02-runtime-bundle
go run .
```

可选观察：

```bash
curl http://127.0.0.1:56021/healthz
curl http://127.0.0.1:56021/v1/shelves/runtime-bundle
curl http://127.0.0.1:56022/diagnostics?pretty=true
```

## 观察点

- `main.go` 只保留 root `yggdrasil.Run(...)`，bundle 组合逻辑全部收敛在 `business.Compose`。
- `/healthz` 由 `RawHTTPBinding` 提供；`/v1/shelves/runtime-bundle` 由 `RESTBinding` 提供；gRPC 则走 `RPCBinding`。
- governor 的 `/diagnostics` 会带上 `BusinessBundle.Diagnostics`，便于确认 bundle 安装结果。

## 关键源码入口

- 生命周期入口：[main.go](/Users/zhangwei/go/src/github.com/codesjoy/yggdrasil/examples/02-runtime-bundle/main.go)
- bundle 组合：[business/compose.go](/Users/zhangwei/go/src/github.com/codesjoy/yggdrasil/examples/02-runtime-bundle/business/compose.go)
- bundle 测试：[business/compose_test.go](/Users/zhangwei/go/src/github.com/codesjoy/yggdrasil/examples/02-runtime-bundle/business/compose_test.go)

## 下一步看什么

- 如果你想看 watchable config、reload 和 spec diff 怎么联动，看 [03-diagnostics-reload](/Users/zhangwei/go/src/github.com/codesjoy/yggdrasil/examples/03-diagnostics-reload)。
- 如果你想看 REST 专项，而不是 bundle 总览，看 [10-rest-gateway](/Users/zhangwei/go/src/github.com/codesjoy/yggdrasil/examples/10-rest-gateway)。
