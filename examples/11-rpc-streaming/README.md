# 11 RPC Streaming

## 体现的框架能力

- 展示一个 `RPCBinding` 如何同时承载 unary、client-streaming、server-streaming 和 bidirectional-streaming。
- 展示流式形态的重点仍然是业务 bundle 安装，而不是额外的 framework entrypoint 差异。
- 保留最小实现，让读者把注意力放在 Yggdrasil 下的 client/server 装配方式。

## 启动方式

服务端：

```bash
cd /Users/zhangwei/go/src/github.com/codesjoy/yggdrasil/examples/11-rpc-streaming/server
go run .
```

客户端：

```bash
cd /Users/zhangwei/go/src/github.com/codesjoy/yggdrasil/examples/11-rpc-streaming/client
go run .
```

## 观察点

- `server/main.go` 已经统一到 root `yggdrasil.Run(...)`，正式安装边界仍然在 `server/business/compose.go`。
- `client/main.go` 使用独立 `app.New(...)->NewClient(...)` bootstrap，因为 root facade 不负责 standalone client 启动。
- 配置里的 service target 已切到 `github.com.codesjoy.yggdrasil.example.11-rpc-streaming`，方便和 `12`、`14` 形成连续阅读路径。

## 关键源码入口

- 生命周期入口：[server/main.go](/Users/zhangwei/go/src/github.com/codesjoy/yggdrasil/examples/11-rpc-streaming/server/main.go)
- bundle 组合：[server/business/compose.go](/Users/zhangwei/go/src/github.com/codesjoy/yggdrasil/examples/11-rpc-streaming/server/business/compose.go)
- 客户端入口：[client/main.go](/Users/zhangwei/go/src/github.com/codesjoy/yggdrasil/examples/11-rpc-streaming/client/main.go)

## 下一步看什么

- 如果你要看 stream 上下文里的 metadata/header/trailer 怎么传，读 [12-transport-metadata](/Users/zhangwei/go/src/github.com/codesjoy/yggdrasil/examples/12-transport-metadata)。
- 如果你要看同一个 client service target 如何跨多个 endpoint 分发，读 [14-client-load-balancing](/Users/zhangwei/go/src/github.com/codesjoy/yggdrasil/examples/14-client-load-balancing)。
