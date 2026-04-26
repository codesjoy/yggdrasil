# 12 Transport Metadata

## 体现的框架能力

- 展示请求 metadata、响应 header、响应 trailer 在 Yggdrasil transport 层的传递方式。
- 同时覆盖 unary 和 streaming 场景，让读者看到 metadata 行为如何附着在同一个 `RPCBinding` 上。
- 保持示例重点在 transport context，而不是业务逻辑本身。

## 启动方式

服务端：

```bash
cd /Users/zhangwei/go/src/github.com/codesjoy/yggdrasil/examples/12-transport-metadata/server
go run .
```

客户端：

```bash
cd /Users/zhangwei/go/src/github.com/codesjoy/yggdrasil/examples/12-transport-metadata/client
go run .
```

## 观察点

- 服务端默认走 root `yggdrasil.Run(...)`；metadata 行为的正式安装边界仍然是 `server/business/compose.go`。
- 客户端使用独立 `app.New(...)->NewClient(...)` bootstrap，然后在每次调用里显式读写 metadata context。
- 这个例子最适合和 [11-rpc-streaming](/Users/zhangwei/go/src/github.com/codesjoy/yggdrasil/examples/11-rpc-streaming) 对照着看：两者共用同一套流式装配模型，只是关注点不同。

## 关键源码入口

- 生命周期入口：[server/main.go](/Users/zhangwei/go/src/github.com/codesjoy/yggdrasil/examples/12-transport-metadata/server/main.go)
- bundle 组合：[server/business/compose.go](/Users/zhangwei/go/src/github.com/codesjoy/yggdrasil/examples/12-transport-metadata/server/business/compose.go)
- 客户端入口：[client/main.go](/Users/zhangwei/go/src/github.com/codesjoy/yggdrasil/examples/12-transport-metadata/client/main.go)

## 下一步看什么

- 如果你关注 transport 行为之外的错误语义，看 [13-error-reason](/Users/zhangwei/go/src/github.com/codesjoy/yggdrasil/examples/13-error-reason)。
- 如果你关注 `RESTBinding` 暴露出来的结构化接口，看 [10-rest-gateway](/Users/zhangwei/go/src/github.com/codesjoy/yggdrasil/examples/10-rest-gateway)。
