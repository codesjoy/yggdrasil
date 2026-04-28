# 10 REST Gateway

## 体现的框架能力

- 展示 proto 生成的 `RESTBinding` 如何和 `RPCBinding` 一起进入同一个 `BusinessBundle`。
- 展示 REST route 的正式安装边界仍然是业务 bundle，而不是框架外维护一套路由表。
- 展示一个“框架外调用者”如何直接验证 bundle 暴露出来的 HTTP/JSON 接口。

## 启动方式

服务端：

```bash
cd examples/10-rest-gateway/server
go run .
```

客户端：

```bash
cd examples/10-rest-gateway/client
go run .
```

## 观察点

- 服务端主入口已经收敛到 root `yggdrasil.Run(ctx, appName, ...)`，而 `RESTBinding` 何时安装仍然完全由 `server/business/compose.go` 决定。
- `server/business/compose.go` 同时返回 `LibraryServiceServiceDesc` 和 `LibraryServiceRestServiceDesc`，这是这个例子的核心边界。
- `client/main.go` 不是 Yggdrasil client，而是一个普通 HTTP 调用者；它的职责是从框架外确认 REST route、JSON 编解码和状态码是否符合预期。

## 关键源码入口

- 生命周期入口：[server/main.go](server/main.go)
- bundle 组合：[server/business/compose.go](server/business/compose.go)
- 外部 HTTP 调用者：[client/main.go](client/main.go)

## 下一步看什么

- 如果你想先理解 `RESTBinding` 只是 `BusinessBundle` 的一个安装面，看 [02-runtime-bundle](../02-runtime-bundle/README_zh_CN.md)。
- 如果你想继续看 transport 上下文如何在调用链里流动，看 [12-transport-metadata](../12-transport-metadata/README_zh_CN.md)。
