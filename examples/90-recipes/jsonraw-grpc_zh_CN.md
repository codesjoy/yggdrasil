# JSON Raw gRPC Recipe

这是一个 transport-level recipe，不是 examples 主线里的 bootstrap 推荐路径。要先把当前框架模型看清楚，优先读 [02-runtime-bundle](../02-runtime-bundle/README_zh_CN.md)；如果你在看自定义 provider 接入，再对照 [20-capability-registration](../20-capability-registration/README_zh_CN.md)。

当你希望 payload 仍然是 JSON 文本字节，但 transport 层不做结构化 JSON 解码时，可以使用 `jsonraw` content subtype。

## Client

```go
ctx := grpc.WithCallOptions(context.Background(), grpc.CallContentSubtype("jsonraw"))

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

## Server

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

## Notes

- content type 是 `application/grpc+jsonraw`
- payload 被视为 JSON 文本字节，但框架不校验 JSON 合法性
- proto 生成代码和默认 protobuf 调用方式不受影响
