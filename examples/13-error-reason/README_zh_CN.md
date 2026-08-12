# 13 Error Reason

## 体现的框架能力

- 展示基于 proto reason enum 的结构化错误返回与客户端解析。
- 展示 reason 到 gRPC code / HTTP code 的映射，以及 metadata 如何随错误一起透传。
- 展示错误语义仍然是业务安装边界的一部分，而不是 transport 之外的附加层。

## 启动方式

服务端：

```bash
cd examples/13-error-reason/server
go run .
```

客户端：

```bash
cd examples/13-error-reason/client
go run .
```

## 观察点

- 服务端主入口已经收敛到 root `yggdrasil.Run(ctx, appName, ...)`，而错误语义服务仍然是通过 `BusinessBundle` 正式安装的。
- 为了把各种错误场景放在一个文件里对照，`server/main.go` 同时保留了 service 实现和 `composeBundle(...)` 入口。
- 客户端使用 `xerror.IsCode(...)` 和 `xerror.IsReason(...)` 统一检查本地与远端错误；需要 HTTP code 或完整 protobuf details 时，仍通过 `status.FromError(...)` 读取传输层状态。

## 关键源码入口

- 生命周期入口与错误场景：[server/main.go](server/main.go)
- bundle 测试：[server/compose_test.go](server/compose_test.go)
- 客户端入口：[client/main.go](client/main.go)

## 下一步看什么

- 如果你想先看 `BusinessBundle` 的安装边界，再回来看错误语义，读 [02-runtime-bundle](../02-runtime-bundle/README_zh_CN.md)。
- 如果你想看多 endpoint 调用路径下的 client runtime 行为，读 [14-client-load-balancing](../14-client-load-balancing/README_zh_CN.md)。
