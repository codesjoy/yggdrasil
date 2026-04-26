# Raw gRPC Recipe

这是一个 transport-level recipe，不是 examples 主线里的 bootstrap 推荐路径。先理解 `App -> Runtime -> BusinessBundle` 主线时，优先阅读 [02-runtime-bundle](/Users/zhangwei/go/src/github.com/codesjoy/yggdrasil/examples/02-runtime-bundle)；如果你在看 provider-only 扩展，再对照 [20-capability-registration](/Users/zhangwei/go/src/github.com/codesjoy/yggdrasil/examples/20-capability-registration)。

当你需要在 gRPC transport 上直接收发 `[]byte`，可以通过 per-call codec 走 `raw` content subtype。

## Client

```go
ctx := grpc.WithCallOptions(context.Background(), grpc.CallContentSubtype("raw"))

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

## Server

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

## Notes

- content type 是 `application/grpc+raw`
- payload 不经过 protobuf 编解码
- 仍然可以复用现有 gRPC transport 的压缩与连接管理
