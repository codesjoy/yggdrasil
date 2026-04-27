# 14 Client Load Balancing

## 体现的框架能力

- 展示一个 client service target 对应多个 endpoint 时的请求分发行为。
- 展示 direct endpoint 配置如何进入 client runtime，而不是依赖额外的服务发现系统。
- 展示同一个 `RPCBinding` 安装结果如何被多个后端实例共同承载。

## 启动方式

三个服务端分别运行：

```bash
cd examples/14-client-load-balancing/server
go run . --port 55884
```

```bash
cd examples/14-client-load-balancing/server
go run . --port 55885
```

```bash
cd examples/14-client-load-balancing/server
go run . --port 55886
```

客户端：

```bash
cd examples/14-client-load-balancing/client
go run .
```

## 观察点

- [client/config.yaml](client/config.yaml) 在一个 service target 下配置了三个 endpoint，这是这个例子真正要证明的 client runtime 行为。
- [server/config.yaml](server/config.yaml) 只描述基础 server 形态；`server/main.go` 再通过 `--port` 驱动的 config override 覆盖实际 grpc listen address。
- `client/main.go` 使用独立 `app.New(...)->NewClient(...)` bootstrap，并通过 trailer 中的 `server` 字段观察请求最终落到哪个后端实例。

## 关键源码入口

- 生命周期入口：[server/main.go](server/main.go)
- bundle 组合：[server/business/compose.go](server/business/compose.go)
- client endpoint 配置：[client/config.yaml](client/config.yaml)

## 下一步看什么

- 如果你先想理解 client service target 是怎样被 canonical mainline app 使用的，回看 [01-quickstart](../01-quickstart/README_zh_CN.md)。
- 如果你想继续看 provider-only 扩展如何进入 planner/runtime，读 [20-capability-registration](../20-capability-registration/README_zh_CN.md)。
