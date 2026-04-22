# JSON Raw gRPC (`[]byte`) 模式

`remote/transport/grpc` 现在支持通过 `jsonraw` content-subtype 直接收发 JSON 文本字节，
底层仍然是 `[]byte` / `*[]byte` 直通，不做结构化 JSON 编解码。

推荐用法：

```go
ctx := grpc.WithCallOptions(context.Background(), grpc.CallContentSubtype("jsonraw"))
```

客户端侧继续走现有 `client.Client`：

```go
st, err := cli.NewStream(ctx, &stream.Desc{}, "/jsonraw.Test/Unary")
if err != nil {
    return err
}
if err := st.SendMsg([]byte(`{"message":"ping"}`)); err != nil {
    return err
}

var reply []byte
if err := st.RecvMsg(&reply); err != nil {
    return err
}
```

服务端侧使用手工注册的 low-level handler，直接按 `[]byte` / `*[]byte` 收发：

```go
func handleJSONRaw(ss remote.ServerStream) {
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
    reply = []byte(`{"message":"pong"}`)
}
```

说明：

- `jsonraw` 使用 `content-type: application/grpc+jsonraw`
- payload 被视为 JSON 文本字节，但框架不做 JSON 合法性校验
- 现有 proto 生成代码和默认 proto 调用方式不变
- `jsonraw` 同样支持现有 gRPC 压缩链路，例如配置 `gzip`
