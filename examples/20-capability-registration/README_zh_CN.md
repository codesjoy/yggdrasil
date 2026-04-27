# 20 Capability Registration

## 体现的框架能力

- 展示 provider-only capability registration，而不是完整 `module.Module` 扩展。
- 展示 `WithCapabilityRegistrations(...)` 如何进入 root server 启动路径，以及 standalone client bootstrap。
- 展示自定义协议名、配置路径、capability provider 名三者必须对齐，planner/runtime 才能选中这组 provider。

## 启动方式

服务端：

```bash
cd examples/20-capability-registration/server
go run .
```

客户端：

```bash
cd examples/20-capability-registration/client
go run .
```

## 观察点

- 服务端走 `yggdrasil.Run(..., yggdrasil.WithCapabilityRegistrations(...))`，这样 registration 会和普通业务 bundle 一起进入当前 root app。
- 客户端走 `app.New(..., WithCapabilityRegistrations(...))->NewClient(...)`，因为 standalone client bootstrap 仍然属于高级入口。
- 扩展点只发生在 `grpcx` transport provider 层，业务侧仍然只是普通的 `GreeterService` 安装。

## 关键源码入口

- registration 定义：[grpcx/registration.go](grpcx/registration.go)
- 服务端入口：[server/main.go](server/main.go)
- 客户端入口：[client/main.go](client/main.go)

## 下一步看什么

- 如果你想回到更基础的 `Runtime` / `BusinessBundle` 视角，读 [02-runtime-bundle](../02-runtime-bundle/README_zh_CN.md)。
- 如果你想看更低层的 transport recipe，而不是完整 app 接入，读 [90-recipes/raw-grpc.md](../90-recipes/raw-grpc.md) 和 [90-recipes/jsonraw-grpc.md](../90-recipes/jsonraw-grpc.md)。
