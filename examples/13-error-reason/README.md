# 13 Error Reason

## 体现的框架能力

- 展示基于 proto reason enum 的结构化错误返回与客户端解析。
- 展示 reason 到 gRPC code / HTTP code 的映射，以及 metadata 如何随错误一起透传。
- 展示错误语义仍然是业务安装边界的一部分，而不是 transport 之外的附加层。

## 启动方式

服务端：

```bash
cd /Users/zhangwei/go/src/github.com/codesjoy/yggdrasil/examples/13-error-reason/server
go run .
```

客户端：

```bash
cd /Users/zhangwei/go/src/github.com/codesjoy/yggdrasil/examples/13-error-reason/client
go run .
```

## 观察点

- 服务端主入口已经收敛到 root `yggdrasil.Run(...)`，而错误语义服务仍然是通过 `BusinessBundle` 正式安装的。
- 为了把各种错误场景放在一个文件里对照，`server/main.go` 同时保留了 service 实现和 `composeBundle(...)` 入口。
- 客户端使用独立 `app.New(...)->NewClient(...)` bootstrap，因为这个例子要把注意力集中在 `status.FromError(...)`、`Code()`、`HTTPCode()` 和 `ErrorInfo()` 的读取方式上。

## 关键源码入口

- 生命周期入口与错误场景：[server/main.go](/Users/zhangwei/go/src/github.com/codesjoy/yggdrasil/examples/13-error-reason/server/main.go)
- bundle 测试：[server/compose_test.go](/Users/zhangwei/go/src/github.com/codesjoy/yggdrasil/examples/13-error-reason/server/compose_test.go)
- 客户端入口：[client/main.go](/Users/zhangwei/go/src/github.com/codesjoy/yggdrasil/examples/13-error-reason/client/main.go)

## 下一步看什么

- 如果你想先看 `BusinessBundle` 的安装边界，再回来看错误语义，读 [02-runtime-bundle](/Users/zhangwei/go/src/github.com/codesjoy/yggdrasil/examples/02-runtime-bundle)。
- 如果你想看多 endpoint 调用路径下的 client runtime 行为，读 [14-client-load-balancing](/Users/zhangwei/go/src/github.com/codesjoy/yggdrasil/examples/14-client-load-balancing)。
