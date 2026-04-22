# Raw gRPC (`[]byte`) 模式

`remote/transport/grpc` 现在支持通过 per-call codec 直接收发 `[]byte`，不经过 protobuf 编解码。

推荐用法：

```go
ctx := grpc.WithCallOptions(context.Background(), grpc.CallContentSubtype("raw"))
```

客户端侧继续走现有 `client.Client`：

```go
st, err := cli.NewStream(ctx, &stream.Desc{}, "/raw.Test/Unary")
if err != nil {
    return err
}
if err := st.SendMsg([]byte("ping")); err != nil {
    return err
}

var reply []byte
if err := st.RecvMsg(&reply); err != nil {
    return err
}
```

服务端侧使用手工注册的 low-level handler，直接按 `[]byte` / `*[]byte` 收发：

```go
func handleRaw(ss remote.ServerStream) {
    var err error
    var reply any
    defer func() { ss.Finish(reply, err) }()

    if err = ss.Start(false, false); err != nil {
        return
    }

    var req []byte
    if err = ss.RecvMsg(&req); err != nil {
        return
    }
    reply = append([]byte("echo:"), req...)
}
```

说明：

- raw 模式使用 `content-type: application/grpc+raw`
- 现有 proto 生成代码和默认 proto 调用方式不变
- raw 模式同样支持现有 gRPC 压缩链路，例如配置 `gzip`
